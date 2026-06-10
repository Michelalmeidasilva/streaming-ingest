package adapters

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestTranscodeRequestPreservesAllFields pins the ingest↔transcode contract:
// every field the transcoder understands must round-trip through this mirror
// struct. The struct previously lacked protocols/segmentSeconds/bitrateKbps, so
// those selections were silently dropped on the upload.started → webhook hop.
func TestTranscodeRequestPreservesAllFields(t *testing.T) {
	in := []byte(`{
		"codecs": ["h265"],
		"protocols": ["dash"],
		"segmentSeconds": 4,
		"renditions": [
			{"width": 1280, "height": 720, "codec": "h265", "bitrateKbps": 2800}
		]
	}`)

	var req TranscodeRequest
	if err := json.Unmarshal(in, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := req.Protocols; len(got) != 1 || got[0] != "dash" {
		t.Fatalf("protocols dropped: %v", got)
	}
	if req.SegmentSeconds != 4 {
		t.Fatalf("segmentSeconds dropped: %d", req.SegmentSeconds)
	}
	if len(req.Renditions) != 1 || req.Renditions[0].BitrateKbps != 2800 {
		t.Fatalf("bitrateKbps dropped: %+v", req.Renditions)
	}

	// Re-marshal must keep the fields so the webhook forwards them unchanged.
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"protocols":["dash"]`, `"segmentSeconds":4`, `"bitrateKbps":2800`} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("re-marshal lost %s: %s", want, out)
		}
	}
}
