#!/bin/bash

# E2E Test Runner Script
# This script runs the end-to-end tests with proper dependency management

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TIMEOUT=120 # 2 minutes (reduced from 5 minutes)

echo -e "${BLUE}🚀 Temporal Seats E2E Test Runner${NC}"
echo "=================================="

# Check if we're in the right directory
if [ ! -f "$PROJECT_ROOT/go.mod" ]; then
    echo -e "${RED}❌ Error: go.mod not found. Please run this script from the project root.${NC}"
    exit 1
fi

cd "$PROJECT_ROOT"

# Check dependencies
echo -e "${YELLOW}🔍 Checking dependencies...${NC}"

# Check Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}❌ Docker not found. E2E tests require Docker to run Temporal server and PostgreSQL database.${NC}"
    exit 1
fi

# Check docker-compose
if ! command -v docker-compose &> /dev/null; then
    echo -e "${RED}❌ docker-compose not found. E2E tests require docker-compose to manage Temporal services.${NC}"
    exit 1
fi

# Check Go
if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ Go not found. E2E tests require Go 1.21 or later.${NC}"
    exit 1
fi

# Check if Docker is running
if ! docker info &> /dev/null; then
    echo -e "${RED}❌ Docker is not running. E2E tests require Docker to run Temporal server and PostgreSQL database.${NC}"
    exit 1
fi

echo -e "${GREEN}✅ All dependencies found${NC}"

# Check if ports are available
echo -e "${YELLOW}🔍 Checking port availability...${NC}"

check_port() {
    local port=$1
    local service=$2
    if lsof -i :$port &> /dev/null; then
        echo -e "${YELLOW}⚠️  Port $port is in use. $service may already be running.${NC}"
        echo "   E2E tests require exclusive access to ports 7233, 8088, and 8080"
        echo "   You can either:"
        echo "   1. Stop the existing service using port $port"
        echo "   2. Or run: SKIP_E2E=true go test ./test/e2e/... to skip E2E tests"
        read -p "   Continue anyway? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
}

check_port 7233 "Temporal"
check_port 8088 "Temporal UI"
check_port 8080 "API Server"

echo -e "${GREEN}✅ Ports available${NC}"

# Cleanup function
cleanup() {
    echo -e "${YELLOW}🧹 Cleaning up...${NC}"
    
    # Kill any remaining processes first (faster)
    pkill -f "go run cmd/api/main.go" &> /dev/null || true
    pkill -f "go run cmd/worker/main.go" &> /dev/null || true
    
    # Stop any running docker-compose services with timeout
    if [ -f "infra/docker-compose.yml" ]; then
        echo -e "${YELLOW}🛑 Stopping Docker services...${NC}"
        timeout 15 docker-compose -f infra/docker-compose.yml down --timeout 10 &> /dev/null || {
            echo -e "${YELLOW}⚠️  Docker cleanup timed out, forcing stop...${NC}"
            docker-compose -f infra/docker-compose.yml kill &> /dev/null || true
            docker-compose -f infra/docker-compose.yml rm -f &> /dev/null || true
        }
    fi
    
    echo -e "${GREEN}✅ Cleanup completed${NC}"
}

# Set up trap for cleanup
trap cleanup EXIT

# Build the project
echo -e "${YELLOW}🔨 Building project...${NC}"
if ! go build ./cmd/api/main.go && go build ./cmd/worker/main.go; then
    echo -e "${RED}❌ Build failed${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Build successful${NC}"

# Run E2E tests
echo -e "${YELLOW}🧪 Running E2E tests...${NC}"
echo "   This may take a few minutes as it starts all services..."
echo "   Timeout: ${TIMEOUT}s"
echo

# Set timeout for the test run
if command -v timeout &> /dev/null; then
    # Use timeout command if available (Linux)
    timeout $TIMEOUT go test -v ./test/e2e/... || {
        exit_code=$?
        if [ $exit_code -eq 124 ]; then
            echo -e "${RED}❌ E2E tests timed out after ${TIMEOUT}s${NC}"
            echo "   This usually means:"
            echo "   1. Temporal server took too long to start"
            echo "   2. API server or worker process is hanging"
            echo "   3. Network connectivity issues with required services"
        else
            echo -e "${RED}❌ E2E tests failed with exit code $exit_code${NC}"
        fi
        exit $exit_code
    }
else
    # Use gtimeout if available (macOS with coreutils)
    if command -v gtimeout &> /dev/null; then
        gtimeout $TIMEOUT go test -v ./test/e2e/... || {
            exit_code=$?
            if [ $exit_code -eq 124 ]; then
                echo -e "${RED}❌ E2E tests timed out after ${TIMEOUT}s${NC}"
                echo "   This usually means:"
                echo "   1. Temporal server took too long to start"
                echo "   2. API server or worker process is hanging"
                echo "   3. Network connectivity issues with required services"
            else
                echo -e "${RED}❌ E2E tests failed with exit code $exit_code${NC}"
            fi
            exit $exit_code
        }
    else
        # Fallback: run without timeout (not recommended for CI)
        echo -e "${YELLOW}⚠️  No timeout command found. Running tests without timeout...${NC}"
        echo "   For better control, install coreutils: brew install coreutils"
        go test -v ./test/e2e/... || {
            exit_code=$?
            echo -e "${RED}❌ E2E tests failed with exit code $exit_code${NC}"
            exit $exit_code
        }
    fi
fi

echo -e "${GREEN}🎉 E2E tests completed successfully!${NC}"
echo
echo "📊 Test Summary:"
echo "   - Full order flow: ✅"
echo "   - Error handling: ✅"
echo "   - Real-time updates: ✅"
echo "   - Service integration: ✅"
