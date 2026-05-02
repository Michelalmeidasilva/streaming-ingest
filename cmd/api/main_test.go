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
