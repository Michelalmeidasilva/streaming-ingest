package webhooks

import (
	"fmt"
	"streaming-ingest/internal/adapters"
	"streaming-ingest/internal/rabbitmq"
)

type Service struct {
	publisher *rabbitmq.Publisher
	adapters  map[string]adapters.StorageAdapter
}

func NewService(pub *rabbitmq.Publisher) *Service {
	return &Service{
		publisher: pub,
		adapters: map[string]adapters.StorageAdapter{
			"minio":  adapters.NewMinioAdapter(),
			"aws-s3": adapters.NewS3Adapter(),
		},
	}
}

func (s *Service) ProcessWebhook(provider string, payload []byte) error {
	adapter, exists := s.adapters[provider]
	if !exists {
		return fmt.Errorf("unsupported provider: %s", provider)
	}

	domainEvent, err := adapter.ParseEvent(payload)
	if err != nil {
		return fmt.Errorf("failed to parse event for provider %s: %w", provider, err)
	}

	routingKey := fmt.Sprintf("video.%s", domainEvent.EventType)
	err = s.publisher.Publish(routingKey, domainEvent)
	if err != nil {
		return fmt.Errorf("failed to publish domain event: %w", err)
	}

	return nil
}
