package adapters

import "time"

// DomainEvent is the standardized event structure published to RabbitMQ
type DomainEvent struct {
	EventType string `json:"eventType"` // e.g. upload.completed
	VideoID   string `json:"videoId"`
	Filename  string `json:"filename"`
	// ObjectKey is the full storage key (e.g. `raw/<videoId>/<filename>`). The
	// transcoder downloads from this exact key; reconstructing it from
	// videoId+filename would drop multi-segment prefixes like `raw/`.
	ObjectKey  string    `json:"objectKey,omitempty"`
	Size       int64     `json:"size"`
	Provider   string    `json:"provider"`
	OccurredAt time.Time `json:"occurredAt"`
	// RawVideo carries the geometry of headerless raw sources (.yuv) supplied at
	// upload time. Storage webhooks cannot infer it, so the gateway looks it up
	// from the persisted video record and forwards it to the transcoder. nil for
	// containers and self-describing formats.
	RawVideo *RawVideoParams `json:"rawVideo,omitempty"`
	// Subtitles are sidecar .srt tracks uploaded with the video, looked up from
	// the persisted record and forwarded so the transcoder can package WebVTT.
	Subtitles []SubtitleRef `json:"subtitles,omitempty"`
	// Transcode is the codec/resolution selection made at upload time, persisted
	// on the record and forwarded to the transcoder. nil = transcoder defaults.
	Transcode *TranscodeRequest `json:"transcode,omitempty"`
}

// SubtitleRef points at a sidecar subtitle source (.srt) in storage and its
// language/label, as supplied at upload time.
type SubtitleRef struct {
	ObjectKey string `json:"objectKey" bson:"object_key"`
	Language  string `json:"language,omitempty" bson:"language,omitempty"`
	Label     string `json:"label,omitempty" bson:"label,omitempty"`
}

// RawVideoParams describes a headerless raw video stream. ffprobe cannot read
// these from the file, so the transcoder needs them as ffmpeg demuxer options.
type RawVideoParams struct {
	Width       int     `json:"width" bson:"width"`
	Height      int     `json:"height" bson:"height"`
	FPS         float64 `json:"fps" bson:"fps"`
	PixelFormat string  `json:"pixelFormat,omitempty" bson:"pixel_format,omitempty"`
}

// TranscodeRequest mirrors streaming-transcode's domain.TranscodeRequest so the
// selection survives the trip through RabbitMQ unchanged.
type TranscodeRequest struct {
	Codecs     []string             `json:"codecs,omitempty" bson:"codecs,omitempty"`
	Renditions []RequestedRendition `json:"renditions,omitempty" bson:"renditions,omitempty"`
}

type RequestedRendition struct {
	Width  int    `json:"width" bson:"width"`
	Height int    `json:"height" bson:"height"`
	Codec  string `json:"codec,omitempty" bson:"codec,omitempty"`
}

// StorageAdapter parses provider-specific webhooks into domain events
type StorageAdapter interface {
	ParseEvent(payload []byte) (*DomainEvent, error)
	ListVideos(bucket string) ([]DomainEvent, error)
	GenerateURL(bucket, key string) (string, error)
}
