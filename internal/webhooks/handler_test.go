package webhooks

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

type mockWebhookProcessor struct {
	err error
}

func (m *mockWebhookProcessor) ProcessWebhook(provider string, payload []byte) error {
	return m.err
}

func setupWebhookApp(handler *Handler) *fiber.App {
	app := fiber.New()
	v1 := app.Group("/api/v1")
	v1.Post("/webhooks/storage/:provider", handler.HandleProviderWebhook)
	return app
}

func doWebhookRequest(app *fiber.App, provider string, body io.Reader) *http.Response {
	req := httptest.NewRequest("POST", "/api/v1/webhooks/storage/"+provider, body)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	return resp
}

func TestHandleProviderWebhook(t *testing.T) {
	tests := []struct {
		name           string
		provider       string
		payload        string
		mockErr        error
		wantStatusCode int
		checkResp      func(t *testing.T, body string)
	}{
		{
			name:           "service_fails",
			provider:       "minio",
			payload:        `{"Records":[]}`,
			mockErr:        errors.New("bad event"),
			wantStatusCode: fiber.StatusBadRequest,
			checkResp: func(t *testing.T, body string) {
				if !contains(body, "bad event") {
					t.Errorf("Response should contain 'bad event', got %v", body)
				}
			},
		},
		{
			name:           "success",
			provider:       "minio",
			payload:        `{"Records":[]}`,
			mockErr:        nil,
			wantStatusCode: fiber.StatusOK,
			checkResp: func(t *testing.T, body string) {
				if !contains(body, "Webhook processed successfully") {
					t.Errorf("Response should contain 'Webhook processed successfully', got %v", body)
				}
			},
		},
		{
			name:           "different_provider",
			provider:       "aws-s3",
			payload:        `{"Records":[]}`,
			mockErr:        nil,
			wantStatusCode: fiber.StatusOK,
			checkResp: func(t *testing.T, body string) {
				if !contains(body, "Webhook processed successfully") {
					t.Errorf("Response should contain 'Webhook processed successfully', got %v", body)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHandler(&mockWebhookProcessor{err: tt.mockErr})
			app := setupWebhookApp(handler)

			bodyReader := bytes.NewBufferString(tt.payload)
			resp := doWebhookRequest(app, tt.provider, bodyReader)

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
