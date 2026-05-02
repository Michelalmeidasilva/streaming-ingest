package events

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

type mockEventProcessor struct {
	processEventErr error
}

func (m *mockEventProcessor) ProcessEvent(event FrontEndEvent) error {
	return m.processEventErr
}

func setupEventApp(handler *Handler) *fiber.App {
	app := fiber.New()
	v1 := app.Group("/api/v1")
	v1.Post("/events", handler.ReceiveEvent)
	return app
}

func doEventRequest(app *fiber.App, body io.Reader) *http.Response {
	req := httptest.NewRequest("POST", "/api/v1/events", body)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	return resp
}

func TestReceiveEvent(t *testing.T) {
	tests := []struct {
		name           string
		body           interface{}
		mockErr        error
		wantStatusCode int
		checkResp      func(t *testing.T, body string)
	}{
		{
			name:           "body_parser_fails",
			body:           "{invalid",
			mockErr:        nil,
			wantStatusCode: fiber.StatusBadRequest,
			checkResp: func(t *testing.T, body string) {
				if !contains(body, "Cannot parse JSON") {
					t.Errorf("Response should contain 'Cannot parse JSON', got %v", body)
				}
			},
		},
		{
			name:           "event_type_empty",
			body:           FrontEndEvent{EventType: "", Payload: EventPayload{"data": "test"}},
			mockErr:        nil,
			wantStatusCode: fiber.StatusBadRequest,
			checkResp: func(t *testing.T, body string) {
				if !contains(body, "eventType is required") {
					t.Errorf("Response should contain 'eventType is required', got %v", body)
				}
			},
		},
		{
			name:           "service_fails",
			body:           FrontEndEvent{EventType: "upload.progress", Payload: EventPayload{"data": "test"}},
			mockErr:        errors.New("service error"),
			wantStatusCode: fiber.StatusInternalServerError,
			checkResp: func(t *testing.T, body string) {
				if !contains(body, "Failed to process event") {
					t.Errorf("Response should contain 'Failed to process event', got %v", body)
				}
			},
		},
		{
			name:           "success",
			body:           FrontEndEvent{EventType: "upload.progress", Payload: EventPayload{"data": "test"}},
			mockErr:        nil,
			wantStatusCode: fiber.StatusAccepted,
			checkResp: func(t *testing.T, body string) {
				if !contains(body, "Event accepted") {
					t.Errorf("Response should contain 'Event accepted', got %v", body)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHandler(&mockEventProcessor{processEventErr: tt.mockErr})
			app := setupEventApp(handler)

			var bodyReader io.Reader
			if str, ok := tt.body.(string); ok {
				bodyReader = bytes.NewBufferString(str)
			} else {
				jsonBody, _ := json.Marshal(tt.body)
				bodyReader = bytes.NewReader(jsonBody)
			}

			resp := doEventRequest(app, bodyReader)

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
