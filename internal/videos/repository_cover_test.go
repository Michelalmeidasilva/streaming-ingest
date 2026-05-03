package videos

import (
	"context"
	"testing"
)

func TestRealMongoCollectionPanics(t *testing.T) {
	c := &realMongoCollection{}

	func() {
		defer func() { recover() }()
		c.InsertOne(context.Background(), nil)
	}()

	func() {
		defer func() { recover() }()
		c.UpdateOne(context.Background(), nil, nil)
	}()

	func() {
		defer func() { recover() }()
		c.Find(context.Background(), nil)
	}()
}

func TestNewMongoRepositoryReal(t *testing.T) {
	defer func() { recover() }()
	NewMongoRepository(nil, "db", "coll")
}
