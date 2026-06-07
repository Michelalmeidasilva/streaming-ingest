package adapters

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Adapter struct {
	client minioClientIface
}

func NewS3Adapter() *S3Adapter {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if accessKey == "" || secretKey == "" {
		return &S3Adapter{client: nil}
	}

	client, err := minio.New(fmt.Sprintf("s3.%s.amazonaws.com", region), &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Region: region,
		Secure: true,
	})
	if err != nil {
		return &S3Adapter{client: nil}
	}

	return &S3Adapter{client: client}
}

func (a *S3Adapter) ParseEvent(payload []byte) (*DomainEvent, error) {
	var event storageEvent
	if err := json.Unmarshal(payload, &event); err == nil && len(event.Records) > 0 {
		record := event.Records[0]
		if record.S3.Object.Key == "" {
			return nil, fmt.Errorf("s3 object key is required")
		}
		if !strings.HasPrefix(record.EventName, "ObjectCreated") {
			return nil, fmt.Errorf("ignoring non-creation event: %s", record.EventName)
		}
		if isPipelineOutputKey(record.S3.Object.Key) {
			return nil, ErrIgnoredObjectKey
		}
		videoID, filename := parseStorageKey(record.S3.Object.Key)
		if videoID == filename {
			videoID = "unknown"
		}
		return newStorageDomainEvent(videoID, filename, record.S3.Object.Key, record.S3.Object.Size, "aws-s3", record.EventTime), nil
	}

	// Fall back to the EventBridge "Object Created" envelope (S3 → EventBridge → ingest).
	var eb eventBridgeEvent
	if err := json.Unmarshal(payload, &eb); err != nil {
		return nil, fmt.Errorf("failed to parse s3 block: %w", err)
	}
	if eb.DetailType != "Object Created" {
		return nil, fmt.Errorf("no records found in s3 event")
	}
	if eb.Detail.Object.Key == "" {
		return nil, fmt.Errorf("s3 object key is required")
	}
	if isPipelineOutputKey(eb.Detail.Object.Key) {
		return nil, ErrIgnoredObjectKey
	}
	videoID, filename := parseStorageKey(eb.Detail.Object.Key)
	if videoID == filename {
		videoID = "unknown"
	}
	return newStorageDomainEvent(videoID, filename, eb.Detail.Object.Key, eb.Detail.Object.Size, "aws-s3", eb.Time), nil
}

func (a *S3Adapter) ListVideos(bucket string) ([]DomainEvent, error) {
	if a.client == nil {
		return nil, fmt.Errorf("s3 client not initialized")
	}
	return listStorageVideos(a.client, bucket, "aws-s3")
}

func (a *S3Adapter) GenerateURL(bucket, key string) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("s3 client not initialized")
	}
	return generatePresignedURL(a.client, bucket, key)
}
