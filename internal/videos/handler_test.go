package videos

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

type mockVideoServiceForHandler struct {
	listResult   []VideoResponse
	listErr      error
	searchResult []VideoResponse
	searchErr    error
}

func (m *mockVideoServiceForHandler) ListAllVideos(ctx context.Context) ([]VideoResponse, error) {
	return m.listResult, m.listErr
}

func (m *mockVideoServiceForHandler) SearchVideos(ctx context.Context, query string) ([]VideoResponse, error) {
	return m.searchResult, m.searchErr
}

func setupVideosApp(handler *Handler) *fiber.App {
	app := fiber.New()
	v1 := app.Group("/api/v1")
	v1.Get("/videos", handler.ListVideos)
	v1.Get("/videos/search", handler.SearchVideos)
	return app
}

func doVideosRequest(app *fiber.App, method, path string) *http.Response {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	return resp
}

func TestListVideos(t *testing.T) {
	tests := []struct {
		name           string
		mockErr        error
		mockVideos     []VideoResponse
		wantStatusCode int
		checkResp      func(t *testing.T, body string)
	}{
		{
			name:           "service_fails",
			mockErr:        errors.New("db error"),
			mockVideos:     nil,
			wantStatusCode: fiber.StatusInternalServerError,
			checkResp: func(t *testing.T, body string) {
				if !contains(body, "error") {
					t.Errorf("Response should contain 'error', got %v", body)
				}
			},
		},
		{
			name:           "no_videos",
			mockErr:        nil,
			mockVideos:     []VideoResponse{},
			wantStatusCode: fiber.StatusOK,
			checkResp: func(t *testing.T, body string) {
				var resp map[string]interface{}
				json.Unmarshal([]byte(body), &resp)
				if videos, ok := resp["videos"]; !ok || videos == nil {
					t.Errorf("Response should contain 'videos' field")
				}
			},
		},
		{
			name:     "with_videos",
			mockErr:  nil,
			mockVideos: []VideoResponse{
				{
					VideoID:   "vid1",
					Filename:  "file.mp4",
					Provider:  "minio",
					Status:    "ready",
					CreatedAt: time.Now().Format("2006-01-02T15:04:05Z"),
				},
			},
			wantStatusCode: fiber.StatusOK,
			checkResp: func(t *testing.T, body string) {
				var resp map[string]interface{}
				json.Unmarshal([]byte(body), &resp)
				if videos, ok := resp["videos"].([]interface{}); !ok || len(videos) != 1 {
					t.Errorf("Response should contain 'videos' array with 1 item")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHandler(&mockVideoServiceForHandler{
				listResult: tt.mockVideos,
				listErr:    tt.mockErr,
			})
			app := setupVideosApp(handler)

			resp := doVideosRequest(app, "GET", "/api/v1/videos")

			if resp.StatusCode != tt.wantStatusCode {
				t.Errorf("StatusCode = %d, want %d", resp.StatusCode, tt.wantStatusCode)
			}

			respBody, _ := io.ReadAll(resp.Body)
			tt.checkResp(t, string(respBody))
		})
	}
}

func TestSearchVideos(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		mockErr        error
		mockVideos     []VideoResponse
		wantStatusCode int
		checkResp      func(t *testing.T, body string)
	}{
		{
			name:           "empty_query_delegates_to_list",
			query:          "",
			mockErr:        nil,
			mockVideos:     []VideoResponse{},
			wantStatusCode: fiber.StatusOK,
			checkResp: func(t *testing.T, body string) {
				var resp map[string]interface{}
				json.Unmarshal([]byte(body), &resp)
				if _, ok := resp["videos"]; !ok {
					t.Errorf("Response should contain 'videos' field")
				}
			},
		},
		{
			name:           "search_fails",
			query:          "test",
			mockErr:        errors.New("search error"),
			mockVideos:     nil,
			wantStatusCode: fiber.StatusInternalServerError,
			checkResp: func(t *testing.T, body string) {
				if !contains(body, "error") {
					t.Errorf("Response should contain 'error', got %v", body)
				}
			},
		},
		{
			name:     "search_success",
			query:    "test",
			mockErr:  nil,
			mockVideos: []VideoResponse{
				{
					VideoID:   "vid1",
					Filename:  "test.mp4",
					Provider:  "minio",
					Status:    "ready",
					CreatedAt: time.Now().Format("2006-01-02T15:04:05Z"),
				},
			},
			wantStatusCode: fiber.StatusOK,
			checkResp: func(t *testing.T, body string) {
				var resp map[string]interface{}
				json.Unmarshal([]byte(body), &resp)
				if videos, ok := resp["videos"].([]interface{}); !ok || len(videos) != 1 {
					t.Errorf("Response should contain 'videos' array with 1 item")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHandler(&mockVideoServiceForHandler{
				searchResult: tt.mockVideos,
				searchErr:    tt.mockErr,
				// For empty query test (delegates to ListVideos)
				listResult: tt.mockVideos,
				listErr:    tt.mockErr,
			})
			app := setupVideosApp(handler)

			path := "/api/v1/videos/search"
			if tt.query != "" {
				path += "?q=" + tt.query
			}
			resp := doVideosRequest(app, "GET", path)

			if resp.StatusCode != tt.wantStatusCode {
				t.Errorf("StatusCode = %d, want %d", resp.StatusCode, tt.wantStatusCode)
			}

			respBody, _ := io.ReadAll(resp.Body)
			tt.checkResp(t, string(respBody))
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
