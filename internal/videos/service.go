package videos

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"streaming-ingest/internal/adapters"
	"strings"
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
	mongoVideos, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list videos from database: %w", err)
	}

	storageVideos, err := s.collectStorageVideos(ctx)
	if err != nil && len(storageVideos) == 0 {
		return s.enrichVideos(mongoVideos), nil
	}

	merged := mergeVideosByMatch(mongoVideos, storageVideos)
	return s.enrichVideos(merged), nil
}

func (s *Service) ListDatabaseVideos(ctx context.Context) ([]VideoResponse, error) {
	videos, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list videos from database: %w", err)
	}

	return s.enrichVideos(videos), nil
}

func (s *Service) collectStorageVideos(ctx context.Context) ([]Video, error) {
	bucket := os.Getenv("STORAGE_BUCKET")
	if bucket == "" {
		bucket = "videos"
	}

	providers := make([]string, 0, len(s.adapters))
	if _, ok := s.adapters["minio"]; ok {
		providers = append(providers, "minio")
	}
	var otherProviders []string
	for provider := range s.adapters {
		if provider == "minio" {
			continue
		}
		otherProviders = append(otherProviders, provider)
	}
	sort.Strings(otherProviders)
	providers = append(providers, otherProviders...)

	var (
		listErrs   []error
		storageSet []Video
	)

	for _, provider := range providers {
		adapter := s.adapters[provider]
		storageVideos, err := adapter.ListVideos(bucket)
		if err != nil {
			listErrs = append(listErrs, err)
			continue
		}

		for _, sv := range storageVideos {
			video := Video{
				VideoID:   sv.VideoID,
				Filename:  sv.Filename,
				Size:      sv.Size,
				Provider:  sv.Provider,
				Status:    "ready",
				CreatedAt: sv.OccurredAt,
				UpdatedAt: sv.OccurredAt,
			}
			if err := s.repo.Save(ctx, &video); err != nil {
				return nil, fmt.Errorf("failed to save video %s from storage: %w", sv.VideoID, err)
			}
			storageSet = append(storageSet, video)
		}
	}

	if len(listErrs) > 0 && len(storageSet) == 0 {
		return nil, errors.Join(listErrs...)
	}

	return storageSet, nil
}

func mergeVideosByMatch(mongoVideos, storageVideos []Video) []Video {
	storageByKey := make(map[string]Video, len(storageVideos))
	for _, video := range storageVideos {
		storageByKey[videoMatchKey(video)] = video
	}

	merged := make([]Video, 0, len(mongoVideos))

	for _, video := range mongoVideos {
		key := videoMatchKey(video)
		if storageVideo, ok := storageByKey[key]; ok {
			merged = append(merged, mergeVideo(video, storageVideo))
			continue
		}
		merged = append(merged, video)
	}

	return merged
}

func mergeVideo(mongoVideo, storageVideo Video) Video {
	merged := mongoVideo
	if merged.Size == 0 {
		merged.Size = storageVideo.Size
	}
	if merged.Provider == "" {
		merged.Provider = storageVideo.Provider
	}
	if merged.Status == "" {
		merged.Status = storageVideo.Status
	}
	if merged.CreatedAt.IsZero() {
		merged.CreatedAt = storageVideo.CreatedAt
	}
	if merged.UpdatedAt.IsZero() {
		merged.UpdatedAt = storageVideo.UpdatedAt
	}
	if merged.VideoID == "" {
		merged.VideoID = storageVideo.VideoID
	}
	if merged.Filename == "" {
		merged.Filename = storageVideo.Filename
	}

	return merged
}

func videoMatchKey(video Video) string {
	return strings.ToLower(fmt.Sprintf("%s|%s|%s", video.Provider, video.VideoID, video.Filename))
}

func (s *Service) SyncVideosFromStorage(ctx context.Context) error {
	_, err := s.collectStorageVideos(ctx)
	return err
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
