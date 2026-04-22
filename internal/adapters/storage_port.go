package adapters

import "time"

// DomainEvent is the standardized event structure published to RabbitMQ
type DomainEvent struct {
	EventType  string    `json:"eventType"` // e.g. upload.completed
	VideoID    string    `json:"videoId"`
	Filename   string    `json:"filename"`
	Size       int64     `json:"size"`
	Provider   string    `json:"provider"`
	OccurredAt time.Time `json:"occurredAt"`
}

// StorageAdapter parses provider-specific webhooks into domain events
type StorageAdapter interface {
	ParseEvent(payload []byte) (*DomainEvent, error)
}
