.PHONY: up down api worker ui tidy run stop status test test-e2e build ci

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

ui:
	@echo "Starting UI in foreground..."
	@cd ui && npm run dev

# Combined development commands (background)
run:
	@echo "Starting worker, API, and UI in the background..."
	@echo "⏳ Waiting for Temporal to be ready (will wait up to 1 minute)..."
	@temporal_ready=false; \
	for i in $$(seq 1 60); do \
		remaining=$$((60 - i)); \
		if nc -z localhost 7233 >/dev/null 2>&1; then \
			echo "✅ Temporal is ready! ($$((60 - i + 1))s remaining)"; \
			temporal_ready=true; \
			break; \
		fi; \
		if [ $$i -eq 60 ]; then \
			echo "❌ Temporal failed to start after 60 seconds!"; \
			echo "💡 Try running 'make up' first to start Temporal server"; \
			exit 1; \
		else \
			echo "⏳ Checking Temporal... ($$remaining seconds remaining)"; \
			sleep 1; \
		fi; \
	done; \
	if [ "$$temporal_ready" = "true" ]; then \
		echo "🚀 Starting services..."; \
		INTEGRATION=1 go run ./cmd/worker & \
		go run ./cmd/api & \
		cd ui && npm run dev & \
		echo ""; \
		echo "🚀 All services are running!"; \
		echo "📱 React UI:     http://localhost:5173"; \
		echo "🔧 API Server:   http://localhost:8080"; \
		echo "⏰ Temporal UI:  http://localhost:8088"; \
		echo ""; \
		echo "Use 'make stop' to kill all processes."; \
	fi

stop:
	@echo "Stopping background processes..."
	@pkill -f "go run ./cmd/worker" || true
	@pkill -f "go run ./cmd/api" || true
	@pkill -f "npm run dev" || true
	@pkill -f "vite" || true
	@pkill -f "temporal-seats" || true
	@pkill -f "cmd/worker" || true
	@pkill -f "cmd/api" || true
	@sleep 1
	@pkill -9 -f "go run ./cmd/worker" 2>/dev/null || true
	@pkill -9 -f "go run ./cmd/api" 2>/dev/null || true
	@pkill -9 -f "npm run dev" 2>/dev/null || true
	@pkill -9 -f "vite" 2>/dev/null || true
	@pkill -9 -f "temporal-seats" 2>/dev/null || true
	@lsof -ti:8080 | xargs kill -9 2>/dev/null || true
	@lsof -ti:5173 | xargs kill -9 2>/dev/null || true
	@echo "Processes stopped."

status:
	@echo "Checking service status..."
	@echo "📱 React UI (5173):     $$(curl -s http://localhost:5173 >/dev/null 2>&1 && echo "✅ Running" || echo "❌ Not running")"
	@echo "🔧 API Server (8080):   $$(curl -s http://localhost:8080/health >/dev/null 2>&1 && echo "✅ Running" || echo "❌ Not running")"
	@echo "⏰ Temporal UI (8088):  $$(curl -s http://localhost:8088 >/dev/null 2>&1 && echo "✅ Running" || echo "❌ Not running")"
	@echo "🔄 Temporal Server:     $$(nc -z localhost 7233 >/dev/null 2>&1 && echo "✅ Running" || echo "❌ Not running")"

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