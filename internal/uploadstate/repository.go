package uploadstate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrNotFound = errors.New("not found")

type PartETag struct {
	PartNumber int    `bson:"part_number" json:"PartNumber"`
	ETag       string `bson:"etag" json:"ETag"`
}

type UploadSession struct {
	ID             string     `bson:"session_id" json:"id"`
	VideoID        string     `bson:"video_id" json:"videoId"`
	TotalChunks    int        `bson:"total_chunks" json:"totalChunks"`
	UploadedChunks int        `bson:"uploaded_chunks" json:"uploadedChunks"`
	ChunkSize      int        `bson:"chunk_size" json:"chunkSize"`
	TotalSize      int64      `bson:"total_size" json:"totalSize"`
	StartedAt      time.Time  `bson:"started_at" json:"startedAt"`
	Filename       string     `bson:"filename" json:"filename"`
	UploadID       string     `bson:"upload_id" json:"uploadId"`
	ETags          []PartETag `bson:"etags" json:"etags"`
}

type Video struct {
	ID              string    `bson:"video_id" json:"id"`
	Filename        string    `bson:"filename" json:"filename"`
	OriginalName    string    `bson:"original_name" json:"originalName"`
	Title           string    `bson:"title" json:"title"`
	Size            int64     `bson:"size" json:"size"`
	Status          string    `bson:"status" json:"status"`
	Progress        float64   `bson:"progress" json:"progress"`
	CreatedAt       time.Time `bson:"created_at" json:"createdAt"`
	UpdatedAt       time.Time `bson:"updated_at" json:"updatedAt"`
	URL             string    `bson:"url,omitempty" json:"url,omitempty"`
	DownloadURL     string    `bson:"download_url,omitempty" json:"downloadUrl,omitempty"`
	ThumbnailURL    string    `bson:"thumbnail_url,omitempty" json:"thumbnailUrl,omitempty"`
	ThumbnailStatus string    `bson:"thumbnail_status,omitempty" json:"thumbnailStatus,omitempty"`
	MimeType        string    `bson:"mime_type,omitempty" json:"mimeType,omitempty"`
	Provider        string    `bson:"provider,omitempty" json:"provider,omitempty"`
	ProcessingStatus string         `bson:"processingStatus,omitempty" json:"processingStatus,omitempty"`
	Source           map[string]any `bson:"source,omitempty" json:"source,omitempty"`
	MediaInfo        map[string]any `bson:"mediaInfo,omitempty" json:"mediaInfo,omitempty"`
	Transcode        map[string]any `bson:"transcode,omitempty" json:"transcode,omitempty"`
	Playback         map[string]any `bson:"playback,omitempty" json:"playback,omitempty"`
	Metrics          map[string]any `bson:"metrics,omitempty" json:"metrics,omitempty"`
}

type UploadState struct {
	Session UploadSession `json:"session"`
	Video   Video         `json:"video"`
}

type mongoCollection interface {
	UpdateOne(ctx context.Context, filter, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error)
	FindOne(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) singleResult
	Find(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (mongoCursor, error)
	DeleteOne(ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error)
}

type singleResult interface {
	Decode(v interface{}) error
}

type mongoCursor interface {
	All(ctx context.Context, results interface{}) error
	Close(ctx context.Context) error
}

type realMongoCollection struct {
	collection *mongo.Collection
}

func (c *realMongoCollection) UpdateOne(ctx context.Context, filter, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	return c.collection.UpdateOne(ctx, filter, update, opts...)
}

func (c *realMongoCollection) FindOne(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) singleResult {
	return c.collection.FindOne(ctx, filter, opts...)
}

func (c *realMongoCollection) Find(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (mongoCursor, error) {
	return c.collection.Find(ctx, filter, opts...)
}

func (c *realMongoCollection) DeleteOne(ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	return c.collection.DeleteOne(ctx, filter, opts...)
}

type MongoRepository struct {
	sessions mongoCollection
	videos   mongoCollection
}

func NewMongoRepository(client *mongo.Client, dbName, sessionsCollectionName, videosCollectionName string) *MongoRepository {
	db := client.Database(dbName)
	return &MongoRepository{
		sessions: &realMongoCollection{collection: db.Collection(sessionsCollectionName)},
		videos:   &realMongoCollection{collection: db.Collection(videosCollectionName)},
	}
}

func (r *MongoRepository) SaveState(ctx context.Context, state UploadState) error {
	if err := r.SaveVideo(ctx, state.Video); err != nil {
		return err
	}

	update := bson.M{
		"$set": state.Session,
	}
	_, err := r.sessions.UpdateOne(
		ctx,
		bson.M{"session_id": state.Session.ID},
		update,
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	return nil
}

func (r *MongoRepository) GetState(ctx context.Context, sessionID string) (*UploadState, error) {
	var session UploadSession
	if err := r.sessions.FindOne(ctx, bson.M{"session_id": sessionID}).Decode(&session); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find session: %w", err)
	}

	video, err := r.GetVideo(ctx, session.VideoID)
	if err != nil {
		return nil, err
	}

	return &UploadState{
		Session: session,
		Video:   *video,
	}, nil
}

func (r *MongoRepository) DeleteSession(ctx context.Context, sessionID string) error {
	_, err := r.sessions.DeleteOne(ctx, bson.M{"session_id": sessionID})
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (r *MongoRepository) SaveVideo(ctx context.Context, video Video) error {
	update := bson.M{
		"$set": video,
	}
	_, err := r.videos.UpdateOne(
		ctx,
		bson.M{"video_id": video.ID},
		update,
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("save video: %w", err)
	}
	return nil
}

func (r *MongoRepository) GetVideo(ctx context.Context, videoID string) (*Video, error) {
	var video Video
	if err := r.videos.FindOne(ctx, bson.M{"video_id": videoID}).Decode(&video); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find video: %w", err)
	}
	return &video, nil
}

func (r *MongoRepository) ListVideos(ctx context.Context, query string) ([]Video, error) {
	filter := bson.M{}
	if query != "" {
		filter = bson.M{
			"$or": []bson.M{
				{"title": bson.M{"$regex": query, "$options": "i"}},
				{"original_name": bson.M{"$regex": query, "$options": "i"}},
				{"filename": bson.M{"$regex": query, "$options": "i"}},
			},
		}
	}

	cursor, err := r.videos.Find(
		ctx,
		filter,
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("list videos: %w", err)
	}
	defer cursor.Close(ctx)

	var videos []Video
	if err := cursor.All(ctx, &videos); err != nil {
		return nil, fmt.Errorf("decode videos: %w", err)
	}

	return videos, nil
}

func (r *MongoRepository) PatchVideo(ctx context.Context, videoID string, patch bson.M) (*Video, error) {
	if len(patch) == 0 {
		return r.GetVideo(ctx, videoID)
	}

	patch["updated_at"] = time.Now().UTC()
	result, err := r.videos.UpdateOne(
		ctx,
		bson.M{"video_id": videoID},
		bson.M{"$set": patch},
	)
	if err != nil {
		return nil, fmt.Errorf("patch video: %w", err)
	}
	if result.MatchedCount == 0 && result.UpsertedCount == 0 {
		return nil, ErrNotFound
	}

	return r.GetVideo(ctx, videoID)
}

func (r *MongoRepository) DeleteVideo(ctx context.Context, videoID string) error {
	_, err := r.videos.DeleteOne(ctx, bson.M{"video_id": videoID})
	if err != nil {
		return fmt.Errorf("delete video: %w", err)
	}
	return nil
}
