package runs

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// RunRendition is one codec/resolution output measured during a transcode run.
type RunRendition struct {
	Name              string  `bson:"name" json:"name"`
	Codec             string  `bson:"codec" json:"codec"`
	Width             int     `bson:"width" json:"width"`
	Height            int     `bson:"height" json:"height"`
	Preset            string  `bson:"preset,omitempty" json:"preset,omitempty"`
	TargetBitrateKbps int     `bson:"target_bitrate_kbps" json:"targetBitrateKbps"`
	OutputBitrateKbps int64   `bson:"output_bitrate_kbps" json:"outputBitrateKbps"`
	ElapsedSeconds    float64 `bson:"elapsed_seconds" json:"elapsedSeconds"`
	AvgCPUPercent     float64 `bson:"avg_cpu_percent" json:"avgCpuPercent"`
	MaxCPUPercent     float64 `bson:"max_cpu_percent" json:"maxCpuPercent"`
	AvgMemoryMB       float64 `bson:"avg_memory_mb" json:"avgMemoryMb"`
	MaxMemoryMB       float64 `bson:"max_memory_mb" json:"maxMemoryMb"`
}

// Run is the persisted record of one video transcode on one machine.
type Run struct {
	ID                   string         `bson:"_id,omitempty" json:"id"`
	JobID                string         `bson:"job_id" json:"jobId"`
	VideoID              string         `bson:"video_id" json:"videoId"`
	MachineLabel         string         `bson:"machine_label" json:"machineLabel"`
	Hostname             string         `bson:"hostname" json:"hostname"`
	CPUCores             int            `bson:"cpu_cores" json:"cpuCores"`
	Profile              string         `bson:"profile" json:"profile"`
	ElapsedSeconds       float64        `bson:"elapsed_seconds" json:"elapsedSeconds"`
	RTF                  float64        `bson:"rtf" json:"rtf"`
	SourceFileSizeBytes  int64          `bson:"source_file_size_bytes" json:"sourceFileSizeBytes"`
	TotalOutputSizeBytes int64          `bson:"total_output_size_bytes" json:"totalOutputSizeBytes"`
	Renditions           []RunRendition `bson:"renditions" json:"renditions"`
	CompletedAt          time.Time      `bson:"completed_at" json:"completedAt"`
	CreatedAt            time.Time      `bson:"created_at" json:"createdAt"`
}

// Filter narrows a List query. Empty fields are ignored.
type Filter struct {
	MachineLabel string
	Codec        string
}

type Repository interface {
	Upsert(ctx context.Context, run Run) error
	List(ctx context.Context, filter Filter) ([]Run, error)
	GetByVideoID(ctx context.Context, videoID string) ([]Run, error)
}

type cursor interface {
	All(ctx context.Context, results interface{}) error
	Close(ctx context.Context) error
}

type collection interface {
	UpdateOne(ctx context.Context, filter, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error)
	Find(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (cursor, error)
}

type realCollection struct{ c *mongo.Collection }

func (r *realCollection) UpdateOne(ctx context.Context, filter, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	return r.c.UpdateOne(ctx, filter, update, opts...)
}

func (r *realCollection) Find(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (cursor, error) {
	return r.c.Find(ctx, filter, opts...)
}

type MongoRepository struct {
	collection collection
}

func NewMongoRepository(client *mongo.Client, dbName, collectionName string) *MongoRepository {
	return &MongoRepository{collection: &realCollection{c: client.Database(dbName).Collection(collectionName)}}
}

func (r *MongoRepository) Upsert(ctx context.Context, run Run) error {
	if run.CreatedAt.IsZero() {
		run.CreatedAt = time.Now().UTC()
	}
	doc, err := bson.Marshal(run)
	if err != nil {
		return err
	}
	var set bson.M
	if err := bson.Unmarshal(doc, &set); err != nil {
		return err
	}
	createdAt := set["created_at"]
	delete(set, "_id")
	delete(set, "created_at")
	_, err = r.collection.UpdateOne(ctx,
		bson.M{"job_id": run.JobID},
		bson.M{"$set": set, "$setOnInsert": bson.M{"created_at": createdAt}},
		options.Update().SetUpsert(true),
	)
	return err
}

func (r *MongoRepository) List(ctx context.Context, filter Filter) ([]Run, error) {
	query := bson.M{}
	if filter.MachineLabel != "" {
		query["machine_label"] = filter.MachineLabel
	}
	if filter.Codec != "" {
		query["renditions.codec"] = filter.Codec
	}
	opts := options.Find().SetSort(bson.D{{Key: "completed_at", Value: -1}})
	cur, err := r.collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var runs []Run
	if err := cur.All(ctx, &runs); err != nil {
		return nil, err
	}
	return runs, nil
}

func (r *MongoRepository) GetByVideoID(ctx context.Context, videoID string) ([]Run, error) {
	cur, err := r.collection.Find(ctx, bson.M{"video_id": videoID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var runs []Run
	if err := cur.All(ctx, &runs); err != nil {
		return nil, err
	}
	return runs, nil
}

var _ Repository = (*MongoRepository)(nil)
