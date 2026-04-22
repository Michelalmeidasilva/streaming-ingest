package adapters

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type MinioAdapter struct{}

func NewMinioAdapter() *MinioAdapter {
	return &MinioAdapter{}
}

// MinioEvent represents the structure sent by MinIO webhooks
type MinioEvent struct {
	EventName string `json:"EventName"`
	Key       string `json:"Key"`
	Records   []struct {
		EventTime time.Time `json:"eventTime"`
		S3        struct {
			Object struct {
				Key  string `json:"key"`
				Size int64  `json:"size"`
			} `json:"object"`
		} `json:"s3"`
	} `json:"Records"`
}

func (a *MinioAdapter) ParseEvent(payload []byte) (*DomainEvent, error) {
	var event MinioEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("failed to parse minio block: %w", err)
	}

	if len(event.Records) == 0 {
		return nil, fmt.Errorf("no records found in minio event")
	}

	record := event.Records[0]
	keyParts := strings.Split(record.S3.Object.Key, "/")
	var videoID, filename string

	if len(keyParts) >= 2 {
		videoID = keyParts[len(keyParts)-2]
		filename = keyParts[len(keyParts)-1]
	} else {
		filename = record.S3.Object.Key
		videoID = "unknown"
	}

	return &DomainEvent{
		EventType:  "upload.completed",
		VideoID:    videoID,
		Filename:   filename,
		Size:       record.S3.Object.Size,
		Provider:   "minio",
		OccurredAt: record.EventTime,
	}, nil
}
