package domain

import "time"

// CreateOrderRequest is the client's request to create a new order.
type CreateOrderRequest struct {
	FlightID string `json:"flightID"`
	OrderID  string `json:"orderID"`
}

// CreateOrderResponse is the server's response after creating an order.
type CreateOrderResponse struct {
	OrderID string `json:"orderId"`
}

// UpdateSeatsRequest is the client's request to select or change seats.
type UpdateSeatsRequest struct {
	Seats []string `json:"seats"`
}

// SubmitPaymentRequest is the client's request to submit a payment code.
type SubmitPaymentRequest struct {
	Code string `json:"code"`
}

// GetStatusResponse represents the current state of an order workflow.
type GetStatusResponse struct {
	State         string     `json:"state"`
	Seats         []string   `json:"seats"`
	HoldExpiresAt *time.Time `json:"holdExpiresAt,omitempty"`
	AttemptsLeft  int        `json:"attemptsLeft"`
	LastError     string     `json:"lastError,omitempty"`
	PaymentStatus string     `json:"paymentStatus,omitempty"` // NEW: trying, retrying, failed, success
}
