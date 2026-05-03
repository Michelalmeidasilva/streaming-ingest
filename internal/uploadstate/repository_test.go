package uploadstate

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type mockCollection struct {
	updateOneFunc func(ctx context.Context, filter, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error)
	findOneFunc   func(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) singleResult
	findFunc      func(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (mongoCursor, error)
	deleteOneFunc func(ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error)
}

func (m *mockCollection) UpdateOne(ctx context.Context, filter, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	if m.updateOneFunc != nil {
		return m.updateOneFunc(ctx, filter, update, opts...)
	}
	return &mongo.UpdateResult{MatchedCount: 1}, nil
}

func (m *mockCollection) FindOne(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) singleResult {
	if m.findOneFunc != nil {
		return m.findOneFunc(ctx, filter, opts...)
	}
	return mockSingleResult{}
}

func (m *mockCollection) Find(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (mongoCursor, error) {
	if m.findFunc != nil {
		return m.findFunc(ctx, filter, opts...)
	}
	return &mockCursor{}, nil
}

func (m *mockCollection) DeleteOne(ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	if m.deleteOneFunc != nil {
		return m.deleteOneFunc(ctx, filter, opts...)
	}
	return &mongo.DeleteResult{DeletedCount: 1}, nil
}

type mockSingleResult struct {
	decodeFunc func(v interface{}) error
}

func (m mockSingleResult) Decode(v interface{}) error {
	if m.decodeFunc != nil {
		return m.decodeFunc(v)
	}
	return nil
}

type mockCursor struct {
	allFunc func(ctx context.Context, results interface{}) error
}

func (m *mockCursor) All(ctx context.Context, results interface{}) error {
	if m.allFunc != nil {
		return m.allFunc(ctx, results)
	}
	return nil
}

func (m *mockCursor) Close(ctx context.Context) error {
	return nil
}

func TestSaveStatePersistsSessionAndVideo(t *testing.T) {
	repo := &MongoRepository{
		sessions: &mockCollection{},
		videos:   &mockCollection{},
	}

	err := repo.SaveState(context.Background(), UploadState{
		Session: UploadSession{ID: "s1", VideoID: "v1"},
		Video:   Video{ID: "v1", Filename: "video.mp4"},
	})

	if err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}
}

func TestNewMongoRepository(t *testing.T) {
	repo := NewMongoRepository(&mongo.Client{}, "streaming", "upload_sessions", "videos")
	if repo == nil || repo.sessions == nil || repo.videos == nil {
		t.Fatalf("NewMongoRepository() = %+v", repo)
	}
}

func TestSaveStateReturnsSessionUpdateError(t *testing.T) {
	repo := &MongoRepository{
		sessions: &mockCollection{
			updateOneFunc: func(ctx context.Context, filter, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
				return nil, errors.New("session failed")
			},
		},
		videos: &mockCollection{},
	}

	err := repo.SaveState(context.Background(), UploadState{
		Session: UploadSession{ID: "s1", VideoID: "v1"},
		Video:   Video{ID: "v1"},
	})
	if err == nil || err.Error() != "save session: session failed" {
		t.Fatalf("SaveState() error = %v", err)
	}
}

func TestSaveStateReturnsVideoSaveError(t *testing.T) {
	repo := &MongoRepository{
		sessions: &mockCollection{},
		videos: &mockCollection{
			updateOneFunc: func(ctx context.Context, filter, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
				return nil, errors.New("video failed")
			},
		},
	}

	err := repo.SaveState(context.Background(), UploadState{
		Session: UploadSession{ID: "s1", VideoID: "v1"},
		Video:   Video{ID: "v1"},
	})
	if err == nil || err.Error() != "save video: video failed" {
		t.Fatalf("SaveState() error = %v", err)
	}
}

func TestGetStateReturnsNotFound(t *testing.T) {
	repo := &MongoRepository{
		sessions: &mockCollection{
			findOneFunc: func(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) singleResult {
				return mockSingleResult{decodeFunc: func(v interface{}) error { return mongo.ErrNoDocuments }}
			},
		},
		videos: &mockCollection{},
	}

	_, err := repo.GetState(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetState() error = %v, want ErrNotFound", err)
	}
}

func TestGetStateReturnsVideoAndSession(t *testing.T) {
	repo := &MongoRepository{
		sessions: &mockCollection{
			findOneFunc: func(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) singleResult {
				return mockSingleResult{decodeFunc: func(v interface{}) error {
					session := v.(*UploadSession)
					*session = UploadSession{ID: "s1", VideoID: "v1"}
					return nil
				}}
			},
		},
		videos: &mockCollection{
			findOneFunc: func(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) singleResult {
				return mockSingleResult{decodeFunc: func(v interface{}) error {
					video := v.(*Video)
					*video = Video{ID: "v1", Title: "Video"}
					return nil
				}}
			},
		},
	}

	state, err := repo.GetState(context.Background(), "s1")
	if err != nil {
		t.Fatalf("GetState() error = %v", err)
	}
	if state.Session.ID != "s1" || state.Video.ID != "v1" {
		t.Fatalf("GetState() = %+v", state)
	}
}

func TestGetStateReturnsVideoLookupError(t *testing.T) {
	repo := &MongoRepository{
		sessions: &mockCollection{
			findOneFunc: func(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) singleResult {
				return mockSingleResult{decodeFunc: func(v interface{}) error {
					session := v.(*UploadSession)
					*session = UploadSession{ID: "s1", VideoID: "v1"}
					return nil
				}}
			},
		},
		videos: &mockCollection{
			findOneFunc: func(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) singleResult {
				return mockSingleResult{decodeFunc: func(v interface{}) error { return errors.New("video broken") }}
			},
		},
	}

	_, err := repo.GetState(context.Background(), "s1")
	if err == nil || err.Error() != "find video: video broken" {
		t.Fatalf("GetState() error = %v", err)
	}
}

func TestListVideosUsesQueryAcrossFields(t *testing.T) {
	var capturedFilter interface{}
	repo := &MongoRepository{
		sessions: &mockCollection{},
		videos: &mockCollection{
			findFunc: func(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (mongoCursor, error) {
				capturedFilter = filter
				return &mockCursor{
					allFunc: func(ctx context.Context, results interface{}) error {
						videos := results.(*[]Video)
						*videos = append(*videos, Video{ID: "v1", Title: "Match"})
						return nil
					},
				}, nil
			},
		},
	}

	videos, err := repo.ListVideos(context.Background(), "match")
	if err != nil {
		t.Fatalf("ListVideos() error = %v", err)
	}
	if len(videos) != 1 {
		t.Fatalf("ListVideos() len = %d, want 1", len(videos))
	}

	filter, ok := capturedFilter.(bson.M)
	if !ok || filter["$or"] == nil {
		t.Fatalf("ListVideos() filter = %#v, want $or regex filter", capturedFilter)
	}
}

func TestPatchVideoReturnsNotFoundWhenNoRowsMatch(t *testing.T) {
	repo := &MongoRepository{
		sessions: &mockCollection{},
		videos: &mockCollection{
			updateOneFunc: func(ctx context.Context, filter, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
				return &mongo.UpdateResult{}, nil
			},
		},
	}

	_, err := repo.PatchVideo(context.Background(), "missing", bson.M{"title": "x"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("PatchVideo() error = %v, want ErrNotFound", err)
	}
}

func TestPatchVideoReturnsUpdatedDocument(t *testing.T) {
	repo := &MongoRepository{
		sessions: &mockCollection{},
		videos: &mockCollection{
			updateOneFunc: func(ctx context.Context, filter, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
				return &mongo.UpdateResult{MatchedCount: 1}, nil
			},
			findOneFunc: func(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) singleResult {
				return mockSingleResult{decodeFunc: func(v interface{}) error {
					video := v.(*Video)
					*video = Video{ID: "v1", Title: "Updated", UpdatedAt: time.Now()}
					return nil
				}}
			},
		},
	}

	video, err := repo.PatchVideo(context.Background(), "v1", bson.M{"title": "Updated"})
	if err != nil {
		t.Fatalf("PatchVideo() error = %v", err)
	}
	if video.Title != "Updated" {
		t.Fatalf("PatchVideo() = %+v", video)
	}
}

func TestPatchVideoWithEmptyPatchReadsCurrentDocument(t *testing.T) {
	repo := &MongoRepository{
		sessions: &mockCollection{},
		videos: &mockCollection{
			findOneFunc: func(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) singleResult {
				return mockSingleResult{decodeFunc: func(v interface{}) error {
					video := v.(*Video)
					*video = Video{ID: "v1", Title: "Existing"}
					return nil
				}}
			},
		},
	}

	video, err := repo.PatchVideo(context.Background(), "v1", bson.M{})
	if err != nil {
		t.Fatalf("PatchVideo() error = %v", err)
	}
	if video.Title != "Existing" {
		t.Fatalf("PatchVideo() = %+v", video)
	}
}

func TestDeleteSessionAndVideo(t *testing.T) {
	repo := &MongoRepository{
		sessions: &mockCollection{},
		videos:   &mockCollection{},
	}

	if err := repo.DeleteSession(context.Background(), "s1"); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}
	if err := repo.DeleteVideo(context.Background(), "v1"); err != nil {
		t.Fatalf("DeleteVideo() error = %v", err)
	}
}

func TestDeleteSessionAndVideoReturnErrors(t *testing.T) {
	repo := &MongoRepository{
		sessions: &mockCollection{
			deleteOneFunc: func(ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
				return nil, errors.New("session delete failed")
			},
		},
		videos: &mockCollection{
			deleteOneFunc: func(ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
				return nil, errors.New("video delete failed")
			},
		},
	}

	if err := repo.DeleteSession(context.Background(), "s1"); err == nil || err.Error() != "delete session: session delete failed" {
		t.Fatalf("DeleteSession() error = %v", err)
	}
	if err := repo.DeleteVideo(context.Background(), "v1"); err == nil || err.Error() != "delete video: video delete failed" {
		t.Fatalf("DeleteVideo() error = %v", err)
	}
}

func TestGetVideoHandlesErrors(t *testing.T) {
	notFoundRepo := &MongoRepository{
		sessions: &mockCollection{},
		videos: &mockCollection{
			findOneFunc: func(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) singleResult {
				return mockSingleResult{decodeFunc: func(v interface{}) error { return mongo.ErrNoDocuments }}
			},
		},
	}

	if _, err := notFoundRepo.GetVideo(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetVideo() error = %v, want ErrNotFound", err)
	}

	errRepo := &MongoRepository{
		sessions: &mockCollection{},
		videos: &mockCollection{
			findOneFunc: func(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) singleResult {
				return mockSingleResult{decodeFunc: func(v interface{}) error { return errors.New("decode failed") }}
			},
		},
	}

	if _, err := errRepo.GetVideo(context.Background(), "v1"); err == nil || err.Error() != "find video: decode failed" {
		t.Fatalf("GetVideo() error = %v", err)
	}
}

func TestListVideosReturnsFindAndDecodeErrors(t *testing.T) {
	findErrRepo := &MongoRepository{
		sessions: &mockCollection{},
		videos: &mockCollection{
			findFunc: func(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (mongoCursor, error) {
				return nil, errors.New("find failed")
			},
		},
	}

	if _, err := findErrRepo.ListVideos(context.Background(), ""); err == nil || err.Error() != "list videos: find failed" {
		t.Fatalf("ListVideos() error = %v", err)
	}

	decodeErrRepo := &MongoRepository{
		sessions: &mockCollection{},
		videos: &mockCollection{
			findFunc: func(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (mongoCursor, error) {
				return &mockCursor{
					allFunc: func(ctx context.Context, results interface{}) error {
						return errors.New("decode failed")
					},
				}, nil
			},
		},
	}

	if _, err := decodeErrRepo.ListVideos(context.Background(), ""); err == nil || err.Error() != "decode videos: decode failed" {
		t.Fatalf("ListVideos() error = %v", err)
	}
}
