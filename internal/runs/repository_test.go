package runs

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type fakeColl struct {
	filters []interface{}
	upserts []bson.M
	docs    []Run
}

func (f *fakeColl) UpdateOne(_ context.Context, filter, update interface{}, _ ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	f.filters = append(f.filters, filter)
	f.upserts = append(f.upserts, update.(bson.M))
	return &mongo.UpdateResult{}, nil
}

func (f *fakeColl) Find(_ context.Context, _ interface{}, _ ...*options.FindOptions) (cursor, error) {
	return &fakeCursor{docs: f.docs}, nil
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
