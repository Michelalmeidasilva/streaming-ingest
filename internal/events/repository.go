package events

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type EventRecord struct {
	ID         string       `bson:"_id,omitempty" json:"id"`
	EventType  string       `bson:"event_type" json:"eventType"`
	RoutingKey string       `bson:"routing_key" json:"routingKey"`
	Payload    EventPayload `bson:"payload" json:"payload"`
	CreatedAt  time.Time    `bson:"created_at" json:"createdAt"`
}

type EventRepository interface {
	Save(ctx context.Context, event *EventRecord) error
}

type eventCollection interface {
	InsertOne(ctx context.Context, document interface{}, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error)
}

type MongoRepository struct {
	collection eventCollection
}

func NewMongoRepository(client *mongo.Client, dbName, collectionName string) *MongoRepository {
	return &MongoRepository{
		collection: client.Database(dbName).Collection(collectionName),
	}
}

func (r *MongoRepository) Save(ctx context.Context, event *EventRecord) error {
	_, err := r.collection.InsertOne(ctx, bson.M{
		"event_type":  event.EventType,
		"routing_key": event.RoutingKey,
		"payload":     event.Payload,
		"created_at":  event.CreatedAt,
	})
	return err
}

var _ EventRepository = (*MongoRepository)(nil)
