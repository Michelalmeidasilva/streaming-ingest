package events

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type mockEventCollection struct {
	insertOneFunc func(ctx context.Context, document interface{}, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error)
	insertCalls   int
}

func (m *mockEventCollection) InsertOne(ctx context.Context, document interface{}, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error) {
	m.insertCalls++
	if m.insertOneFunc != nil {
		return m.insertOneFunc(ctx, document, opts...)
	}
	return &mongo.InsertOneResult{}, nil
}

func TestNewMongoRepository(t *testing.T) {
	collection := &mockEventCollection{}
	repo := &MongoRepository{
		collection: collection,
	}

	if repo == nil {
		t.Errorf("NewMongoRepository() returned nil")
	}

	if repo.collection == nil {
		t.Errorf("MongoRepository collection is nil")
	}
}

func TestMongoRepositorySave(t *testing.T) {
	tests := []struct {
		name    string
		event   *EventRecord
		mockErr error
		wantErr bool
	}{
		{
			name: "save event succeeds",
			event: &EventRecord{
				EventType:  "upload.progress",
				RoutingKey: "video.upload.progress",
				Payload:    EventPayload{"videoId": "vid-1"},
				CreatedAt:  time.Now(),
			},
		},
		{
			name: "save event fails",
			event: &EventRecord{
				EventType:  "upload.completed",
				RoutingKey: "video.upload.completed",
				Payload:    EventPayload{"videoId": "vid-2"},
				CreatedAt:  time.Now(),
			},
			mockErr: errors.New("insert failed"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockEventCollection{
				insertOneFunc: func(ctx context.Context, document interface{}, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error) {
					if tt.mockErr != nil {
						return nil, tt.mockErr
					}

					doc, ok := document.(bson.M)
					if !ok {
						t.Fatalf("document type = %T, want bson.M", document)
					}
					if doc["routing_key"] != tt.event.RoutingKey {
						t.Fatalf("routing_key = %v, want %v", doc["routing_key"], tt.event.RoutingKey)
					}
					return &mongo.InsertOneResult{}, nil
				},
			}

			repo := &MongoRepository{collection: mock}
			err := repo.Save(context.Background(), tt.event)

			if (err != nil) != tt.wantErr {
				t.Errorf("Save() error = %v, wantErr %v", err, tt.wantErr)
			}

			if mock.insertCalls != 1 {
				t.Errorf("InsertOne() calls = %d, want 1", mock.insertCalls)
			}
		})
	}
}

func TestEventRepository_Interface(t *testing.T) {
	var _ EventRepository = (*MongoRepository)(nil)
}
