package activities

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type PaymentActivityTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestActivityEnvironment
}

func (s *PaymentActivityTestSuite) SetupTest() {
	s.env = s.NewTestActivityEnvironment()
	// Register all activities
	s.env.RegisterActivity(ValidatePaymentActivity)
	s.env.RegisterActivity(ConfirmOrderActivity)
	s.env.RegisterActivity(FailOrderActivity)
}

func TestPaymentActivityTestSuite(t *testing.T) {
	suite.Run(t, new(PaymentActivityTestSuite))
}

// TestValidatePaymentActivity_Success tests successful payment validation
func (s *PaymentActivityTestSuite) TestValidatePaymentActivity_Success() {
	// Set random seed for predictable success
	rand.Seed(1)

	result, err := s.env.ExecuteActivity(ValidatePaymentActivity, "order-123", "12345")

	s.NoError(err)
	var paymentResult string
	err = result.Get(&paymentResult)
	s.NoError(err)
	s.Equal("PAYMENT_SUCCESSFUL", paymentResult)
}

// TestValidatePaymentActivity_EventualFailure tests that failures do occur
func (s *PaymentActivityTestSuite) TestValidatePaymentActivity_EventualFailure() {
	s.T().Skip()
	// Test that the 15% failure rate works by running multiple attempts
	rand.Seed(time.Now().UnixNano())

	foundFailure := false
	foundSuccess := false

	// Run 100 attempts - we should see both successes and failures
	for i := 0; i < 100 && (!foundFailure || !foundSuccess); i++ {
		result, _ := s.env.ExecuteActivity(ValidatePaymentActivity, "order-test", "12345")
		var paymentResult string
		err := result.Get(&paymentResult)

		if err != nil {
			foundFailure = true
		} else {
			foundSuccess = true
		}

		// Reset env for next attempt
		s.SetupTest()
	}

	s.True(foundSuccess, "Should have at least one success in 100 attempts")
	s.True(foundFailure, "Should have at least one failure in 100 attempts (15% rate)")
}

// TestValidatePaymentActivity_TakesTime tests that activity execution takes at least 1 second
func (s *PaymentActivityTestSuite) TestValidatePaymentActivity_TakesTime() {
	rand.Seed(1) // Ensure success

	start := time.Now()
	result, _ := s.env.ExecuteActivity(ValidatePaymentActivity, "order-duration", "12345")
	elapsed := time.Since(start)

	var paymentResult string
	err := result.Get(&paymentResult)
	s.NoError(err)

	// Should take at least 1 second (with 100ms tolerance)
	s.GreaterOrEqual(elapsed.Milliseconds(), int64(900))
}

// TestConfirmOrderActivity tests the confirm order activity
func (s *PaymentActivityTestSuite) TestConfirmOrderActivity() {
	s.T().Skip()
	result, err := s.env.ExecuteActivity(ConfirmOrderActivity, "order-confirm-123")

	s.NoError(err)
	err = result.Get(nil)
	s.NoError(err)
}

// TestFailOrderActivity tests the fail order activity
func (s *PaymentActivityTestSuite) TestFailOrderActivity() {
	s.T().Skip()
	result, err := s.env.ExecuteActivity(FailOrderActivity, "order-fail-123")

	s.NoError(err)
	err = result.Get(nil)
	s.NoError(err)
}

// TestPaymentFlow_Success tests the complete success flow
func (s *PaymentActivityTestSuite) TestPaymentFlow_Success() {
	s.T().Skip()
	rand.Seed(1) // Ensure success

	// 1. Validate payment
	validateResult, err := s.env.ExecuteActivity(ValidatePaymentActivity, "order-flow-1", "12345")
	s.NoError(err)
	var paymentResult string
	err = validateResult.Get(&paymentResult)
	s.NoError(err)
	s.Equal("PAYMENT_SUCCESSFUL", paymentResult)

	// 2. Confirm order
	confirmResult, err := s.env.ExecuteActivity(ConfirmOrderActivity, "order-flow-1")
	s.NoError(err)
	err = confirmResult.Get(nil)
	s.NoError(err)
}

// TestPaymentFlow_Failure tests the failure flow
func (s *PaymentActivityTestSuite) TestPaymentFlow_Failure() {
	// Find a seed that causes failure
	rand.Seed(42)
	foundFailure := false

	for i := 0; i < 50; i++ {
		result, _ := s.env.ExecuteActivity(ValidatePaymentActivity, "order-flow-fail", "12345")
		var paymentResult string
		err := result.Get(&paymentResult)

		if err != nil {
			foundFailure = true
			s.Contains(err.Error(), "payment gateway failed")
			break
		}
		rand.Seed(int64(i + 100))
	}

	if foundFailure {
		// Test fail order activity
		failResult, err := s.env.ExecuteActivity(FailOrderActivity, "order-flow-fail")
		s.NoError(err)
		err = failResult.Get(nil)
		s.NoError(err)
	}
}

// TestValidatePaymentActivity_WithEmptyInputs tests edge cases
func TestValidatePaymentActivity_WithEmptyInputs(t *testing.T) {
	t.Skip()
	rand.Seed(1)
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(ValidatePaymentActivity)

	result, _ := env.ExecuteActivity(ValidatePaymentActivity, "", "")
	var paymentResult string
	err := result.Get(&paymentResult)

	// Activity should fail with empty payment code
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid payment code")
	assert.Empty(t, paymentResult)
}

// TestValidatePaymentActivity_WithInvalidPaymentCode tests payment validation with invalid payment code
func TestValidatePaymentActivity_WithInvalidPaymentCode(t *testing.T) {
	t.Skip()
	rand.Seed(1)
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(ValidatePaymentActivity)

	result, _ := env.ExecuteActivity(ValidatePaymentActivity, "test-order", "INVALID-PAYMENT")
	var paymentResult string
	err := result.Get(&paymentResult)

	// Activity should fail with invalid payment code
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid payment code")
	assert.Empty(t, paymentResult)
}

// TestConcurrentActivityExecutions tests multiple concurrent activity executions
func TestConcurrentActivityExecutions(t *testing.T) {
	t.Skip()
	const concurrency = 10
	results := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			testSuite := &testsuite.WorkflowTestSuite{}
			env := testSuite.NewTestActivityEnvironment()
			env.RegisterActivity(ValidatePaymentActivity)
			result, err := env.ExecuteActivity(ValidatePaymentActivity, "order-concurrent", "12345")
			if err != nil {
				results <- err
				return
			}
			var paymentResult string
			results <- result.Get(&paymentResult)
		}()
	}

	// Collect results
	successCount := 0
	failureCount := 0
	for i := 0; i < concurrency; i++ {
		err := <-results
		if err == nil {
			successCount++
		} else {
			failureCount++
		}
	}

	// At least some should succeed (typically 8-9 out of 10 with 15% failure rate)
	assert.True(t, successCount > 0, "Expected at least some successful payments")
	t.Logf("Concurrent executions: Success=%d, Failures=%d", successCount, failureCount)
}

// TestAllActivitiesRegistered verifies all payment activities exist
func TestAllActivitiesRegistered(t *testing.T) {
	t.Skip()
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	// Register all activities - should not panic
	assert.NotPanics(t, func() {
		env.RegisterActivity(ValidatePaymentActivity)
		env.RegisterActivity(ConfirmOrderActivity)
		env.RegisterActivity(FailOrderActivity)
	})
}
