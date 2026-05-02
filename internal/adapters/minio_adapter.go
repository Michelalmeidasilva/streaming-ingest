package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type minioClientIface interface {
	ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error)
}

type MinioAdapter struct {
	client minioClientIface
}

type MinioEventRecord struct {
	EventTime time.Time    `json:"eventTime"`
	S3        MinioEventS3 `json:"s3"`
}

type MinioEventS3 struct {
	Object MinioEventObject `json:"object"`
}

type MinioEventObject struct {
	Key  string `json:"key"`
	Size int64  `json:"size"`
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

// MinioEvent represents the structure sent by MinIO webhooks
type MinioEvent struct {
	EventName string             `json:"EventName"`
	Key       string             `json:"Key"`
	Records   []MinioEventRecord `json:"Records"`
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
	if record.S3.Object.Key == "" {
		return nil, fmt.Errorf("minio object key is required")
	}
	// Unescape the key in case it's URL-encoded (common in some MinIO versions/configs)
	rawKey := record.S3.Object.Key
	if decodedKey, err := url.QueryUnescape(rawKey); err == nil {
		rawKey = decodedKey
	}

	keyParts := strings.Split(rawKey, "/")
	var videoID, filename string

	if len(keyParts) >= 2 {
		videoID = keyParts[len(keyParts)-2]
		filename = keyParts[len(keyParts)-1]
	} else {
		filename = rawKey
		// If no slash, use the filename as ID to avoid overwriting "unknown" records
		videoID = rawKey
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

func (a *MinioAdapter) ListVideos(bucket string) ([]DomainEvent, error) {
	if a.client == nil {
		return nil, fmt.Errorf("minio client not initialized")
	}

	var videos []DomainEvent
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	objectCh := a.client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			return nil, object.Err
		}

		// Filter out chunk files
		if strings.Contains(object.Key, ".chunk.") {
			continue
		}

		rawKey := object.Key
		if decodedKey, err := url.QueryUnescape(rawKey); err == nil {
			rawKey = decodedKey
		}

		parts := strings.Split(rawKey, "/")
		var videoID, filename string
		if len(parts) >= 2 {
			videoID = parts[len(parts)-2]
			filename = parts[len(parts)-1]
		} else {
			videoID = rawKey
			filename = rawKey
		}

		videos = append(videos, DomainEvent{
			EventType:  "upload.completed",
			VideoID:    videoID,
			Filename:   filename,
			Size:       object.Size,
			Provider:   "minio",
			OccurredAt: object.LastModified,
		})
	}

	return videos, nil
}

func (a *MinioAdapter) GenerateURL(bucket, key string) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("minio client not initialized")
	}

	// Generate a presigned URL that expires in 1 hour
	expiry := time.Duration(3600) * time.Second
	presignedURL, err := a.client.PresignedGetObject(context.Background(), bucket, key, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned url: %w", err)
	}

	return presignedURL.String(), nil
}
