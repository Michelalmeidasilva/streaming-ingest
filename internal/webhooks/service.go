package webhooks

import (
	"context"
	"errors"
	"fmt"
	"log"
	"streaming-ingest/internal/adapters"
	"streaming-ingest/internal/rabbitmq"
	"streaming-ingest/internal/videos"
	"time"
)

type Service struct {
	publisher rabbitmq.MessagePublisher
	adapters  map[string]adapters.StorageAdapter
	repo      videos.VideoRepository
}

func NewService(pub rabbitmq.MessagePublisher, storageAdapters map[string]adapters.StorageAdapter, repo videos.VideoRepository) *Service {
	return &Service{
		publisher: pub,
		adapters:  storageAdapters,
		repo:      repo,
	}
}

func (s *Service) ProcessWebhook(provider string, payload []byte) error {
	adapter, exists := s.adapters[provider]
	if !exists {
		return fmt.Errorf("unsupported provider: %s", provider)
	}

	domainEvent, err := adapter.ParseEvent(payload)
	if err != nil {
		if errors.Is(err, adapters.ErrIgnoredObjectKey) {
			// Pipeline output (transcoded segments, metrics, thumbnails) — not a
			// new upload. Skip silently to avoid re-ingestion loops.
			log.Printf("webhook: ignoring pipeline-output object for provider %s", provider)
			return nil
		}
		return fmt.Errorf("failed to parse event for provider %s: %w", provider, err)
	}

	// Raw uploads (.yuv) cannot be inspected from the storage webhook, so the
	// geometry persisted at upload.started is carried forward to the transcoder
	// and preserved on the record (Save below would otherwise overwrite it).
	if existing, lookupErr := s.repo.FindByVideoID(context.Background(), domainEvent.VideoID); lookupErr == nil && existing != nil {
		if existing.RawVideo != nil {
			domainEvent.RawVideo = existing.RawVideo
		}
		if len(existing.Subtitles) > 0 {
			domainEvent.Subtitles = existing.Subtitles
		}
	}

	// Save metadata to MongoDB
	video := &videos.Video{
		VideoID:   domainEvent.VideoID,
		Filename:  domainEvent.Filename,
		Size:      domainEvent.Size,
		Provider:  domainEvent.Provider,
		Status:    "uploaded",
		RawVideo:  domainEvent.RawVideo,
		Subtitles: domainEvent.Subtitles,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = s.repo.Save(context.Background(), video)
	if err != nil {
		return fmt.Errorf("failed to save video metadata: %w", err)
	}

	routingKey := fmt.Sprintf("video.%s", domainEvent.EventType)
	err = s.publisher.Publish(routingKey, domainEvent)
	if err != nil {
		return fmt.Errorf("failed to publish domain event: %w", err)
	}

	return nil
}
