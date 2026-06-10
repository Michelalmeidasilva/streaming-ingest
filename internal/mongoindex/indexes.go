// Package mongoindex creates the MongoDB indexes the Event Gateway relies on.
// Without these, every lookup by video_id/session_id and the upload-state list
// sort fall back to full collection scans (COLLSCAN), which dominate latency as
// the catalog grows. Index creation is idempotent: re-running is a no-op.
package mongoindex

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// IndexManager creates indexes for a single collection. *mongo.IndexView (from
// Collection.Indexes()) satisfies it.
type IndexManager interface {
	CreateMany(ctx context.Context, models []mongo.IndexModel, opts ...*options.CreateIndexesOptions) ([]string, error)
}

// collectionIndexes maps each collection to the indexes it needs. video_id is
// unique on both the catalog and the canonical upload-state document (one doc
// per video — this also guards against the duplicate-document bug). created_at
// backs the upload-state list sort polled by the admin UI.
func collectionIndexes() map[string][]mongo.IndexModel {
	uniqueVideoID := []mongo.IndexModel{
		{Keys: bson.D{{Key: "video_id", Value: 1}}, Options: options.Index().SetUnique(true)},
	}
	return map[string][]mongo.IndexModel{
		"videos_catalog": uniqueVideoID,
		"videos": {
			{Keys: bson.D{{Key: "video_id", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "created_at", Value: -1}}},
		},
		"upload_sessions": {
			{Keys: bson.D{{Key: "session_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		},
		"transcode_runs": {
			{Keys: bson.D{{Key: "job_id", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "machine_label", Value: 1}, {Key: "completed_at", Value: -1}}},
		},
	}
}

// EnsureIndexes creates every required index. collFor returns the IndexManager
// for a given collection name (typically client.Database(db).Collection(name).Indexes()).
func EnsureIndexes(ctx context.Context, collFor func(name string) IndexManager) error {
	for name, models := range collectionIndexes() {
		if _, err := collFor(name).CreateMany(ctx, models); err != nil {
			return fmt.Errorf("create indexes for %s: %w", name, err)
		}
	}
	return nil
}
