package adapters

import (
	"testing"
)

func TestS3AdapterParseEvent(t *testing.T) {
	adapter := NewS3Adapter()

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
	adapter := NewS3Adapter()

	videos, err := adapter.ListVideos("test-bucket")
	if err != nil {
		t.Fatalf("ListVideos() unexpected error: %v", err)
	}
	if len(videos) != 0 {
		t.Errorf("ListVideos() should return empty slice, got %v items", len(videos))
	}
}

func TestS3AdapterGenerateURL(t *testing.T) {
	adapter := NewS3Adapter()

	url, err := adapter.GenerateURL("my-bucket", "vid1/file.mp4")
	if err != nil {
		t.Fatalf("GenerateURL() unexpected error: %v", err)
	}
	if url == "" {
		t.Errorf("GenerateURL() returned empty string")
	}
	if !contains(url, "my-bucket") {
		t.Errorf("GenerateURL() = %v, want to contain bucket name", url)
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
