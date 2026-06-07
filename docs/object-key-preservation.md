# Full object key preservation on the storage webhook event

## Problem

The platform-upload stores every source object under `raw/<videoId>/<filename>`
(on MinIO **and** S3). On `ObjectCreated`, the storage webhook parsed the key
with `parseStorageKey`, which keeps only the **last two** path segments:

```
raw/<videoId>/<filename>  ->  videoId=<videoId>, filename=<filename>   (basename only)
```

The published `DomainEvent` carried only `videoId` and `filename`, not the full
key. The transcoder's `ParseUploadCompleted` then reconstructed the source key
via `resolveObjectKey(videoId, filename)` → `<videoId>/<filename>` — **dropping
the `raw/` prefix**. The worker downloaded `videos/<videoId>/<filename>`, which
does not exist, and the job failed:

```
job failed: download videos/<id>/<file>: The specified key does not exist.
```

This broke the dev `webhook → RabbitMQ → worker` path for every real upload. (The
prod AWS Batch path was unaffected: its entrypoint receives the full key as an
argument and sets it directly as `objectKey`.)

## Fix

Carry the **full** storage key end-to-end. `adapters.DomainEvent` gained an
`objectKey` field (`json:"objectKey,omitempty"`), populated by both adapters in
all envelope shapes:

| Adapter | Envelope | Source of full key |
|---|---|---|
| `minio_adapter.go` | classic `Records[].s3.object.key` | `record.S3.Object.Key` |
| `s3_adapter.go` | classic `Records[]` | `record.S3.Object.Key` |
| `s3_adapter.go` | EventBridge `detail.object.key` | `eb.Detail.Object.Key` |

`newStorageDomainEvent` now takes the full key as a parameter. No transcoder
change was needed: `ParseUploadCompleted` already prefers `event.ObjectKey` when
present (`if event.ObjectKey == "" { … reconstruct … }`), so it now downloads the
exact key, prefix included.

## Contract

The published `video.upload.completed` message now includes:

```json
{ "videoId": "<id>", "filename": "<basename>", "objectKey": "raw/<id>/<basename>", ... }
```

`objectKey` is the authoritative download path. `videoId`/`filename` remain for
correlation and display. Consumers should download from `objectKey`.

## Verified

Live dev E2E (`docker compose`): fresh upload via the storage webhook →
RabbitMQ → worker now transcodes successfully (`status: ready` in ~20s) and the
video plays in `streaming-web-client`. Unit tests in
`internal/adapters/objectkey_test.go` assert `objectKey` is the full key across
the minio, s3-records, and s3-eventbridge envelopes.
