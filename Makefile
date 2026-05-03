.PHONY: help deps dev build test test-mongo-integration test-integration stop

GOCACHE ?= /private/tmp/go-cache

GOCACHE ?= /private/tmp/go-cache

help:
	@echo "streaming-ingest targets:"
	@echo "  deps              - Download and install Go dependencies"
	@echo "  dev               - Start infra if needed, then run go run ./cmd/api"
	@echo "  build             - Compile binary to bin/ingest"
	@echo "  test              - Run unit tests (excludes integration tests)"
	@echo "  test-mongo-integration - Run MongoDB persistence integration tests"
	@echo "  test-integration  - Run all tests including integration tests"
	@echo "  stop              - Stop running instance"

deps:
	GOCACHE=$(GOCACHE) go mod tidy
	GOCACHE=$(GOCACHE) go mod download
	@echo "✓ Dependencies installed"

dev: deps
	@if lsof -ti tcp:8080 >/dev/null 2>&1; then lsof -ti tcp:8080 | xargs kill -9; fi
	GOCACHE=$(GOCACHE) go run ./cmd/api

build: deps
	mkdir -p bin
	GOCACHE=$(GOCACHE) go build -o bin/ingest ./cmd/api

test: deps
	GOCACHE=$(GOCACHE) go test -v -coverprofile=coverage.out ./internal/...
	GOCACHE=$(GOCACHE) go tool cover -func=coverage.out

test-mongo-integration: deps
	@if [ -z "$$MONGODB_URI" ]; then echo "MONGODB_URI is required"; exit 1; fi
	GOCACHE=$(GOCACHE) go test -v ./internal/videos ./internal/webhooks ./cmd/api

test-integration: deps
	GOCACHE=$(GOCACHE) go test -v -tags integration ./internal/...

stop:
	@if lsof -ti tcp:8080 >/dev/null 2>&1; then lsof -ti tcp:8080 | xargs kill -9; fi
	@echo "✓ Stopped streaming-ingest"
