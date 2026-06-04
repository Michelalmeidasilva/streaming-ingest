# Upload status stages — ingest signals

## Motivation

The streaming-platform-upload UI surfaces 6 upload lifecycle stages to the operator:

| Stage | Signal |
|---|---|
| 1 | Upload initiated |
| 2 | Upload in progress (chunks) |
| 3 | Available for preview (storage confirmed) |
| 4 | Transcoding queued |
| 5 | Transcoding in progress |
| 6 | Ready |

Stages 1 and 2 come directly from the frontend emitting `upload.started` /
`upload.progress` events to the Event Gateway. Stage 4 onwards comes from the
transcoder and distribution services. Stage 3 is the ingest responsibility:
when the storage provider fires an `ObjectCreated` webhook, the gateway writes a
`storage_confirmed_at` timestamp onto the upload-state document **before**
publishing the `video.upload.completed` domain event. This sequencing guarantees
the UI sees stage 3 before stage 4.

## The `storage_confirmed_at` signal

`storage_confirmed_at` is an optional RFC3339 timestamp on the upload-state video
document (`videos` MongoDB collection, bson field `storage_confirmed_at`, JSON
field `storageConfirmedAt`). It is written by `webhooks.Service.ProcessWebhook`
immediately after saving video metadata and before publishing to RabbitMQ:

```
ObjectCreated webhook received
  → parse provider envelope → DomainEvent
  → save video metadata to videos_catalog
  → PATCH upload-state { storage_confirmed_at: now }   ← stage 3 signal
  → publish video.upload.completed to RabbitMQ         ← stage 4 trigger
```

The write goes through `UploadStatePatcher` (interface satisfied by
`*uploadstate.Service`), wired in `cmd/api/main.go`. It is provider-agnostic:
the same code path runs for both `minio` and `s3`.

### ErrNotFound handling

If no upload-state document exists for the `videoId` — which happens when a
video is uploaded directly to the bucket without going through the platform-upload
UI — `uploadstate.ErrNotFound` is logged as a warning and processing continues.
The domain event is still published and the transcode proceeds normally; only the
stage 3 timestamp is absent.

## Multi-provider delivery

### Local — MinIO bucket notification

MinIO sends the classic S3 notification format:

```json
{
  "EventName": "s3:ObjectCreated:Put",
  "Records": [{ "s3": { "bucket": { "name": "videos" }, "object": { "key": "<videoId>/<filename>" } } }]
}
```

Object keys use the flat format `<videoId>/<filename>`. `parseStorageKey` takes
the penultimate path segment (`<videoId>`) and the last segment (`<filename>`).

### AWS — S3 → EventBridge → API Destination → webhook

On AWS the delivery path is:

```
S3 ObjectCreated (raw/ prefix)
  → EventBridge rule (filter: source=aws.s3, detail-type=Object Created)
  → API Destination  → POST /api/v1/webhooks/storage/s3
```

No input transformer is configured, so the gateway receives the raw EventBridge
envelope:

```json
{
  "detail-type": "Object Created",
  "detail": {
    "bucket": { "name": "<bucket>" },
    "object": { "key": "raw/<videoId>/<filename>" }
  }
}
```

`s3_adapter.go` `ParseEvent` detects the absence of a `Records` key and switches
to EventBridge parsing, then maps it to the same `DomainEvent` as the classic
path. The classic `Records[]` path is unchanged.

The `raw/` prefix is required on the S3 bucket prefix filter so that only raw
uploads (not transcoded outputs) trigger the EventBridge rule. `parseStorageKey`
takes the **penultimate** path segment, so `raw/<videoId>/<filename>` →
`videoId=<videoId>`.

## Caveats

- **ErrNotFound is tolerated.** If the upload-state document does not exist when
  the webhook fires (e.g. direct-to-bucket upload), the `storage_confirmed_at`
  patch is skipped and a warning is logged. Publishing proceeds; no retry or
  alerting is triggered.
- **No DLQ on the AWS API Destination path.** If the ingest service is down when
  the EventBridge API Destination fires, EventBridge retries for its configured
  window (default 24 h) and then drops the event. The `storage_confirmed_at`
  timestamp will never be written for that video — it will remain nil — but
  subsequent transcode events (`video.upload.completed` published by the eventual
  successful delivery, or manually triggered) will still advance the video through
  stages 4–6. The missing stage 3 signal is a UI cosmetic gap, not a data-loss
  condition.
