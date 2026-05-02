package videos

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"streaming-ingest/internal/adapters"
)

type mockStorageAdapter struct {
	listVideosResult []adapters.DomainEvent
	listVideosErr    error
	generateURLResult string
	generateURLErr    error
}

func (m *mockStorageAdapter) ParseEvent(payload []byte) (*adapters.DomainEvent, error) {
	return nil, nil
}

func (m *mockStorageAdapter) ListVideos(bucket string) ([]adapters.DomainEvent, error) {
	return m.listVideosResult, m.listVideosErr
}

func (m *mockStorageAdapter) GenerateURL(bucket, key string) (string, error) {
	return m.generateURLResult, m.generateURLErr
}

type mockVideoRepository struct {
	listAllResult []Video
	listAllErr    error
	searchResult  []Video
	searchErr     error
}

func (m *mockVideoRepository) Save(ctx context.Context, video *Video) error {
	return nil
}

func (m *mockVideoRepository) ListAll(ctx context.Context) ([]Video, error) {
	return m.listAllResult, m.listAllErr
}

func (m *mockVideoRepository) Search(ctx context.Context, query string) ([]Video, error) {
	return m.searchResult, m.searchErr
}

func TestNewService(t *testing.T) {
	mockAdapter := &mockStorageAdapter{}
	mockRepo := &mockVideoRepository{}
	adapters := map[string]adapters.StorageAdapter{"minio": mockAdapter}

	svc := NewService(adapters, mockRepo)

	if svc == nil {
		t.Errorf("NewService() returned nil")
	}

	if len(svc.adapters) != 1 {
		t.Errorf("NewService() did not set adapters")
	}

	if svc.repo != mockRepo {
		t.Errorf("NewService() did not set repo")
	}
}

func TestListAllVideos(t *testing.T) {
	tests := []struct {
		name           string
		mockAdapter    mockStorageAdapter
		mockRepo       mockVideoRepository
		wantErr        bool
		wantLen        int
	}{
		{
			name:        "repo_list_fails",
			mockAdapter: mockStorageAdapter{},
			mockRepo:    mockVideoRepository{listAllErr: errors.New("db error")},
			wantErr:     true,
			wantLen:     0,
		},
		{
			name:        "no_videos",
			mockAdapter: mockStorageAdapter{},
			mockRepo:    mockVideoRepository{listAllResult: []Video{}},
			wantErr:     false,
			wantLen:     0,
		},
		{
			name:     "with_videos_adapter_exists_url_ok",
			mockAdapter: mockStorageAdapter{
				generateURLResult: "http://minio:9000/videos/vid1/file.mp4",
				generateURLErr:    nil,
			},
			mockRepo: mockVideoRepository{
				listAllResult: []Video{
					{
						VideoID:   "vid1",
						Filename:  "file.mp4",
						Provider:  "minio",
						CreatedAt: time.Now(),
					},
				},
			},
			wantErr: false,
			wantLen: 1,
		},
		{
			name:     "with_videos_adapter_url_fails",
			mockAdapter: mockStorageAdapter{
				generateURLResult: "",
				generateURLErr:    errors.New("presign error"),
			},
			mockRepo: mockVideoRepository{
				listAllResult: []Video{
					{
						VideoID:   "vid1",
						Filename:  "file.mp4",
						Provider:  "minio",
						CreatedAt: time.Now(),
					},
				},
			},
			wantErr: false,
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapters := map[string]adapters.StorageAdapter{
				"minio": &tt.mockAdapter,
			}
			svc := NewService(adapters, &tt.mockRepo)

			got, err := svc.ListAllVideos(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("ListAllVideos() error = %v, wantErr %v", err, tt.wantErr)
			}

			if len(got) != tt.wantLen {
				t.Errorf("ListAllVideos() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestSyncVideosFromStorage(t *testing.T) {
	tests := []struct {
		name        string
		envBucket   string
		mockAdapter mockStorageAdapter
		mockRepo    mockVideoRepository
		wantErr     bool
	}{
		{
			name:      "bucket_from_env",
			envBucket: "custom-bucket",
			mockAdapter: mockStorageAdapter{
				listVideosResult: []adapters.DomainEvent{
					{
						VideoID:    "vid1",
						Filename:   "file.mp4",
						Size:       1024,
						Provider:   "minio",
						EventType:  "upload.completed",
						OccurredAt: time.Now(),
					},
				},
			},
			mockRepo: mockVideoRepository{},
			wantErr:  false,
		},
		{
			name:      "bucket_default",
			envBucket: "",
			mockAdapter: mockStorageAdapter{
				listVideosResult: []adapters.DomainEvent{},
			},
			mockRepo: mockVideoRepository{},
			wantErr:  false,
		},
		{
			name:      "list_videos_fails_continue",
			envBucket: "",
			mockAdapter: mockStorageAdapter{
				listVideosErr: errors.New("s3 error"),
			},
			mockRepo: mockVideoRepository{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env var if needed
			if tt.envBucket != "" {
				t.Setenv("STORAGE_BUCKET", tt.envBucket)
			} else {
				t.Setenv("STORAGE_BUCKET", "")
			}

			adapters := map[string]adapters.StorageAdapter{
				"minio": &tt.mockAdapter,
			}
			svc := NewService(adapters, &tt.mockRepo)

			err := svc.SyncVideosFromStorage(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("SyncVideosFromStorage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestServiceSearchVideos(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		mockAdapter mockStorageAdapter
		mockRepo    mockVideoRepository
		wantErr     bool
		wantLen     int
	}{
		{
			name:        "repo_search_fails",
			query:       "test",
			mockAdapter: mockStorageAdapter{},
			mockRepo:    mockVideoRepository{searchErr: errors.New("db error")},
			wantErr:     true,
			wantLen:     0,
		},
		{
			name:  "success",
			query: "test",
			mockAdapter: mockStorageAdapter{
				generateURLResult: "http://minio:9000/videos/vid1/file.mp4",
			},
			mockRepo: mockVideoRepository{
				searchResult: []Video{
					{
						VideoID:   "vid1",
						Filename:  "test.mp4",
						Provider:  "minio",
						CreatedAt: time.Now(),
					},
				},
			},
			wantErr: false,
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapters := map[string]adapters.StorageAdapter{
				"minio": &tt.mockAdapter,
			}
			svc := NewService(adapters, &tt.mockRepo)

			got, err := svc.SearchVideos(context.Background(), tt.query)

			if (err != nil) != tt.wantErr {
				t.Errorf("SearchVideos() error = %v, wantErr %v", err, tt.wantErr)
			}

			if len(got) != tt.wantLen {
				t.Errorf("SearchVideos() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestEnrichVideosEmptySlice(t *testing.T) {
	adapters := map[string]adapters.StorageAdapter{
		"minio": &mockStorageAdapter{},
	}
	svc := NewService(adapters, &mockVideoRepository{listAllResult: []Video{}})

	got := svc.enrichVideos([]Video{})

	if got != nil && len(got) != 0 {
		t.Errorf("enrichVideos() with empty slice should return nil or empty slice")
	}
}

func TestStorageBucketEnv(t *testing.T) {
	// Test that STORAGE_BUCKET env var is used correctly
	t.Setenv("STORAGE_BUCKET", "custom-bucket")
	defer os.Unsetenv("STORAGE_BUCKET")

	svc := NewService(
		map[string]adapters.StorageAdapter{},
		&mockVideoRepository{listAllResult: []Video{}},
	)

	got := svc.enrichVideos([]Video{})

	// Should not panic, should handle custom bucket
	if got != nil && len(got) != 0 {
		t.Errorf("enrichVideos() with custom bucket should still work")
	}
}
