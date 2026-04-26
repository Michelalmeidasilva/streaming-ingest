package videos

import (
	"context"
	"fmt"
	"os"
	"streaming-ingest/internal/adapters"
)

// VideoResponse enriches a Video with a storage download URL.
type VideoResponse struct {
	VideoID   string `json:"videoId"`
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	Provider  string `json:"provider"`
	Status    string `json:"status"`
	URL       string `json:"url"`
	CreatedAt string `json:"createdAt"`
}

type Service struct {
	adapters map[string]adapters.StorageAdapter
	repo     VideoRepository
}

func NewService(storageAdapters map[string]adapters.StorageAdapter, repo VideoRepository) *Service {
	return &Service{
		adapters: storageAdapters,
		repo:     repo,
	}
}

func (s *Service) ListAllVideos(ctx context.Context) ([]VideoResponse, error) {
	// Always try to sync from storage to detect new/manual uploads
	_ = s.SyncVideosFromStorage(ctx)

	all, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list videos from database: %w", err)
	}

	return s.enrichVideos(all), nil
}

func (s *Service) SyncVideosFromStorage(ctx context.Context) error {
	bucket := os.Getenv("STORAGE_BUCKET")
	if bucket == "" {
		bucket = "videos"
	}

	for _, adapter := range s.adapters {
		storageVideos, err := adapter.ListVideos(bucket)
		if err != nil {
			continue
		}

		for _, sv := range storageVideos {
			video := &Video{
				VideoID:   sv.VideoID,
				Filename:  sv.Filename,
				Size:      sv.Size,
				Provider:  sv.Provider,
				Status:    "ready",
				CreatedAt: sv.OccurredAt,
				UpdatedAt: sv.OccurredAt,
			}
			_ = s.repo.Save(ctx, video)
		}
	}
	return nil
}

func (s *Service) SearchVideos(ctx context.Context, query string) ([]VideoResponse, error) {
	all, err := s.repo.Search(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to search videos from database: %w", err)
	}

	return s.enrichVideos(all), nil
}

func (s *Service) enrichVideos(all []Video) []VideoResponse {
	bucket := os.Getenv("STORAGE_BUCKET")
	if bucket == "" {
		bucket = "videos"
	}

	var results []VideoResponse
	for _, v := range all {
		// Build the object key: <videoID>/<filename>
		key := fmt.Sprintf("%s/%s", v.VideoID, v.Filename)

		adapter, exists := s.adapters[v.Provider]
		var downloadURL string
		if exists {
			var err error
			downloadURL, err = adapter.GenerateURL(bucket, key)
			if err != nil {
				// Non-fatal: return empty URL rather than failing the whole list
				downloadURL = ""
			}
		}

		results = append(results, VideoResponse{
			VideoID:   v.VideoID,
			Filename:  v.Filename,
			Size:      v.Size,
			Provider:  v.Provider,
			Status:    v.Status,
			URL:       downloadURL,
			CreatedAt: v.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	return results
}
