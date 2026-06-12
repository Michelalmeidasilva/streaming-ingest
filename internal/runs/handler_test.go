package runs

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

type stubReader struct {
	listFilter Filter
	listResult []Run
	byVideo    []Run
	gotVideoID string
}

func (s *stubReader) List(_ context.Context, f Filter) ([]Run, error) {
	s.listFilter = f
	return s.listResult, nil
}
func (s *stubReader) GetByVideoID(_ context.Context, videoID string) ([]Run, error) {
	s.gotVideoID = videoID
	return s.byVideo, nil
}

type stubWriter struct{ last Run }

func (s *stubWriter) RecordBenchmarkRun(_ context.Context, run Run) error { s.last = run; return nil }

func TestCreateBenchmarkRun(t *testing.T) {
	w := &stubWriter{}
	app := fiber.New()
	h := NewHandler(&stubReader{})
	h.SetBenchmarkWriter(w)
	app.Post("/api/v1/benchmark-runs", h.CreateBenchmarkRun)

	body := `{"machineLabel":"c5.xlarge","clip":"a.mp4","repetition":1,` +
		`"sourceWidth":1920,"sourceHeight":1080,"sourceDurationSeconds":31.5,"sourceFps":30,` +
		`"sourceCodec":"vp9","sourceBitrateKbps":4200,"sourceFileSizeBytes":1234,` +
		`"renditions":[{"codec":"av1","width":1280,"height":720,"elapsedSeconds":9,"outputFileSizeBytes":2000000}]}`
	req := httptest.NewRequest("POST", "/api/v1/benchmark-runs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 202 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if w.last.MachineLabel != "c5.xlarge" || len(w.last.Renditions) != 1 || w.last.Renditions[0].Codec != "av1" {
		t.Fatalf("not recorded: %#v", w.last)
	}
	if w.last.SourceWidth != 1920 || w.last.SourceHeight != 1080 || w.last.SourceDurationSeconds != 31.5 ||
		w.last.SourceFPS != 30 || w.last.SourceCodec != "vp9" || w.last.SourceBitrateKbps != 4200 {
		t.Fatalf("source fields not parsed: %#v", w.last)
	}
	if w.last.Renditions[0].OutputFileSizeBytes != 2000000 {
		t.Fatalf("output file size not persisted: %#v", w.last.Renditions[0])
	}
}

func TestListRunsBenchmarkParam(t *testing.T) {
	stub := &stubReader{listResult: []Run{{JobID: "a"}}}
	app := fiber.New()
	h := NewHandler(stub)
	app.Get("/api/v1/runs", h.ListRuns)
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/runs?benchmark=true", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if stub.listFilter.Benchmark == nil || *stub.listFilter.Benchmark != true {
		t.Fatalf("benchmark filter not parsed: %#v", stub.listFilter.Benchmark)
	}
}

func TestListRunsAppliesFilters(t *testing.T) {
	stub := &stubReader{listResult: []Run{{JobID: "a"}}}
	app := fiber.New()
	h := NewHandler(stub)
	app.Get("/api/v1/runs", h.ListRuns)

	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/runs?machineLabel=c7g&codec=av1", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if stub.listFilter.MachineLabel != "c7g" || stub.listFilter.Codec != "av1" {
		t.Fatalf("filter not applied: %#v", stub.listFilter)
	}
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Runs []Run `json:"runs"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Runs) != 1 {
		t.Fatalf("want 1 run, got %d", len(out.Runs))
	}
}

func TestGetRunsByVideoID(t *testing.T) {
	stub := &stubReader{byVideo: []Run{{JobID: "a"}, {JobID: "b"}}}
	app := fiber.New()
	h := NewHandler(stub)
	app.Get("/api/v1/runs/:videoId", h.GetRuns)

	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/runs/v1", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Runs []Run `json:"runs"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Runs) != 2 {
		t.Fatalf("want 2 runs, got %d", len(out.Runs))
	}
	if stub.gotVideoID != "v1" {
		t.Fatalf("videoId not forwarded: got %q", stub.gotVideoID)
	}
}
