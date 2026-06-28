package sessions

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ---- fake mongo collection ----

type fakeSessColl struct {
	inserted []interface{}
	lastFind interface{}
	docs     []Session
}

func (f *fakeSessColl) InsertOne(_ context.Context, doc interface{}, _ ...*options.InsertOneOptions) (*mongo.InsertOneResult, error) {
	f.inserted = append(f.inserted, doc)
	return &mongo.InsertOneResult{}, nil
}

func (f *fakeSessColl) Find(_ context.Context, filter interface{}, _ ...*options.FindOptions) (sessCursor, error) {
	f.lastFind = filter
	return &fakeSessCursor{docs: f.docs}, nil
}

type fakeSessCursor struct{ docs []Session }

func (c *fakeSessCursor) All(_ context.Context, results interface{}) error {
	out := results.(*[]Session)
	*out = c.docs
	return nil
}
func (c *fakeSessCursor) Close(_ context.Context) error { return nil }

// ---- repository tests ----

func TestInsertWritesSessionDoc(t *testing.T) {
	fc := &fakeSessColl{}
	repo := &MongoSessionRepository{collection: fc}
	err := repo.Insert(context.Background(), Session{
		SessionID:     "s1",
		InstanceTypes: []string{"c5.xlarge", "g6.xlarge"},
		RequestedBy:   "a@x.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(fc.inserted) != 1 {
		t.Fatalf("want 1 insert, got %d", len(fc.inserted))
	}
	got := fc.inserted[0].(Session)
	if got.SessionID != "s1" {
		t.Fatalf("sessionId not stored: got %q", got.SessionID)
	}
	if len(got.InstanceTypes) != 2 {
		t.Fatalf("instanceTypes not stored: %v", got.InstanceTypes)
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("createdAt must be set on insert")
	}
}

func TestListSessionsReturnsDocs(t *testing.T) {
	fc := &fakeSessColl{
		docs: []Session{
			{SessionID: "s1", InstanceTypes: []string{"c5.xlarge"}},
			{SessionID: "s2", InstanceTypes: []string{"g6.xlarge"}},
		},
	}
	repo := &MongoSessionRepository{collection: fc}
	got, err := repo.List(context.Background(), SessionFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 sessions, got %d", len(got))
	}
}

func TestListSessionsSessionIDFilter(t *testing.T) {
	fc := &fakeSessColl{
		docs: []Session{{SessionID: "s1"}},
	}
	repo := &MongoSessionRepository{collection: fc}
	if _, err := repo.List(context.Background(), SessionFilter{SessionID: "s1"}); err != nil {
		t.Fatal(err)
	}
	q, ok := fc.lastFind.(bson.M)
	if !ok || q["sessionId"] != "s1" {
		t.Fatalf("sessionId filter not applied to query: %#v", fc.lastFind)
	}
}
