# streaming-ingest — SPEC

Event Gateway (pipeline stages 2–3). Port 8080. Go + Fiber + MongoDB + RabbitMQ.
Single bridge between HTTP (frontend upload events + MinIO/S3 webhooks) and RabbitMQ.
**Never consumes queues.**

## Endpoints

### POST /api/v1/events
Frontend lifecycle events (`upload.started`, `upload.progress`, `upload.completed`, …).

202:
```json
{ "message": "Event accepted" }
```

- 400 validation failure
- 500 broker or processing error

The `upload.started` payload may carry optional fields persisted on the upload-state video record:
- `rawVideo` — `{ width, height, fps, pixelFormat }` for headerless `.yuv` uploads.
- `subtitles` — `[{ objectKey, language, label }]` sidecar subtitle references.

### POST /api/v1/webhooks/storage/:provider
Object-storage `ObjectCreated` webhook. Provider must be `minio` or `s3`.

200:
```json
{ "message": "Webhook processed successfully" }
```

- 400 unsupported provider or parse error

**Processing order on `ObjectCreated`** (all steps for `minio` and `s3`):
1. Parse provider envelope → `DomainEvent`. The event carries `objectKey` (the **full** storage key, e.g. `raw/<videoId>/<filename>`) alongside the derived `videoId`/`filename`, so the transcoder downloads the exact key instead of reconstructing it (which would drop the `raw/` prefix).
2. Save video metadata to `videos_catalog` collection.
3. Patch upload-state video with `{ storage_confirmed_at: now }` (RFC3339). If no upload-state document exists (`ErrNotFound`), the error is logged and processing continues — this handles uploads done outside the platform-upload UI.
4. Publish `video.upload.completed` to RabbitMQ `video_events` Topic Exchange.

The patch-before-publish ordering guarantees that downstream consumers (platform-upload UI) see the "available for preview" stage (stage 3) before the "transcoding queued" stage (stage 4).

**Ignored keys:** object keys under `transcoded/`, `metrics/`, `thumbnails/`, `subtitles/` return a no-op success to prevent re-ingesting pipeline outputs as new uploads.

### GET /api/v1/videos
Storage-synced video list. Array of `DomainEvent`-shaped records.

### GET /api/v1/videos/database
MongoDB catalog (`videos_catalog` collection). Array of `VideoRecord`.

### Telemetry (CloudWatch EMF)
Per-request telemetry is emitted to stdout as CloudWatch Embedded Metric Format (EMF).
Each request produces one JSON line with RED metrics (`RequestCount`, `RequestLatency` ms,
`ErrorCount`) under namespace `VOD/streaming-ingest`, dimensions `service/route/method`.
`GET /metrics` has been removed. See `docs/cloudwatch-emf-telemetry.md`.

### GET /health
```json
200 { "status": "ok" }
```

## Data Models

### Upload-state video document (`videos` collection)

Canonical per-video lifecycle document. Key fields:

| Field (bson) | JSON | Type | Description |
|---|---|---|---|
| `video_id` | `videoId` | string | Platform-upload UUID, correlation key |
| `status` | `status` | string | Lifecycle status |
| `storage_confirmed_at` | `storageConfirmedAt` | string (RFC3339)? | Set when the storage `ObjectCreated` webhook is processed. Nil when the upload bypassed the platform-upload UI. |
| `raw_video` | `rawVideo` | object? | Geometry for headerless `.yuv` uploads |
| `subtitles` | `subtitles` | array? | Sidecar subtitle references |

### Videos catalog document (`videos_catalog` collection)

Populated by storage webhooks and the storage-sync backfill. Contains `videoId`, `filename`, `size`, `provider`, `status`, `url`, `createdAt`.

### Indexes

Ensured at startup (`internal/mongoindex`, wired in `cmd/api/main.go`); idempotent and non-fatal on failure:

| Collection | Index | Purpose |
|---|---|---|
| `videos` | `video_id` (unique) | per-video lookups/upserts; guards against duplicate docs |
| `videos` | `created_at` (desc) | upload-state list sort polled by the admin UI |
| `videos_catalog` | `video_id` (unique) | catalog upsert/lookup key |
| `upload_sessions` | `session_id` (unique) | multipart session lookup key |

## Storage Adapters

### MinIO (`minio_adapter.go`)
Parses the classic MinIO `EventNotification` envelope:
```json
{ "EventName": "s3:ObjectCreated:Put", "Records": [ { "s3": { … } } ] }
```
Object key format: `<videoId>/<filename>`.

### S3 (`s3_adapter.go`)
Accepts **two** envelope formats:

1. **Classic S3 notification** (`Records[]` present):
   ```json
   { "Records": [ { "eventName": "ObjectCreated:Put", "s3": { … } } ] }
   ```

2. **AWS EventBridge "Object Created" envelope** (no `Records` key):
   ```json
   { "detail-type": "Object Created", "detail": { "object": { "key": "…" }, "bucket": { "name": "…" } } }
   ```
   This format is required on the AWS delivery path: `S3 ObjectCreated → EventBridge rule → API Destination → POST /api/v1/webhooks/storage/s3` with no input transformer. Both formats map to the same `DomainEvent`.

Object key format: `raw/<videoId>/<filename>` (both local MinIO and AWS — the platform-upload writes the `raw/` prefix everywhere). `parseStorageKey` takes the penultimate path segment as `videoId` and the last as `filename`; the **full** key is also forwarded on the event as `objectKey` so the transcoder downloads it verbatim (a reconstructed `<videoId>/<filename>` would drop `raw/` and fail).

## RabbitMQ

- Exchange: `video_events` (Topic)
- Routing keys: `video.upload.started`, `video.upload.progress`, `video.upload.completed`, etc.
- Publisher only — no consumer.

## Env

```
RABBITMQ_URL         amqp://guest:guest@rabbitmq:5672/
MONGODB_URI          mongodb://mongodb:27017/streaming
SERVER_PORT          8080
MINIO_ENDPOINT       minio:9000
MINIO_ROOT_USER      admin
MINIO_ROOT_PASSWORD  password123
```
