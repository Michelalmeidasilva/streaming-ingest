.PHONY: help deps dev build test stop

help:
	@echo "streaming-ingest targets:"
	@echo "  deps              - Download and install Go dependencies"
	@echo "  dev               - Run with go run ./cmd/api (port 8080)"
	@echo "  build             - Compile binary to bin/ingest"
	@echo "  test              - Run unit tests (excludes integration tests)"
	@echo "  test-integration  - Run all tests including integration tests"
	@echo "  stop              - Stop running instance"

deps:
	go mod tidy
	go mod download
	@echo "✓ Dependencies installed"

dev: deps
	go run ./cmd/api

build: deps
	mkdir -p bin
	go build -o bin/ingest ./cmd/api

test: deps
	go test -v -coverprofile=coverage.out ./internal/...
	go tool cover -func=coverage.out

test-integration: deps
	go test -v -tags integration ./internal/...

stop:
	pkill -f "go run.*cmd/api" || true
	@echo "✓ Stopped streaming-ingest"
