# Testing Guide for streaming-ingest

## Unit Tests

Run all unit tests with:
```bash
go test -v -coverprofile=coverage.out -covermode=atomic ./internal/...
```

View coverage report:
```bash
go tool cover -func=coverage.out | grep total
go tool cover -html=coverage.out
```

**Current Coverage:** 81.7% (exceeds 80% target)

## Integration Tests

Integration tests require external services (RabbitMQ, MongoDB) to be running.

### Run Integration Tests Only

```bash
go test -tags integration -v ./internal/rabbitmq/...
```

### Prerequisites for RabbitMQ Integration Tests

**Option 1: Docker**
```bash
docker run -d --name rabbitmq -p 5672:5672 rabbitmq:3.12-alpine
```

**Option 2: Docker Compose** (from repository root)
```bash
cd .. && docker-compose up -d rabbitmq
```

**Option 3: Local RabbitMQ**
Install RabbitMQ and ensure it's running on `localhost:5672` with default credentials (`guest:guest`).

### Integration Test Behavior

If the required service is not available, integration tests skip gracefully:
```
--- SKIP: TestPublisherNewWithRealBroker (0.00s)
    Skipping: RabbitMQ not available at amqp://guest:guest@localhost:5672/...
```

## Local Test Execution

### Full Test Suite (Unit + Integration Skipped)
```bash
go test -v ./internal/...
```

### With Local Services Running

Start services:
```bash
cd ..
docker-compose up -d
```

Run all tests including integration:
```bash
go test -tags integration -v ./internal/...
```

## CI/CD Pipeline

The CI pipeline runs:
1. **Unit tests only** (no integration tag) - checked on every push
2. **Security scans** with Semgrep (SAST)
3. **Coverage requirements** - minimum 80% on `internal/`

## Test Coverage by Package

- `adapters`: 88.2%
- `events`: 100.0%
- `rabbitmq`: 20.0% (unit only, see integration tests)
- `videos`: 85.9%
- `webhooks`: 100.0%

Note: `rabbitmq` package has low unit test coverage because most functionality requires a live AMQP broker. Integration tests provide full coverage when RabbitMQ is available.
