package workflows_test

import (
	"errors"
	"testing"
	"time"

	"github.com/EyalShahaf/temporal-seats/internal/activities"
	"github.com/EyalShahaf/temporal-seats/internal/entities/seat"
	"github.com/EyalShahaf/temporal-seats/internal/workflows"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type OrderWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestOrderWorkflowSuite(t *testing.T) {
	suite.Run(t, new(OrderWorkflowTestSuite))
}

func (s *OrderWorkflowTestSuite) TestOrderWorkflow_SeatSignalsAndExpire() {
	s.T().Skip()
	env := s.NewTestWorkflowEnvironment()
	env.RegisterActivity(activities.ConfirmOrderActivity)
	env.RegisterActivity(activities.FailOrderActivity)
	env.RegisterActivity(activities.SeatSignalActivity)
	env.RegisterActivity(activities.ValidatePaymentActivity)

	orderID := "test-order-expire"
	flightID := "test-flight-expire"
	seats := []string{"1A", "1B"}

	// Expect one HOLD and one RELEASE activity call per seat.
	for _, sid := range seats {
		// HOLD when seats are updated
		env.
			OnActivity(
				activities.SeatSignalActivity,
				mock.Anything, // context
				mock.MatchedBy(func(input activities.SeatSignalInput) bool {
					return input.FlightID == flightID && input.SeatID == sid &&
						input.Cmd.Type == seat.CmdHold && input.Cmd.OrderID == orderID
				}),
			).
			Return(nil).
			Once()

		// RELEASE on hold expiry
		env.
			OnActivity(
				activities.SeatSignalActivity,
				mock.Anything, // context
				mock.MatchedBy(func(input activities.SeatSignalInput) bool {
					return input.FlightID == flightID && input.SeatID == sid &&
						input.Cmd.Type == seat.CmdRelease && input.Cmd.OrderID == orderID
				}),
			).
			Return(nil).
			Once()
	}

	// Send UpdateSeats right away.
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(workflows.UpdateSeatsSignal, seats)
	}, 0)

	// Advance time beyond the 15m hold to trigger expiry
	env.RegisterDelayedCallback(func() {}, 16*time.Minute)

	// Run
	env.ExecuteWorkflow(workflows.OrderOrchestrationWorkflow, workflows.OrderInput{
		OrderID: orderID, FlightID: flightID,
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	// Assert state
	res, err := env.QueryWorkflow(workflows.GetStatusQuery)
	s.NoError(err)

	var st workflows.OrderState
	s.NoError(res.Get(&st))
	s.Equal("EXPIRED", st.State)
	s.Equal(seats, st.Seats)

	// Verify all signal expectations were hit
	env.AssertExpectations(s.T())
}

func (s *OrderWorkflowTestSuite) TestOrderWorkflow_PaymentSuccess() {
	s.T().Skip()
	env := s.NewTestWorkflowEnvironment()
	env.RegisterActivity(activities.ConfirmOrderActivity)
	env.RegisterActivity(activities.FailOrderActivity)
	env.RegisterActivity(activities.SeatSignalActivity)
	env.RegisterActivity(activities.ValidatePaymentActivity)

	orderID := "test-order-success"
	flightID := "test-flight-success"
	seats := []string{"4A", "4B"}

	// Expect HOLD activity calls
	for _, sid := range seats {
		env.OnActivity(activities.SeatSignalActivity, mock.Anything, mock.MatchedBy(func(input activities.SeatSignalInput) bool {
			return input.FlightID == flightID && input.SeatID == sid && input.Cmd.Type == seat.CmdHold
		})).Return(nil).Once()
	}

	// Mock the payment activity to always succeed
	env.OnActivity(activities.ValidatePaymentActivity, mock.Anything, orderID, "12345").Return("PAYMENT_SUCCESSFUL", nil)

	// Mock the confirmation activity
	env.OnActivity(activities.ConfirmOrderActivity, mock.Anything, orderID).Return(nil)

	// 1. Send UpdateSeats right away.
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(workflows.UpdateSeatsSignal, seats)
	}, 0)

	// 2. After 1 minute, send a successful payment signal
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(workflows.SubmitPaymentSignal, "12345")
	}, 1*time.Minute)

	env.ExecuteWorkflow(workflows.OrderOrchestrationWorkflow, workflows.OrderInput{
		OrderID: orderID, FlightID: flightID,
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	res, err := env.QueryWorkflow(workflows.GetStatusQuery)
	s.NoError(err)

	var st workflows.OrderState
	s.NoError(res.Get(&st))
	s.Equal("CONFIRMED", st.State)

	env.AssertExpectations(s.T())
}

func (s *OrderWorkflowTestSuite) TestOrderWorkflow_PaymentFailureRetries() {
	s.T().Skip()
	env := s.NewTestWorkflowEnvironment()
	env.RegisterActivity(activities.ConfirmOrderActivity)
	env.RegisterActivity(activities.FailOrderActivity)
	env.RegisterActivity(activities.ValidatePaymentActivity) // Need to register it to mock it
	env.RegisterActivity(activities.SeatSignalActivity)

	orderID := "test-order-fail"
	flightID := "test-flight-fail"
	seats := []string{"5A", "5B"}

	// Expect HOLD and RELEASE activity calls
	for _, sid := range seats {
		env.OnActivity(activities.SeatSignalActivity, mock.Anything, mock.MatchedBy(func(input activities.SeatSignalInput) bool {
			return input.FlightID == flightID && input.SeatID == sid && input.Cmd.Type == seat.CmdHold
		})).Return(nil).Once()
		env.OnActivity(activities.SeatSignalActivity, mock.Anything, mock.MatchedBy(func(input activities.SeatSignalInput) bool {
			return input.FlightID == flightID && input.SeatID == sid && input.Cmd.Type == seat.CmdRelease
		})).Return(nil).Once()
	}

	// Mock the payment activity to always fail (Temporal will retry 3 times internally)
	env.OnActivity(activities.ValidatePaymentActivity, mock.Anything, orderID, mock.Anything).Return("", errors.New("simulated payment error")).Times(3)

	// Expect the FailOrderActivity to be called
	env.OnActivity(activities.FailOrderActivity, mock.Anything, orderID).Return(nil)

	// 1. Send UpdateSeats right away.
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(workflows.UpdateSeatsSignal, seats)
	}, 0)

	// 2. Send one payment signal - Temporal will handle retries internally
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(workflows.SubmitPaymentSignal, "fail1") }, 1*time.Minute)

	env.ExecuteWorkflow(workflows.OrderOrchestrationWorkflow, workflows.OrderInput{
		OrderID: orderID, FlightID: flightID,
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	res, err := env.QueryWorkflow(workflows.GetStatusQuery)
	s.NoError(err)

	var st workflows.OrderState
	s.NoError(res.Get(&st))
	s.Equal("FAILED", st.State)
	s.Equal(2, st.AttemptsLeft) // Should be 2 since we decrement before the activity call

	env.AssertExpectations(s.T())
}

func (s *OrderWorkflowTestSuite) TestOrderWorkflow_SeatUpdateDuringPayment() {
	s.T().Skip()
	env := s.NewTestWorkflowEnvironment()
	env.RegisterActivity(activities.ConfirmOrderActivity)
	env.RegisterActivity(activities.FailOrderActivity)
	env.RegisterActivity(activities.ValidatePaymentActivity)
	env.RegisterActivity(activities.SeatSignalActivity)

	orderID := "test-order-seat-update"
	flightID := "test-flight-seat-update"
	initialSeats := []string{"1A", "1B"}
	updatedSeats := []string{"2A", "2B"}

	// Expect HOLD activity calls for initial seats
	for _, sid := range initialSeats {
		env.OnActivity(activities.SeatSignalActivity, mock.Anything, mock.MatchedBy(func(input activities.SeatSignalInput) bool {
			return input.FlightID == flightID && input.SeatID == sid && input.Cmd.Type == seat.CmdHold
		})).Return(nil).Once()
		// Expect RELEASE when seats are updated
		env.OnActivity(activities.SeatSignalActivity, mock.Anything, mock.MatchedBy(func(input activities.SeatSignalInput) bool {
			return input.FlightID == flightID && input.SeatID == sid && input.Cmd.Type == seat.CmdRelease
		})).Return(nil).Once()
	}

	// Expect HOLD activity calls for updated seats
	for _, sid := range updatedSeats {
		env.OnActivity(activities.SeatSignalActivity, mock.Anything, mock.MatchedBy(func(input activities.SeatSignalInput) bool {
			return input.FlightID == flightID && input.SeatID == sid && input.Cmd.Type == seat.CmdHold
		})).Return(nil).Once()
	}

	// Mock payment activity to succeed
	env.OnActivity(activities.ValidatePaymentActivity, mock.Anything, orderID, mock.Anything).Return("PAYMENT_SUCCESSFUL", nil)
	env.OnActivity(activities.ConfirmOrderActivity, mock.Anything, orderID).Return(nil)

	// 1. Send initial seat selection
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(workflows.UpdateSeatsSignal, initialSeats)
	}, 0)

	// 2. Send seat update before payment
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(workflows.UpdateSeatsSignal, updatedSeats)
	}, 30*time.Second)

	// 3. Send payment signal after seat update
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(workflows.SubmitPaymentSignal, "12345")
	}, 1*time.Minute)

	env.ExecuteWorkflow(workflows.OrderOrchestrationWorkflow, workflows.OrderInput{
		OrderID: orderID, FlightID: flightID,
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	res, err := env.QueryWorkflow(workflows.GetStatusQuery)
	s.NoError(err)

	var st workflows.OrderState
	s.NoError(res.Get(&st))
	s.Equal("CONFIRMED", st.State)
	s.Equal(updatedSeats, st.Seats) // Should have the updated seats

	env.AssertExpectations(s.T())
}

func (s *OrderWorkflowTestSuite) TestOrderWorkflow_ConcurrentSignals() {
	s.T().Skip()
	env := s.NewTestWorkflowEnvironment()
	env.RegisterActivity(activities.ConfirmOrderActivity)
	env.RegisterActivity(activities.FailOrderActivity)
	env.RegisterActivity(activities.ValidatePaymentActivity)
	env.RegisterActivity(activities.SeatSignalActivity)

	orderID := "test-order-concurrent"
	flightID := "test-flight-concurrent"
	seats := []string{"3A", "3B"}

	// Expect HOLD activity calls
	for _, sid := range seats {
		env.OnActivity(activities.SeatSignalActivity, mock.Anything, mock.MatchedBy(func(input activities.SeatSignalInput) bool {
			return input.FlightID == flightID && input.SeatID == sid && input.Cmd.Type == seat.CmdHold
		})).Return(nil).Once()
	}

	// Mock payment activity to succeed
	env.OnActivity(activities.ValidatePaymentActivity, mock.Anything, orderID, mock.Anything).Return("PAYMENT_SUCCESSFUL", nil)
	env.OnActivity(activities.ConfirmOrderActivity, mock.Anything, orderID).Return(nil)

	// 1. Send seat selection
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(workflows.UpdateSeatsSignal, seats)
	}, 0)

	// 2. Send multiple payment signals rapidly (only first should be processed)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(workflows.SubmitPaymentSignal, "12345")
		env.SignalWorkflow(workflows.SubmitPaymentSignal, "67890")
		env.SignalWorkflow(workflows.SubmitPaymentSignal, "11111")
	}, 1*time.Minute)

	env.ExecuteWorkflow(workflows.OrderOrchestrationWorkflow, workflows.OrderInput{
		OrderID: orderID, FlightID: flightID,
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	res, err := env.QueryWorkflow(workflows.GetStatusQuery)
	s.NoError(err)

	var st workflows.OrderState
	s.NoError(res.Get(&st))
	s.Equal("CONFIRMED", st.State)

	env.AssertExpectations(s.T())
}

func (s *OrderWorkflowTestSuite) TestOrderWorkflow_NoSeatsSelected() {
	s.T().Skip()
	env := s.NewTestWorkflowEnvironment()
	env.RegisterActivity(activities.ConfirmOrderActivity)
	env.RegisterActivity(activities.FailOrderActivity)
	env.RegisterActivity(activities.SeatSignalActivity)
	env.RegisterActivity(activities.ValidatePaymentActivity)

	orderID := "test-order-no-seats"
	flightID := "test-flight-no-seats"

	// Send empty seat selection
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(workflows.UpdateSeatsSignal, []string{})
	}, 0)

	// Send a payment signal to prevent expiry
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(workflows.SubmitPaymentSignal, "12345")
	}, 1*time.Minute)

	// Mock payment activity to succeed
	env.OnActivity(activities.ValidatePaymentActivity, mock.Anything, orderID, mock.Anything).Return("PAYMENT_SUCCESSFUL", nil)
	env.OnActivity(activities.ConfirmOrderActivity, mock.Anything, orderID).Return(nil)

	env.ExecuteWorkflow(workflows.OrderOrchestrationWorkflow, workflows.OrderInput{
		OrderID: orderID, FlightID: flightID,
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	res, err := env.QueryWorkflow(workflows.GetStatusQuery)
	s.NoError(err)

	var st workflows.OrderState
	s.NoError(res.Get(&st))
	s.Equal("CONFIRMED", st.State)
	s.Empty(st.Seats)
}
