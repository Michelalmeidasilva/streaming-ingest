package adapters

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
)

func TestNewS3Adapter(t *testing.T) {
	t.Run("without credentials", func(t *testing.T) {
		t.Setenv("AWS_ACCESS_KEY_ID", "")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "")
		t.Setenv("AWS_REGION", "")

		adapter := NewS3Adapter()
		if adapter == nil {
			t.Fatalf("NewS3Adapter() returned nil")
		}
		if adapter.client != nil {
			t.Fatalf("NewS3Adapter() client = %v, want nil without credentials", adapter.client)
		}
	})

	t.Run("with credentials", func(t *testing.T) {
		t.Setenv("AWS_ACCESS_KEY_ID", "test-access")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
		t.Setenv("AWS_REGION", "us-east-2")

		adapter := NewS3Adapter()
		if adapter == nil {
			t.Fatalf("NewS3Adapter() returned nil")
		}
		if adapter.client == nil {
			t.Fatalf("NewS3Adapter() client is nil with credentials present")
		}
	})
}

func TestS3AdapterParseEvent(t *testing.T) {
	adapter := &S3Adapter{client: nil}

	tests := []struct {
		name      string
		payload   []byte
		wantErr   bool
		errMsg    string
		wantVidID string
		wantFile  string
	}{
		{
			name:    "invalid json",
			payload: []byte("not json"),
			wantErr: true,
			errMsg:  "failed to parse s3 block",
		},
		{
			name:    "empty records",
			payload: []byte(`{"Records":[]}`),
			wantErr: true,
			errMsg:  "no records found",
		},
		{
			name:    "non-creation event",
			payload: []byte(`{"Records":[{"eventName":"ObjectRemoved:Delete","s3":{"object":{"key":"file.mp4"}}}]}`),
			wantErr: true,
			errMsg:  "ignoring non-creation event",
		},
		{
			name:      "key with slash",
			payload:   []byte(`{"Records":[{"eventName":"ObjectCreated:Put","s3":{"object":{"key":"abc123/video.mp4","size":1024}}}]}`),
			wantErr:   false,
			wantVidID: "abc123",
			wantFile:  "video.mp4",
		},
		{
			name:      "key without slash",
			payload:   []byte(`{"Records":[{"eventName":"ObjectCreated:Put","s3":{"object":{"key":"video.mp4","size":2048}}}]}`),
			wantErr:   false,
			wantVidID: "unknown",
			wantFile:  "video.mp4",
		},
		{
			name:      "url encoded key",
			payload:   []byte(`{"Records":[{"eventName":"ObjectCreated:Put","s3":{"object":{"key":"abc123/video%20name.mp4","size":2048}}}]}`),
			wantErr:   false,
			wantVidID: "abc123",
			wantFile:  "video name.mp4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := adapter.ParseEvent(tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil {
				// Check error message contains expected substring
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("ParseEvent() error = %v, want to contain %v", err.Error(), tt.errMsg)
				}
			}
			if !tt.wantErr {
				if got.VideoID != tt.wantVidID {
					t.Errorf("ParseEvent() VideoID = %v, want %v", got.VideoID, tt.wantVidID)
				}
				if got.Filename != tt.wantFile {
					t.Errorf("ParseEvent() Filename = %v, want %v", got.Filename, tt.wantFile)
				}
				if got.Provider != "aws-s3" {
					t.Errorf("ParseEvent() Provider = %v, want aws-s3", got.Provider)
				}
			}
		})
	}
}

func TestS3AdapterListVideos(t *testing.T) {
	t.Run("client nil", func(t *testing.T) {
		adapter := &S3Adapter{client: nil}
		_, err := adapter.ListVideos("test-bucket")
		if err == nil || !contains(err.Error(), "s3 client not initialized") {
			t.Fatalf("ListVideos() error = %v, want s3 client not initialized", err)
		}
	})

	t.Run("list objects", func(t *testing.T) {
		adapter := &S3Adapter{
			client: &mockMinioClient{
				listObjectsFunc: func(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
					ch := make(chan minio.ObjectInfo, 2)
					ch <- minio.ObjectInfo{Key: "vid-1/movie.mp4", Size: 1024, LastModified: time.Now()}
					ch <- minio.ObjectInfo{Key: "vid-2/movie%20two.mp4", Size: 2048, LastModified: time.Now()}
					close(ch)
					return ch
				},
				presignedGetObjFunc: func(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
					return nil, nil
				},
			},
		}

		videos, err := adapter.ListVideos("test-bucket")
		if err != nil {
			t.Fatalf("ListVideos() unexpected error: %v", err)
		}
		if len(videos) != 2 {
			t.Fatalf("ListVideos() returned %d videos, want 2", len(videos))
		}
		if videos[1].Filename != "movie two.mp4" {
			t.Fatalf("ListVideos() decoded filename = %q, want %q", videos[1].Filename, "movie two.mp4")
		}
	})
}

func TestS3AdapterGenerateURL(t *testing.T) {
	t.Run("client nil", func(t *testing.T) {
		adapter := &S3Adapter{client: nil}
		_, err := adapter.GenerateURL("my-bucket", "vid1/file.mp4")
		if err == nil || !contains(err.Error(), "s3 client not initialized") {
			t.Fatalf("GenerateURL() error = %v, want s3 client not initialized", err)
		}
	})

	t.Run("presigned url", func(t *testing.T) {
		adapter := &S3Adapter{
			client: &mockMinioClient{
				listObjectsFunc: func(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
					ch := make(chan minio.ObjectInfo)
					close(ch)
					return ch
				},
				presignedGetObjFunc: func(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
					return url.Parse("https://my-bucket.s3.us-east-2.amazonaws.com/vid1/file.mp4?X-Amz-Signature=abc")
				},
			},
		}

		signedURL, err := adapter.GenerateURL("my-bucket", "vid1/file.mp4")
		if err != nil {
			t.Fatalf("GenerateURL() unexpected error: %v", err)
		}
		if !contains(signedURL, "my-bucket") {
			t.Fatalf("GenerateURL() = %v, want to contain bucket name", signedURL)
		}
	})
}

func TestS3AdapterListVideosPropagatesErrors(t *testing.T) {
	adapter := &S3Adapter{
		client: &mockMinioClient{
			listObjectsFunc: func(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
				ch := make(chan minio.ObjectInfo, 1)
				ch <- minio.ObjectInfo{Err: errors.New("list failed")}
				close(ch)
				return ch
			},
			presignedGetObjFunc: func(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
				return nil, nil
			},
		},
	}

	_, err := adapter.ListVideos("test-bucket")
	if err == nil || !contains(err.Error(), "list failed") {
		t.Fatalf("ListVideos() error = %v, want list failed", err)
	}
}

func TestS3AdapterParseEvent_EventBridgeEnvelope(t *testing.T) {
	adapter := NewS3Adapter()
	payload := []byte(`{
		"detail-type": "Object Created",
		"source": "aws.s3",
		"time": "2026-06-04T12:00:00Z",
		"detail": {
			"bucket": { "name": "videos" },
			"object": { "key": "raw/abc-123/movie.mp4", "size": 2048 }
		}
	}`)

	event, err := adapter.ParseEvent(payload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if event.VideoID != "abc-123" {
		t.Errorf("VideoID = %q, want %q", event.VideoID, "abc-123")
	}
	if event.Filename != "movie.mp4" {
		t.Errorf("Filename = %q, want %q", event.Filename, "movie.mp4")
	}
	if event.Size != 2048 {
		t.Errorf("Size = %d, want 2048", event.Size)
	}
	if event.EventType != "upload.completed" {
		t.Errorf("EventType = %q, want %q", event.EventType, "upload.completed")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
