//go:build integration

package rabbitmq

import (
	"testing"
)

// Integration tests for Publisher with real RabbitMQ.
// These tests require a live RabbitMQ broker.
// Run with: go test -tags integration ./internal/rabbitmq/...
//
// To run RabbitMQ locally:
//   docker run -d --name rabbitmq -p 5672:5672 rabbitmq:3.12-alpine
//
// Or use docker-compose:
//   cd ../ && docker-compose up rabbitmq

const defaultRabbitMQURL = "amqp://guest:guest@localhost:5672/"

func TestPublisherNewWithRealBroker(t *testing.T) {
	pub, err := NewPublisher(defaultRabbitMQURL)
	if err != nil {
		t.Skipf("Skipping: RabbitMQ not available at %s: %v", defaultRabbitMQURL, err)
	}

	if pub == nil {
		t.Errorf("NewPublisher() returned nil, want *Publisher")
	}

	if pub.channel == nil {
		t.Errorf("NewPublisher() channel is nil, want non-nil")
	}

	if pub.conn == nil {
		t.Errorf("NewPublisher() conn is nil, want non-nil")
	}

	pub.Close()
}

func TestPublisherPublishSuccess(t *testing.T) {
	pub, err := NewPublisher(defaultRabbitMQURL)
	if err != nil {
		t.Skipf("Skipping: RabbitMQ not available at %s: %v", defaultRabbitMQURL, err)
	}
	defer pub.Close()

	payload := map[string]string{
		"video_id": "test-123",
		"status":   "uploaded",
	}

	err = pub.Publish("video.upload.completed", payload)
	if err != nil {
		t.Errorf("Publish() error = %v, want nil", err)
	}
}

func TestPublisherPublishEmptyRoutingKey(t *testing.T) {
	pub, err := NewPublisher(defaultRabbitMQURL)
	if err != nil {
		t.Skipf("Skipping: RabbitMQ not available at %s: %v", defaultRabbitMQURL, err)
	}
	defer pub.Close()

	err = pub.Publish("", map[string]string{"test": "data"})
	if err == nil {
		t.Errorf("Publish() with empty routing key expected error, got nil")
	}

	if !contains(err.Error(), "routing key cannot be empty") {
		t.Errorf("Publish() error = %v, want to contain 'routing key cannot be empty'", err.Error())
	}
}

func TestPublisherCloseWithNonNilFields(t *testing.T) {
	pub, err := NewPublisher(defaultRabbitMQURL)
	if err != nil {
		t.Skipf("Skipping: RabbitMQ not available at %s: %v", defaultRabbitMQURL, err)
	}

	if pub.channel == nil || pub.conn == nil {
		t.Fatalf("Publisher fields are nil, want non-nil")
	}

	pub.Close()
}

func TestPublisherCloseIdempotent(t *testing.T) {
	pub, err := NewPublisher(defaultRabbitMQURL)
	if err != nil {
		t.Skipf("Skipping: RabbitMQ not available at %s: %v", defaultRabbitMQURL, err)
	}

	pub.Close()
	pub.Close()
}

func TestPublisherPublishMultiple(t *testing.T) {
	pub, err := NewPublisher(defaultRabbitMQURL)
	if err != nil {
		t.Skipf("Skipping: RabbitMQ not available at %s: %v", defaultRabbitMQURL, err)
	}
	defer pub.Close()

	tests := []struct {
		routingKey string
		payload    interface{}
	}{
		{"video.upload.started", map[string]string{"event": "started"}},
		{"video.upload.progress", map[string]int{"percent": 50}},
		{"video.upload.completed", map[string]string{"status": "done"}},
	}

	for _, tt := range tests {
		err = pub.Publish(tt.routingKey, tt.payload)
		if err != nil {
			t.Errorf("Publish(%q) error = %v, want nil", tt.routingKey, err)
		}
	}
}

func TestPublisherPublishWithDifferentPayloads(t *testing.T) {
	pub, err := NewPublisher(defaultRabbitMQURL)
	if err != nil {
		t.Skipf("Skipping: RabbitMQ not available at %s: %v", defaultRabbitMQURL, err)
	}
	defer pub.Close()

	tests := []struct {
		name       string
		routingKey string
		payload    interface{}
	}{
		{
			name:       "string_payload",
			routingKey: "video.test.string",
			payload:    "test string",
		},
		{
			name:       "int_payload",
			routingKey: "video.test.int",
			payload:    123,
		},
		{
			name:       "bool_payload",
			routingKey: "video.test.bool",
			payload:    true,
		},
		{
			name:       "map_payload",
			routingKey: "video.test.map",
			payload:    map[string]interface{}{"key": "value", "number": 42},
		},
		{
			name:       "slice_payload",
			routingKey: "video.test.slice",
			payload:    []string{"item1", "item2", "item3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err = pub.Publish(tt.routingKey, tt.payload)
			if err != nil {
				t.Errorf("Publish(%q) with %T error = %v, want nil", tt.routingKey, tt.payload, err)
			}
		})
	}
}
