# streaming-ingest

Event Gateway for the VOD platform. Bridges HTTP (frontend events + storage webhooks) to RabbitMQ. The only service with AMQP credentials — it ingests and publishes; never consumes.

**Stack:** Go 1.23+ · Fiber · MongoDB · RabbitMQ  
**Port:** 8080  
**Full docs:** [`obsidian-vault/services/streaming-ingest.md`](../obsidian-vault/services/streaming-ingest.md)

## Quick Start

```bash
docker compose up --build
```
