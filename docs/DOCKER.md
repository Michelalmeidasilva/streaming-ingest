# Docker Setup for streaming-ingest

This service uses the **centralized infrastructure** defined in the root `infra/` directory.

## Prerequisites

The infrastructure (MongoDB, RabbitMQ, MinIO) must be running before starting this service.

### Start the Centralized Infrastructure

```bash
cd ../../infra
docker compose up -d
```

Verify all services are healthy:
```bash
make ps
make test-all
```

## Running streaming-ingest

### Option 1: Run Locally with Docker Compose

You can still use Docker Compose to run just this service alongside the central infrastructure:

```bash
# From the root infra directory
cd ../../infra
docker compose up -d

# From this directory (streaming-ingest)
docker build -t streaming-ingest .
docker run --network vod-network \
  -e RABBITMQ_URL=amqp://guest:guest@rabbitmq:5672/ \
  -e MONGODB_URI=mongodb://admin:password@mongodb:27017/streaming \
  -e SERVER_PORT=8080 \
  -e MINIO_ENDPOINT=minio:9000 \
  -e MINIO_ROOT_USER=admin \
  -e MINIO_ROOT_PASSWORD=password123 \
  -p 8080:8080 \
  streaming-ingest
```

### Option 1b: Use the Dev Target

From `streaming-ingest`, `make dev` now brings up the shared infra stack first and then starts the Go API:

```bash
make dev
```

### Option 2: Run Locally with Go

```bash
# Set environment variables
export RABBITMQ_URL=amqp://guest:guest@localhost:5672/
export MONGODB_URI=mongodb://admin:password@localhost:27017/streaming
export SERVER_PORT=8080
export MINIO_ENDPOINT=http://localhost:9000
export MINIO_ROOT_USER=admin
export MINIO_ROOT_PASSWORD=password123

# Run the service
go run cmd/api/main.go
```

## Configuration

All environment variables are documented in `../../infra/.env.example`

Key variables for this service:
- `RABBITMQ_URL` - RabbitMQ connection string
- `MONGODB_URI` - MongoDB connection string
- `MINIO_ENDPOINT` - MinIO S3 endpoint
- `MINIO_ROOT_USER` - MinIO credentials
- `MINIO_ROOT_PASSWORD` - MinIO credentials

## Docker Network

When running in Docker, this service connects to the centralized infrastructure via the `vod-network` Docker network. This network is automatically created by the centralized `infra/docker-compose.yml`.

Service names on this network:
- `mongodb` - MongoDB database
- `rabbitmq` - RabbitMQ message broker
- `minio` - MinIO object storage

## Troubleshooting

If the service can't connect to infrastructure:

1. **Verify infrastructure is running**:
   ```bash
   cd ../../infra && make ps
   ```

2. **Check network connectivity**:
   ```bash
   docker network ls | grep vod-network
   ```

3. **Check environment variables**:
   ```bash
   env | grep -E "RABBIT|MONGO|MINIO"
   ```

4. **View logs**:
   ```bash
   cd ../../infra && make logs-ingest
   # or
   docker logs streaming-ingest
   ```

## See Also

- `../../infra/INDEX.md` - Complete infrastructure documentation
- `../../infra/SERVICES_INTEGRATION.md` - Service integration guide
- `./SPEC.md` - API specification for this service
