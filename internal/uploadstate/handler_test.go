package uploadstate

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
)

type stubRepo struct {
	saveStateFunc     func(ctx context.Context, state UploadState) error
	getStateFunc      func(ctx context.Context, sessionID string) (*UploadState, error)
	deleteSessionFunc func(ctx context.Context, sessionID string) error
	saveVideoFunc     func(ctx context.Context, video Video) error
	getVideoFunc      func(ctx context.Context, videoID string) (*Video, error)
	listVideosFunc    func(ctx context.Context, query string) ([]Video, error)
	patchVideoFunc    func(ctx context.Context, videoID string, patch bson.M) (*Video, error)
	deleteVideoFunc   func(ctx context.Context, videoID string) error
}

func (s stubRepo) SaveState(ctx context.Context, state UploadState) error {
	return s.saveStateFunc(ctx, state)
}
func (s stubRepo) GetState(ctx context.Context, sessionID string) (*UploadState, error) {
	return s.getStateFunc(ctx, sessionID)
}
func (s stubRepo) DeleteSession(ctx context.Context, sessionID string) error {
	return s.deleteSessionFunc(ctx, sessionID)
}
func (s stubRepo) SaveVideo(ctx context.Context, video Video) error {
	return s.saveVideoFunc(ctx, video)
}
func (s stubRepo) GetVideo(ctx context.Context, videoID string) (*Video, error) {
	return s.getVideoFunc(ctx, videoID)
}
func (s stubRepo) ListVideos(ctx context.Context, query string) ([]Video, error) {
	return s.listVideosFunc(ctx, query)
}
func (s stubRepo) PatchVideo(ctx context.Context, videoID string, patch bson.M) (*Video, error) {
	return s.patchVideoFunc(ctx, videoID, patch)
}
func (s stubRepo) DeleteVideo(ctx context.Context, videoID string) error {
	return s.deleteVideoFunc(ctx, videoID)
}

func setupUploadStateApp(handler *Handler) *fiber.App {
	app := fiber.New()
	v1 := app.Group("/api/v1/upload-state")
	v1.Put("/sessions/:sessionId", handler.SaveState)
	v1.Get("/sessions/:sessionId", handler.GetState)
	v1.Delete("/sessions/:sessionId", handler.DeleteSession)
	v1.Put("/videos/:videoId", handler.SaveVideo)
	v1.Get("/videos", handler.ListVideos)
	v1.Get("/videos/:videoId", handler.GetVideo)
	v1.Patch("/videos/:videoId", handler.PatchVideo)
	v1.Delete("/videos/:videoId", handler.DeleteVideo)
	return app
}

func TestUploadStateHandlers(t *testing.T) {
	handler := NewHandler(NewService(stubRepo{
		saveStateFunc: func(ctx context.Context, state UploadState) error { return nil },
		getStateFunc: func(ctx context.Context, sessionID string) (*UploadState, error) {
			return &UploadState{Session: UploadSession{ID: sessionID}, Video: Video{ID: "v1"}}, nil
		},
		deleteSessionFunc: func(ctx context.Context, sessionID string) error { return nil },
		saveVideoFunc:     func(ctx context.Context, video Video) error { return nil },
		getVideoFunc: func(ctx context.Context, videoID string) (*Video, error) {
			return &Video{ID: videoID, Title: "Stored"}, nil
		},
		listVideosFunc: func(ctx context.Context, query string) ([]Video, error) {
			return []Video{{ID: "v1", Title: "Stored"}}, nil
		},
		patchVideoFunc: func(ctx context.Context, videoID string, patch bson.M) (*Video, error) {
			return &Video{ID: videoID, Title: patch["title"].(string)}, nil
		},
		deleteVideoFunc: func(ctx context.Context, videoID string) error { return nil },
	}))
	app := setupUploadStateApp(handler)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		status int
	}{
		{"save state", http.MethodPut, "/api/v1/upload-state/sessions/s1", `{"session":{"id":"s1","videoId":"v1"},"video":{"id":"v1"}}`, fiber.StatusNoContent},
		{"get state", http.MethodGet, "/api/v1/upload-state/sessions/s1", "", fiber.StatusOK},
		{"delete session", http.MethodDelete, "/api/v1/upload-state/sessions/s1", "", fiber.StatusNoContent},
		{"save video", http.MethodPut, "/api/v1/upload-state/videos/v1", `{"id":"v1","title":"Video"}`, fiber.StatusNoContent},
		{"list videos", http.MethodGet, "/api/v1/upload-state/videos?q=vid", "", fiber.StatusOK},
		{"get video", http.MethodGet, "/api/v1/upload-state/videos/v1", "", fiber.StatusOK},
		{"patch video", http.MethodPatch, "/api/v1/upload-state/videos/v1", `{"title":"Updated"}`, fiber.StatusOK},
		{"delete video", http.MethodDelete, "/api/v1/upload-state/videos/v1", "", fiber.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			if resp.StatusCode != tt.status {
				t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, tt.status)
			}
		})
	}
}

func TestUploadStateHandlerNotFound(t *testing.T) {
	handler := NewHandler(NewService(stubRepo{
		saveStateFunc:     func(ctx context.Context, state UploadState) error { return nil },
		getStateFunc:      func(ctx context.Context, sessionID string) (*UploadState, error) { return nil, ErrNotFound },
		deleteSessionFunc: func(ctx context.Context, sessionID string) error { return nil },
		saveVideoFunc:     func(ctx context.Context, video Video) error { return nil },
		getVideoFunc:      func(ctx context.Context, videoID string) (*Video, error) { return nil, ErrNotFound },
		listVideosFunc:    func(ctx context.Context, query string) ([]Video, error) { return nil, nil },
		patchVideoFunc: func(ctx context.Context, videoID string, patch bson.M) (*Video, error) {
			return nil, ErrNotFound
		},
		deleteVideoFunc: func(ctx context.Context, videoID string) error { return nil },
	}))
	app := setupUploadStateApp(handler)

	for _, path := range []string{
		"/api/v1/upload-state/sessions/missing",
		"/api/v1/upload-state/videos/missing",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp, _ := app.Test(req, -1)
		if resp.StatusCode != fiber.StatusNotFound {
			t.Fatalf("GET %s status = %d, want 404", path, resp.StatusCode)
		}
	}

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/upload-state/videos/missing", strings.NewReader(`{"title":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("PATCH missing status = %d, want 404", resp.StatusCode)
	}
}

func TestUploadStateHandlerValidationAndErrors(t *testing.T) {
	handler := NewHandler(NewService(stubRepo{
		saveStateFunc:     func(ctx context.Context, state UploadState) error { return errors.New("boom") },
		getStateFunc:      func(ctx context.Context, sessionID string) (*UploadState, error) { return nil, errors.New("boom") },
		deleteSessionFunc: func(ctx context.Context, sessionID string) error { return errors.New("boom") },
		saveVideoFunc:     func(ctx context.Context, video Video) error { return errors.New("boom") },
		getVideoFunc:      func(ctx context.Context, videoID string) (*Video, error) { return nil, errors.New("boom") },
		listVideosFunc:    func(ctx context.Context, query string) ([]Video, error) { return nil, errors.New("boom") },
		patchVideoFunc: func(ctx context.Context, videoID string, patch bson.M) (*Video, error) {
			return nil, errors.New("boom")
		},
		deleteVideoFunc: func(ctx context.Context, videoID string) error { return errors.New("boom") },
	}))
	app := setupUploadStateApp(handler)

	resp, _ := app.Test(httptest.NewRequest(http.MethodPut, "/api/v1/upload-state/sessions/s1", strings.NewReader(`{`)), -1)
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("invalid state body status = %d, want 400", resp.StatusCode)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/upload-state/videos/v1", strings.NewReader(`{"title":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("missing video id status = %d, want 400", resp.StatusCode)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/upload-state/videos", nil)
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("list error status = %d, want 500", resp.StatusCode)
	}
}

func TestUploadStateListResponseShape(t *testing.T) {
	handler := NewHandler(NewService(stubRepo{
		saveStateFunc:     func(ctx context.Context, state UploadState) error { return nil },
		getStateFunc:      func(ctx context.Context, sessionID string) (*UploadState, error) { return nil, ErrNotFound },
		deleteSessionFunc: func(ctx context.Context, sessionID string) error { return nil },
		saveVideoFunc:     func(ctx context.Context, video Video) error { return nil },
		getVideoFunc:      func(ctx context.Context, videoID string) (*Video, error) { return nil, ErrNotFound },
		listVideosFunc: func(ctx context.Context, query string) ([]Video, error) {
			return []Video{{ID: "v1", Title: "Stored"}}, nil
		},
		patchVideoFunc:  func(ctx context.Context, videoID string, patch bson.M) (*Video, error) { return nil, nil },
		deleteVideoFunc: func(ctx context.Context, videoID string) error { return nil },
	}))
	app := setupUploadStateApp(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/upload-state/videos", nil)
	resp, _ := app.Test(req, -1)
	var body map[string][]Video
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body["videos"]) != 1 {
		t.Fatalf("videos len = %d, want 1", len(body["videos"]))
	}
}
