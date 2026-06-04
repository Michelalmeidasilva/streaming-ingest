# Raw video (.yuv) metadata propagation

## Motivation
Headerless raw uploads (`.yuv`) carry no geometry, so the transcoder needs the
width/height/fps/pixel-format supplied at upload time. The storage webhook that
triggers transcoding (`ObjectCreated` → `video.upload.completed`) cannot infer
this from the object, so the gateway has to carry it across.

## Design
The geometry rides the existing lifecycle, the gateway never inspects media:

1. **Persist** — the upload platform includes `rawVideo` in the `upload.started`
   payload. `events.Service` (via `pendingVideoFromEvent` →
   `rawVideoFromPayload`) stores it on the video record as `raw_video`
   (`adapters.RawVideoParams`). Returns nil for non-raw formats.
2. **Forward** — on the storage `ObjectCreated` webhook, `webhooks.Service`
   looks the record up with `VideoRepository.FindByVideoID(videoID)` and, if
   `RawVideo` is present, attaches it to the published `DomainEvent.RawVideo`.
   It is also re-applied on the webhook's `Save` so the "uploaded" upsert does
   not clobber the geometry.

## Contract changes
- `adapters.DomainEvent.RawVideo *RawVideoParams` (published to RabbitMQ on
  `video.upload.completed`).
- `adapters.RawVideoParams { Width, Height, FPS, PixelFormat }`.
- `videos.Video.RawVideo` (`bson:"raw_video"`).
- `videos.VideoRepository.FindByVideoID(ctx, videoID) (*Video, error)` —
  returns nil when no record exists (Mongo + in-memory implementations).

## Ordering & caveats
- `upload.started` is emitted before the file finishes uploading, so it is
  persisted before the `ObjectCreated` webhook fires — the lookup normally finds
  the geometry. If it is missing (race / non-raw), `RawVideo` stays nil and the
  transcoder rejects `.yuv` with a terminal error.
- The gateway still "only ingests and publishes" — it reads its own persisted
  record, it does not probe media.
