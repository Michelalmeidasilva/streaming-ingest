package events

import (
	"context"
	"fmt"
	"time"

	"streaming-ingest/internal/adapters"
	"streaming-ingest/internal/rabbitmq"
	"streaming-ingest/internal/videos"
)

type EventPayload map[string]interface{}

type FrontEndEvent struct {
	EventType string       `json:"eventType"`
	Payload   EventPayload `json:"payload"`
}

type Service struct {
	publisher rabbitmq.MessagePublisher
	repo      EventRepository
	videoRepo videos.VideoRepository
}

func NewService(pub rabbitmq.MessagePublisher, repo EventRepository, videoRepo videos.VideoRepository) *Service {
	return &Service{
		publisher: pub,
		repo:      repo,
		videoRepo: videoRepo,
	}
}

func (s *Service) ProcessEvent(event FrontEndEvent) error {
	record := eventToRecord(event)
	if err := s.repo.Save(context.Background(), record); err != nil {
		return fmt.Errorf("failed to save event metadata: %w", err)
	}

	if event.EventType == "upload.started" && s.videoRepo != nil {
		video, err := pendingVideoFromEvent(event)
		if err != nil {
			return fmt.Errorf("failed to build video metadata: %w", err)
		}

		if err := s.videoRepo.Create(context.Background(), video); err != nil {
			return fmt.Errorf("failed to create video metadata: %w", err)
		}
	}

	err := s.publisher.Publish(record.RoutingKey, event.Payload)
	if err != nil {
		return fmt.Errorf("failed to process and publish event: %w", err)
	}

	return nil
}

func eventToRecord(event FrontEndEvent) *EventRecord {
	now := time.Now().UTC()

	return &EventRecord{
		EventType:  event.EventType,
		RoutingKey: fmt.Sprintf("video.%s", event.EventType),
		Payload:    event.Payload,
		CreatedAt:  now,
	}
}

func pendingVideoFromEvent(event FrontEndEvent) (*videos.Video, error) {
	videoID := firstString(event.Payload, "videoId", "videoID")
	if videoID == "" {
		return nil, fmt.Errorf("videoId is required")
	}

	filename := firstString(event.Payload, "filename", "name")
	if filename == "" {
		return nil, fmt.Errorf("filename is required")
	}

	size := firstInt64(event.Payload, "totalBytes", "size", "uploadedBytes")
	now := time.Now().UTC()

	return &videos.Video{
		VideoID:   videoID,
		Filename:  filename,
		Size:      size,
		Provider:  firstString(event.Payload, "provider"),
		Status:    "uploading",
		RawVideo:  rawVideoFromPayload(event.Payload),
		Subtitles: subtitlesFromPayload(event.Payload),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// subtitlesFromPayload extracts sidecar subtitle references from an upload
// event. Each entry needs an objectKey; entries without one are skipped.
func subtitlesFromPayload(payload EventPayload) []adapters.SubtitleRef {
	raw, ok := payload["subtitles"].([]interface{})
	if !ok || len(raw) == 0 {
		return nil
	}
	refs := make([]adapters.SubtitleRef, 0, len(raw))
	for _, item := range raw {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		nested := EventPayload(entry)
		objectKey := firstString(nested, "objectKey", "object_key", "key")
		if objectKey == "" {
			continue
		}
		refs = append(refs, adapters.SubtitleRef{
			ObjectKey: objectKey,
			Language:  firstString(nested, "language", "lang"),
			Label:     firstString(nested, "label"),
		})
	}
	if len(refs) == 0 {
		return nil
	}
	return refs
}

// rawVideoFromPayload extracts headerless raw geometry (.yuv) from an upload
// event, if present. Returns nil when no usable width/height/fps are supplied,
// so containers and self-describing formats persist nothing.
func rawVideoFromPayload(payload EventPayload) *adapters.RawVideoParams {
	raw, ok := payload["rawVideo"].(map[string]interface{})
	if !ok {
		return nil
	}
	nested := EventPayload(raw)
	width := int(firstInt64(nested, "width"))
	height := int(firstInt64(nested, "height"))
	fps := firstFloat64(nested, "fps")
	if width <= 0 || height <= 0 || fps <= 0 {
		return nil
	}
	return &adapters.RawVideoParams{
		Width:       width,
		Height:      height,
		FPS:         fps,
		PixelFormat: firstString(nested, "pixelFormat", "pixel_format"),
	}
}

func firstFloat64(payload EventPayload, keys ...string) float64 {
	for _, key := range keys {
		if raw, ok := payload[key]; ok {
			switch value := raw.(type) {
			case float64:
				return value
			case int64:
				return float64(value)
			case int:
				return float64(value)
			}
		}
	}
	return 0
}

func firstString(payload EventPayload, keys ...string) string {
	for _, key := range keys {
		if raw, ok := payload[key]; ok {
			if value, ok := raw.(string); ok {
				return value
			}
		}
	}
	return ""
}

func firstInt64(payload EventPayload, keys ...string) int64 {
	for _, key := range keys {
		if raw, ok := payload[key]; ok {
			switch value := raw.(type) {
			case int64:
				return value
			case int:
				return int64(value)
			case float64:
				return int64(value)
			}
		}
	}
	return 0
}
