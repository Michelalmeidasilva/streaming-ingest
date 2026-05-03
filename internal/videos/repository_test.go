package videos

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// mockCollection is a mock implementation of mongoCollection for testing
type mockCollection struct {
	insertOneFunc func(ctx context.Context, document interface{}, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error)
	updateOneFunc func(ctx context.Context, filter, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error)
	findFunc      func(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (*mongo.Cursor, error)
	insertCalls   int
	updateCalls   int
}

func (m *mockCollection) InsertOne(ctx context.Context, document interface{}, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error) {
	m.insertCalls++
	if m.insertOneFunc != nil {
		return m.insertOneFunc(ctx, document, opts...)
	}
	return &mongo.InsertOneResult{}, nil
}

func (m *mockCollection) UpdateOne(ctx context.Context, filter, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	m.updateCalls++
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
		name    string
		video   *Video
		mockErr error
		wantErr bool
		errMsg  string
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
				VideoID:  "test-vid-456",
				Filename: "test2.mp4",
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
			stdout := os.Stdout
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("os.Pipe() error = %v", err)
			}
			os.Stdout = w

			saveErr := repo.Save(context.Background(), tt.video)

			if err := w.Close(); err != nil {
				t.Fatalf("close stdout writer: %v", err)
			}
			os.Stdout = stdout

			out, err := io.ReadAll(r)
			if err != nil {
				t.Fatalf("read stdout: %v", err)
			}
			_ = r.Close()

			if (saveErr != nil) != tt.wantErr {
				t.Errorf("Save() error = %v, wantErr %v", saveErr, tt.wantErr)
			}
			if tt.wantErr && saveErr != nil && tt.errMsg != "" && !contains(saveErr.Error(), tt.errMsg) {
				t.Errorf("Save() error %q should contain %q", saveErr.Error(), tt.errMsg)
			}
			if tt.wantErr && !strings.Contains(string(out), "error saving video to database") {
				t.Errorf("expected stdout to contain save error message, got %q", string(out))
			}
			if mock.updateCalls != 1 {
				t.Errorf("UpdateOne() calls = %d, want 1", mock.updateCalls)
			}
		})
	}
}

func TestMongoRepositoryCreate(t *testing.T) {
	tests := []struct {
		name    string
		video   *Video
		mockErr error
		wantErr bool
		errMsg  string
	}{
		{
			name: "create valid video succeeds",
			video: &Video{
				VideoID:   "test-vid-123",
				Filename:  "test.mp4",
				Size:      1024,
				Provider:  "minio",
				Status:    "uploading",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			mockErr: nil,
			wantErr: false,
		},
		{
			name: "create with database error",
			video: &Video{
				VideoID:  "test-vid-456",
				Filename: "test2.mp4",
			},
			mockErr: mongo.ErrClientDisconnected,
			wantErr: true,
			errMsg:  "disconnected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockCollection{
				insertOneFunc: func(ctx context.Context, document interface{}, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error) {
					return &mongo.InsertOneResult{}, tt.mockErr
				},
			}

			repo := &MongoRepository{collection: mock}
			createErr := repo.Create(context.Background(), tt.video)

			if (createErr != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", createErr, tt.wantErr)
			}
			if tt.wantErr && createErr != nil && tt.errMsg != "" && !contains(createErr.Error(), tt.errMsg) {
				t.Errorf("Create() error %q should contain %q", createErr.Error(), tt.errMsg)
			}
			if mock.insertCalls != 1 {
				t.Errorf("InsertOne() calls = %d, want 1", mock.insertCalls)
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

func TestMemoryRepository(t *testing.T) {
	repo := NewMemoryRepository()

	first := &Video{
		VideoID:  "vid-1",
		Filename: "intro.mp4",
		Provider: "minio",
		Status:   "uploaded",
	}
	if err := repo.Save(context.Background(), first); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	second := &Video{
		VideoID:  "vid-2",
		Filename: "lesson.mp4",
		Provider: "minio",
		Status:   "uploaded",
	}
	if err := repo.Save(context.Background(), second); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	updated := &Video{
		VideoID:  "vid-1",
		Filename: "intro-final.mp4",
		Provider: "minio",
		Status:   "ready",
	}
	if err := repo.Save(context.Background(), updated); err != nil {
		t.Fatalf("Save() update error = %v", err)
	}

	all, err := repo.ListAll(context.Background())
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("ListAll() len = %d, want 2", len(all))
	}

	found, err := repo.Search(context.Background(), "intro")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("Search() len = %d, want 1", len(found))
	}
	if found[0].Filename != "intro-final.mp4" {
		t.Fatalf("Search() filename = %s, want intro-final.mp4", found[0].Filename)
	}
}

// MockVideoRepository is a test mock for VideoRepository
type MockVideoRepository struct {
	CreateFunc  func(ctx context.Context, video *Video) error
	SaveFunc    func(ctx context.Context, video *Video) error
	ListAllFunc func(ctx context.Context) ([]Video, error)
	SearchFunc  func(ctx context.Context, query string) ([]Video, error)
}

func (m *MockVideoRepository) Create(ctx context.Context, video *Video) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, video)
	}
	return nil
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
