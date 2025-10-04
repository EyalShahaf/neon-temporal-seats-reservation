.PHONY: up down api worker tidy run stop test test-e2e build ci

# Docker commands
up:
	@echo "Starting Temporal + Temporal UI via Docker..."
	@docker compose -f infra/docker-compose.yml up -d

down:
	@echo "Stopping and removing Temporal containers..."
	@docker compose -f infra/docker-compose.yml down -v

# Go application commands (foreground)
api:
	@echo "Starting API server in foreground..."
	@go run ./cmd/api

worker:
	@echo "Starting Temporal worker in foreground..."
	@INTEGRATION=1 go run ./cmd/worker

# Combined development commands (background)
run:
	@echo "Starting worker and API in the background..."
	@echo "Waiting for Temporal to be ready..."
	@for i in $$(seq 1 30); do curl -s http://localhost:7233/api/v1/namespaces >/dev/null 2>&1 && break; sleep 1; done || echo "Temporal not ready after 30s, starting anyway..."
	@INTEGRATION=1 go run ./cmd/worker &
	@go run ./cmd/api &
	@echo "API and worker are running. Use 'make stop' to kill them."

stop:
	@echo "Stopping background Go processes..."
	@pkill -f "go run ./cmd/worker" || true
	@pkill -f "go run ./cmd/api" || true
	@pkill -f "temporal-seats" || true
	@pkill -f "cmd/worker" || true
	@pkill -f "cmd/api" || true
	@sleep 1
	@pkill -9 -f "go run ./cmd/worker" 2>/dev/null || true
	@pkill -9 -f "go run ./cmd/api" 2>/dev/null || true
	@pkill -9 -f "temporal-seats" 2>/dev/null || true
	@lsof -ti:8080 | xargs kill -9 2>/dev/null || true
	@echo "Processes stopped."

tidy:
	@go mod tidy

# Testing and Building
test:
	@echo "Running Go tests with race detector..."
	@go test -v -race ./...

test-e2e:
	@echo "Running E2E tests with full service integration..."
	@./scripts/run-e2e.sh

test-ui:
	@echo "Running UI tests..."
	@cd ui && npm test

test-all: test test-ui
	@echo "All unit and integration tests completed"

build:
	@echo "Building API and worker binaries..."
	@go build -o ./bin/api ./cmd/api
	@go build -o ./bin/worker ./cmd/worker
	@echo "Binaries are in ./bin/"

ci: tidy test build
	@echo "CI check passed: dependencies tidy, tests passed, binaries built."
