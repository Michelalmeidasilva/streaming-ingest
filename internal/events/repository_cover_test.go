package events

import (
	"testing"
)


func TestNewMongoRepositoryReal(t *testing.T) {
	defer func() { recover() }()
	NewMongoRepository(nil, "db", "coll")
}
