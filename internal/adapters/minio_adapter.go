package adapters

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioAdapter struct {
	client minioClientIface
}

func NewMinioAdapter() *MinioAdapter {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:9000"
	}
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

	accessKey := os.Getenv("MINIO_ROOT_USER")
	secretKey := os.Getenv("MINIO_ROOT_PASSWORD")

	if accessKey == "" || secretKey == "" {
		log.Println("WARNING: MinIO credentials not configured via environment variables")
		return &MinioAdapter{client: nil}
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		log.Println("ERROR: Failed to initialize MinIO client")
		return &MinioAdapter{client: nil}
	}

	return &MinioAdapter{client: client}
}

func (a *MinioAdapter) ParseEvent(payload []byte) (*DomainEvent, error) {
	var event storageEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("failed to parse minio block: %w", err)
	}

	if len(event.Records) == 0 {
		return nil, fmt.Errorf("no records found in minio event")
	}

	record := event.Records[0]
	if record.S3.Object.Key == "" {
		return nil, fmt.Errorf("minio object key is required")
	}

	videoID, filename := parseStorageKey(record.S3.Object.Key)

	return newStorageDomainEvent(videoID, filename, record.S3.Object.Size, "minio", record.EventTime), nil
}

func (a *MinioAdapter) ListVideos(bucket string) ([]DomainEvent, error) {
	if a.client == nil {
		return nil, fmt.Errorf("minio client not initialized")
	}
	return listStorageVideos(a.client, bucket, "minio")
}

func (a *MinioAdapter) GenerateURL(bucket, key string) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("minio client not initialized")
	}
	return generatePresignedURL(a.client, bucket, key)
}

