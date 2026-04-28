# Padrões Arquiteturais e Diretrizes para LLMs/IAs

Este documento descreve as decisões arquiteturais e padrões de codificação adotados neste repositório (`streaming-ingest`), servindo como base de conhecimento para que futuras inteligências artificiais (e desenvolvedores) consigam expandir o projeto de modo coeso.

## 1. Visão Geral do Sistema (O Event Gateway)
O **Event Gateway** atua como uma ponte (Adapter) entre:
* Aplicações Frontend (ex: recebimento de status de progresso).
* Provedores de Storage (MinIO, S3 - via webhooks quando o object é gerado).
* RabbitMQ (O único ponto onde conectamos e despachamos eventos formatados `DomainEvent`).

### Tech Stack
- **Linguagem**: Go (Golang) 1.23+
- **Framework Web**: Fiber (`github.com/gofiber/fiber/v2`)
- **Mensageria**: RabbitMQ (`amqp091-go`)
- **Infraestrutura**: Docker & Docker Compose (`alpine` based)

## 2. Estrutura do Projeto (Vertical Slices + Hexagonal)
Nós nos baseamos fortemente em domínios verticais em vez de agrupar tudo por pastas técnicas (ex: todos os `controllers` juntos).
- **`/cmd/api/main.go`**: Único ponto de injeção de dependências e roteamento. Nenhuma regra de negócio aqui.
- **`/internal/events/`**: Recebe requisições do frontend. (Contém `handler.go` limitando HTTP e `service.go` aplicando a injeção do rabbit).
- **`/internal/webhooks/`**: Roteador flexível de providers.
- **`/internal/adapters/`**: O `StorageAdapter` (porta) que extrai informações de payloads complexos da Nuvem (MinIO e AWS S3) e os mapeia para o evento de domínio `upload.completed`.
- **`/internal/rabbitmq/`**: Apenas wrapper da conexão e pool AMQP, expõe publicadores para os services.

## 3. Fluxo de Expansão (Prompting / Hands-on)

### Ao Adicionar um Novo Provedor de Storage (ex: Google Cloud Storage)
1. **Não altere** as lógicas existentes de RabbitMQ ou do `handler` base.
2. Adicione um arquivo em `internal/adapters/gcs_adapter.go`.
3. Crie uma struct que assuma o contrato da interface `StorageAdapter`.
4. Defina as structs mapeadoras relativas ao JSON da Cloud.
5. Em `internal/webhooks/service.go`, importe o novo Adapter no mapa local (ex: `"gcs": adapters.NewGCSAdapter()`).

### Ao Alterar Lógicas do Frontend
1. Atue em `internal/events/`. O `FrontEndEvent` é genérico por padrão. 
2. Se precisar persistir dados (MongoDB), injete o repositório (`repo`) diretamente no `NewService`, não acople banco de dados no `handler` do Fiber.

## 4. Orientações de Infraestrutura (Docker)
Devido ao isolamento, todos os microserviços dependem do Docker.
- O Go é buildado em um ambiente `multistage` minimalista.
- **Lidando com Módulos**: Como pode não haver binários de Go nas hosts de desenvolvimento, delegamos a responsabilidade dos comandos de módulo (`go mod tidy` e `download`) para ocorrer **dentro** do contêiner builder no `Dockerfile`. Isso mascara problemas que exigiriam o arquivo `go.sum` presente de antemão.

## 5. Regras de Ouro
1. **O Gateway não consome mensagens**: Sua característica principal é ingerir e despachar. Ele é puramente um roteador/publisher e não consome workers da fila.
2. **Fail Fast**: Retorne erros de parsing de JSON o mais rápido possível na camada Controller/Handler, prevenindo Pânicos no Go por manipulação indevida de dados vazios.
3. **Mantenha os Adaptadores em Isolamento**: As especificidades dos Webhooks da AWS e do MinIO devem ficar **isoladas** nos arquivos correspondentes. Interface gráfica / Domain layer jamais saberão os atributos exclusivos de suas Nuvens.
