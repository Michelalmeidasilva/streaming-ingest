package adapters

import "testing"

// The transcoder downloads the source from the exact object key. Reconstructing
// it from videoID+filename drops any multi-segment prefix (e.g. the upload's
// `raw/`), so the adapters must carry the full key through on the domain event.
func TestParseEventPreservesFullObjectKey(t *testing.T) {
	const key = "raw/abc-123/movie.mp4"

	t.Run("minio records", func(t *testing.T) {
		payload := []byte(`{"Records":[{"eventTime":"2024-01-01T00:00:00Z","s3":{"object":{"key":"` + key + `","size":1024}}}]}`)
		ev, err := (&MinioAdapter{}).ParseEvent(payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ev.ObjectKey != key {
			t.Errorf("ObjectKey = %q, want %q", ev.ObjectKey, key)
		}
		if ev.VideoID != "abc-123" || ev.Filename != "movie.mp4" {
			t.Errorf("VideoID/Filename = %q/%q, want abc-123/movie.mp4", ev.VideoID, ev.Filename)
		}
	})

	t.Run("s3 records", func(t *testing.T) {
		payload := []byte(`{"Records":[{"eventName":"ObjectCreated:Put","eventTime":"2024-01-01T00:00:00Z","s3":{"object":{"key":"` + key + `","size":1024}}}]}`)
		ev, err := (&S3Adapter{}).ParseEvent(payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ev.ObjectKey != key {
			t.Errorf("ObjectKey = %q, want %q", ev.ObjectKey, key)
		}
	})

	t.Run("s3 eventbridge", func(t *testing.T) {
		payload := []byte(`{"detail-type":"Object Created","source":"aws.s3","time":"2026-06-04T12:00:00Z","detail":{"bucket":{"name":"videos"},"object":{"key":"` + key + `","size":2048}}}`)
		ev, err := (&S3Adapter{}).ParseEvent(payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ev.ObjectKey != key {
			t.Errorf("ObjectKey = %q, want %q", ev.ObjectKey, key)
		}
	})
}
