package events

import (
	"context"
	"fmt"
	"time"

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
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
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
