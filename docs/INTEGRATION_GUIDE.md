# Guia de Integração - Event Gateway

Este documento é direcionado a engenheiros e outros sistemas (Front-ends clientes, Node APIs ou Cloud Providers) que necessitam se integrar de forma mínima e direta com a API do **Event Gateway (VOD)**.

O Event Gateway atua em `http://<SEU-DOMINIO-AQUI>:8080/api/v1` em escopo corporativo. Ele é a ponte inteligente da arquitetura e a *única interface* autorizada a engatilhar o message broker de processamentos de workers. Seu sistema cliente jamais tentará se comunicar com as filas AMQP diretamente, você só precisará realizar requisições REST convencionais.

---

## 1. Conexão Cliente-Servidor (Aplicações Front-end)

O seu sistema cliente (aplicativo Web React, Next.js ou Mobile) deve se comunicar com nosso sistema a fim de relatar ações do ciclo de vida da transferência dos bytes antes de chegar ao fim.

### Endpoint-alvo: `POST /api/v1/events`
- **Por que chamar?** Para avisar o back-end que o seu usuário engatilhou um vídeo novo, notificar se a rede caiu, ou notificar as atualizações incrementais enquanto a barra carrega.
- **Formato da Requisição**: REST / JSON genérico.

**Contrato Base Obrigado:**
```json
{
  "eventType": "NOME_DO_SEU_EVENTO",
  "payload": {
      "videoId": "UUID-UNICO-AQUI",
      "filename": "nome-original.mp4"
      // ... qualquer outro campo meta que quiser engatilhar, como {progress: 55}
  }
}
```
> [!TIP]
> Todo valor imposto em `eventType` é amarrado como tópico. Uma mensagem de `upload.progress` passa a ser roteada perfeitamente no cluster do RabbitMQ pela API escutando na chave `video.upload.progress`.

---

## 2. Conexão Nuvem-Servidor (Storage Webhooks)

A sua infraestrutura final não pode confiar dados de persistência baseados nos reportes picados via HTTP da UI Web. O provedor de Storage físico suporta a autoridade máxima e manda para nós o Webhook assim que finalizado.

### Endpoint-alvo: `POST /api/v1/webhooks/storage/:provider`
- **Por que chamar?** Quando o bucket S3 ou Minio consolida o objeto de mídia no disco, apontamos essa notificação nativa para nós, que destrincharemos a confirmação.
- **URL's Suportadas Até o Momento:**
  - `minio` (`/storage/minio`)
  - `aws-s3` (`/storage/aws-s3`)

**Contrato Requerido (Qual formato seguir?):**
Você não precisa se preocupar em moldar o JSON! Nossa API abraça totalmente o conceito de _Adapters de Infraestrutura_. Diferente da rota de Application lá em cima, você **apenas cola essa rota** no painel da sua Nuvem (Event Notification). 
A API receberá e "traduzirá" de forma isolada os JSONs bizarros ou gigantes da nuvem originária descobrindo a Key, Size e convertendo-o num disparo único padrão: `video.upload.completed`.

---

> Para visualizar formalmente cada formato de Tipos e Parâmetros que essas 2 rotas aceitam com schemas estritos em caso de Mock, valide cruzando com as definições explícitas contidas geradas em `/swagger.yaml`.
