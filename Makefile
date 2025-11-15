.PHONY: help build run-backend run-frontend run-all clean

help:
	@echo "gnat - Load Testing Service"
	@echo ""
	@echo "Available targets:"
	@echo "  build          - Build both backend and frontend"
	@echo "  run-backend    - Run backend service"
	@echo "  run-frontend   - Run frontend service"
	@echo "  run-all        - Run both services concurrently"
	@echo "  clean          - Remove built binaries"

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