# Publish AMQP resiliente (reconnect + retry)

## Motivação

O ingest é o único dono do RabbitMQ: `POST /api/v1/events` (e os webhooks) publicam no
exchange `video_events`. O `Publisher` (`internal/rabbitmq/publisher.go`) abre **uma**
conexão + canal AMQP no startup (`NewPublisher`) e os reusa.

Em Lambda isso é frágil: a conexão de longa duração é derrubada pelo **idle timeout do
CloudAMQP** e pelo **freeze/thaw** da Lambda entre invocações. O primeiro publish depois de
um thaw bate num canal/conexão fechado e falha → o handler devolve **500**.

Impacto observado (2026-06-08): jobs de transcode logavam `POST /events returned 500` em
todos os estágios; com o worker tratando o evento `ready` como fatal, os jobs eram marcados
`FAILED` mesmo com a mídia pronta. (O worker também foi endurecido — ver
`streaming-transcode/docs/event-publish-nonfatal.md`.)

## Correção

`Publish` agora, na falha, **reconecta e re-tenta uma vez**:

```go
p.mu.Lock(); defer p.mu.Unlock()
pubErr := p.publish(routingKey, body)
if pubErr != nil && p.url != "" {
    if rcErr := p.reconnect(); rcErr == nil {
        pubErr = p.publish(routingKey, body)
    }
}
```

- `url` é guardado no `NewPublisher` para permitir o redial.
- `reconnect()` disca, abre canal, **re-declara o exchange** `video_events` e só então troca
  a conexão/canal — um reconnect que falha deixa a conexão anterior no lugar (tenta de novo
  na próxima chamada), em vez de zerar o publisher.
- Um `sync.Mutex` serializa publish/reconnect (a app Fiber pode receber requisições
  concorrentes).
- Sem `url` (ex.: testes que injetam conn/channel direto), a reconexão é pulada — o
  comportamento antigo de erro é preservado.

## Testes

- `TestPublishReconnectsAndRetriesOnStaleConnection`: 1º publish falha (conexão velha),
  `dialAMQP` devolve conexão saudável, retry tem sucesso (1 redial).
- `TestPublishNoReconnectWithoutURL`: sem `url`, falha de publish retorna erro sem redial.

## Limitações / próximos passos

- Retry único (suficiente para o caso stale-after-thaw). Não há backoff — uma indisponibilidade
  real e prolongada do broker ainda retorna 500 (correto; o chamador decide).
- Não há publisher confirms; um publish "fire-and-forget" pode não detectar todas as quedas
  silenciosas. Endurecer com confirms é um passo futuro.
