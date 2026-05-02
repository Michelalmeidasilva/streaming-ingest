package videos

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Video struct {
	ID         string    `bson:"_id,omitempty" json:"id"`
	VideoID    string    `bson:"video_id" json:"videoId"`
	Filename   string    `bson:"filename" json:"filename"`
	Size       int64     `bson:"size" json:"size"`
	Provider   string    `bson:"provider" json:"provider"`
	Status     string    `bson:"status" json:"status"`
	CreatedAt  time.Time `bson:"created_at" json:"createdAt"`
	UpdatedAt  time.Time `bson:"updated_at" json:"updatedAt"`
}

type VideoRepository interface {
	Save(ctx context.Context, video *Video) error
	ListAll(ctx context.Context) ([]Video, error)
	Search(ctx context.Context, query string) ([]Video, error)
}

type mongoCollection interface {
	UpdateOne(ctx context.Context, filter, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error)
	Find(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (*mongo.Cursor, error)
}

type MongoRepository struct {
	collection mongoCollection
}

func NewMongoRepository(client *mongo.Client, dbName, collectionName string) *MongoRepository {
	return &MongoRepository{
		collection: client.Database(dbName).Collection(collectionName),
	}
}

func (r *MongoRepository) Save(ctx context.Context, video *Video) error {
	filter := bson.M{"video_id": video.VideoID}
	update := bson.M{
		"$set": video,
	}
	opts := options.Update().SetUpsert(true)

	_, err := r.collection.UpdateOne(ctx, filter, update, opts)
	return err
}

func (r *MongoRepository) ListAll(ctx context.Context) ([]Video, error) {
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var videos []Video
	if err := cursor.All(ctx, &videos); err != nil {
		return nil, err
	}

	return videos, nil
}

func (r *MongoRepository) Search(ctx context.Context, query string) ([]Video, error) {
	filter := bson.M{
		"filename": bson.M{
			"$regex":   query,
			"$options": "i", // Case-insensitive
		},
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var videos []Video
	if err := cursor.All(ctx, &videos); err != nil {
		return nil, err
	}

	return videos, nil
}
