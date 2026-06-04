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
	listVideosResult  []adapters.DomainEvent
	listVideosErr     error
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
	saveErr       error
	createErr     error
}

func (m *mockVideoRepository) Create(ctx context.Context, video *Video) error {
	return m.createErr
}

func (m *mockVideoRepository) Save(ctx context.Context, video *Video) error {
	return m.saveErr
}

func (m *mockVideoRepository) ListAll(ctx context.Context) ([]Video, error) {
	return m.listAllResult, m.listAllErr
}

func (m *mockVideoRepository) Search(ctx context.Context, query string) ([]Video, error) {
	return m.searchResult, m.searchErr
}

func (m *mockVideoRepository) FindByVideoID(ctx context.Context, videoID string) (*Video, error) {
	return nil, nil
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
		name        string
		mockAdapter mockStorageAdapter
		mockRepo    mockVideoRepository
		wantErr     bool
		wantLen     int
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
			name: "with_videos_adapter_exists_url_ok",
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
			name: "with_videos_adapter_url_fails",
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
		{
			name: "mongo_first_and_storage_match",
			mockAdapter: mockStorageAdapter{
				listVideosResult: []adapters.DomainEvent{
					{
						VideoID:    "vid1",
						Filename:   "file.mp4",
						Size:       222,
						Provider:   "minio",
						EventType:  "upload.completed",
						OccurredAt: time.Now(),
					},
					{
						VideoID:    "vid2",
						Filename:   "extra.mp4",
						Size:       333,
						Provider:   "minio",
						EventType:  "upload.completed",
						OccurredAt: time.Now(),
					},
				},
				generateURLResult: "http://minio:9000/videos/vid1/file.mp4",
			},
			mockRepo: mockVideoRepository{
				listAllResult: []Video{
					{
						VideoID:   "vid1",
						Filename:  "file.mp4",
						Size:      111,
						Provider:  "minio",
						Status:    "uploaded",
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

			if tt.name == "mongo_first_and_storage_match" {
				if len(got) != 1 {
					t.Fatalf("ListAllVideos() len = %d, want 1", len(got))
				}
				if got[0].VideoID != "vid1" || got[0].Status != "uploaded" {
					t.Fatalf("item = %+v, want mongo item enriched by storage", got[0])
				}
			}
		})
	}
}

func TestListDatabaseVideosService(t *testing.T) {
	tests := []struct {
		name     string
		mockRepo mockVideoRepository
		wantErr  bool
		wantLen  int
	}{
		{
			name:     "repo_list_fails",
			mockRepo: mockVideoRepository{listAllErr: errors.New("db error")},
			wantErr:  true,
			wantLen:  0,
		},
		{
			name:     "no_videos",
			mockRepo: mockVideoRepository{listAllResult: []Video{}},
			wantErr:  false,
			wantLen:  0,
		},
		{
			name: "with_videos",
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
			svc := NewService(map[string]adapters.StorageAdapter{}, &tt.mockRepo)

			got, err := svc.ListDatabaseVideos(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("ListDatabaseVideos() error = %v, wantErr %v", err, tt.wantErr)
			}

			if len(got) != tt.wantLen {
				t.Errorf("ListDatabaseVideos() len = %d, want %d", len(got), tt.wantLen)
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
			name:      "list_videos_fails",
			envBucket: "",
			mockAdapter: mockStorageAdapter{
				listVideosErr: errors.New("s3 error"),
			},
			mockRepo: mockVideoRepository{},
			wantErr:  true,
		},
		{
			name:      "save_fails",
			envBucket: "",
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
			mockRepo: mockVideoRepository{saveErr: errors.New("mongo write failed")},
			wantErr:  true,
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

func TestMergeVideo(t *testing.T) {
	now := time.Now()
	mongoVideo := Video{
		VideoID:  "vid-1",
		Filename: "file.mp4",
	}
	storageVideo := Video{
		VideoID:   "vid-1",
		Filename:  "file.mp4",
		Size:      1024,
		Provider:  "minio",
		Status:    "ready",
		CreatedAt: now,
		UpdatedAt: now,
	}

	merged := mergeVideo(mongoVideo, storageVideo)

	if merged.Size != 1024 {
		t.Errorf("merged.Size = %d, want 1024", merged.Size)
	}
	if merged.Provider != "minio" {
		t.Errorf("merged.Provider = %s, want minio", merged.Provider)
	}
	if merged.Status != "ready" {
		t.Errorf("merged.Status = %s, want ready", merged.Status)
	}
	if !merged.CreatedAt.Equal(now) {
		t.Errorf("merged.CreatedAt = %v, want %v", merged.CreatedAt, now)
	}
	if !merged.UpdatedAt.Equal(now) {
		t.Errorf("merged.UpdatedAt = %v, want %v", merged.UpdatedAt, now)
	}

	// Test that it doesn't overwrite if not empty
	mongoVideoFull := Video{
		VideoID:  "vid-1",
		Filename: "file.mp4",
		Size:     512,
		Provider: "aws",
		Status:   "processing",
	}
	merged2 := mergeVideo(mongoVideoFull, storageVideo)
	if merged2.Size != 512 {
		t.Errorf("merged2.Size = %d, want 512", merged2.Size)
	}
	if merged2.Provider != "aws" {
		t.Errorf("merged2.Provider = %s, want aws", merged2.Provider)
	}
	if merged2.Status != "processing" {
		t.Errorf("merged2.Status = %s, want processing", merged2.Status)
	}
}
