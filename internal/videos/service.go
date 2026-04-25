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
	all, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list videos from database: %w", err)
	}

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

	return results, nil
}
