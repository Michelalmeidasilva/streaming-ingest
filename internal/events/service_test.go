package events

import (
	"errors"
	"testing"
)

type mockPublisher struct {
	publishErr error
	calledWith []string
}

func (m *mockPublisher) Publish(routingKey string, payload interface{}) error {
	m.calledWith = append(m.calledWith, routingKey)
	return m.publishErr
}

func TestNewService(t *testing.T) {
	mock := &mockPublisher{}
	svc := NewService(mock)

	if svc == nil {
		t.Errorf("NewService() returned nil")
	}

	if svc.publisher != mock {
		t.Errorf("NewService() did not set publisher correctly")
	}
}

func TestProcessEvent(t *testing.T) {
	tests := []struct {
		name      string
		event     FrontEndEvent
		publishErr error
		wantErr   bool
		wantKey   string
	}{
		{
			name: "publish_fails",
			event: FrontEndEvent{
				EventType: "upload.progress",
				Payload:   EventPayload{"data": "test"},
			},
			publishErr: errors.New("amqp down"),
			wantErr:    true,
			wantKey:    "video.upload.progress",
		},
		{
			name: "success",
			event: FrontEndEvent{
				EventType: "upload.progress",
				Payload:   EventPayload{"data": "test"},
			},
			publishErr: nil,
			wantErr:    false,
			wantKey:    "video.upload.progress",
		},
		{
			name: "different_event_type",
			event: FrontEndEvent{
				EventType: "upload.completed",
				Payload:   EventPayload{"videoID": "123"},
			},
			publishErr: nil,
			wantErr:    false,
			wantKey:    "video.upload.completed",
		},
		{
			name: "event_type_with_dots",
			event: FrontEndEvent{
				EventType: "transcode.started",
				Payload:   EventPayload{"rendition": "1080p"},
			},
			publishErr: nil,
			wantErr:    false,
			wantKey:    "video.transcode.started",
		},
		{
			name: "empty_payload",
			event: FrontEndEvent{
				EventType: "ping",
				Payload:   EventPayload{},
			},
			publishErr: nil,
			wantErr:    false,
			wantKey:    "video.ping",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPublisher{publishErr: tt.publishErr}
			svc := &Service{publisher: mock}

			err := svc.ProcessEvent(tt.event)

			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessEvent() error = %v, wantErr %v", err, tt.wantErr)
			}

			if len(mock.calledWith) > 0 && mock.calledWith[0] != tt.wantKey {
				t.Errorf("ProcessEvent() routingKey = %v, want %v", mock.calledWith[0], tt.wantKey)
			}

			if tt.wantErr && err != nil && !contains(err.Error(), "failed to process and publish event") {
				t.Errorf("ProcessEvent() error = %v, want to contain 'failed to process and publish event'", err.Error())
			}
		})
	}
}
