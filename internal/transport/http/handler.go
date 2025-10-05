package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/EyalShahaf/temporal-seats/internal/domain"
	"github.com/EyalShahaf/temporal-seats/internal/entities/seat"
	"github.com/EyalShahaf/temporal-seats/internal/workflows"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

// OrderHandler holds dependencies for the order API handlers.
type OrderHandler struct {
	temporal client.Client
}

// NewOrderHandler creates a new OrderHandler with its dependencies.
func NewOrderHandler(temporal client.Client) *OrderHandler {
	return &OrderHandler{temporal: temporal}
}

func (h *OrderHandler) attachOrderRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /orders", h.createOrderHandler)
	mux.HandleFunc("POST /orders/{id}/seats", h.updateSeatsHandler)
	mux.HandleFunc("POST /orders/{id}/payment", h.submitPaymentHandler)
	mux.HandleFunc("GET /orders/{id}/status", h.getStatusHandler)
	mux.HandleFunc("GET /orders/{id}/events", h.sseHandler)
	mux.HandleFunc("GET /flights/{flightID}/available-seats", h.getAvailableSeatsHandler)
}

func (h *OrderHandler) createOrderHandler(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Println("Handler called: createOrderHandler with OrderID:", req.OrderID)

	opts := client.StartWorkflowOptions{
		ID:        "order::" + req.OrderID,
		TaskQueue: "order-tq",
	}
	input := workflows.OrderInput{
		OrderID:  req.OrderID,
		FlightID: req.FlightID,
	}

	we, err := h.temporal.ExecuteWorkflow(r.Context(), opts, workflows.OrderOrchestrationWorkflow, input)
	if err != nil {
		log.Printf("Failed to start workflow: %v", err)
		http.Error(w, "Failed to start order process", http.StatusInternalServerError)
		return
	}

	log.Printf("Started workflow. WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	resp := domain.CreateOrderResponse{OrderID: req.OrderID}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *OrderHandler) updateSeatsHandler(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("id")
	workflowID := "order::" + orderID

	var req domain.UpdateSeatsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Handler called: updateSeatsHandler for order %s with seats %v\n", orderID, req.Seats)

	// Seat entity workflows will be started automatically via SignalWithStart
	// when the order workflow calls the SeatSignalActivity

	err := h.temporal.SignalWorkflow(r.Context(), workflowID, "", workflows.UpdateSeatsSignal, req.Seats)
	if err != nil {
		log.Printf("Failed to signal workflow: %v", err)
		http.Error(w, "Failed to update seats", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *OrderHandler) submitPaymentHandler(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("id")
	workflowID := "order::" + orderID

	var req domain.SubmitPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Handler called: submitPaymentHandler for order %s\n", orderID)

	err := h.temporal.SignalWorkflow(r.Context(), workflowID, "", workflows.SubmitPaymentSignal, req.Code)
	if err != nil {
		log.Printf("Failed to signal workflow: %v", err)
		http.Error(w, "Failed to submit payment", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *OrderHandler) getStatusHandler(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("id")
	log.Printf("Handler called: getStatusHandler for order %s\n", orderID)

	workflowID := "order::" + orderID
	resp, err := h.temporal.QueryWorkflow(r.Context(), workflowID, "", workflows.GetStatusQuery)
	if err != nil {
		var notFoundErr *serviceerror.NotFound
		if errors.As(err, &notFoundErr) {
			http.Error(w, "Order not found", http.StatusNotFound)
			return
		}

		log.Printf("Failed to query workflow: %v", err)
		http.Error(w, "Failed to get order status", http.StatusInternalServerError)
		return
	}

	var state workflows.OrderState
	if err := resp.Get(&state); err != nil {
		log.Printf("Failed to decode workflow state: %v", err)
		http.Error(w, "Failed to get order status", http.StatusInternalServerError)
		return
	}

	// For now, we'll just return the raw workflow state.
	// Later, we can map this to the domain.GetStatusResponse DTO.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

func (h *OrderHandler) sseHandler(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("id")
	workflowID := "order::" + orderID
	log.Printf("Handler called: sseHandler for order %s\n", orderID)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	headersWritten := false

	writeUpdate := func() bool {
		resp, err := h.temporal.QueryWorkflow(r.Context(), workflowID, "", workflows.GetStatusQuery)
		if err != nil {
			var notFoundErr *serviceerror.NotFound
			if errors.As(err, &notFoundErr) {
				http.Error(w, "Order not found", http.StatusNotFound)
				return false
			}

			// Don't log here as it will be noisy if workflow is not found yet, just stop streaming
			return false
		}

		var state workflows.OrderState
		if err := resp.Get(&state); err != nil {
			log.Printf("Failed to decode workflow state for SSE: %v", err)
			return true // try again next tick
		}

		if !headersWritten {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			headersWritten = true
		}

		jsonData, err := json.Marshal(state)
		if err != nil {
			log.Printf("Failed to marshal state for SSE: %v", err)
			return true
		}

		if _, err := w.Write([]byte("data: ")); err != nil {
			return false
		}
		if _, err := w.Write(jsonData); err != nil {
			return false
		}
		if _, err := w.Write([]byte("\n\n")); err != nil {
			return false
		}

		flusher.Flush()

		return true
	}

	if !writeUpdate() {
		return
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			log.Println("Client disconnected from SSE stream")
			return
		case <-ticker.C:
			if !writeUpdate() {
				return
			}
		}
	}
}

// generateAllSeats creates a list of all possible seat IDs for a flight
func generateAllSeats() []string {
	seats := []string{}
	rows := []string{"1", "2", "3", "4", "5"}
	cols := []string{"A", "B", "C", "D", "E", "F"}

	for _, row := range rows {
		for _, col := range cols {
			seats = append(seats, row+col)
		}
	}
	return seats
}

// getAvailableSeatsHandler returns the availability status of all seats for a flight
func (h *OrderHandler) getAvailableSeatsHandler(w http.ResponseWriter, r *http.Request) {
	flightID := r.PathValue("flightID")
	if flightID == "" {
		http.Error(w, "Flight ID is required", http.StatusBadRequest)
		return
	}

	log.Println("Handler called: getAvailableSeatsHandler for flight", flightID)

	allSeats := generateAllSeats()
	available := []string{}
	held := []string{}
	confirmed := []string{}

	// Query each seat's state
	for _, seatID := range allSeats {
		wfID := fmt.Sprintf("seat::%s::%s", flightID, seatID)

		// Try to query the seat workflow
		resp, err := h.temporal.QueryWorkflow(r.Context(), wfID, "", "GetState")
		if err != nil {
			// If workflow doesn't exist, seat is available
			available = append(available, seatID)
			continue
		}

		var state seat.SeatState
		if err := resp.Get(&state); err != nil {
			// If we can't get state, assume available
			available = append(available, seatID)
			continue
		}

		// Categorize seat based on state
		if state.IsConfirmed {
			confirmed = append(confirmed, seatID)
		} else if state.IsHeld {
			held = append(held, seatID)
		} else {
			available = append(available, seatID)
		}
	}

	response := map[string]interface{}{
		"flightID":  flightID,
		"available": available,
		"held":      held,
		"confirmed": confirmed,
		"total":     len(allSeats),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
