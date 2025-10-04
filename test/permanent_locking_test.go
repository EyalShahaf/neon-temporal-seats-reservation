package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPermanentSeatLocking tests the complete flow of seat selection and permanent locking
func TestPermanentSeatLocking(t *testing.T) {
	// Test configuration
	baseURL := "http://localhost:8080"
	flightID := "FL-001"
	seats := []string{"1A", "2A"}
	paymentCode := "12345"

	// Step 1: Create an order
	t.Log("Step 1: Creating order...")
	orderID := fmt.Sprintf("test-order-%d", time.Now().Unix())
	createOrderReq := map[string]string{
		"orderID":  orderID,
		"flightID": flightID,
	}

	createResp, err := http.Post(
		fmt.Sprintf("%s/orders", baseURL),
		"application/json",
		jsonBody(createOrderReq),
	)
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

	// Step 3: Select seats
	t.Log("Step 3: Selecting seats...")
	selectSeatsReq := map[string]interface{}{
		"seats": seats,
	}

	selectResp, err := http.Post(
		fmt.Sprintf("%s/orders/%s/seats", baseURL, orderID),
		"application/json",
		jsonBody(selectSeatsReq),
	)
	require.NoError(t, err)
	defer selectResp.Body.Close()
	assert.Equal(t, http.StatusOK, selectResp.StatusCode)

	// Step 4: Wait a moment for seat entities to be created and signals to be processed
	t.Log("Step 4: Waiting for seat entities to process hold signals...")
	time.Sleep(2 * time.Second)

	// Step 5: Check that seats are now held
	t.Log("Step 5: Verifying seats are held...")
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

	paymentResp, err := http.Post(
		fmt.Sprintf("%s/orders/%s/payment", baseURL, orderID),
		"application/json",
		jsonBody(paymentReq),
	)
	require.NoError(t, err)
	defer paymentResp.Body.Close()
	assert.Equal(t, http.StatusOK, paymentResp.StatusCode)

	// Step 7: Wait for payment processing and confirm signals
	t.Log("Step 7: Waiting for payment processing and confirm signals...")
	time.Sleep(3 * time.Second)

	// Step 8: Check that seats are now permanently confirmed
	t.Log("Step 8: Verifying seats are permanently confirmed...")
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

	createResp2, err := http.Post(
		fmt.Sprintf("%s/orders", baseURL),
		"application/json",
		jsonBody(createOrderReq2),
	)
	require.NoError(t, err)
	defer createResp2.Body.Close()
	assert.Equal(t, http.StatusCreated, createResp2.StatusCode)

	// Try to select the same seats with the second order
	selectSeatsReq2 := map[string]interface{}{
		"seats": seats, // Same seats
	}

	selectResp2, err := http.Post(
		fmt.Sprintf("%s/orders/%s/seats", baseURL, orderID2),
		"application/json",
		jsonBody(selectSeatsReq2),
	)
	require.NoError(t, err)
	defer selectResp2.Body.Close()
	assert.Equal(t, http.StatusOK, selectResp2.StatusCode) // API call succeeds

	// Wait and check that the seats are still confirmed (not held by the second order)
	time.Sleep(2 * time.Second)
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
	flightID := "FL-001"
	seats := []string{"1B", "2B"}

	// Create two orders
	orderID1 := fmt.Sprintf("cross-test-1-%d", time.Now().Unix())
	orderID2 := fmt.Sprintf("cross-test-2-%d", time.Now().Unix())

	// Create first order
	createOrder(t, baseURL, orderID1, flightID)
	createOrder(t, baseURL, orderID2, flightID)

	// First order selects seats
	selectSeats(t, baseURL, orderID1, seats)
	time.Sleep(1 * time.Second)

	// Check that seats are held by first order
	availability := getSeatAvailability(t, baseURL, flightID)
	assert.Contains(t, availability.Held, seats[0], "Seat 1B should be held by first order")
	assert.Contains(t, availability.Held, seats[1], "Seat 2B should be held by first order")

	// Second order tries to select the same seats
	selectSeats(t, baseURL, orderID2, seats)
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
	resp, err := http.Get(fmt.Sprintf("%s/flights/%s/available-seats", baseURL, flightID))
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

func createOrder(t *testing.T, baseURL, orderID, flightID string) {
	req := map[string]string{
		"orderID":  orderID,
		"flightID": flightID,
	}

	resp, err := http.Post(
		fmt.Sprintf("%s/orders", baseURL),
		"application/json",
		jsonBody(req),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func selectSeats(t *testing.T, baseURL, orderID string, seats []string) {
	req := map[string]interface{}{
		"seats": seats,
	}

	resp, err := http.Post(
		fmt.Sprintf("%s/orders/%s/seats", baseURL, orderID),
		"application/json",
		jsonBody(req),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func jsonBody(data interface{}) *strings.Reader {
	jsonData, _ := json.Marshal(data)
	return strings.NewReader(string(jsonData))
}
