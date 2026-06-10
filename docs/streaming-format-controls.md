# Streaming Format Controls (Ingest Passthrough)

## Motivation

The Event Gateway persists the upload-time `transcode` selection on the video record at
`upload.started` and forwards it to the transcoder on the storage `upload.completed` webhook.
The selection is (de)serialized through a typed mirror struct, `adapters.TranscodeRequest`.

**Any field not declared on that struct is silently dropped** on the round-trip. This is why
per-rendition `bitrateKbps` — already honored by `streaming-transcode` — never reached the
worker, and why the new `protocols` / `segmentSeconds` choices would have been lost too.

## Change

`internal/adapters/storage_port.go` widens the mirror structs:

```go
type TranscodeRequest struct {
    Codecs         []string             `json:"codecs,omitempty" bson:"codecs,omitempty"`
    Protocols      []string             `json:"protocols,omitempty" bson:"protocols,omitempty"`
    SegmentSeconds int                  `json:"segmentSeconds,omitempty" bson:"segmentSeconds,omitempty"`
    Renditions     []RequestedRendition `json:"renditions,omitempty" bson:"renditions,omitempty"`
}

type RequestedRendition struct {
    Width       int    `json:"width" bson:"width"`
    Height      int    `json:"height" bson:"height"`
    Codec       string `json:"codec,omitempty" bson:"codec,omitempty"`
    BitrateKbps int    `json:"bitrateKbps,omitempty" bson:"bitrateKbps,omitempty"`
}
```

The persistence path (`internal/videos/repository.go`) and the webhook forwarding
(`internal/webhooks/service.go`, which already copies `existing.Transcode` wholesale) require
no logic change — only the wider struct. `internal/uploadstate/repository.go` already stores
`Transcode` as `map[string]any` (lossless) and is unaffected.

## Contract test

`internal/adapters/transcode_contract_test.go` round-trips a transcoder-shaped JSON payload
(`protocols`, `segmentSeconds`, `bitrateKbps`) through `TranscodeRequest` and asserts every
field survives both unmarshal and re-marshal — pinning the ingest↔transcode contract so a
future field can't leak again.

## Caveat

The gateway remains a pass-through for this object: it does not validate protocol/segment/
bitrate values. The transcoder normalizes/clamps them (`ResolveProtocols`,
`ResolveSegmentSeconds`, default bitrate ladder).

## Related

- `../../docs/design-docs/specs/2026-06-10-streaming-format-controls-design.md` — design spec
- `streaming-transcode/docs/streaming-format-controls.md` — the consumer side
