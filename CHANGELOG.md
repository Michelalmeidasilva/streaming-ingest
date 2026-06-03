# Changelog

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
