# Changelog

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
