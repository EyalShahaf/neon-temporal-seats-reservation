# End-to-End (E2E) Tests

This directory contains comprehensive end-to-end tests for the Temporal Seats application. These tests validate the complete system integration from API calls through Temporal workflows to real-time UI updates.

## 🎯 What E2E Tests Cover

### **Full Order Flow**
- ✅ Order creation via REST API
- ✅ Seat selection and updates
- ✅ Payment processing with retry logic
- ✅ Real-time status updates via Server-Sent Events (SSE)
- ✅ Complete workflow state transitions

### **Error Handling**
- ✅ Invalid order IDs
- ✅ Payment failures and retries
- ✅ Service unavailability scenarios
- ✅ Timeout handling

### **Service Integration**
- ✅ Temporal server connectivity
- ✅ Temporal UI accessibility
- ✅ API server health checks
- ✅ Worker process management

## 🚀 Quick Start

### **Prerequisites**

Before running E2E tests, ensure you have:

1. **Docker Desktop** running
   - macOS: [Docker Desktop for Mac](https://docs.docker.com/desktop/mac/install/)
   - Linux: [Docker Engine](https://docs.docker.com/engine/install/)
   - Windows: [Docker Desktop for Windows](https://docs.docker.com/desktop/windows/install/)

2. **docker-compose** installed
   ```bash
   # macOS
   brew install docker-compose
   
   # Linux
   sudo curl -L "https://github.com/docker/compose/releases/download/v2.20.0/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
   sudo chmod +x /usr/local/bin/docker-compose
   ```

3. **Go 1.21+** installed
   - [Download Go](https://golang.org/dl/)

4. **Available ports**: 7233, 8088, 8080

### **Running E2E Tests**

#### **Option 1: Using Make (Recommended)**
```bash
# Run E2E tests with full dependency management
make test-e2e
```

#### **Option 2: Direct Script Execution**
```bash
# Make script executable and run
chmod +x scripts/run-e2e.sh
./scripts/run-e2e.sh
```

#### **Option 3: Go Test Directly**
```bash
# Run E2E tests directly (requires manual service management)
go test -v ./test/e2e/...
```

## 📋 Test Structure

### **Test Files**

- `e2e_test.go` - Main E2E test suite
  - `TestFullOrderFlow` - Complete order lifecycle
  - `TestErrorHandling` - Error scenarios and edge cases

### **Test Configuration**

The E2E tests use the following default configuration:

```go
type E2ETestConfig struct {
    APIBaseURL     string        // http://localhost:8080
    TemporalURL    string        // localhost:7233
    TemporalUIURL  string        // http://localhost:8088
    TestTimeout    time.Duration // 2 minutes
    StartupTimeout time.Duration // 30 seconds
}
```

### **Service Management**

The E2E test suite automatically:

1. **Starts Temporal Services**
   - PostgreSQL database
   - Temporal server (port 7233)
   - Temporal UI (port 8088)

2. **Starts Application Services**
   - Temporal worker process
   - API server (port 8080)

3. **Performs Health Checks**
   - Validates Temporal connectivity
   - Verifies API server readiness
   - Ensures worker registration

4. **Cleans Up Resources**
   - Stops all processes
   - Removes Docker containers
   - Releases ports

## 🔧 Troubleshooting

### **Common Issues**

#### **"Docker not found"**
E2E tests require Docker to run Temporal server and PostgreSQL database.

#### **"Port already in use"**
E2E tests require exclusive access to ports 7233, 8088, and 8080.

```bash
# Check what's using the ports
lsof -i :8080  # API server
lsof -i :7233  # Temporal
lsof -i :8088  # Temporal UI

# Stop conflicting services
pkill -f "go run cmd/api/main.go"
docker-compose -f infra/docker-compose.yml down
```

#### **"Temporal server not ready"**
E2E tests require Temporal server running on port 7233, Temporal UI on port 8088, and PostgreSQL database.

```bash
# Check Temporal logs
docker-compose -f infra/docker-compose.yml logs temporal

# Check if ports are accessible
curl http://localhost:8088  # Temporal UI
telnet localhost 7233        # Temporal server
```

#### **"API server not ready"**
E2E tests require API server running on port 8080, Temporal worker process, and connection to Temporal server.

```bash
# Check API server logs
go run cmd/api/main.go

# Verify Temporal connection
go run cmd/worker/main.go
```

#### **"Build failed"**
```bash
# Clean and rebuild
go clean -cache
go mod tidy
go build ./cmd/api/main.go
go build ./cmd/worker/main.go
```

### **Environment Variables**

You can customize E2E test behavior:

```bash
# Skip E2E tests entirely
SKIP_E2E=true go test ./...

# Run in CI mode (limited output)
CI=true make test-e2e

# Custom timeout (seconds)
E2E_TIMEOUT=600 make test-e2e
```

### **Debug Mode**

For detailed debugging:

```bash
# Run with verbose output
go test -v ./test/e2e/... -args -test.v

# Run specific test
go test -v ./test/e2e/... -run TestFullOrderFlow

# Keep services running after test
SKIP_CLEANUP=true go test ./test/e2e/...
```

## 📊 Test Results

### **Expected Output**

```
🚀 Temporal Seats E2E Test Runner
==================================
🔍 Checking dependencies...
✅ All dependencies found
🔍 Checking port availability...
✅ Ports available
🔨 Building project...
✅ Build successful
🧪 Running E2E tests...
   This may take a few minutes as it starts all services...
   Timeout: 300s

=== RUN   TestFullOrderFlow
🚀 Setting up E2E test environment...
🐳 Starting Temporal services with docker-compose...
✅ Temporal services started
⏳ Waiting for Temporal to be ready...
✅ Temporal is ready
👷 Starting Temporal worker...
✅ Worker started
🌐 Starting API server...
✅ API server started
⏳ Waiting for API server to be ready...
✅ API server is ready
✅ E2E test environment ready!
=== RUN   TestFullOrderFlow/TestFullOrderFlow
📝 Testing order creation...
✅ Order created successfully
🪑 Testing seat updates...
✅ Seats updated successfully
💳 Testing payment submission...
✅ Payment submitted successfully
👀 Testing real-time status monitoring...
📡 Received state update: PENDING
📡 Received state update: CONFIRMED
✅ Order reached final state
🎉 Full order flow test completed successfully!
=== RUN   TestErrorHandling
❌ Testing invalid order ID handling...
✅ Invalid order ID handled correctly
❌ Testing invalid payment code handling...
✅ Invalid payment code handled correctly
--- PASS: TestFullOrderFlow (45.2s)
--- PASS: TestErrorHandling (12.8s)
PASS
🎉 E2E tests completed successfully!

📊 Test Summary:
   - Full order flow: ✅
   - Error handling: ✅
   - Real-time updates: ✅
   - Service integration: ✅
```

## 🏗️ CI/CD Integration

### **GitHub Actions Example**

```yaml
name: E2E Tests
on: [push, pull_request]

jobs:
  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
        with:
          go-version: '1.21'
      - name: Install Docker Compose
        run: |
          sudo curl -L "https://github.com/docker/compose/releases/download/v2.20.0/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
          sudo chmod +x /usr/local/bin/docker-compose
      - name: Run E2E Tests
        run: make test-e2e
```

### **Local Development**

```bash
# Run all tests (unit + E2E)
make test-all

# Run only E2E tests
make test-e2e

# Run with custom configuration
E2E_TIMEOUT=600 make test-e2e
```

## 📚 Additional Resources

- [Temporal Documentation](https://docs.temporal.io/)
- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [Go Testing Documentation](https://golang.org/pkg/testing/)
- [Project README](../README.md)
