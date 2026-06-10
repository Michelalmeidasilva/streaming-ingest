package runs

import (
	"context"
	"testing"
)

type capturingRepo struct{ last Run }

func (c *capturingRepo) Upsert(_ context.Context, run Run) error             { c.last = run; return nil }
func (c *capturingRepo) List(context.Context, Filter) ([]Run, error)         { return nil, nil }
func (c *capturingRepo) GetByVideoID(context.Context, string) ([]Run, error) { return nil, nil }

func sampleCompletedPayload() map[string]interface{} {
	return map[string]interface{}{
		"videoId":        "v1",
		"jobId":          "v1-123",
		"profile":        "production",
		"elapsedSeconds": 42.5,
		"rtf":            1.5,
		"machineLabel":   "c7g.xlarge",
		"completedAt":    "2026-06-09T12:00:00Z",
		"observability": map[string]interface{}{
			"hostname":             "ip-10-0-0-1",
			"cpuCores":             float64(4),
			"machineLabel":         "c7g.xlarge",
			"sourceFileSizeBytes":  float64(1000),
			"totalOutputSizeBytes": float64(800),
		},
		"renditions": []interface{}{
			map[string]interface{}{
				"name":        "h264-720p",
				"codec":       "h264",
				"width":       float64(1280),
				"height":      float64(720),
				"preset":      "veryfast",
				"bitrateKbps": float64(3000),
				"metrics": map[string]interface{}{
					"elapsedSeconds":    float64(20),
					"targetBitrateKbps": float64(3000),
					"outputBitrateKbps": float64(2950),
					"resourceUsage": map[string]interface{}{
						"avgCpuPercent": float64(180),
						"maxCpuPercent": float64(320),
						"avgMemoryMb":   float64(150),
						"maxMemoryMb":   float64(220),
					},
				},
			},
		},
	}
}

func TestUpsertFromEventMapsFields(t *testing.T) {
	repo := &capturingRepo{}
	svc := NewService(repo)
	if err := svc.UpsertFromEvent(context.Background(), sampleCompletedPayload()); err != nil {
		t.Fatal(err)
	}
	got := repo.last
	if got.JobID != "v1-123" || got.VideoID != "v1" || got.MachineLabel != "c7g.xlarge" {
		t.Fatalf("job-level mapping wrong: %#v", got)
	}
	if got.CPUCores != 4 || got.ElapsedSeconds != 42.5 {
		t.Fatalf("job metrics wrong: %#v", got)
	}
	if len(got.Renditions) != 1 {
		t.Fatalf("want 1 rendition, got %d", len(got.Renditions))
	}
	r := got.Renditions[0]
	if r.Codec != "h264" || r.ElapsedSeconds != 20 || r.AvgCPUPercent != 180 || r.OutputBitrateKbps != 2950 {
		t.Fatalf("rendition mapping wrong: %#v", r)
	}
}

func TestUpsertFromEventSkipsWithoutJobID(t *testing.T) {
	repo := &capturingRepo{}
	svc := NewService(repo)
	if err := svc.UpsertFromEvent(context.Background(), map[string]interface{}{"videoId": "v1"}); err != nil {
		t.Fatal(err)
	}
	if repo.last.VideoID != "" {
		t.Fatalf("expected no upsert when jobId is missing")
	}
}
