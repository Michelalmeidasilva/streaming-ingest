package runs

import (
	"context"
	"encoding/json"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type fakeColl struct {
	filters  []interface{}
	upserts  []bson.M
	docs     []Run
	inserted []interface{}
	lastFind interface{}
}

func (f *fakeColl) UpdateOne(_ context.Context, filter, update interface{}, _ ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	f.filters = append(f.filters, filter)
	f.upserts = append(f.upserts, update.(bson.M))
	return &mongo.UpdateResult{}, nil
}

func (f *fakeColl) Find(_ context.Context, filter interface{}, _ ...*options.FindOptions) (cursor, error) {
	f.lastFind = filter
	return &fakeCursor{docs: f.docs}, nil
}

func (f *fakeColl) InsertOne(_ context.Context, doc interface{}, _ ...*options.InsertOneOptions) (*mongo.InsertOneResult, error) {
	f.inserted = append(f.inserted, doc)
	return &mongo.InsertOneResult{}, nil
}

type fakeCursor struct{ docs []Run }

func (c *fakeCursor) All(_ context.Context, results interface{}) error {
	out := results.(*[]Run)
	*out = c.docs
	return nil
}
func (c *fakeCursor) Close(_ context.Context) error { return nil }

func TestUpsertIsKeyedByJobID(t *testing.T) {
	fc := &fakeColl{}
	repo := &MongoRepository{collection: fc}
	if err := repo.Upsert(context.Background(), Run{JobID: "job-1", VideoID: "v1", MachineLabel: "c7g"}); err != nil {
		t.Fatal(err)
	}
	if len(fc.filters) != 1 {
		t.Fatalf("want 1 update, got %d", len(fc.filters))
	}
	filter, ok := fc.filters[0].(bson.M)
	if !ok || filter["job_id"] != "job-1" {
		t.Fatalf("upsert not keyed by job_id: %#v", fc.filters[0])
	}
	set, ok := fc.upserts[0]["$set"].(bson.M)
	if !ok || set["job_id"] != "job-1" {
		t.Fatalf("unexpected $set: %#v", fc.upserts[0]["$set"])
	}
	if _, found := set["created_at"]; found {
		t.Fatalf("created_at must be in $setOnInsert, not $set")
	}
	if _, found := set["_id"]; found {
		t.Fatalf("_id must not be in $set")
	}
	onInsert, ok := fc.upserts[0]["$setOnInsert"].(bson.M)
	if !ok || onInsert["created_at"] == nil {
		t.Fatalf("created_at missing from $setOnInsert: %#v", fc.upserts[0]["$setOnInsert"])
	}
}

func TestGetByVideoIDReturnsDocs(t *testing.T) {
	fc := &fakeColl{docs: []Run{{JobID: "x", VideoID: "v99"}}}
	repo := &MongoRepository{collection: fc}
	got, err := repo.GetByVideoID(context.Background(), "v99")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 run, got %d", len(got))
	}
}

func TestListReturnsDocs(t *testing.T) {
	fc := &fakeColl{docs: []Run{{JobID: "a"}, {JobID: "b"}}}
	repo := &MongoRepository{collection: fc}
	got, err := repo.List(context.Background(), Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 runs, got %d", len(got))
	}
}

func TestInsertWritesBenchmarkDoc(t *testing.T) {
	fc := &fakeColl{}
	repo := &MongoRepository{collection: fc}
	if err := repo.Insert(context.Background(), Run{MachineLabel: "c5.xlarge", Benchmark: true, Clip: "a.mp4", Repetition: 1}); err != nil {
		t.Fatal(err)
	}
	if len(fc.inserted) != 1 {
		t.Fatalf("want 1 insert, got %d", len(fc.inserted))
	}
	got := fc.inserted[0].(Run)
	if !got.Benchmark || got.Clip != "a.mp4" || got.CreatedAt.IsZero() {
		t.Fatalf("bad inserted run: %#v", got)
	}
}

func TestRunRenditionQualityFieldsJSON(t *testing.T) {
	in := []byte(`{"codec":"h264","width":1920,"height":1080,"qualityParam":"q31","vmaf":92.3,"psnr":41.1}`)
	var r RunRendition
	if err := json.Unmarshal(in, &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.QualityParam != "q31" || r.VMAF != 92.3 || r.PSNR != 41.1 {
		t.Fatalf("parsed = %+v", r)
	}
}

func TestListBenchmarkFilter(t *testing.T) {
	fc := &fakeColl{docs: []Run{{JobID: "a"}}}
	repo := &MongoRepository{collection: fc}
	bt := true
	if _, err := repo.List(context.Background(), Filter{Benchmark: &bt}); err != nil {
		t.Fatal(err)
	}
	q, ok := fc.lastFind.(bson.M)
	if !ok || q["benchmark"] != true {
		t.Fatalf("benchmark=true filter not applied: %#v", fc.lastFind)
	}

	bf := false
	if _, err := repo.List(context.Background(), Filter{Benchmark: &bf}); err != nil {
		t.Fatal(err)
	}
	q2, _ := fc.lastFind.(bson.M)
	if _, hasNe := q2["benchmark"].(bson.M); !hasNe {
		t.Fatalf("benchmark=false should use $ne:true, got %#v", q2["benchmark"])
	}
}
