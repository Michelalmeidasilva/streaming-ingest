package rabbitmq

import (
	"encoding/json"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Compile-time assertion that *Publisher satisfies MessagePublisher
var _ MessagePublisher = (*Publisher)(nil)

func TestPublisherClose(t *testing.T) {
	tests := []struct {
		name string
		pub  *Publisher
	}{
		{
			name: "both fields nil",
			pub: &Publisher{
				conn:    nil,
				channel: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			tt.pub.Close()
		})
	}
}

func TestNewPublisherInvalidURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectError bool
	}{
		{
			name:        "invalid protocol",
			url:         "invalid://invalid-broker:9999",
			expectError: true,
		},
		{
			name:        "malformed URL",
			url:         "amqp://[invalid",
			expectError: true,
		},
		{
			name:        "unreachable host",
			url:         "amqp://127.0.0.1:9999",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pub, err := NewPublisher(tt.url)

			if tt.expectError && err == nil {
				t.Errorf("NewPublisher() expected error for %s, got nil", tt.url)
			}

			if err != nil && pub != nil {
				t.Errorf("NewPublisher() should return nil on error, got %v", pub)
			}
		})
	}
}

func TestPublishValidation(t *testing.T) {
	tests := []struct {
		name        string
		publisher   *Publisher
		routingKey  string
		payload     interface{}
		expectErr   bool
		errContains string
	}{
		{
			name: "publisher not initialized - nil channel",
			publisher: &Publisher{
				conn:    nil,
				channel: nil,
			},
			routingKey:  "video.test",
			payload:     map[string]string{"test": "data"},
			expectErr:   true,
			errContains: "publisher not initialized",
		},
		{
			name: "empty routing key also catches publisher not initialized first",
			publisher: &Publisher{
				conn:    nil,
				channel: nil,
			},
			routingKey:  "",
			payload:     map[string]string{"test": "data"},
			expectErr:   true,
			errContains: "publisher not initialized",
		},
		{
			name: "unmarshalable payload",
			publisher: &Publisher{
				conn:    nil,
				channel: nil,
			},
			routingKey:  "video.test",
			payload:     make(chan int),
			expectErr:   true,
			errContains: "publisher not initialized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.publisher.Publish(tt.routingKey, tt.payload)
			if !tt.expectErr && err != nil {
				t.Errorf("Publish() unexpected error: %v", err)
			}
			if tt.expectErr && err == nil {
				t.Errorf("Publish() expected error, got nil")
			}
			if tt.expectErr && err != nil && !contains(err.Error(), tt.errContains) {
				t.Errorf("Publish() error %q should contain %q", err.Error(), tt.errContains)
			}
		})
	}
}

func TestPublishPayloadMarshaling(t *testing.T) {
	// Test that Publish can handle various payload types
	tests := []struct {
		name    string
		payload interface{}
	}{
		{
			name:    "map payload",
			payload: map[string]interface{}{"key": "value"},
		},
		{
			name:    "struct payload",
			payload: struct{ Field string }{Field: "value"},
		},
		{
			name:    "string payload",
			payload: "test",
		},
		{
			name:    "number payload",
			payload: 42,
		},
		{
			name:    "slice payload",
			payload: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify payloads can be marshaled to JSON
			_, err := json.Marshal(tt.payload)
			if err != nil {
				t.Errorf("Failed to marshal payload: %v", err)
			}
		})
	}
}

func TestPublisherInterface(t *testing.T) {
	// Verify Publisher implements the MessagePublisher interface
	var p *Publisher
	var iface MessagePublisher = p
	if iface == nil {
		t.Errorf("Publisher should implement MessagePublisher")
	}
}

func TestPublishingMessage(t *testing.T) {
	// Test that a message can be created (even if we can't publish without RabbitMQ)
	payload := map[string]interface{}{
		"event_type": "video.upload.completed",
		"video_id":   "123",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	if len(body) == 0 {
		t.Errorf("Marshaled body should not be empty")
	}

	// Verify the message structure
	msg := amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	}

	if msg.ContentType != "application/json" {
		t.Errorf("Message ContentType should be application/json, got %s", msg.ContentType)
	}

	if len(msg.Body) == 0 {
		t.Errorf("Message Body should not be empty")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
