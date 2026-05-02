package adapters

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type S3Adapter struct{}

func NewS3Adapter() *S3Adapter {
	return &S3Adapter{}
}

type S3Event struct {
	Records []S3EventRecord `json:"Records"`
}

type S3EventRecord struct {
	EventName string    `json:"eventName"`
	EventTime time.Time `json:"eventTime"`
	S3        S3EventS3 `json:"s3"`
}

type S3EventS3 struct {
	Object S3EventObject `json:"object"`
}

type S3EventObject struct {
	Key  string `json:"key"`
	Size int64  `json:"size"`
}

func (a *S3Adapter) ParseEvent(payload []byte) (*DomainEvent, error) {
	var event S3Event
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("failed to parse s3 block: %w", err)
	}

	if len(event.Records) == 0 {
		return nil, fmt.Errorf("no records found in s3 event")
	}

	record := event.Records[0]
	if record.S3.Object.Key == "" {
		return nil, fmt.Errorf("s3 object key is required")
	}
	if !strings.HasPrefix(record.EventName, "ObjectCreated") {
		return nil, fmt.Errorf("ignoring non-creation event: %s", record.EventName)
	}

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
		Provider:   "aws-s3",
		OccurredAt: record.EventTime,
	}, nil
}

func (a *S3Adapter) ListVideos(bucket string) ([]DomainEvent, error) {
	return make([]DomainEvent, 0), nil
}

func (a *S3Adapter) GenerateURL(bucket, key string) (string, error) {
	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucket, key), nil
}
