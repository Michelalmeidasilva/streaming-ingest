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

type MinioAdapter struct {
	client *minio.Client
}

func NewMinioAdapter() *MinioAdapter {
	endpoint := os.Getenv("MINIO_ENDPOINT") // e.g. "localhost:9000"
	if endpoint == "" {
		endpoint = "localhost:9000"
	}
	// Remove protocol if present
	endpoint = strings.Replace(endpoint, "http://", "", 1)
	endpoint = strings.Replace(endpoint, "https://", "", 1)

	accessKey := os.Getenv("MINIO_ROOT_USER")
	if accessKey == "" {
		accessKey = "admin"
	}
	secretKey := os.Getenv("MINIO_ROOT_PASSWORD")
	if secretKey == "" {
		secretKey = "password123"
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		log.Printf("Failed to initialize MinIO client: %v", err)
	}

	return &MinioAdapter{client: client}
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

	// Replace internal Docker hostname with public localhost for browser access
	urlStr := presignedURL.String()
	urlStr = strings.Replace(urlStr, "http://minio:9000", "http://localhost:9000", 1)
	urlStr = strings.Replace(urlStr, "https://minio:9000", "http://localhost:9000", 1)

	return urlStr, nil
}
