package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"streaming-ingest/internal/events"
	"streaming-ingest/internal/uploadstate"
	"streaming-ingest/internal/videos"
	"streaming-ingest/internal/webhooks"

	"github.com/gofiber/fiber/v2"
)

func TestLoadDotEnvLoadsUnsetVariables(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, ".env")

	if err := os.WriteFile(filename, []byte("TEST_LOADER=value\n"), 0o600); err != nil {
		t.Fatalf("failed to write temp env file: %v", err)
	}

	if err := loadDotEnv(filename); err != nil {
		t.Fatalf("loadDotEnv returned error: %v", err)
	}

	if got := os.Getenv("TEST_LOADER"); got != "value" {
		t.Fatalf("expected TEST_LOADER=value, got %q", got)
	}
}

func TestLoadDotEnvDoesNotOverrideExistingVariables(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, ".env")

	if err := os.WriteFile(filename, []byte("TEST_KEEP=from-file\n"), 0o600); err != nil {
		t.Fatalf("failed to write temp env file: %v", err)
	}

	t.Setenv("TEST_KEEP", "from-env")

	if err := loadDotEnv(filename); err != nil {
		t.Fatalf("loadDotEnv returned error: %v", err)
	}

	if got := os.Getenv("TEST_KEEP"); got != "from-env" {
		t.Fatalf("expected TEST_KEEP to remain from-env, got %q", got)
	}
}

func TestLoadDotEnvMissingFileAndMalformedInput(t *testing.T) {
	if err := loadDotEnv(filepath.Join(t.TempDir(), ".missing")); err != nil {
		t.Fatalf("expected missing file to be ignored, got %v", err)
	}

	dir := t.TempDir()
	invalid := filepath.Join(dir, ".env")
	if err := os.WriteFile(invalid, []byte("NOT_VALID\n"), 0o600); err != nil {
		t.Fatalf("failed to write invalid env file: %v", err)
	}

	if err := loadDotEnv(invalid); err == nil {
		t.Fatalf("expected malformed env file to return error")
	}
}

func TestConnectMongoInvalidURI(t *testing.T) {
	client, err := connectMongo("mongodb://127.0.0.1:1")
	if err == nil {
		if client != nil {
			_ = client.Disconnect(context.Background())
		}
		t.Fatalf("expected connection failure for invalid mongo endpoint")
	}
}

func TestNewAppAndRegisterRoutes(t *testing.T) {
	app := newApp()
	registerRoutes(
		app,
		events.NewHandler(events.NewService(nil, nil, nil)),
		webhooks.NewHandler(webhooks.NewService(nil, nil, videos.NewMemoryRepository(), nil)),
		videos.NewHandler(videos.NewService(nil, videos.NewMemoryRepository())),
		uploadstate.NewHandler(uploadstate.NewService(nil)),
	)

	req := httptest.NewRequest("GET", "/api/v1/videos", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode == 404 {
		t.Fatalf("route registration failed, got 404")
	}
}

func TestCreateStorageAdapters(t *testing.T) {
	t.Setenv("MINIO_ROOT_USER", "admin")
	t.Setenv("MINIO_ROOT_PASSWORD", "password123")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("AWS_REGION", "us-east-1")

	adapters := createStorageAdapters()
	if adapters["minio"] == nil || adapters["aws-s3"] == nil {
		t.Fatalf("expected both minio and aws-s3 adapters, got %+v", adapters)
	}
}

type mockShutdowner struct {
	shutdownFunc func() error
}

func (m *mockShutdowner) Shutdown() error {
	if m.shutdownFunc != nil {
		return m.shutdownFunc()
	}
	return nil
}

func TestInstallGracefulShutdown(t *testing.T) {
	signals := make(chan os.Signal, 1)
	done := make(chan struct{})
	app := &mockShutdowner{
		shutdownFunc: func() error {
			close(done)
			return nil
		},
	}

	installGracefulShutdown(app, signals)
	signals <- os.Interrupt

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("shutdown was not triggered")
	}
}

func TestRetry(t *testing.T) {
	t.Run("eventual success", func(t *testing.T) {
		attempts := 0
		got, err := retry(3, 0, func() (string, error) {
			attempts++
			if attempts < 3 {
				return "", errors.New("not yet")
			}
			return "ok", nil
		}, nil)
		if err != nil || got != "ok" {
			t.Fatalf("retry() = (%q, %v), want (ok, nil)", got, err)
		}
	})

	t.Run("invalid attempts", func(t *testing.T) {
		_, err := retry[string](0, 0, func() (string, error) {
			return "", nil
		}, nil)
		if err == nil {
			t.Fatalf("expected error for invalid attempts")
		}
	})
}

func TestRequireEnv(t *testing.T) {
	t.Setenv("REQUIRED_ENV_TEST", "value")
	if got := requireEnv("REQUIRED_ENV_TEST"); got != "value" {
		t.Fatalf("requireEnv() = %q, want value", got)
	}
}

func TestIsMongoAuthError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "auth failed",
			err:  testError("authentication failed"),
			want: true,
		},
		{
			name: "sasl conversation",
			err:  testError("connection() error occurred during connection handshake: auth error: sasl conversation error: unable to authenticate using mechanism \"SCRAM-SHA-1\""),
			want: true,
		},
		{
			name: "other failure",
			err:  testError("connection refused"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMongoAuthError(tt.err); got != tt.want {
				t.Fatalf("isMongoAuthError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRedactMongoURI(t *testing.T) {
	got := redactMongoURI("mongodb+srv://user:secret@cluster0.example.mongodb.net/db?retryWrites=true&w=majority")
	want := "mongodb+srv://user@cluster0.example.mongodb.net/db?retryWrites=true&w=majority"

	if got != want {
		t.Fatalf("redactMongoURI() = %q, want %q", got, want)
	}
}

type testError string

func (e testError) Error() string { return string(e) }

func TestMetricsEndpointExposesRED(t *testing.T) {
	app := newApp()
	app.Get("/__ping", func(c *fiber.Ctx) error { return c.SendString("ok") })

	// generate one request so the counter is non-zero
	if _, err := app.Test(httptest.NewRequest(http.MethodGet, "/__ping", nil)); err != nil {
		t.Fatalf("ping request: %v", err)
	}

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if err != nil {
		t.Fatalf("metrics request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metrics status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	for _, want := range []string{"http_requests_total", "http_request_duration_seconds", `service="streaming-ingest"`} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("metrics body missing %q", want)
		}
	}
}
