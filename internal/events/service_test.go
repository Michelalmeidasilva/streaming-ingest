package events

import (
	"context"
	"errors"
	"testing"

	"streaming-ingest/internal/videos"
)

type mockPublisher struct {
	publishErr error
	calledWith []string
}

type mockEventRepository struct {
	saveErr error
	saved   []*EventRecord
}

type mockVideoRepository struct {
	createErr error
	created   []*videos.Video
}

func (m *mockPublisher) Publish(routingKey string, payload interface{}) error {
	m.calledWith = append(m.calledWith, routingKey)
	return m.publishErr
}

func TestNewService(t *testing.T) {
	mock := &mockPublisher{}
	mockRepo := &mockEventRepository{}
	mockVideoRepo := &mockVideoRepository{}
	svc := NewService(mock, mockRepo, mockVideoRepo)

	if svc == nil {
		t.Errorf("NewService() returned nil")
	}

	if svc.publisher != mock {
		t.Errorf("NewService() did not set publisher correctly")
	}

	if svc.repo != mockRepo {
		t.Errorf("NewService() did not set repo correctly")
	}

	if svc.videoRepo != mockVideoRepo {
		t.Errorf("NewService() did not set video repo correctly")
	}
}

func TestProcessEvent(t *testing.T) {
	tests := []struct {
		name        string
		event       FrontEndEvent
		publishErr  error
		saveErr     error
		wantErr     bool
		wantKey     string
		wantSaveLen int
		wantCreate  int
	}{
		{
			name: "save_fails",
			event: FrontEndEvent{
				EventType: "upload.progress",
				Payload:   EventPayload{"videoId": "vid-1", "filename": "video.mp4"},
			},
			saveErr:     errors.New("db down"),
			wantErr:     true,
			wantKey:     "",
			wantSaveLen: 1,
			wantCreate:  0,
		},
		{
			name: "publish_fails",
			event: FrontEndEvent{
				EventType: "upload.progress",
				Payload:   EventPayload{"videoId": "vid-1", "filename": "video.mp4"},
			},
			publishErr:  errors.New("amqp down"),
			wantErr:     true,
			wantKey:     "video.upload.progress",
			wantSaveLen: 1,
			wantCreate:  0,
		},
		{
			name: "success",
			event: FrontEndEvent{
				EventType: "upload.progress",
				Payload:   EventPayload{"videoId": "vid-1", "filename": "video.mp4", "size": 123},
			},
			publishErr:  nil,
			wantErr:     false,
			wantKey:     "video.upload.progress",
			wantSaveLen: 1,
			wantCreate:  0,
		},
		{
			name: "different_event_type",
			event: FrontEndEvent{
				EventType: "upload.completed",
				Payload:   EventPayload{"videoID": "123", "filename": "done.mp4"},
			},
			publishErr:  nil,
			wantErr:     false,
			wantKey:     "video.upload.completed",
			wantSaveLen: 1,
			wantCreate:  0,
		},
		{
			name: "event_type_with_dots",
			event: FrontEndEvent{
				EventType: "transcode.started",
				Payload:   EventPayload{"rendition": "1080p"},
			},
			publishErr:  nil,
			wantErr:     false,
			wantKey:     "video.transcode.started",
			wantSaveLen: 1,
			wantCreate:  0,
		},
		{
			name: "empty_payload",
			event: FrontEndEvent{
				EventType: "ping",
				Payload:   EventPayload{},
			},
			publishErr:  nil,
			wantErr:     false,
			wantKey:     "video.ping",
			wantSaveLen: 1,
			wantCreate:  0,
		},
		{
			name: "upload_started_creates_video_record",
			event: FrontEndEvent{
				EventType: "upload.started",
				Payload:   EventPayload{"videoId": "vid-1", "filename": "video.mp4", "totalBytes": 1234},
			},
			publishErr:  nil,
			wantErr:     false,
			wantKey:     "video.upload.started",
			wantSaveLen: 1,
			wantCreate:  1,
		},
		{
			name: "upload_started_missing_videoId",
			event: FrontEndEvent{
				EventType: "upload.started",
				Payload:   EventPayload{"filename": "video.mp4"},
			},
			wantErr:     true,
			wantSaveLen: 1,
			wantCreate:  0,
		},
		{
			name: "upload_started_missing_filename",
			event: FrontEndEvent{
				EventType: "upload.started",
				Payload:   EventPayload{"videoId": "vid-1"},
			},
			wantErr:     true,
			wantSaveLen: 1,
			wantCreate:  0,
		},
		{
			name: "upload_started_create_fails",
			event: FrontEndEvent{
				EventType: "upload.started",
				Payload:   EventPayload{"videoId": "vid-1", "filename": "video.mp4"},
			},
			wantErr:     true,
			wantSaveLen: 1,
			wantCreate:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPublisher{publishErr: tt.publishErr}
			mockRepo := &mockEventRepository{saveErr: tt.saveErr}
			mockVideoRepo := &mockVideoRepository{}
			if tt.name == "upload_started_creates_video_record" {
				mockVideoRepo = &mockVideoRepository{}
			}
			svc := &Service{publisher: mock, repo: mockRepo, videoRepo: mockVideoRepo}
			if tt.name == "upload_started_create_fails" {
				mockVideoRepo.createErr = errors.New("db error")
			}

			err := svc.ProcessEvent(tt.event)

			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessEvent() error = %v, wantErr %v", err, tt.wantErr)
			}

			if len(mock.calledWith) > 0 && mock.calledWith[0] != tt.wantKey {
				t.Errorf("ProcessEvent() routingKey = %v, want %v", mock.calledWith[0], tt.wantKey)
			}

			if len(mockRepo.saved) != tt.wantSaveLen {
				t.Errorf("ProcessEvent() saved len = %d, want %d", len(mockRepo.saved), tt.wantSaveLen)
			}

			if len(mockVideoRepo.created) != tt.wantCreate {
				t.Errorf("ProcessEvent() created len = %d, want %d", len(mockVideoRepo.created), tt.wantCreate)
			}

			if tt.saveErr != nil && err != nil && !contains(err.Error(), "failed to save event metadata") {
				t.Errorf("ProcessEvent() error = %v, want to contain 'failed to save event metadata'", err.Error())
			}

			if tt.saveErr == nil && tt.publishErr != nil && err != nil && !contains(err.Error(), "failed to process and publish event") {
				t.Errorf("ProcessEvent() error = %v, want to contain 'failed to process and publish event'", err.Error())
			}
		})
	}
}

func TestProcessEventPublishesTranscodeLifecycleRoutingKeys(t *testing.T) {
	events := map[string]string{
		"transcode.queued":    "video.transcode.queued",
		"transcode.started":   "video.transcode.started",
		"transcode.progress":  "video.transcode.progress",
		"packaging.completed": "video.packaging.completed",
		"transcode.completed": "video.transcode.completed",
		"transcode.failed":    "video.transcode.failed",
		"ready":               "video.ready",
	}

	for eventType, wantKey := range events {
		t.Run(eventType, func(t *testing.T) {
			publisher := &mockPublisher{}
			repo := &mockEventRepository{}
			svc := NewService(publisher, repo, nil)

			err := svc.ProcessEvent(FrontEndEvent{
				EventType: eventType,
				Payload: EventPayload{
					"videoId": "video-1",
					"jobId":   "job-1",
				},
			})
			if err != nil {
				t.Fatalf("ProcessEvent() error = %v", err)
			}
			if len(publisher.calledWith) != 1 || publisher.calledWith[0] != wantKey {
				t.Fatalf("routing keys = %v, want %s", publisher.calledWith, wantKey)
			}
			if len(repo.saved) != 1 || repo.saved[0].RoutingKey != wantKey {
				t.Fatalf("saved record = %+v, want routing key %s", repo.saved, wantKey)
			}
		})
	}
}

func (m *mockEventRepository) Save(ctx context.Context, event *EventRecord) error {
	if event != nil {
		m.saved = append(m.saved, event)
	}
	return m.saveErr
}

func (m *mockVideoRepository) Create(ctx context.Context, video *videos.Video) error {
	if video != nil {
		m.created = append(m.created, video)
	}
	return m.createErr
}

func (m *mockVideoRepository) Save(ctx context.Context, video *videos.Video) error {
	return nil
}

func (m *mockVideoRepository) ListAll(ctx context.Context) ([]videos.Video, error) {
	return nil, nil
}

func (m *mockVideoRepository) Search(ctx context.Context, query string) ([]videos.Video, error) {
	return []videos.Video{}, nil
}

func (m *mockVideoRepository) FindByVideoID(ctx context.Context, videoID string) (*videos.Video, error) {
	return nil, nil
}

func TestTranscodeFromPayload(t *testing.T) {
	payload := EventPayload{
		"transcode": map[string]interface{}{
			"codecs": []interface{}{"h264", "av1"},
			"renditions": []interface{}{
				map[string]interface{}{"width": float64(1280), "height": float64(720), "codec": "h264"},
				map[string]interface{}{"width": float64(1280), "height": float64(720), "codec": "av1"},
			},
		},
	}
	got := transcodeFromPayload(payload)
	if got == nil || len(got.Renditions) != 2 || len(got.Codecs) != 2 {
		t.Fatalf("expected 2 renditions and 2 codecs, got %+v", got)
	}
	if got.Renditions[0].Width != 1280 || got.Renditions[0].Height != 720 || got.Renditions[0].Codec != "h264" {
		t.Fatalf("unexpected first rendition: %+v", got.Renditions[0])
	}
}

func TestTranscodeFromPayloadNilWhenAbsent(t *testing.T) {
	if got := transcodeFromPayload(EventPayload{}); got != nil {
		t.Fatalf("expected nil when no transcode key, got %+v", got)
	}
}

type fakeRunWriter struct {
	called      int
	lastPayload EventPayload
}

func (f *fakeRunWriter) UpsertFromEvent(_ context.Context, p map[string]interface{}) error {
	f.called++
	f.lastPayload = p
	return nil
}

func TestProcessEventWritesRunOnTranscodeCompleted(t *testing.T) {
	publisher := &mockPublisher{}
	repo := &mockEventRepository{}
	svc := NewService(publisher, repo, nil)
	writer := &fakeRunWriter{}
	svc.SetRunWriter(writer)

	if err := svc.ProcessEvent(FrontEndEvent{
		EventType: "transcode.completed",
		Payload:   EventPayload{"jobId": "v1-1", "videoId": "v1"},
	}); err != nil {
		t.Fatal(err)
	}
	if writer.called != 1 {
		t.Fatalf("run writer called %d times, want 1", writer.called)
	}
}

func TestProcessEventSkipsRunForOtherEvents(t *testing.T) {
	publisher := &mockPublisher{}
	repo := &mockEventRepository{}
	svc := NewService(publisher, repo, nil)
	writer := &fakeRunWriter{}
	svc.SetRunWriter(writer)
	_ = svc.ProcessEvent(FrontEndEvent{EventType: "upload.progress", Payload: EventPayload{"videoId": "v1"}})
	if writer.called != 0 {
		t.Fatalf("run writer should not be called for upload.progress")
	}
}

func TestFirstInt64(t *testing.T) {
	payload := EventPayload{
		"int":     1,
		"int64":   int64(2),
		"float64": float64(3.0),
		"string":  "4",
	}

	if got := firstInt64(payload, "int"); got != 1 {
		t.Errorf("firstInt64(int) = %v, want 1", got)
	}
	if got := firstInt64(payload, "int64"); got != 2 {
		t.Errorf("firstInt64(int64) = %v, want 2", got)
	}
	if got := firstInt64(payload, "float64"); got != 3 {
		t.Errorf("firstInt64(float64) = %v, want 3", got)
	}
	if got := firstInt64(payload, "string"); got != 0 {
		t.Errorf("firstInt64(string) = %v, want 0", got)
	}
	if got := firstInt64(payload, "missing"); got != 0 {
		t.Errorf("firstInt64(missing) = %v, want 0", got)
	}
}
