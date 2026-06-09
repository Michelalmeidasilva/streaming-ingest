# Transcode Selection Propagation

## Overview

When an upload starts, the platform UI can send a `transcode` selection in the
`upload.started` event payload. The ingest service persists this selection on
the video record and, when the storage `ObjectCreated` webhook fires, looks it
up and forwards it on the `DomainEvent` published to RabbitMQ. The transcoder
worker reads `event.transcode` and uses it instead of its built-in defaults.

This mirrors exactly how `rawVideo` (headerless geometry) and `subtitles`
(sidecar tracks) are propagated through the same pipeline.

## Flow

```
upload.started  { transcode: { codecs: [...], renditions: [...] } }
      │
      ▼
events.Service.ProcessEvent
      │  transcodeFromPayload(payload) → *TranscodeRequest
      │  videos.Video{ ..., Transcode: <value> }
      ▼
MongoDB videos_catalog   ← persisted on the record

ObjectCreated webhook
      │
      ▼
webhooks.Service.ProcessWebhook
      │  repo.FindByVideoID(videoID) → existing.Transcode
      │  domainEvent.Transcode = existing.Transcode
      ▼
RabbitMQ video_events  (video.upload.completed)
      │  DomainEvent{ ..., Transcode: { codecs, renditions } }
      ▼
streaming-transcode worker  ← reads event.transcode
```

## JSON Shape

The `transcode` field in the `upload.started` payload must match
`streaming-transcode`'s `domain.TranscodeRequest`:

```json
{
  "transcode": {
    "codecs": ["h264", "av1"],
    "renditions": [
      { "width": 1280, "height": 720, "codec": "h264" },
      { "width": 1280, "height": 720, "codec": "av1" }
    ]
  }
}
```

The same shape is published on the `DomainEvent` and consumed by the
transcoder as-is (`json:"transcode,omitempty"`).

## Behaviour

- **Absent field**: if `transcode` is not present in the `upload.started`
  payload, `transcodeFromPayload` returns `nil`. The `Video` record stores
  nothing and the `DomainEvent` omits the field. The transcoder falls back to
  its built-in bitrate ladder and codec defaults.
- **Partial selection**: renditions without a positive `width` or `height` are
  silently skipped. A request with only `codecs` and no `renditions` (or vice
  versa) is valid and forwarded as-is.
- **Idempotency**: the storage webhook always reads the persisted record, so
  replayed webhooks (e.g. MinIO retry) forward the same selection.

## Implementation Notes

- `adapters.TranscodeRequest` and `adapters.RequestedRendition` are defined in
  `internal/adapters/storage_port.go` and tagged for both JSON and BSON so
  they round-trip through RabbitMQ and MongoDB without transformation.
- `transcodeFromPayload` in `internal/events/service.go` reuses the existing
  `firstInt64`/`firstString` helpers for type-safe extraction from the
  `map[string]interface{}` payload.
- The webhook copy block in `internal/webhooks/service.go` is structurally
  identical to the `RawVideo`/`Subtitles` blocks immediately above it.
