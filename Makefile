.PHONY: help build run-backend run-frontend run-all clean test test-race lint coverage tools

help:
	@echo "gnat - Load Testing Service"
	@echo ""
	@echo "Available targets:"
	@echo "  build          - Build both backend and frontend"
	@echo "  run-backend    - Run backend service"
	@echo "  run-frontend   - Run frontend service"
	@echo "  run-all        - Run both services concurrently"
	@echo "  clean          - Remove built binaries"
	@echo ""
	@echo "Development targets:"
	@echo "  test           - Run all tests"
	@echo "  test-race      - Run tests with race detector"
	@echo "  lint           - Run golangci-lint"
	@echo "  coverage       - Generate coverage report"
	@echo "  tools          - Install development tools"

build:
	@echo "Building backend..."
	@go build -o bin/gnat ./cmd/gnat
	@echo "Building frontend..."
	@go build -o bin/gnat-frontend ./cmd/gnat-frontend
	@echo "Build complete!"

run-backend:
	@APPLICATION_PORT=8778 go run ./cmd/gnat

run-frontend:
	@FRONTEND_PORT=3000 API_URL=http://localhost:8778 go run ./cmd/gnat-frontend

run-all:
	@echo "Starting backend on :8778 and frontend on :3000..."
	@APPLICATION_PORT=8778 go run ./cmd/gnat & \
	sleep 1 && \
	FRONTEND_PORT=3000 API_URL=http://localhost:8778 go run ./cmd/gnat-frontend

clean:
	@rm -rf bin/
	@echo "Cleaned build artifacts"

# Development targets

test:
	@go test ./...

test-race:
	@go test -race ./...

lint:
	@golangci-lint run

coverage:
	@go test -coverprofile=coverage.out -covermode=atomic \
		-coverpkg=./internal/config,./internal/models,./internal/storage/...,./internal/converters,./internal/service,./internal/runner,./internal/server/...,./pkg/... \
		./...
	@go tool cover -func=coverage.out
	@echo ""
	@echo "Coverage report generated: coverage.out"

tools:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
	@go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
	@echo "Tools installed successfully!"