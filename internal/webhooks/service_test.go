package webhooks

import (
	"context"
	"errors"
	"testing"
	"time"

	"streaming-ingest/internal/adapters"
	"streaming-ingest/internal/uploadstate"
	"streaming-ingest/internal/videos"
)

type mockPublisher struct {
	publishErr error
	calledWith []string
	payloads   []interface{}
}

func (m *mockPublisher) Publish(routingKey string, payload interface{}) error {
	m.calledWith = append(m.calledWith, routingKey)
	m.payloads = append(m.payloads, payload)
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
	saveErr     error
	findResult  *videos.Video
	findErr     error
	savedVideos []*videos.Video
}

func (m *mockVideoRepository) Create(ctx context.Context, video *videos.Video) error {
	return nil
}

func (m *mockVideoRepository) Save(ctx context.Context, video *videos.Video) error {
	m.savedVideos = append(m.savedVideos, video)
	return m.saveErr
}

func (m *mockVideoRepository) ListAll(ctx context.Context) ([]videos.Video, error) {
	return nil, nil
}

func (m *mockVideoRepository) Search(ctx context.Context, query string) ([]videos.Video, error) {
	return nil, nil
}

func (m *mockVideoRepository) FindByVideoID(ctx context.Context, videoID string) (*videos.Video, error) {
	return m.findResult, m.findErr
}

type mockUploadStatePatcher struct {
	patchedID  string
	patchedKey []string
	patchErr   error
}

func (m *mockUploadStatePatcher) PatchVideo(ctx context.Context, videoID string, patch map[string]any) (*uploadstate.Video, error) {
	m.patchedID = videoID
	for k := range patch {
		m.patchedKey = append(m.patchedKey, k)
	}
	return nil, m.patchErr
}

func TestProcessWebhook_PatchesStorageConfirmedAt(t *testing.T) {
	pub := &mockPublisher{}
	patcher := &mockUploadStatePatcher{}
	adapter := &mockStorageAdapter{
		parseEventResult: &adapters.DomainEvent{
			VideoID:   "abc-123",
			Filename:  "movie.mp4",
			EventType: "upload.completed",
			Provider:  "aws-s3",
			Size:      2048,
		},
	}
	svc := NewService(pub, map[string]adapters.StorageAdapter{"s3": adapter}, &mockVideoRepository{}, patcher)

	if err := svc.ProcessWebhook("s3", []byte(`{}`)); err != nil {
		t.Fatalf("ProcessWebhook returned error: %v", err)
	}
	if patcher.patchedID != "abc-123" {
		t.Errorf("patched videoID = %q, want %q", patcher.patchedID, "abc-123")
	}
	found := false
	for _, k := range patcher.patchedKey {
		if k == "storage_confirmed_at" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected storage_confirmed_at in patch, got keys %v", patcher.patchedKey)
	}
	if len(pub.calledWith) != 1 {
		t.Errorf("expected exactly 1 publish, got %d", len(pub.calledWith))
	}
}

func TestProcessWebhook_ToleratesPatchNotFound(t *testing.T) {
	pub := &mockPublisher{}
	patcher := &mockUploadStatePatcher{patchErr: uploadstate.ErrNotFound}
	adapter := &mockStorageAdapter{
		parseEventResult: &adapters.DomainEvent{
			VideoID: "missing", Filename: "x.mp4", EventType: "upload.completed", Provider: "minio",
		},
	}
	svc := NewService(pub, map[string]adapters.StorageAdapter{"minio": adapter}, &mockVideoRepository{}, patcher)

	if err := svc.ProcessWebhook("minio", []byte(`{}`)); err != nil {
		t.Fatalf("ProcessWebhook should tolerate ErrNotFound, got %v", err)
	}
	if len(pub.calledWith) != 1 {
		t.Errorf("expected publish to still happen, got %d publishes", len(pub.calledWith))
	}
}

func TestNewService(t *testing.T) {
	mock := &mockPublisher{}
	mockAdapter := &mockStorageAdapter{}
	mockRepo := &mockVideoRepository{}
	adapters := map[string]adapters.StorageAdapter{"minio": mockAdapter}

	svc := NewService(mock, adapters, mockRepo, &mockUploadStatePatcher{})

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
		name          string
		provider      string
		adapters      map[string]adapters.StorageAdapter
		mockAdapter   *mockStorageAdapter
		mockRepo      *mockVideoRepository
		mockPublisher *mockPublisher
		wantErr       bool
		errMsg        string
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

func TestProcessWebhookForwardsPersistedRawVideo(t *testing.T) {
	pub := &mockPublisher{}
	raw := &adapters.RawVideoParams{Width: 1920, Height: 1080, FPS: 30, PixelFormat: "yuv420p"}
	repo := &mockVideoRepository{findResult: &videos.Video{VideoID: "v1", RawVideo: raw}}
	svc := &Service{
		publisher: pub,
		adapters: map[string]adapters.StorageAdapter{
			"minio": &mockStorageAdapter{parseEventResult: &adapters.DomainEvent{
				EventType: "upload.completed", VideoID: "v1", Filename: "source.yuv",
			}},
		},
		repo: repo,
	}

	if err := svc.ProcessWebhook("minio", []byte(`{}`)); err != nil {
		t.Fatalf("ProcessWebhook() error = %v", err)
	}

	// The published event must carry the geometry the transcoder needs.
	if len(pub.payloads) != 1 {
		t.Fatalf("published %d events, want 1", len(pub.payloads))
	}
	event, ok := pub.payloads[0].(*adapters.DomainEvent)
	if !ok || event.RawVideo == nil || event.RawVideo.Width != 1920 || event.RawVideo.FPS != 30 {
		t.Fatalf("published rawVideo = %+v", pub.payloads[0])
	}

	// And the saved record must preserve it (Save would otherwise overwrite).
	if len(repo.savedVideos) != 1 || repo.savedVideos[0].RawVideo == nil {
		t.Fatalf("saved rawVideo not preserved: %+v", repo.savedVideos)
	}
}

func TestProcessWebhookForwardsPersistedTranscode(t *testing.T) {
	pub := &mockPublisher{}
	tr := &adapters.TranscodeRequest{
		Codecs:     []string{"h264"},
		Renditions: []adapters.RequestedRendition{{Width: 1280, Height: 720, Codec: "h264"}},
	}
	repo := &mockVideoRepository{findResult: &videos.Video{VideoID: "v2", Transcode: tr}}
	svc := &Service{
		publisher: pub,
		adapters: map[string]adapters.StorageAdapter{
			"minio": &mockStorageAdapter{parseEventResult: &adapters.DomainEvent{
				EventType: "upload.completed", VideoID: "v2", Filename: "video.mp4",
			}},
		},
		repo: repo,
	}

	if err := svc.ProcessWebhook("minio", []byte(`{}`)); err != nil {
		t.Fatalf("ProcessWebhook() error = %v", err)
	}

	// The published event must carry the transcode selection.
	if len(pub.payloads) != 1 {
		t.Fatalf("published %d events, want 1", len(pub.payloads))
	}
	event, ok := pub.payloads[0].(*adapters.DomainEvent)
	if !ok || event.Transcode == nil || len(event.Transcode.Renditions) != 1 || event.Transcode.Renditions[0].Height != 720 {
		t.Fatalf("published transcode = %+v", pub.payloads[0])
	}

	// And the saved record must preserve it.
	if len(repo.savedVideos) != 1 || repo.savedVideos[0].Transcode == nil {
		t.Fatalf("saved transcode not preserved: %+v", repo.savedVideos)
	}
}
