package main

import (
	"os"
	"path/filepath"
	"testing"
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
