package webhooks

import (
	"context"
	"errors"
	"fmt"
	"log"
	"streaming-ingest/internal/adapters"
	"streaming-ingest/internal/rabbitmq"
	"streaming-ingest/internal/uploadstate"
	"streaming-ingest/internal/videos"
	"time"
)

// UploadStatePatcher patches the per-video upload-state document. Satisfied by
// *uploadstate.Service. Used to mark the storage-confirmed moment (stage 3).
type UploadStatePatcher interface {
	PatchVideo(ctx context.Context, videoID string, patch map[string]any) (*uploadstate.Video, error)
}

type Service struct {
	publisher   rabbitmq.MessagePublisher
	adapters    map[string]adapters.StorageAdapter
	repo        videos.VideoRepository
	uploadState UploadStatePatcher
}

func NewService(pub rabbitmq.MessagePublisher, storageAdapters map[string]adapters.StorageAdapter, repo videos.VideoRepository, uploadState UploadStatePatcher) *Service {
	return &Service{
		publisher:   pub,
		adapters:    storageAdapters,
		repo:        repo,
		uploadState: uploadState,
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
		if existing.Transcode != nil {
			domainEvent.Transcode = existing.Transcode
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
		Transcode: domainEvent.Transcode,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = s.repo.Save(context.Background(), video)
	if err != nil {
		return fmt.Errorf("failed to save video metadata: %w", err)
	}

	// Stage 3: mark the object as confirmed/available in the upload-state store
	// before publishing the event that triggers transcode, so the platform-upload
	// surfaces "available" ahead of "queued". Provider-agnostic (minio + s3).
	if s.uploadState != nil {
		if _, perr := s.uploadState.PatchVideo(context.Background(), domainEvent.VideoID, map[string]any{
			"storage_confirmed_at": time.Now().UTC(),
		}); perr != nil && !errors.Is(perr, uploadstate.ErrNotFound) {
			log.Printf("webhook: failed to patch storage_confirmed_at for %s: %v", domainEvent.VideoID, perr)
		}
	}

	routingKey := fmt.Sprintf("video.%s", domainEvent.EventType)
	err = s.publisher.Publish(routingKey, domainEvent)
	if err != nil {
		return fmt.Errorf("failed to publish domain event: %w", err)
	}

	return nil
}
