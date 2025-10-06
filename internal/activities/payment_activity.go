package activities

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"go.temporal.io/sdk/activity"
)

// ValidatePaymentActivity simulates a payment validation process.
// It validates payment codes and has a 15% chance of failing to simulate a flaky payment gateway.
func ValidatePaymentActivity(ctx context.Context, orderID string, paymentCode string) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Validating payment", "OrderID", orderID, "PaymentCode", paymentCode)

	// Check for invalid payment codes
	if paymentCode == "INVALID-PAYMENT" || paymentCode == "" {
		logger.Error("Payment validation failed due to invalid payment code", "OrderID", orderID, "PaymentCode", paymentCode)
		return "", errors.New("invalid payment code")
	}

	// Force failure for debugging
	if paymentCode == "INVALID-PAYMENT" {
		logger.Error("FORCED FAILURE for INVALID-PAYMENT", "OrderID", orderID, "PaymentCode", paymentCode)
		return "", errors.New("FORCED FAILURE for INVALID-PAYMENT")
	}

	// Deterministic success for E2E tests
	if paymentCode == "E2E-OK" {
		logger.Info("E2E deterministic payment - always succeed", "OrderID", orderID, "PaymentCode", paymentCode)
		time.Sleep(1 * time.Second)
		logger.Info("Payment validated successfully", "OrderID", orderID)
		return "PAYMENT_SUCCESSFUL", nil
	}

	logger.Info("Payment code is valid, proceeding with validation", "OrderID", orderID, "PaymentCode", paymentCode)

	// Simulate random failure for valid payment codes
	// Seed the random number generator to ensure different results each time
	rand.Seed(time.Now().UnixNano())
	// Temporarily disable random failures for debugging
	// if rand.Float32() < 0.15 {
	//	logger.Error("Payment validation failed due to simulated random error.", "OrderID", orderID)
	//	return "", errors.New("payment gateway failed")
	// }

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
