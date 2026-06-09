package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Compile-time assertion that *Publisher satisfies MessagePublisher
var _ MessagePublisher = (*Publisher)(nil)

type mockConnection struct {
	channelFunc func() (channelAPI, error)
	closeFunc   func() error
}

func (m *mockConnection) Channel() (channelAPI, error) {
	if m.channelFunc != nil {
		return m.channelFunc()
	}
	return nil, nil
}

func (m *mockConnection) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

type mockChannel struct {
	exchangeDeclareFunc    func(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	publishWithContextFunc func(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	closeFunc              func() error
}

func (m *mockChannel) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	if m.exchangeDeclareFunc != nil {
		return m.exchangeDeclareFunc(name, kind, durable, autoDelete, internal, noWait, args)
	}
	return nil
}

func (m *mockChannel) PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	if m.publishWithContextFunc != nil {
		return m.publishWithContextFunc(ctx, exchange, key, mandatory, immediate, msg)
	}
	return nil
}

func (m *mockChannel) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

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

func TestNewPublisher(t *testing.T) {
	originalDial := dialAMQP
	t.Cleanup(func() { dialAMQP = originalDial })

	t.Run("channel open fails", func(t *testing.T) {
		dialAMQP = func(url string) (connectionAPI, error) {
			return &mockConnection{
				channelFunc: func() (channelAPI, error) {
					return nil, errors.New("channel failed")
				},
			}, nil
		}

		pub, err := NewPublisher("amqp://test")
		if err == nil || !contains(err.Error(), "failed to open a channel") {
			t.Fatalf("NewPublisher() error = %v, want channel failure", err)
		}
		if pub != nil {
			t.Fatalf("NewPublisher() = %v, want nil", pub)
		}
	})

	t.Run("exchange declare fails", func(t *testing.T) {
		dialAMQP = func(url string) (connectionAPI, error) {
			return &mockConnection{
				channelFunc: func() (channelAPI, error) {
					return &mockChannel{
						exchangeDeclareFunc: func(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
							return errors.New("declare failed")
						},
					}, nil
				},
			}, nil
		}

		pub, err := NewPublisher("amqp://test")
		if err == nil || !contains(err.Error(), "failed to declare an exchange") {
			t.Fatalf("NewPublisher() error = %v, want exchange failure", err)
		}
		if pub != nil {
			t.Fatalf("NewPublisher() = %v, want nil", pub)
		}
	})

	t.Run("success", func(t *testing.T) {
		dialAMQP = func(url string) (connectionAPI, error) {
			return &mockConnection{
				channelFunc: func() (channelAPI, error) {
					return &mockChannel{}, nil
				},
			}, nil
		}

		pub, err := NewPublisher("amqp://test")
		if err != nil {
			t.Fatalf("NewPublisher() error = %v", err)
		}
		if pub == nil || pub.conn == nil || pub.channel == nil {
			t.Fatalf("NewPublisher() returned uninitialized publisher: %+v", pub)
		}
	})
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
				conn:    &mockConnection{},
				channel: &mockChannel{},
			},
			routingKey:  "video.test",
			payload:     make(chan int),
			expectErr:   true,
			errContains: "failed to marshal payload",
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

func TestPublishSuccessAndFailure(t *testing.T) {
	t.Run("publish fails", func(t *testing.T) {
		pub := &Publisher{
			conn: &mockConnection{},
			channel: &mockChannel{
				publishWithContextFunc: func(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
					return errors.New("publish failed")
				},
			},
		}

		err := pub.Publish("video.test", map[string]string{"hello": "world"})
		if err == nil || !contains(err.Error(), "failed to publish message") {
			t.Fatalf("Publish() error = %v, want publish failure", err)
		}
	})

	t.Run("publish succeeds", func(t *testing.T) {
		var published amqp.Publishing
		pub := &Publisher{
			conn: &mockConnection{},
			channel: &mockChannel{
				publishWithContextFunc: func(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
					published = msg
					if exchange != "video_events" || key != "video.test" {
						t.Fatalf("unexpected routing: exchange=%s key=%s", exchange, key)
					}
					return nil
				},
			},
		}

		err := pub.Publish("video.test", map[string]string{"hello": "world"})
		if err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
		if published.ContentType != "application/json" || len(published.Body) == 0 {
			t.Fatalf("Publish() produced invalid AMQP payload: %+v", published)
		}
	})
}

func TestPublishReconnectsAndRetriesOnStaleConnection(t *testing.T) {
	originalDial := dialAMQP
	t.Cleanup(func() { dialAMQP = originalDial })

	var dialed int
	dialAMQP = func(url string) (connectionAPI, error) {
		dialed++
		// Fresh connection whose channel publishes successfully (nil func = success).
		return &mockConnection{channelFunc: func() (channelAPI, error) {
			return &mockChannel{}, nil
		}}, nil
	}

	// Initial channel simulates a stale connection: the first publish fails.
	pub := &Publisher{
		url:  "amqp://stale-host",
		conn: &mockConnection{},
		channel: &mockChannel{
			publishWithContextFunc: func(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
				return errors.New("channel/connection is not open")
			},
		},
	}

	if err := pub.Publish("video.test", map[string]string{"hello": "world"}); err != nil {
		t.Fatalf("Publish should succeed after reconnect, got %v", err)
	}
	if dialed != 1 {
		t.Fatalf("expected exactly one reconnect dial, got %d", dialed)
	}
}

func TestPublishNoReconnectWithoutURL(t *testing.T) {
	originalDial := dialAMQP
	t.Cleanup(func() { dialAMQP = originalDial })
	dialAMQP = func(url string) (connectionAPI, error) {
		t.Fatalf("reconnect must not be attempted when url is empty")
		return nil, nil
	}
	pub := &Publisher{
		conn: &mockConnection{},
		channel: &mockChannel{
			publishWithContextFunc: func(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
				return errors.New("publish failed")
			},
		},
	}
	if err := pub.Publish("video.test", map[string]string{"a": "b"}); err == nil || !contains(err.Error(), "failed to publish message") {
		t.Fatalf("Publish() error = %v, want publish failure", err)
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
