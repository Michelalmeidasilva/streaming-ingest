package mongoindex

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type recordingIndexManager struct {
	name string
	rec  map[string][]mongo.IndexModel
}

func (r *recordingIndexManager) CreateMany(ctx context.Context, models []mongo.IndexModel, opts ...*options.CreateIndexesOptions) ([]string, error) {
	r.rec[r.name] = append(r.rec[r.name], models...)
	names := make([]string, len(models))
	return names, nil
}

func firstKey(model mongo.IndexModel) string {
	keys, ok := model.Keys.(bson.D)
	if !ok || len(keys) == 0 {
		return ""
	}
	return keys[0].Key
}

func hasUniqueIndexOn(models []mongo.IndexModel, key string) bool {
	for _, m := range models {
		if firstKey(m) == key && m.Options != nil && m.Options.Unique != nil && *m.Options.Unique {
			return true
		}
	}
	return false
}

func hasIndexOn(models []mongo.IndexModel, key string) bool {
	for _, m := range models {
		if firstKey(m) == key {
			return true
		}
	}
	return false
}

func TestEnsureIndexes_CreatesExpectedIndexes(t *testing.T) {
	rec := map[string][]mongo.IndexModel{}
	collFor := func(name string) IndexManager {
		return &recordingIndexManager{name: name, rec: rec}
	}

	if err := EnsureIndexes(context.Background(), collFor); err != nil {
		t.Fatalf("EnsureIndexes: %v", err)
	}

	if !hasUniqueIndexOn(rec["videos_catalog"], "video_id") {
		t.Errorf("videos_catalog: missing unique index on video_id, got %v", rec["videos_catalog"])
	}
	if !hasUniqueIndexOn(rec["videos"], "video_id") {
		t.Errorf("videos: missing unique index on video_id, got %v", rec["videos"])
	}
	if !hasIndexOn(rec["videos"], "created_at") {
		t.Errorf("videos: missing index on created_at (used by the upload-state list sort), got %v", rec["videos"])
	}
	if !hasUniqueIndexOn(rec["upload_sessions"], "session_id") {
		t.Errorf("upload_sessions: missing unique index on session_id, got %v", rec["upload_sessions"])
	}
	if !hasUniqueIndexOn(rec["transcode_runs"], "job_id") {
		t.Errorf("transcode_runs: missing unique index on job_id, got %v", rec["transcode_runs"])
	}
	if !hasIndexOn(rec["transcode_runs"], "machine_label") {
		t.Errorf("transcode_runs: missing index on machine_label, got %v", rec["transcode_runs"])
	}
}
