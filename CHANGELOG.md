# Changelog

## [Unreleased] 2026-06-07
### Fixed
- Storage webhook `DomainEvent` now carries `objectKey` (the full storage key, e.g. `raw/<videoId>/<filename>`). Previously only the derived `videoId`/`filename` (basename) were published, so the transcoder reconstructed `<videoId>/<filename>` and **dropped the `raw/` prefix** → download failed with `The specified key does not exist` on the dev RabbitMQ→worker path. The `objectKey` field is populated in both adapters (`minio`/`s3`, classic `Records[]` and EventBridge envelopes) and the worker downloads it verbatim. Field added to `adapters.DomainEvent`; `newStorageDomainEvent` now takes the full key.

## [Unreleased] 2026-06-06
### Added
- MongoDB index creation at startup (`internal/mongoindex`, wired in `cmd/api/main.go`): unique `video_id` on `videos_catalog` and `videos`, unique `session_id` on `upload_sessions`, and `created_at` (desc) on `videos`. These back the per-video lookups and the upload-state list sort polled by the admin UI; without them the queries were full collection scans (COLLSCAN). Idempotent and non-fatal on failure (logs a warning, e.g. on a pre-existing duplicate that would break a unique index).
### Changed
- Telemetry now emits CloudWatch EMF to stdout (RED per request: `RequestCount`, `RequestLatency`, `ErrorCount`; dimensions `service/route/method`; namespace `VOD/streaming-ingest`).
### Removed
- OTel SDK push pipeline (`internal/otel/`, `otelfiber` middleware) and the Prometheus `/metrics` endpoint (`fiberprometheus` middleware) with related deps.

## [Unreleased] 2026-06-04
### Added
- Webhook writes `storage_confirmed_at` to the upload-state video on `ObjectCreated`,
  providing the "available for preview" signal (upload stage 3) to the platform.
  Provider-agnostic (minio + s3); patched before publishing; `ErrNotFound` tolerated.
- `s3` adapter now parses the AWS EventBridge "Object Created" envelope in addition to
  the classic S3 `Records[]` format, so S3 → EventBridge → ingest (API Destination)
  works without an input transformer.

## [Unreleased] 2026-06-04
### Added
- Raw video geometry (`rawVideo`: width/height/fps/pixelFormat) is now carried
  end-to-end for headerless `.yuv` uploads. The events service persists it from
  the `upload.started` payload onto the video record (`raw_video`), and the
  storage webhook handler looks it up by `videoId` and attaches it to the
  published `video.upload.completed` `DomainEvent` so the transcoder can demux
  the raw stream. The lookup also re-applies it on `Save` so the record is not
  clobbered to a geometry-less state.
- `adapters.RawVideoParams`, `DomainEvent.RawVideo`, `Video.RawVideo` and
  `VideoRepository.FindByVideoID(ctx, videoID)`.
- Sidecar subtitle references (`subtitles`: objectKey/language/label) propagate
  the same way: persisted from `upload.started` (`Video.Subtitles`) and attached
  to `video.upload.completed` by the webhook handler. `adapters.SubtitleRef`,
  `DomainEvent.Subtitles`, `Video.Subtitles`. Storage webhooks for the
  `subtitles/` prefix are ignored so a `.srt` upload does not trigger its own
  transcode.

## [Unreleased] 2026-06-04
### Fixed
- Eliminated duplicate video documents (two per `video_id`). The videos service
  (events `upload.started` → `videoRepo.Create` / `InsertOne`, storage webhooks,
  catalog handler) shared the `videos` collection with the upload-state store
  (which upserts the canonical lifecycle document by `video_id`). The unconditional
  `InsertOne` produced a second, minimal `status:"uploading"` document per video.

### Changed
- The videos service now uses its own `videos_catalog` collection
  (`cmd/api/main.go`). The upload-state store keeps the canonical per-video
  lifecycle document (thumbnail/transcode/playback) in `videos`. `videos_catalog`
  self-populates from storage webhooks and the storage-sync backfill
  (`collectStorageVideos`). Existing duplicate `videos` documents were removed in a
  one-time cleanup.

## [Unreleased] 2026-06-03
### Added
- Endpoint `GET /metrics` expondo métricas Prometheus RED
  (`http_requests_total`, `http_request_duration_seconds`) com labels
  `service,status_code,method,path`. Permite ao streaming-telemetry coletar
  requests/erros/latência (sinais 1/4/5) por scrape.

## [Unreleased] 2026-05-31 — End-to-end playback integration
### Added
- Pipeline-output guard in the storage webhook path: object keys under
  `transcoded/`, `metrics/` and `thumbnails/` are ignored (`ErrIgnoredObjectKey`
  in `internal/adapters/common_storage.go`, applied by `minio_adapter.go` and
  `s3_adapter.go`; handled as a no-op success in `webhooks/service.go`). This
  prevents the gateway from re-ingesting its own downstream outputs as new
  uploads (which would spawn bogus transcode jobs / loops).
### Notes
- The active upload trigger in local dev is the streaming-platform-upload in-app
  bridge (`notifyIngestStorageCompletion` → `POST /api/v1/webhooks/storage/minio`),
  which fires on upload completion with the full object key `<videoId>/<filename>`.
