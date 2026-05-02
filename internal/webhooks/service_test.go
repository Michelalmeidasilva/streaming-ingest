package webhooks

import (
	"context"
	"errors"
	"testing"
	"time"

	"streaming-ingest/internal/adapters"
	"streaming-ingest/internal/videos"
)

type mockPublisher struct {
	publishErr error
	calledWith []string
}

func (m *mockPublisher) Publish(routingKey string, payload interface{}) error {
	m.calledWith = append(m.calledWith, routingKey)
	return m.publishErr
}

type mockStorageAdapter struct {
	parseEventResult *adapters.DomainEvent
	parseEventErr    error
}

func (m *mockStorageAdapter) ParseEvent(payload []byte) (*adapters.DomainEvent, error) {
	return m.parseEventResult, m.parseEventErr
}

func (m *mockStorageAdapter) ListVideos(bucket string) ([]adapters.DomainEvent, error) {
	return nil, nil
}

func (m *mockStorageAdapter) GenerateURL(bucket, key string) (string, error) {
	return "", nil
}

type mockVideoRepository struct {
	saveErr error
}

func (m *mockVideoRepository) Save(ctx context.Context, video *videos.Video) error {
	return m.saveErr
}

func (m *mockVideoRepository) ListAll(ctx context.Context) ([]videos.Video, error) {
	return nil, nil
}

func (m *mockVideoRepository) Search(ctx context.Context, query string) ([]videos.Video, error) {
	return nil, nil
}

func TestNewService(t *testing.T) {
	mock := &mockPublisher{}
	mockAdapter := &mockStorageAdapter{}
	mockRepo := &mockVideoRepository{}
	adapters := map[string]adapters.StorageAdapter{"minio": mockAdapter}

	svc := NewService(mock, adapters, mockRepo)

	if svc == nil {
		t.Errorf("NewService() returned nil")
	}

	if svc.publisher != mock {
		t.Errorf("NewService() did not set publisher correctly")
	}

	if len(svc.adapters) != 1 {
		t.Errorf("NewService() did not set adapters correctly")
	}

	if svc.repo != mockRepo {
		t.Errorf("NewService() did not set repo correctly")
	}
}

func TestProcessWebhook(t *testing.T) {
	tests := []struct {
		name             string
		provider         string
		adapters         map[string]adapters.StorageAdapter
		mockAdapter      *mockStorageAdapter
		mockRepo         *mockVideoRepository
		mockPublisher    *mockPublisher
		wantErr          bool
		errMsg           string
	}{
		{
			name:     "unsupported_provider",
			provider: "unknown-provider",
			adapters: map[string]adapters.StorageAdapter{
				"minio": &mockStorageAdapter{},
			},
			mockAdapter:   &mockStorageAdapter{},
			mockRepo:      &mockVideoRepository{},
			mockPublisher: &mockPublisher{},
			wantErr:       true,
			errMsg:        "unsupported provider",
		},
		{
			name:     "parse_event_fails",
			provider: "minio",
			adapters: map[string]adapters.StorageAdapter{
				"minio": &mockStorageAdapter{parseEventErr: errors.New("invalid event")},
			},
			mockAdapter:   &mockStorageAdapter{parseEventErr: errors.New("invalid event")},
			mockRepo:      &mockVideoRepository{},
			mockPublisher: &mockPublisher{},
			wantErr:       true,
			errMsg:        "failed to parse event",
		},
		{
			name:     "repo_save_fails",
			provider: "minio",
			adapters: map[string]adapters.StorageAdapter{
				"minio": &mockStorageAdapter{
					parseEventResult: &adapters.DomainEvent{
						EventType:  "upload.completed",
						VideoID:    "vid1",
						Filename:   "video.mp4",
						Size:       1024,
						Provider:   "minio",
						OccurredAt: time.Now(),
					},
				},
			},
			mockAdapter: &mockStorageAdapter{
				parseEventResult: &adapters.DomainEvent{
					EventType:  "upload.completed",
					VideoID:    "vid1",
					Filename:   "video.mp4",
					Size:       1024,
					Provider:   "minio",
					OccurredAt: time.Now(),
				},
			},
			mockRepo:      &mockVideoRepository{saveErr: errors.New("db error")},
			mockPublisher: &mockPublisher{},
			wantErr:       true,
			errMsg:        "failed to save video metadata",
		},
		{
			name:     "publish_fails",
			provider: "minio",
			adapters: map[string]adapters.StorageAdapter{
				"minio": &mockStorageAdapter{
					parseEventResult: &adapters.DomainEvent{
						EventType:  "upload.completed",
						VideoID:    "vid1",
						Filename:   "video.mp4",
						Size:       1024,
						Provider:   "minio",
						OccurredAt: time.Now(),
					},
				},
			},
			mockAdapter: &mockStorageAdapter{
				parseEventResult: &adapters.DomainEvent{
					EventType:  "upload.completed",
					VideoID:    "vid1",
					Filename:   "video.mp4",
					Size:       1024,
					Provider:   "minio",
					OccurredAt: time.Now(),
				},
			},
			mockRepo:      &mockVideoRepository{},
			mockPublisher: &mockPublisher{publishErr: errors.New("amqp error")},
			wantErr:       true,
			errMsg:        "failed to publish domain event",
		},
		{
			name:     "success",
			provider: "minio",
			adapters: map[string]adapters.StorageAdapter{
				"minio": &mockStorageAdapter{
					parseEventResult: &adapters.DomainEvent{
						EventType:  "upload.completed",
						VideoID:    "vid1",
						Filename:   "video.mp4",
						Size:       1024,
						Provider:   "minio",
						OccurredAt: time.Now(),
					},
				},
			},
			mockAdapter: &mockStorageAdapter{
				parseEventResult: &adapters.DomainEvent{
					EventType:  "upload.completed",
					VideoID:    "vid1",
					Filename:   "video.mp4",
					Size:       1024,
					Provider:   "minio",
					OccurredAt: time.Now(),
				},
			},
			mockRepo:      &mockVideoRepository{},
			mockPublisher: &mockPublisher{},
			wantErr:       false,
			errMsg:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.adapters == nil {
				tt.adapters = map[string]adapters.StorageAdapter{
					"minio": tt.mockAdapter,
				}
			}

			svc := &Service{
				publisher: tt.mockPublisher,
				adapters:  tt.adapters,
				repo:      tt.mockRepo,
			}

			payload := []byte(`{"EventName":"ObjectCreated:Put","Records":[]}`)
			err := svc.ProcessWebhook(tt.provider, payload)

			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessWebhook() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil && tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
				t.Errorf("ProcessWebhook() error = %v, want to contain %v", err.Error(), tt.errMsg)
			}
		})
	}
}
