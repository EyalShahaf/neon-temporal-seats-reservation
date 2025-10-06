package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HTTP client with timeout for all requests
var httpc = &http.Client{Timeout: 10 * time.Second}

// jsonBody creates a JSON reader from any value, with error checking
func jsonBody(t *testing.T, v any) io.Reader {
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewReader(b)
}

// containsAll checks if all items in 'items' are present in 'container'
func containsAll(container []string, items []string) bool {
	for _, item := range items {
		found := false
		for _, c := range container {
			if c == item {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// TestPermanentSeatLocking tests the complete flow of seat selection and permanent locking
func TestPermanentSeatLocking(t *testing.T) {
	// Test configuration
	baseURL := "http://localhost:8080"
	flightID := fmt.Sprintf("FL-%d", time.Now().UnixNano()) // Unique flight ID
	seats := []string{"1A", "2A"}
	paymentCode := "E2E-OK" // Deterministic payment for E2E

	// Step 1: Create an order
	t.Log("Step 1: Creating order...")
	orderID := fmt.Sprintf("test-order-%d", time.Now().Unix())
	createOrderReq := map[string]string{
		"orderID":  orderID,
		"flightID": flightID,
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/orders", baseURL), jsonBody(t, createOrderReq))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	createResp, err := httpc.Do(req)
	require.NoError(t, err)
	defer createResp.Body.Close()
	assert.Equal(t, http.StatusCreated, createResp.StatusCode)

	// Step 2: Check initial seat availability (all seats should be available)
	t.Log("Step 2: Checking initial seat availability...")
	availability := getSeatAvailability(t, baseURL, flightID)
	assert.Contains(t, availability.Available, seats[0], "Seat 1A should be available initially")
	assert.Contains(t, availability.Available, seats[1], "Seat 2A should be available initially")
	assert.Empty(t, availability.Held, "No seats should be held initially")
	assert.Empty(t, availability.Confirmed, "No seats should be confirmed initially")

	// Step 3: Wait for workflow to be ready before selecting seats
	t.Log("Step 3: Waiting for workflow to be ready...")
	require.Eventually(t, func() bool {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/orders/%s/status", baseURL, orderID), nil)
		if err != nil {
			return false
		}
		resp, err := httpc.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == 200
	}, 20*time.Second, 500*time.Millisecond, "workflow not ready")

	// Step 4: Select seats
	t.Log("Step 4: Selecting seats...")
	selectSeatsReq := map[string]interface{}{
		"seats": seats,
	}

	req, err = http.NewRequest("POST", fmt.Sprintf("%s/orders/%s/seats", baseURL, orderID), jsonBody(t, selectSeatsReq))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	selectResp, err := httpc.Do(req)
	require.NoError(t, err)
	defer selectResp.Body.Close()
	assert.Equal(t, http.StatusOK, selectResp.StatusCode)

	// Step 5: Poll until seats are held
	t.Log("Step 5: Waiting for seats to be held...")
	require.Eventually(t, func() bool {
		availability := getSeatAvailability(t, baseURL, flightID)
		return containsAll(availability.Held, seats)
	}, 10*time.Second, 500*time.Millisecond, "seats not held")

	// Verify seats are held
	availability = getSeatAvailability(t, baseURL, flightID)
	assert.NotContains(t, availability.Available, seats[0], "Seat 1A should no longer be available")
	assert.NotContains(t, availability.Available, seats[1], "Seat 2A should no longer be available")
	assert.Contains(t, availability.Held, seats[0], "Seat 1A should be held")
	assert.Contains(t, availability.Held, seats[1], "Seat 2A should be held")
	assert.Empty(t, availability.Confirmed, "No seats should be confirmed yet")

	// Step 6: Submit payment
	t.Log("Step 6: Submitting payment...")
	paymentReq := map[string]string{
		"code": paymentCode,
	}

	req, err = http.NewRequest("POST", fmt.Sprintf("%s/orders/%s/payment", baseURL, orderID), jsonBody(t, paymentReq))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	paymentResp, err := httpc.Do(req)
	require.NoError(t, err)
	defer paymentResp.Body.Close()
	assert.Equal(t, http.StatusOK, paymentResp.StatusCode)

	// Step 7: Poll until order is confirmed
	t.Log("Step 7: Waiting for order confirmation...")
	require.Eventually(t, func() bool {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/orders/%s/status", baseURL, orderID), nil)
		if err != nil {
			return false
		}
		resp, err := httpc.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return false
		}

		var status map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			return false
		}

		// Check for CONFIRMED state
		if state, ok := status["State"].(string); ok {
			return state == "CONFIRMED"
		}
		return false
	}, 15*time.Second, 500*time.Millisecond, "order not confirmed")

	// Step 8: Poll until seats are confirmed
	t.Log("Step 8: Waiting for seats to be confirmed...")
	require.Eventually(t, func() bool {
		availability := getSeatAvailability(t, baseURL, flightID)
		return containsAll(availability.Confirmed, seats)
	}, 10*time.Second, 500*time.Millisecond, "seats not confirmed")

	// Verify final state
	availability = getSeatAvailability(t, baseURL, flightID)
	assert.NotContains(t, availability.Available, seats[0], "Seat 1A should not be available after confirmation")
	assert.NotContains(t, availability.Available, seats[1], "Seat 2A should not be available after confirmation")
	assert.NotContains(t, availability.Held, seats[0], "Seat 1A should no longer be held after confirmation")
	assert.NotContains(t, availability.Held, seats[1], "Seat 2A should no longer be held after confirmation")
	assert.Contains(t, availability.Confirmed, seats[0], "Seat 1A should be permanently confirmed")
	assert.Contains(t, availability.Confirmed, seats[1], "Seat 2A should be permanently confirmed")

	// Step 9: Test that confirmed seats cannot be selected by another order
	t.Log("Step 9: Testing that confirmed seats cannot be selected by another order...")
	orderID2 := fmt.Sprintf("test-order-2-%d", time.Now().Unix())
	createOrderReq2 := map[string]string{
		"orderID":  orderID2,
		"flightID": flightID,
	}

	req, err = http.NewRequest("POST", fmt.Sprintf("%s/orders", baseURL), jsonBody(t, createOrderReq2))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	createResp2, err := httpc.Do(req)
	require.NoError(t, err)
	defer createResp2.Body.Close()
	assert.Equal(t, http.StatusCreated, createResp2.StatusCode)

	// Wait for second workflow to be ready
	require.Eventually(t, func() bool {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/orders/%s/status", baseURL, orderID2), nil)
		if err != nil {
			return false
		}
		resp, err := httpc.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == 200
	}, 20*time.Second, 500*time.Millisecond, "second workflow not ready")

	// Try to select the same seats with the second order
	selectSeatsReq2 := map[string]interface{}{
		"seats": seats, // Same seats
	}

	req, err = http.NewRequest("POST", fmt.Sprintf("%s/orders/%s/seats", baseURL, orderID2), jsonBody(t, selectSeatsReq2))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	selectResp2, err := httpc.Do(req)
	require.NoError(t, err)
	defer selectResp2.Body.Close()
	assert.Equal(t, http.StatusOK, selectResp2.StatusCode) // API call succeeds

	// Wait and check that the seats are still confirmed (not held by the second order)
	time.Sleep(1 * time.Second) // Brief wait for any processing
	availability = getSeatAvailability(t, baseURL, flightID)
	assert.Contains(t, availability.Confirmed, seats[0], "Seat 1A should still be confirmed, not held by second order")
	assert.Contains(t, availability.Confirmed, seats[1], "Seat 2A should still be confirmed, not held by second order")

	// The second order should not have these seats in held
	assert.NotContains(t, availability.Held, seats[0], "Seat 1A should not be held by second order")
	assert.NotContains(t, availability.Held, seats[1], "Seat 2A should not be held by second order")

	t.Log("✅ Test completed successfully - permanent locking is working!")
}

// TestCrossOrderProtection tests that multiple orders cannot select the same seats
func TestCrossOrderProtection(t *testing.T) {
	baseURL := "http://localhost:8080"
	flightID := fmt.Sprintf("FL-%d", time.Now().UnixNano()) // Unique flight ID
	seats := []string{"1B", "2B"}

	// Create two orders
	orderID1 := fmt.Sprintf("cross-test-1-%d", time.Now().Unix())
	orderID2 := fmt.Sprintf("cross-test-2-%d", time.Now().Unix())

	// Create first order
	createOrderWithHTTPC(t, baseURL, orderID1, flightID)
	createOrderWithHTTPC(t, baseURL, orderID2, flightID)

	// Wait for both workflows to be ready
	require.Eventually(t, func() bool {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/orders/%s/status", baseURL, orderID1), nil)
		if err != nil {
			return false
		}
		resp, err := httpc.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == 200
	}, 20*time.Second, 500*time.Millisecond, "first workflow not ready")

	require.Eventually(t, func() bool {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/orders/%s/status", baseURL, orderID2), nil)
		if err != nil {
			return false
		}
		resp, err := httpc.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == 200
	}, 20*time.Second, 500*time.Millisecond, "second workflow not ready")

	// First order selects seats
	selectSeatsWithHTTPC(t, baseURL, orderID1, seats)

	// Wait until seats are held by first order
	require.Eventually(t, func() bool {
		availability := getSeatAvailability(t, baseURL, flightID)
		return containsAll(availability.Held, seats)
	}, 10*time.Second, 500*time.Millisecond, "seats not held by first order")

	// Verify seats are held by first order
	availability := getSeatAvailability(t, baseURL, flightID)
	assert.Contains(t, availability.Held, seats[0], "Seat 1B should be held by first order")
	assert.Contains(t, availability.Held, seats[1], "Seat 2B should be held by first order")

	// Second order tries to select the same seats
	selectSeatsWithHTTPC(t, baseURL, orderID2, seats)

	// Wait a moment for any processing
	time.Sleep(1 * time.Second)

	// Check that seats are still held by first order (cross-order protection)
	availability = getSeatAvailability(t, baseURL, flightID)
	assert.Contains(t, availability.Held, seats[0], "Seat 1B should still be held by first order")
	assert.Contains(t, availability.Held, seats[1], "Seat 2B should still be held by first order")

	t.Log("✅ Cross-order protection test completed successfully!")
}

// Helper functions

type SeatAvailability struct {
	Available []string `json:"available"`
	Held      []string `json:"held"`
	Confirmed []string `json:"confirmed"`
	FlightID  string   `json:"flightID"`
	Total     int      `json:"total"`
}

func getSeatAvailability(t *testing.T, baseURL, flightID string) SeatAvailability {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/flights/%s/available-seats", baseURL, flightID), nil)
	require.NoError(t, err)

	resp, err := httpc.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var availability SeatAvailability
	err = json.NewDecoder(resp.Body).Decode(&availability)
	require.NoError(t, err)

	t.Logf("Seat availability: Available=%v, Held=%v, Confirmed=%v",
		availability.Available, availability.Held, availability.Confirmed)

	return availability
}

func createOrderWithHTTPC(t *testing.T, baseURL, orderID, flightID string) {
	req := map[string]string{
		"orderID":  orderID,
		"flightID": flightID,
	}

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/orders", baseURL), jsonBody(t, req))
	require.NoError(t, err)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := httpc.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func selectSeatsWithHTTPC(t *testing.T, baseURL, orderID string, seats []string) {
	req := map[string]interface{}{
		"seats": seats,
	}

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/orders/%s/seats", baseURL, orderID), jsonBody(t, req))
	require.NoError(t, err)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := httpc.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
