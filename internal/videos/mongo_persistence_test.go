package videos_test

import (
	"context"
	"os"
	"testing"
	"time"

	"streaming-ingest/internal/adapters"
	"streaming-ingest/internal/videos"
	"streaming-ingest/internal/webhooks"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type integrationPublisher struct {
	publishErr error
}

func (m *integrationPublisher) Publish(routingKey string, payload interface{}) error {
	return m.publishErr
}

type integrationStorageAdapter struct {
	parseEventResult *adapters.DomainEvent
	parseEventErr    error
	listVideosResult []adapters.DomainEvent
	listVideosErr    error
}

func (m *integrationStorageAdapter) ParseEvent(payload []byte) (*adapters.DomainEvent, error) {
	return m.parseEventResult, m.parseEventErr
}

func (m *integrationStorageAdapter) ListVideos(bucket string) ([]adapters.DomainEvent, error) {
	return m.listVideosResult, m.listVideosErr
}

func (m *integrationStorageAdapter) GenerateURL(bucket, key string) (string, error) {
	return "", nil
}

func newMongoTestClient(t *testing.T) (*mongo.Client, string) {
	t.Helper()

	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("MONGODB_URI not set; skipping Mongo persistence integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().
		ApplyURI(uri).
		SetServerSelectionTimeout(5*time.Second))
	if err != nil {
		t.Skipf("MongoDB unavailable at %s: %v", uri, err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		t.Skipf("MongoDB unavailable at %s: %v", uri, err)
	}

	return client, "streaming_test"
}

func newMongoTestRepo(t *testing.T) (*mongo.Client, *videos.MongoRepository, string) {
	t.Helper()

	client, dbName := newMongoTestClient(t)
	collectionName := "videos_test_" + primitive.NewObjectID().Hex()
	repo := videos.NewMongoRepository(client, dbName, collectionName)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = client.Database(dbName).Collection(collectionName).Drop(ctx)
		_ = client.Disconnect(ctx)
	})

	return client, repo, collectionName
}

func TestMongoPersistsVideoOnWebhookUpload(t *testing.T) {
	_, repo, _ := newMongoTestRepo(t)

	eventTime := time.Now().UTC().Truncate(time.Second)
	mockAdapter := &integrationStorageAdapter{
		parseEventResult: &adapters.DomainEvent{
			EventType:  "upload.completed",
			VideoID:    "vid-upload-1",
			Filename:   "upload.mp4",
			Size:       2048,
			Provider:   "minio",
			OccurredAt: eventTime,
		},
	}

	svc := webhooks.NewService(
		&integrationPublisher{},
		map[string]adapters.StorageAdapter{"minio": mockAdapter},
		repo,
	)

	if err := svc.ProcessWebhook("minio", []byte(`{"ignored":true}`)); err != nil {
		t.Fatalf("ProcessWebhook() error = %v", err)
	}

	videosFromDB, err := repo.ListAll(context.Background())
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}

	if len(videosFromDB) != 1 {
		t.Fatalf("expected 1 video in Mongo, got %d", len(videosFromDB))
	}

	got := videosFromDB[0]
	if got.VideoID != "vid-upload-1" {
		t.Fatalf("VideoID = %q, want %q", got.VideoID, "vid-upload-1")
	}
	if got.Filename != "upload.mp4" {
		t.Fatalf("Filename = %q, want %q", got.Filename, "upload.mp4")
	}
	if got.Status != "uploaded" {
		t.Fatalf("Status = %q, want %q", got.Status, "uploaded")
	}
	if got.Provider != "minio" {
		t.Fatalf("Provider = %q, want %q", got.Provider, "minio")
	}
	if got.Size != 2048 {
		t.Fatalf("Size = %d, want %d", got.Size, 2048)
	}
	if got.CreatedAt.IsZero() {
		t.Fatalf("CreatedAt was not persisted")
	}
}

func TestMongoPersistsVideoWhenListingVideos(t *testing.T) {
	_, repo, _ := newMongoTestRepo(t)

	storageVideo := adapters.DomainEvent{
		EventType:  "upload.completed",
		VideoID:    "vid-list-1",
		Filename:   "list.mp4",
		Size:       4096,
		Provider:   "minio",
		OccurredAt: time.Now().UTC().Truncate(time.Second),
	}

	svc := videos.NewService(
		map[string]adapters.StorageAdapter{
			"minio": &integrationStorageAdapter{
				listVideosResult: []adapters.DomainEvent{storageVideo},
			},
		},
		repo,
	)

	gotVideos, err := svc.ListAllVideos(context.Background())
	if err != nil {
		t.Fatalf("ListAllVideos() error = %v", err)
	}

	if len(gotVideos) != 1 {
		t.Fatalf("expected 1 video in response, got %d", len(gotVideos))
	}
	if gotVideos[0].VideoID != "vid-list-1" {
		t.Fatalf("VideoID = %q, want %q", gotVideos[0].VideoID, "vid-list-1")
	}

	videosFromDB, err := repo.ListAll(context.Background())
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}

	if len(videosFromDB) != 1 {
		t.Fatalf("expected 1 video in Mongo after list sync, got %d", len(videosFromDB))
	}

	got := videosFromDB[0]
	if got.VideoID != storageVideo.VideoID {
		t.Fatalf("VideoID = %q, want %q", got.VideoID, storageVideo.VideoID)
	}
	if got.Filename != storageVideo.Filename {
		t.Fatalf("Filename = %q, want %q", got.Filename, storageVideo.Filename)
	}
	if got.Status != "ready" {
		t.Fatalf("Status = %q, want %q", got.Status, "ready")
	}
	if got.Provider != storageVideo.Provider {
		t.Fatalf("Provider = %q, want %q", got.Provider, storageVideo.Provider)
	}
	if got.Size != storageVideo.Size {
		t.Fatalf("Size = %d, want %d", got.Size, storageVideo.Size)
	}
	if got.CreatedAt.IsZero() {
		t.Fatalf("CreatedAt was not persisted")
	}
}
