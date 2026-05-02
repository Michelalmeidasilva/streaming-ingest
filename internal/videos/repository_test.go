package videos

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// mockCollection is a mock implementation of mongoCollection for testing
type mockCollection struct {
	updateOneFunc func(ctx context.Context, filter, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error)
	findFunc      func(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (*mongo.Cursor, error)
}

func (m *mockCollection) UpdateOne(ctx context.Context, filter, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	if m.updateOneFunc != nil {
		return m.updateOneFunc(ctx, filter, update, opts...)
	}
	return &mongo.UpdateResult{UpsertedCount: 1}, nil
}

func (m *mockCollection) Find(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (*mongo.Cursor, error) {
	if m.findFunc != nil {
		return m.findFunc(ctx, filter, opts...)
	}
	return nil, nil
}

func TestNewMongoRepository(t *testing.T) {
	collection := &mockCollection{}
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
		name     string
		video    *Video
		mockErr  error
		wantErr  bool
		errMsg   string
	}{
		{
			name: "save valid video succeeds",
			video: &Video{
				VideoID:   "test-vid-123",
				Filename:  "test.mp4",
				Size:      1024,
				Provider:  "minio",
				Status:    "ready",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			mockErr: nil,
			wantErr: false,
		},
		{
			name: "save with database error",
			video: &Video{
				VideoID:   "test-vid-456",
				Filename:  "test2.mp4",
			},
			mockErr: mongo.ErrClientDisconnected,
			wantErr: true,
			errMsg:  "disconnected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockCollection{
				updateOneFunc: func(ctx context.Context, filter, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
					return &mongo.UpdateResult{UpsertedCount: 1}, tt.mockErr
				},
			}

			repo := &MongoRepository{collection: mock}
			err := repo.Save(context.Background(), tt.video)

			if (err != nil) != tt.wantErr {
				t.Errorf("Save() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
				t.Errorf("Save() error %q should contain %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestMongoRepositoryListAll(t *testing.T) {
	tests := []struct {
		name    string
		mockErr error
		wantErr bool
	}{
		{
			name:    "list all - find error",
			mockErr: errors.New("connection refused"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockCollection{
				findFunc: func(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (*mongo.Cursor, error) {
					return nil, tt.mockErr
				},
			}

			repo := &MongoRepository{collection: mock}
			videos, err := repo.ListAll(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("ListAll() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				if videos == nil {
					t.Errorf("ListAll() returned nil slice with no error")
				}
			}
		})
	}
}

func TestMongoRepositorySearch(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		mockErr error
		wantErr bool
	}{
		{
			name:    "search with error",
			query:   "test",
			mockErr: errors.New("search failed"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockCollection{
				findFunc: func(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (*mongo.Cursor, error) {
					return nil, tt.mockErr
				},
			}

			repo := &MongoRepository{collection: mock}
			videos, err := repo.Search(context.Background(), tt.query)

			if (err != nil) != tt.wantErr {
				t.Errorf("Search() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				if videos == nil {
					t.Errorf("Search() returned nil slice with no error")
				}
			}
		})
	}
}

func TestVideoRepository_Interface(t *testing.T) {
	// Compile-time assertion that MongoRepository implements VideoRepository
	var _ VideoRepository = (*MongoRepository)(nil)
}

// MockVideoRepository is a test mock for VideoRepository
type MockVideoRepository struct {
	SaveFunc    func(ctx context.Context, video *Video) error
	ListAllFunc func(ctx context.Context) ([]Video, error)
	SearchFunc  func(ctx context.Context, query string) ([]Video, error)
}

func (m *MockVideoRepository) Save(ctx context.Context, video *Video) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, video)
	}
	return nil
}

func (m *MockVideoRepository) ListAll(ctx context.Context) ([]Video, error) {
	if m.ListAllFunc != nil {
		return m.ListAllFunc(ctx)
	}
	return []Video{}, nil
}

func (m *MockVideoRepository) Search(ctx context.Context, query string) ([]Video, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(ctx, query)
	}
	return []Video{}, nil
}
