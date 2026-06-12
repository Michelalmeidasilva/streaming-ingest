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
- 500 broker or processing error — the AMQP publisher reconnects and retries once on a
  stale connection before returning 500 (Lambda keeps a long-lived connection that the
  broker idle-times-out / that freeze-thaw breaks). See `docs/amqp-publish-reconnect.md`.

The `upload.started` payload may carry optional fields persisted on the upload-state video record:
- `rawVideo` — `{ width, height, fps, pixelFormat }` for headerless `.yuv` uploads.
- `subtitles` — `[{ objectKey, language, label }]` sidecar subtitle references.
- `transcode` — `{ codecs: string[], renditions: [{ width, height, codec }] }` codec/resolution selection made at upload time. Persisted on the video record and forwarded to the transcoder on `ObjectCreated` (same mechanism as `rawVideo`). When absent the transcoder falls back to defaults.

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

### POST /api/v1/benchmark-runs
Inserts a single benchmark measurement into the `transcode_runs` collection with
`benchmark=true`. Called by `cmd/benchmark` in `streaming-transcode` for each
codec×resolution×clip×repeat cell. Each call is an **insert** (not upsert) — duplicate
`jobId` values result in a unique-index error, which the caller logs and continues.

Benchmark runs are never patched onto upload-state video records and never affect the
`videos_catalog` or the video lifecycle.

Request body: same `JobObservability` shape as a `transcode.completed` event, plus the
benchmark-specific fields:

```json
{
  "jobId": "<uuid>",
  "machineLabel": "c5.xlarge",
  "hostname": "ip-10-0-1-42.us-east-2.compute.internal",
  "cpuCores": 4,
  "clip": "benchmark/corpus/sample-720p.mp4",
  "repetition": 1,
  "benchmark": true,
  "elapsedSeconds": 28.4,
  "renditions": [ … ]
}
```

Response `202`:
```json
{ "message": "Benchmark run recorded" }
```

- 400 validation failure
- 500 database error

### GET /api/v1/runs
Returns transcode run documents from the `transcode_runs` collection, sorted by `completedAt` descending.

Query parameters:
- `machineLabel` (optional) — filter by exact machine label
- `codec` (optional) — filter by codec (matches against any rendition's codec field)
- `benchmark` (optional) — `true` to return only benchmark runs; `false` (default) to
  return only production runs. Production filter uses `{ benchmark: { $ne: true } }` so
  documents that pre-date the benchmark field are included in the production view.

Response `200`:
```json
[
  {
    "jobId": "<uuid>",
    "videoId": "<uuid>",
    "machineLabel": "c5.xlarge",
    "hostname": "ip-10-0-1-42.us-east-2.compute.internal",
    "cpuCores": 4,
    "profile": "production",
    "elapsedSeconds": 142.3,
    "rtf": 0.58,
    "sourceFileSizeBytes": 524288000,
    "totalOutputSizeBytes": 89128960,
    "completedAt": "2026-06-09T14:32:00Z",
    "createdAt": "2026-06-09T14:29:38Z",
    "renditions": [
      {
        "name": "360p",
        "codec": "h264",
        "width": 640,
        "height": 360,
        "preset": "fast",
        "targetBitrateKbps": 800,
        "outputBitrateKbps": 793,
        "elapsedSeconds": 28.1,
        "avgCpuPercent": 92.4,
        "maxCpuPercent": 99.1,
        "avgMemoryMb": 310.5,
        "maxMemoryMb": 412.0
      }
    ]
  }
]
```

### GET /api/v1/runs/:videoId
Returns the single transcode run document for the given `videoId`, or `404` if not found.

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

### Transcode runs document (`transcode_runs` collection)

One document per completed transcode job, upserted best-effort when the gateway receives a
`transcode.completed` event on `POST /api/v1/events`. The write is idempotent: `$setOnInsert`
is used for `createdAt` so re-delivery of the same `jobId` does not clobber the original
timestamp. A failure to write the run document is logged but never fails the event response
or the RabbitMQ publish — the gateway remains ingest-and-publish only; it does not consume
any queues.

| Field (bson) | JSON | Type | Description |
|---|---|---|---|
| `job_id` | `jobId` | string | Unique transcode job identifier (unique index) |
| `video_id` | `videoId` | string | Platform video UUID |
| `machine_label` | `machineLabel` | string | `TRANSCODE_MACHINE_LABEL` from the worker, or hostname fallback |
| `hostname` | `hostname` | string | Worker container/instance hostname |
| `cpu_cores` | `cpuCores` | int | CPU core count reported by the worker |
| `profile` | `profile` | string | Rendition profile identifier |
| `elapsed_seconds` | `elapsedSeconds` | float64 | Wall-clock seconds for the full job |
| `rtf` | `rtf` | float64 | Real-time factor (elapsed / source duration) |
| `source_file_size_bytes` | `sourceFileSizeBytes` | int64 | Raw source file size |
| `total_output_size_bytes` | `totalOutputSizeBytes` | int64 | Sum of all packaged output sizes |
| `completed_at` | `completedAt` | time.Time | Timestamp from the event payload |
| `created_at` | `createdAt` | time.Time | Time the run document was first inserted (`$setOnInsert`) |
| `benchmark` | `benchmark` | bool? | `true` for benchmark runs; absent/false for production runs |
| `clip` | `clip` | string? | S3 key of the corpus clip (benchmark runs only) |
| `repetition` | `repetition` | int? | Repeat index within the matrix cell (benchmark runs only) |
| `source_width` | `sourceWidth` | int? | Source clip width in pixels (benchmark runs) |
| `source_height` | `sourceHeight` | int? | Source clip height in pixels (benchmark runs) |
| `source_duration_seconds` | `sourceDurationSeconds` | float64? | Source clip duration in seconds (benchmark runs) |
| `source_fps` | `sourceFps` | float64? | Source clip frame rate (benchmark runs) |
| `source_codec` | `sourceCodec` | string? | Source clip video codec, e.g. `h264` (benchmark runs) |
| `source_bitrate_kbps` | `sourceBitrateKbps` | int? | Source clip container bitrate in kbps (benchmark runs) |
| `renditions` | `renditions` | array | Per-rendition metrics (see below) |

Each rendition entry: `name`, `codec`, `width`, `height`, `preset`,
`targetBitrateKbps`, `outputBitrateKbps`, `outputFileSizeBytes`, `elapsedSeconds`,
`avgCpuPercent`, `maxCpuPercent`, `avgMemoryMb`, `maxMemoryMb`.

### Indexes

Ensured at startup (`internal/mongoindex`, wired in `cmd/api/main.go`); idempotent and non-fatal on failure:

| Collection | Index | Purpose |
|---|---|---|
| `videos` | `video_id` (unique) | per-video lookups/upserts; guards against duplicate docs |
| `videos` | `created_at` (desc) | upload-state list sort polled by the admin UI |
| `videos_catalog` | `video_id` (unique) | catalog upsert/lookup key |
| `upload_sessions` | `session_id` (unique) | multipart session lookup key |
| `transcode_runs` | `job_id` (unique) | idempotent upsert key; prevents duplicate run documents |
| `transcode_runs` | `machine_label` asc + `completed_at` desc | filter-by-label queries sorted by recency |

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

## Transcode Selection Passthrough

The gateway persists the upload-time `transcode` selection on the video record at
`upload.started` and forwards it verbatim on the storage `upload.completed` webhook. The
mirror struct `adapters.TranscodeRequest` carries:

| Field | Type | Notes |
|---|---|---|
| `codecs` | `[]string` | requested codecs |
| `protocols` | `[]string` | subset of `{hls,dash}` |
| `segmentSeconds` | `int` | segment duration preset |
| `renditions[]` | objects | `width`, `height`, `codec`, `bitrateKbps` |

**Invariant:** every field the transcoder understands must be declared on this struct —
anything missing is silently dropped on the round-trip. A contract test
(`internal/adapters/transcode_contract_test.go`) pins the JSON shape. The gateway does not
validate these values; the transcoder normalizes them. See `docs/streaming-format-controls.md`.
