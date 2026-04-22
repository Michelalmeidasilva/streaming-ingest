package events

import (
	"fmt"
	"streaming-ingest/internal/rabbitmq"
)

type EventPayload map[string]interface{}

type FrontEndEvent struct {
	EventType string       `json:"eventType"`
	Payload   EventPayload `json:"payload"`
}

type Service struct {
	publisher *rabbitmq.Publisher
}

func NewService(pub *rabbitmq.Publisher) *Service {
	return &Service{
		publisher: pub,
	}
}

func (s *Service) ProcessEvent(event FrontEndEvent) error {
	routingKey := fmt.Sprintf("video.%s", event.EventType) // ex: video.upload.progress
	err := s.publisher.Publish(routingKey, event.Payload)
	if err != nil {
		return fmt.Errorf("failed to process and publish event: %w", err)
	}
	
	return nil
}
