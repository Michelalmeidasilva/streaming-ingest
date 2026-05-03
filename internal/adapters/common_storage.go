package adapters

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

type minioClientIface interface {
	ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error)
}

type storageEvent struct {
	Records []storageEventRecord `json:"Records"`
}

type storageEventRecord struct {
	EventName string             `json:"eventName"`
	EventTime time.Time          `json:"eventTime"`
	S3        storageEventDetail `json:"s3"`
}

type storageEventDetail struct {
	Object storageEventObject `json:"object"`
}

type storageEventObject struct {
	Key  string `json:"key"`
	Size int64  `json:"size"`
}

func parseStorageKey(key string) (videoID, filename string) {
	rawKey := key
	if decodedKey, err := url.QueryUnescape(rawKey); err == nil {
		rawKey = decodedKey
	}

	parts := strings.Split(rawKey, "/")
	if len(parts) >= 2 {
		videoID = parts[len(parts)-2]
		filename = parts[len(parts)-1]
	} else {
		videoID = rawKey
		filename = rawKey
	}
	return videoID, filename
}

func newStorageDomainEvent(videoID, filename string, size int64, provider string, occurredAt time.Time) *DomainEvent {
	return &DomainEvent{
		EventType:  "upload.completed",
		VideoID:    videoID,
		Filename:   filename,
		Size:       size,
		Provider:   provider,
		OccurredAt: occurredAt,
	}
}

func generatePresignedURL(client minioClientIface, bucket, key string) (string, error) {
	expiry := time.Duration(3600) * time.Second
	presignedURL, err := client.PresignedGetObject(context.Background(), bucket, key, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned url: %w", err)
	}

	return presignedURL.String(), nil
}

func listStorageVideos(client minioClientIface, bucket, provider string) ([]DomainEvent, error) {
	var videos []DomainEvent
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	objectCh := client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			return nil, object.Err
		}
		if strings.Contains(object.Key, ".chunk.") {
			continue
		}

		videoID, filename := parseStorageKey(object.Key)
		videos = append(videos, *newStorageDomainEvent(videoID, filename, object.Size, provider, object.LastModified))
	}

	return videos, nil
}
