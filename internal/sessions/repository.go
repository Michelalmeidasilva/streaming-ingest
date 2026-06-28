package sessions

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Session is the persisted record of one launched benchmark session.
type Session struct {
	SessionID     string    `bson:"sessionId" json:"sessionId"`
	InstanceTypes []string  `bson:"instanceTypes" json:"instanceTypes"`
	RequestedBy   string    `bson:"requestedBy,omitempty" json:"requestedBy,omitempty"`
	CreatedAt     time.Time `bson:"createdAt" json:"createdAt"`
}

// SessionFilter narrows a List query. Empty fields are ignored.
type SessionFilter struct {
	SessionID string
}

// SessionRepository is the persistence interface for Session documents.
type SessionRepository interface {
	Insert(ctx context.Context, sess Session) error
	List(ctx context.Context, filter SessionFilter) ([]Session, error)
}

// sessCursor abstracts mongo.Cursor for testing.
type sessCursor interface {
	All(ctx context.Context, results interface{}) error
	Close(ctx context.Context) error
}

// sessCollection abstracts *mongo.Collection for testing.
type sessCollection interface {
	InsertOne(ctx context.Context, document interface{}, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error)
	Find(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (sessCursor, error)
}

// realSessCollection wraps *mongo.Collection to satisfy sessCollection.
type realSessCollection struct{ c *mongo.Collection }

func (r *realSessCollection) InsertOne(ctx context.Context, doc interface{}, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error) {
	return r.c.InsertOne(ctx, doc, opts...)
}

func (r *realSessCollection) Find(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (sessCursor, error) {
	return r.c.Find(ctx, filter, opts...)
}

// MongoSessionRepository implements SessionRepository backed by MongoDB.
type MongoSessionRepository struct {
	collection sessCollection
}

// NewMongoSessionRepository constructs a MongoSessionRepository.
func NewMongoSessionRepository(client *mongo.Client, dbName, collectionName string) *MongoSessionRepository {
	return &MongoSessionRepository{
		collection: &realSessCollection{c: client.Database(dbName).Collection(collectionName)},
	}
}

// Insert persists a new Session document, setting CreatedAt if unset.
func (r *MongoSessionRepository) Insert(ctx context.Context, sess Session) error {
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = time.Now().UTC()
	}
	_, err := r.collection.InsertOne(ctx, sess)
	return err
}

// List returns all sessions matching the filter, newest first.
func (r *MongoSessionRepository) List(ctx context.Context, filter SessionFilter) ([]Session, error) {
	query := bson.M{}
	if filter.SessionID != "" {
		query["sessionId"] = filter.SessionID
	}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cur, err := r.collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var sessions []Session
	if err := cur.All(ctx, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

var _ SessionRepository = (*MongoSessionRepository)(nil)
