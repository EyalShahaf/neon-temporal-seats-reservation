package activities

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"go.temporal.io/sdk/activity"
)

// ValidatePaymentActivity simulates a payment validation process.
// It has a 15% chance of failing to simulate a flaky payment gateway.
func ValidatePaymentActivity(ctx context.Context, orderID string, paymentCode string) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Validating payment", "OrderID", orderID)

	// Simulate random failure
	if rand.Float32() < 0.15 {
		logger.Error("Payment validation failed due to simulated random error.", "OrderID", orderID)
		return "", errors.New("payment gateway failed")
	}

	// Simulate work
	time.Sleep(1 * time.Second)

	logger.Info("Payment validated successfully", "OrderID", orderID)
	return "PAYMENT_SUCCESSFUL", nil
}

// ConfirmOrderActivity is a placeholder for any logic that should run after an order is successfully confirmed.
func ConfirmOrderActivity(ctx context.Context, orderID string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Confirming order", "OrderID", orderID)
	// e.g., send confirmation email, finalize booking
	return nil
}

// FailOrderActivity is a placeholder for any logic that should run after an order has failed.
func FailOrderActivity(ctx context.Context, orderID string) error {
	logger := activity.GetLogger(ctx)
	logger.Warn("Failing order", "OrderID", orderID)
	// e.g., send failure notification, release holds explicitly if needed
	return nil
}
