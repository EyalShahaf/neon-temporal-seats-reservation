package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// E2ETestConfig holds configuration for the E2E test
type E2ETestConfig struct {
	APIBaseURL     string
	TemporalURL    string
	TemporalUIURL  string
	TestTimeout    time.Duration
	StartupTimeout time.Duration
}

// DefaultE2EConfig returns the default configuration for E2E tests
func DefaultE2ETestConfig() *E2ETestConfig {
	return &E2ETestConfig{
		APIBaseURL:     "http://localhost:8080",
		TemporalURL:    "localhost:7233",
		TemporalUIURL:  "http://localhost:8088",
		TestTimeout:    90 * time.Second, // Reduced from 2 minutes
		StartupTimeout: 20 * time.Second, // Reduced from 30 seconds
	}
}

// E2ETestSuite manages the E2E test environment
type E2ETestSuite struct {
	config        *E2ETestConfig
	apiProcess    *exec.Cmd
	workerProcess *exec.Cmd
	cleanup       func()
}

// NewE2ETestSuite creates a new E2E test suite
func NewE2ETestSuite(config *E2ETestConfig) *E2ETestSuite {
	return &E2ETestSuite{
		config: config,
		cleanup: func() {
			// Default cleanup - will be overridden
		},
	}
}

// Setup starts all required services for E2E testing
func (s *E2ETestSuite) Setup(t *testing.T) {
	t.Helper()

	t.Log("🚀 Setting up E2E test environment...")

	// Check if we're in the right directory
	if err := s.checkProjectStructure(); err != nil {
		t.Fatalf("❌ Project structure check failed: %v", err)
	}

	// Start Temporal services
	if err := s.startTemporalServices(t); err != nil {
		t.Fatalf("❌ Failed to start Temporal services: %v", err)
	}

	// Wait for Temporal to be ready
	if err := s.waitForTemporal(t); err != nil {
		t.Fatalf("❌ Temporal service not ready: %v", err)
	}

	// Start the worker
	if err := s.startWorker(t); err != nil {
		t.Fatalf("❌ Failed to start worker: %v", err)
	}

	// Start the API server
	if err := s.startAPIServer(t); err != nil {
		t.Fatalf("❌ Failed to start API server: %v", err)
	}

	// Wait for API server to be ready
	if err := s.waitForAPIServer(t); err != nil {
		t.Fatalf("❌ API server not ready: %v", err)
	}

	t.Log("✅ E2E test environment ready!")
}

// Teardown stops all services and cleans up
func (s *E2ETestSuite) Teardown(t *testing.T) {
	t.Helper()

	t.Log("🧹 Cleaning up E2E test environment...")

	if s.cleanup != nil {
		s.cleanup()
	}

	// Stop API server
	if s.apiProcess != nil && s.apiProcess.Process != nil {
		s.apiProcess.Process.Kill()
		s.apiProcess.Wait()
	}

	// Stop worker
	if s.workerProcess != nil && s.workerProcess.Process != nil {
		s.workerProcess.Process.Kill()
		s.workerProcess.Wait()
	}

	// Stop Temporal services
	s.stopTemporalServices(t)

	t.Log("✅ E2E test environment cleaned up!")
}

// checkProjectStructure verifies we're in the correct project directory
func (s *E2ETestSuite) checkProjectStructure() error {
	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %v", err)
	}

	// Check if we're in the project root by looking for go.mod
	// If not found, try going up one level (in case we're in test/e2e)
	projectRoot := wd
	if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
		// Try parent directory
		parentDir := filepath.Dir(wd)
		if _, err := os.Stat(filepath.Join(parentDir, "go.mod")); err == nil {
			projectRoot = parentDir
		} else {
			// Try going up two levels (in case we're in test/e2e)
			grandparentDir := filepath.Dir(parentDir)
			if _, err := os.Stat(filepath.Join(grandparentDir, "go.mod")); err == nil {
				projectRoot = grandparentDir
			} else {
				return fmt.Errorf("required file not found: go.mod (are you in the project root?)")
			}
		}
	}

	// Change to project root
	if err := os.Chdir(projectRoot); err != nil {
		return fmt.Errorf("failed to change to project root %s: %v", projectRoot, err)
	}

	requiredFiles := []string{
		"go.mod",
		"cmd/api/main.go",
		"cmd/worker/main.go",
		"infra/docker-compose.yml",
	}

	for _, file := range requiredFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return fmt.Errorf("required file not found: %s", file)
		}
	}

	return nil
}

// startTemporalServices starts Temporal using docker-compose
func (s *E2ETestSuite) startTemporalServices(t *testing.T) error {
	t.Log("🐳 Starting Temporal services with docker-compose...")

	// Check if docker-compose is available
	if _, err := exec.LookPath("docker-compose"); err != nil {
		return fmt.Errorf("docker-compose not found: %v\n\nE2E tests require docker-compose to manage Temporal services", err)
	}

	// Check if Docker is running
	if err := exec.Command("docker", "ps").Run(); err != nil {
		return fmt.Errorf("Docker is not running: %v\n\nE2E tests require Docker to run Temporal server and PostgreSQL database", err)
	}

	// Start services
	cmd := exec.Command("docker-compose", "-f", "infra/docker-compose.yml", "up", "-d")
	cmd.Dir = "."

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start Temporal services: %v\nOutput: %s", err, string(output))
	}

	// Set up cleanup
	s.cleanup = func() {
		t.Log("🛑 Stopping Temporal services...")
		stopCmd := exec.Command("docker-compose", "-f", "infra/docker-compose.yml", "down", "--timeout", "10")
		stopCmd.Dir = "."

		// Run with timeout to prevent hanging
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		stopCmd = exec.CommandContext(ctx, "docker-compose", "-f", "infra/docker-compose.yml", "down", "--timeout", "10")
		stopCmd.Dir = "."
		stopCmd.Run()
	}

	t.Log("✅ Temporal services started")
	return nil
}

// waitForTemporal waits for Temporal to be ready
func (s *E2ETestSuite) waitForTemporal(t *testing.T) error {
	t.Log("⏳ Waiting for Temporal to be ready...")

	timeout := time.After(s.config.StartupTimeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("Temporal server not ready after %v\n\nE2E tests require:\n  - Temporal server running on port 7233\n  - Temporal UI running on port 8088\n  - PostgreSQL database running", s.config.StartupTimeout)
		case <-ticker.C:
			// Try to connect to Temporal
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Get(s.config.TemporalUIURL)
			if err == nil && resp.StatusCode == 200 {
				resp.Body.Close()
				t.Log("✅ Temporal is ready")
				return nil
			}
		}
	}
}

// startWorker starts the Temporal worker
func (s *E2ETestSuite) startWorker(t *testing.T) error {
	t.Log("👷 Starting Temporal worker...")

	cmd := exec.Command("go", "run", "cmd/worker/main.go")
	cmd.Dir = "."

	// Capture output for debugging
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start worker: %v", err)
	}

	s.workerProcess = cmd

	// Give worker time to start
	time.Sleep(2 * time.Second)

	t.Log("✅ Worker started")
	return nil
}

// startAPIServer starts the API server
func (s *E2ETestSuite) startAPIServer(t *testing.T) error {
	t.Log("🌐 Starting API server...")

	cmd := exec.Command("go", "run", "cmd/api/main.go")
	cmd.Dir = "."

	// Capture output for debugging
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start API server: %v", err)
	}

	s.apiProcess = cmd

	t.Log("✅ API server started")
	return nil
}

// waitForAPIServer waits for the API server to be ready
func (s *E2ETestSuite) waitForAPIServer(t *testing.T) error {
	t.Log("⏳ Waiting for API server to be ready...")

	timeout := time.After(s.config.StartupTimeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("API server not ready after %v\n\nE2E tests require:\n  - API server running on port 8080\n  - Temporal worker process running\n  - Connection to Temporal server", s.config.StartupTimeout)
		case <-ticker.C:
			// Try to connect to API server
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Get(s.config.APIBaseURL + "/health")
			if err == nil && resp.StatusCode == 200 {
				resp.Body.Close()
				t.Log("✅ API server is ready")
				return nil
			}
		}
	}
}

// stopTemporalServices stops Temporal services
func (s *E2ETestSuite) stopTemporalServices(t *testing.T) {
	if s.cleanup != nil {
		s.cleanup()
	}

	// Additional cleanup - force kill any remaining containers
	t.Log("🧹 Force cleaning up any remaining containers...")
	exec.Command("docker", "kill", "$(docker ps -q --filter 'name=infra_')").Run()
	exec.Command("docker", "rm", "-f", "$(docker ps -aq --filter 'name=infra_')").Run()
}

// TestFullOrderFlow tests the complete order flow from creation to completion
func TestFullOrderFlow(t *testing.T) {
	config := DefaultE2ETestConfig()
	suite := NewE2ETestSuite(config)

	// Setup
	suite.Setup(t)
	defer suite.Teardown(t)

	// Test data
	orderID := fmt.Sprintf("e2e-test-%d", time.Now().Unix())
	flightID := "E2E-FL001"
	paymentCode := "E2E-PAY-123"

	t.Run("Create Order", func(t *testing.T) {
		t.Log("📝 Testing order creation...")

		// Create order
		orderData := map[string]interface{}{
			"orderID":  orderID,
			"flightID": flightID,
		}

		resp, err := suite.makeRequest(t, "POST", "/orders", orderData)
		require.NoError(t, err)
		require.Equal(t, 201, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, orderID, result["orderId"])
		// Note: The create order endpoint only returns order_id, not the full state
		// The full state will be available via the status endpoint after workflow starts

		t.Log("✅ Order created successfully")
	})

	t.Run("Wait for Workflow", func(t *testing.T) {
		t.Log("⏳ Waiting for workflow to start...")

		// Wait a moment for the workflow to start
		time.Sleep(5 * time.Second)

		// Check that the workflow is running by querying status
		resp, err := suite.makeRequest(t, "GET", fmt.Sprintf("/orders/%s/status", orderID), nil)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)

		var status map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&status)
		require.NoError(t, err)

		// Should have a state (PENDING, CONFIRMED, etc.)
		// Note: The struct uses capitalized field names in JSON
		assert.NotNil(t, status["State"])

		t.Log("✅ Workflow is running")
	})

	t.Run("Update Seats", func(t *testing.T) {
		t.Log("🪑 Testing seat updates...")

		newSeats := []string{"2A", "2B", "2C"}

		updateData := map[string]interface{}{
			"seats": newSeats,
		}

		resp, err := suite.makeRequest(t, "POST", fmt.Sprintf("/orders/%s/seats", orderID), updateData)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)

		t.Log("✅ Seats updated successfully")
	})

	t.Run("Submit Payment", func(t *testing.T) {
		t.Log("💳 Testing payment submission...")

		paymentData := map[string]interface{}{
			"code": paymentCode,
		}

		resp, err := suite.makeRequest(t, "POST", fmt.Sprintf("/orders/%s/payment", orderID), paymentData)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)

		t.Log("✅ Payment submitted successfully")
	})

	t.Run("Monitor Order Status", func(t *testing.T) {
		t.Log("👀 Testing real-time status monitoring...")

		// Test SSE endpoint
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(suite.config.APIBaseURL + fmt.Sprintf("/orders/%s/events", orderID))
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)
		require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

		// Read SSE events
		scanner := bufio.NewScanner(resp.Body)
		eventCount := 0
		finalState := ""

		timeout := time.After(30 * time.Second)

		for {
			select {
			case <-timeout:
				t.Fatalf("Timeout waiting for final state. Last state: %s", finalState)
			default:
				if scanner.Scan() {
					line := scanner.Text()
					if strings.HasPrefix(line, "data: ") {
						eventCount++
						var event map[string]interface{}
						if err := json.Unmarshal([]byte(line[6:]), &event); err == nil {
							if state, ok := event["State"].(string); ok {
								finalState = state
								t.Logf("📡 Received state update: %s", state)

								if state == "CONFIRMED" || state == "FAILED" || state == "EXPIRED" {
									resp.Body.Close()
									assert.Equal(t, "CONFIRMED", state)
									t.Log("✅ Order reached final state")
									return
								}
							}
						}
					}
				} else {
					resp.Body.Close()
					t.Fatalf("SSE stream ended unexpectedly. Final state: %s", finalState)
				}
			}
		}
	})

	t.Log("🎉 Full order flow test completed successfully!")
}

// TestErrorHandling tests error scenarios
func TestErrorHandling(t *testing.T) {
	config := DefaultE2ETestConfig()
	suite := NewE2ETestSuite(config)

	// Setup
	suite.Setup(t)
	defer suite.Teardown(t)

	t.Run("Invalid Order ID", func(t *testing.T) {
		t.Log("❌ Testing invalid order ID handling...")

		resp, err := suite.makeRequest(t, "GET", "/orders/invalid-id", nil)
		require.NoError(t, err)
		require.Equal(t, 404, resp.StatusCode)

		t.Log("✅ Invalid order ID handled correctly")
	})

	t.Run("Invalid Payment Code", func(t *testing.T) {
		t.Log("❌ Testing invalid payment code handling...")

		// Create order first
		orderID := fmt.Sprintf("e2e-error-test-%d", time.Now().Unix())
		orderData := map[string]interface{}{
			"orderID":  orderID,
			"flightID": "E2E-ERROR-FL001",
		}

		resp, err := suite.makeRequest(t, "POST", "/orders", orderData)
		require.NoError(t, err)
		require.Equal(t, 201, resp.StatusCode)

		// Submit invalid payment
		paymentData := map[string]interface{}{
			"code": "INVALID-PAYMENT",
		}

		resp, err = suite.makeRequest(t, "POST", fmt.Sprintf("/orders/%s/payment", orderID), paymentData)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)

		// Wait for failure
		time.Sleep(10 * time.Second)

		// Check final state
		resp, err = suite.makeRequest(t, "GET", fmt.Sprintf("/orders/%s", orderID), nil)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "FAILED", result["state"])

		t.Log("✅ Invalid payment handled correctly")
	})
}

// makeRequest is a helper method for making HTTP requests
func (s *E2ETestSuite) makeRequest(t *testing.T, method, path string, data interface{}) (*http.Response, error) {
	t.Helper()

	var body io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		require.NoError(t, err)
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, s.config.APIBaseURL+path, body)
	require.NoError(t, err)

	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	return client.Do(req)
}

// TestMain runs setup and teardown for all E2E tests
func TestMain(m *testing.M) {
	// Check if we should skip E2E tests
	if os.Getenv("SKIP_E2E") == "true" {
		fmt.Println("⏭️  Skipping E2E tests (SKIP_E2E=true)")
		os.Exit(0)
	}

	// Check if we're in CI environment
	if os.Getenv("CI") == "true" {
		fmt.Println("🏗️  Running in CI environment - E2E tests may be limited")
	}

	// Run tests
	code := m.Run()

	// Cleanup any remaining processes
	fmt.Println("🧹 Final cleanup...")

	os.Exit(code)
}
