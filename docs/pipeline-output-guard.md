# Pipeline-output guard on the storage webhook

**Date:** 2026-05-31
**Status:** Implemented

## Motivation

The gateway turns storage `ObjectCreated` events into `video.upload.completed`
domain events. Downstream stages (streaming-transcode/packaging) write their
outputs back into the **same** `videos` bucket under `transcoded/`, plus
`metrics/` and `thumbnails/`. If those writes reached the webhook, the gateway
would treat them as brand-new uploads and publish more `video.upload.completed`
events — spawning bogus transcode jobs and, in the worst case, loops.

## Behaviour

`internal/adapters/common_storage.go` defines:

- `ErrIgnoredObjectKey` — sentinel for "this object is a pipeline output".
- `isPipelineOutputKey(key)` — true when the (URL-decoded) key starts with
  `transcoded/`, `metrics/` or `thumbnails/`.

Both `MinioAdapter.ParseEvent` and `S3Adapter.ParseEvent` return
`ErrIgnoredObjectKey` for such keys. `webhooks/service.go` treats it as a
no-op success (logs and returns `nil`), distinct from a parse failure.

## Why keys still map correctly

Uploads land at `<videoId>/<filename>` (the upload service builds this key).
`parseStorageKey` takes the penultimate path segment as `videoId` and the last
as `filename`, so `<uuid>/beauty.mp4` → `videoId=<uuid>`, and transcode rebuilds
the same object key. Pipeline outputs (`transcoded/...`) are filtered out before
that parsing.

## Tests

`internal/adapters/minio_adapter_test.go`:
- `TestMinioAdapterParseEvent_IgnoresPipelineOutputs` — transcoded/metrics/thumbnails keys → `ErrIgnoredObjectKey`.
- `TestMinioAdapterParseEvent_AcceptsUploadKey` — `<uuid>/<file>.mp4` → parsed normally.
