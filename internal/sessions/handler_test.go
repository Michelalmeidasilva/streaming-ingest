package sessions

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// ---- stubs ----

type stubSessionReader struct {
	listFilter SessionFilter
	listResult []Session
}

func (s *stubSessionReader) List(_ context.Context, f SessionFilter) ([]Session, error) {
	s.listFilter = f
	return s.listResult, nil
}

type stubSessionWriter struct{ last Session }

func (s *stubSessionWriter) CreateSession(_ context.Context, sess Session) error {
	s.last = sess
	return nil
}

// ---- handler tests ----

func TestCreateSessionStoresDoc(t *testing.T) {
	w := &stubSessionWriter{}
	app := fiber.New()
	h := NewHandler(&stubSessionReader{})
	h.SetWriter(w)
	app.Post("/api/v1/benchmark-sessions", h.CreateSession)

	body := `{"sessionId":"s1","instanceTypes":["c5.xlarge","g6.xlarge"],"requestedBy":"a@x.com"}`
	req := httptest.NewRequest("POST", "/api/v1/benchmark-sessions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 202 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if w.last.SessionID != "s1" {
		t.Fatalf("sessionId not stored: got %q", w.last.SessionID)
	}
	if len(w.last.InstanceTypes) != 2 || w.last.InstanceTypes[0] != "c5.xlarge" {
		t.Fatalf("instanceTypes not stored: %v", w.last.InstanceTypes)
	}
	if w.last.RequestedBy != "a@x.com" {
		t.Fatalf("requestedBy not stored: got %q", w.last.RequestedBy)
	}
}

func TestListSessionsReturnsAll(t *testing.T) {
	stub := &stubSessionReader{
		listResult: []Session{
			{SessionID: "s1", InstanceTypes: []string{"c5.xlarge"}},
			{SessionID: "s2", InstanceTypes: []string{"g6.xlarge"}},
		},
	}
	app := fiber.New()
	h := NewHandler(stub)
	app.Get("/api/v1/benchmark-sessions", h.ListSessions)

	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/benchmark-sessions", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Sessions []Session `json:"sessions"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Sessions) != 2 {
		t.Fatalf("want 2 sessions, got %d", len(out.Sessions))
	}
}

func TestListSessionsFiltersSessionID(t *testing.T) {
	stub := &stubSessionReader{
		listResult: []Session{{SessionID: "s1", InstanceTypes: []string{"c5.xlarge"}}},
	}
	app := fiber.New()
	h := NewHandler(stub)
	app.Get("/api/v1/benchmark-sessions", h.ListSessions)

	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/benchmark-sessions?sessionId=s1", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if stub.listFilter.SessionID != "s1" {
		t.Fatalf("sessionId filter not applied: got %q", stub.listFilter.SessionID)
	}
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Sessions []Session `json:"sessions"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Sessions) != 1 {
		t.Fatalf("want 1 session, got %d", len(out.Sessions))
	}
}
