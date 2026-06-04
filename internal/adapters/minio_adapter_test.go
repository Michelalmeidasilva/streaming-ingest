package adapters

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
)

type mockMinioClient struct {
	listObjectsFunc     func(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	presignedGetObjFunc func(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error)
}

func (m *mockMinioClient) ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	return m.listObjectsFunc(ctx, bucketName, opts)
}

func (m *mockMinioClient) PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
	return m.presignedGetObjFunc(ctx, bucketName, objectName, expires, reqParams)
}

func TestMinioAdapterParseEvent(t *testing.T) {
	adapter := &MinioAdapter{client: nil} // ParseEvent doesn't use client

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
			errMsg:  "failed to parse minio block",
		},
		{
			name:    "empty records",
			payload: []byte(`{"Records":[]}`),
			wantErr: true,
			errMsg:  "no records found",
		},
		{
			name: "key_with_slash",
			payload: []byte(`{
				"Records": [{
					"eventTime": "2024-01-01T00:00:00Z",
					"s3": {
						"object": {
							"key": "video123/movie.mp4",
							"size": 1024
						}
					}
				}]
			}`),
			wantErr:   false,
			wantVidID: "video123",
			wantFile:  "movie.mp4",
		},
		{
			name: "key_without_slash",
			payload: []byte(`{
				"Records": [{
					"eventTime": "2024-01-01T00:00:00Z",
					"s3": {
						"object": {
							"key": "movie.mp4",
							"size": 1024
						}
					}
				}]
			}`),
			wantErr:   false,
			wantVidID: "movie.mp4",
			wantFile:  "movie.mp4",
		},
		{
			name: "url-encoded_key",
			payload: []byte(`{
				"Records": [{
					"eventTime": "2024-01-01T00:00:00Z",
					"s3": {
						"object": {
							"key": "video456/movie%20name.mp4",
							"size": 2048
						}
					}
				}]
			}`),
			wantErr:   false,
			wantVidID: "video456",
			wantFile:  "movie name.mp4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := adapter.ParseEvent(tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
				t.Errorf("ParseEvent() error = %v, want to contain %v", err.Error(), tt.errMsg)
			}
			if !tt.wantErr {
				if event.VideoID != tt.wantVidID {
					t.Errorf("ParseEvent() VideoID = %v, want %v", event.VideoID, tt.wantVidID)
				}
				if event.Filename != tt.wantFile {
					t.Errorf("ParseEvent() Filename = %v, want %v", event.Filename, tt.wantFile)
				}
			}
		})
	}
}

func TestMinioAdapterParseEvent_IgnoresPipelineOutputs(t *testing.T) {
	adapter := &MinioAdapter{client: nil}
	cases := []string{
		`{"Records":[{"eventName":"s3:ObjectCreated:Put","s3":{"object":{"key":"transcoded/abc123/hls/master.m3u8","size":10}}}]}`,
		`{"Records":[{"eventName":"s3:ObjectCreated:Put","s3":{"object":{"key":"transcoded/abc123/hls/720p/init.mp4","size":10}}}]}`,
		`{"Records":[{"eventName":"s3:ObjectCreated:Put","s3":{"object":{"key":"metrics/abc123/transcode-result.json","size":10}}}]}`,
		`{"Records":[{"eventName":"s3:ObjectCreated:Put","s3":{"object":{"key":"thumbnails/abc123.jpg","size":10}}}]}`,
	}
	for _, payload := range cases {
		_, err := adapter.ParseEvent([]byte(payload))
		if !errors.Is(err, ErrIgnoredObjectKey) {
			t.Errorf("ParseEvent(%s): expected ErrIgnoredObjectKey, got %v", payload, err)
		}
	}
}

func TestMinioAdapterParseEvent_AcceptsUploadKey(t *testing.T) {
	adapter := &MinioAdapter{client: nil}
	payload := `{"Records":[{"eventName":"s3:ObjectCreated:CompleteMultipartUpload","s3":{"object":{"key":"550e8400-e29b-41d4-a716-446655440000/beauty-medium-001.mp4","size":24074810}}}]}`
	event, err := adapter.ParseEvent([]byte(payload))
	if err != nil {
		t.Fatalf("ParseEvent: unexpected error %v", err)
	}
	if event.VideoID != "550e8400-e29b-41d4-a716-446655440000" || event.Filename != "beauty-medium-001.mp4" {
		t.Errorf("ParseEvent: got videoID=%q filename=%q", event.VideoID, event.Filename)
	}
}

func TestMinioAdapterListVideos(t *testing.T) {
	tests := []struct {
		name     string
		adapter  *MinioAdapter
		wantErr  bool
		errMsg   string
		checkLen func(int) bool
	}{
		{
			name: "client_nil",
			adapter: &MinioAdapter{
				client: nil,
			},
			wantErr: true,
			errMsg:  "minio client not initialized",
		},
		{
			name: "object_error",
			adapter: &MinioAdapter{
				client: &mockMinioClient{
					listObjectsFunc: func(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
						ch := make(chan minio.ObjectInfo, 1)
						ch <- minio.ObjectInfo{Err: errors.New("list error")}
						close(ch)
						return ch
					},
				},
			},
			wantErr: true,
			errMsg:  "list error",
		},
		{
			name: "skip_chunk_file",
			adapter: &MinioAdapter{
				client: &mockMinioClient{
					listObjectsFunc: func(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
						ch := make(chan minio.ObjectInfo, 2)
						ch <- minio.ObjectInfo{Key: "vid1/movie.mp4.chunk", Size: 1024, LastModified: time.Now()}
						ch <- minio.ObjectInfo{Key: "vid2/other.mp4", Size: 2048, LastModified: time.Now()}
						close(ch)
						return ch
					},
				},
			},
			wantErr: false,
			checkLen: func(l int) bool {
				return l == 2
			},
		},
		{
			name: "key_with_slash",
			adapter: &MinioAdapter{
				client: &mockMinioClient{
					listObjectsFunc: func(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
						ch := make(chan minio.ObjectInfo, 1)
						ch <- minio.ObjectInfo{Key: "vid1/movie.mp4", Size: 1024, LastModified: time.Now()}
						close(ch)
						return ch
					},
				},
			},
			wantErr: false,
			checkLen: func(l int) bool {
				return l == 1
			},
		},
		{
			name: "empty_result",
			adapter: &MinioAdapter{
				client: &mockMinioClient{
					listObjectsFunc: func(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
						ch := make(chan minio.ObjectInfo)
						close(ch)
						return ch
					},
				},
			},
			wantErr: false,
			checkLen: func(l int) bool {
				return l == 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			videos, err := tt.adapter.ListVideos("test-bucket")
			if (err != nil) != tt.wantErr {
				t.Errorf("ListVideos() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
				t.Errorf("ListVideos() error = %v, want to contain %v", err.Error(), tt.errMsg)
			}
			if !tt.wantErr && tt.checkLen != nil && !tt.checkLen(len(videos)) {
				t.Errorf("ListVideos() returned %d videos", len(videos))
			}
		})
	}
}

func TestMinioAdapterGenerateURL(t *testing.T) {
	tests := []struct {
		name    string
		adapter *MinioAdapter
		bucket  string
		key     string
		wantErr bool
		errMsg  string
	}{
		{
			name: "client_nil",
			adapter: &MinioAdapter{
				client: nil,
			},
			bucket:  "test",
			key:     "file.mp4",
			wantErr: true,
			errMsg:  "minio client not initialized",
		},
		{
			name: "presigned_error",
			adapter: &MinioAdapter{
				client: &mockMinioClient{
					presignedGetObjFunc: func(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
						return nil, errors.New("presign error")
					},
				},
			},
			bucket:  "test",
			key:     "file.mp4",
			wantErr: true,
			errMsg:  "failed to generate presigned url",
		},
		{
			name: "success",
			adapter: &MinioAdapter{
				client: &mockMinioClient{
					presignedGetObjFunc: func(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
						u, _ := url.Parse("http://minio:9000/test/file.mp4?signature=abc")
						return u, nil
					},
				},
			},
			bucket:  "test",
			key:     "file.mp4",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.adapter.GenerateURL(tt.bucket, tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
				t.Errorf("GenerateURL() error = %v, want to contain %v", err.Error(), tt.errMsg)
			}
			if !tt.wantErr && got == "" {
				t.Errorf("GenerateURL() returned empty string")
			}
		})
	}
}

func TestNewMinioAdapter(t *testing.T) {
	tests := []struct {
		name       string
		setupEnv   func()
		cleanupEnv func()
		wantNil    bool
	}{
		{
			name: "credentials_not_set",
			setupEnv: func() {
				os.Unsetenv("MINIO_ROOT_USER")
				os.Unsetenv("MINIO_ROOT_PASSWORD")
			},
			cleanupEnv: func() {},
			wantNil:    true,
		},
		{
			name: "only_user_set",
			setupEnv: func() {
				os.Setenv("MINIO_ROOT_USER", "admin")
				os.Unsetenv("MINIO_ROOT_PASSWORD")
			},
			cleanupEnv: func() {
				os.Unsetenv("MINIO_ROOT_USER")
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()
			defer tt.cleanupEnv()

			adapter := NewMinioAdapter()
			if tt.wantNil && adapter.client != nil {
				t.Errorf("NewMinioAdapter() with missing credentials should have nil client")
			}
		})
	}
}

func TestNewMinioAdapterSuccess(t *testing.T) {
	t.Setenv("MINIO_ROOT_USER", "admin")
	t.Setenv("MINIO_ROOT_PASSWORD", "password123")
	t.Setenv("MINIO_ENDPOINT", "localhost:9000")
	t.Setenv("MINIO_USE_SSL", "false")

	adapter := NewMinioAdapter()
	if adapter.client == nil {
		t.Errorf("NewMinioAdapter() should have initialized client")
	}
}
