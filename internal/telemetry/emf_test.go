package telemetry

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestEmitterWritesValidEMF(t *testing.T) {
	var buf bytes.Buffer
	e := &Emitter{Service: "streaming-ingest", Out: &buf, Now: func() time.Time { return time.UnixMilli(1717689600000) }}

	e.Emit("/api/v1/events", "POST", 201, 42*time.Millisecond)

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if got["RequestCount"].(float64) != 1 {
		t.Errorf("RequestCount = %v, want 1", got["RequestCount"])
	}
	if got["ErrorCount"].(float64) != 0 {
		t.Errorf("ErrorCount = %v, want 0 for status 201", got["ErrorCount"])
	}
	if got["RequestLatency"].(float64) != 42 {
		t.Errorf("RequestLatency = %v, want 42", got["RequestLatency"])
	}
	if got["service"] != "streaming-ingest" || got["route"] != "/api/v1/events" || got["method"] != "POST" {
		t.Errorf("dimensions wrong: %v", got)
	}
	aws := got["_aws"].(map[string]any)
	cwm := aws["CloudWatchMetrics"].([]any)[0].(map[string]any)
	if cwm["Namespace"] != "VOD/streaming-ingest" {
		t.Errorf("Namespace = %v", cwm["Namespace"])
	}
}

func TestEmitterCountsServerErrors(t *testing.T) {
	var buf bytes.Buffer
	e := &Emitter{Service: "streaming-ingest", Out: &buf, Now: time.Now}
	e.Emit("/x", "GET", 503, time.Millisecond)

	var got map[string]any
	_ = json.Unmarshal(buf.Bytes(), &got)
	if got["ErrorCount"].(float64) != 1 {
		t.Errorf("ErrorCount = %v, want 1 for status 503", got["ErrorCount"])
	}
}
