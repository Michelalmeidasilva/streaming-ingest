package runs

import (
	"context"
	"time"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// UpsertFromEvent maps a transcode.completed payload to a Run and persists it.
// A payload without a jobId is ignored (it is not a completed transcode).
func (s *Service) UpsertFromEvent(ctx context.Context, payload map[string]interface{}) error {
	jobID := mapString(payload, "jobId")
	if jobID == "" {
		return nil
	}
	obs := mapChild(payload, "observability")
	run := Run{
		JobID:                jobID,
		VideoID:              mapString(payload, "videoId"),
		MachineLabel:         firstNonEmpty(mapString(payload, "machineLabel"), mapString(obs, "machineLabel")),
		Hostname:             mapString(obs, "hostname"),
		CPUCores:             int(mapFloat(obs, "cpuCores")),
		Profile:              mapString(payload, "profile"),
		ElapsedSeconds:       mapFloat(payload, "elapsedSeconds"),
		RTF:                  mapFloat(payload, "rtf"),
		SourceFileSizeBytes:  int64(mapFloat(obs, "sourceFileSizeBytes")),
		TotalOutputSizeBytes: int64(mapFloat(obs, "totalOutputSizeBytes")),
		Renditions:           mapRenditions(payload),
		CompletedAt:          parseTime(mapString(payload, "completedAt")),
		CreatedAt:            time.Now().UTC(),
	}
	return s.repo.Upsert(ctx, run)
}

func mapRenditions(payload map[string]interface{}) []RunRendition {
	raw, ok := payload["renditions"].([]interface{})
	if !ok {
		return nil
	}
	out := make([]RunRendition, 0, len(raw))
	for _, item := range raw {
		r, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		metrics := mapChild(r, "metrics")
		usage := mapChild(metrics, "resourceUsage")
		out = append(out, RunRendition{
			Name:              mapString(r, "name"),
			Codec:             mapString(r, "codec"),
			Width:             int(mapFloat(r, "width")),
			Height:            int(mapFloat(r, "height")),
			Preset:            mapString(r, "preset"),
			TargetBitrateKbps: int(mapFloat(metrics, "targetBitrateKbps")),
			OutputBitrateKbps: int64(mapFloat(metrics, "outputBitrateKbps")),
			ElapsedSeconds:    mapFloat(metrics, "elapsedSeconds"),
			AvgCPUPercent:     mapFloat(usage, "avgCpuPercent"),
			MaxCPUPercent:     mapFloat(usage, "maxCpuPercent"),
			AvgMemoryMB:       mapFloat(usage, "avgMemoryMb"),
			MaxMemoryMB:       mapFloat(usage, "maxMemoryMb"),
		})
	}
	return out
}

func mapChild(m map[string]interface{}, key string) map[string]interface{} {
	if m == nil {
		return nil
	}
	child, _ := m[key].(map[string]interface{})
	return child
}

func mapString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

func mapFloat(m map[string]interface{}, key string) float64 {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	case int:
		return float64(v)
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}
