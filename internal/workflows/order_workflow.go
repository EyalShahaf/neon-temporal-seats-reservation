package workflows

import (
	"time"

	"github.com/EyalShahaf/temporal-seats/internal/activities"
	"github.com/EyalShahaf/temporal-seats/internal/entities/seat"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	UpdateSeatsSignal   = "UpdateSeats"
	SubmitPaymentSignal = "SubmitPayment"
	GetStatusQuery      = "GetStatus"
)

// OrderInput defines the required inputs to start the order workflow.
type OrderInput struct {
	OrderID  string
	FlightID string
}

// OrderState represents the current state of the order process.
type OrderState struct {
	State          string    `json:"State"`
	Seats          []string  `json:"Seats"`
	HoldExpiresAt  time.Time `json:"HoldExpiresAt"`
	AttemptsLeft   int       `json:"AttemptsLeft"`
	LastPaymentErr string    `json:"LastPaymentErr,omitempty"`
}

// OrderOrchestrationWorkflow is the main Temporal workflow for an entire seat reservation and payment process.
func OrderOrchestrationWorkflow(ctx workflow.Context, input OrderInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting OrderOrchestrationWorkflow", "OrderID", input.OrderID, "FlightID", input.FlightID)

	// Set up initial state
	state := OrderState{
		State: "PENDING",
		Seats: []string{},
	}

	// Register query handler
	err := workflow.SetQueryHandler(ctx, GetStatusQuery, func() (OrderState, error) {
		return state, nil
	})
	if err != nil {
		logger.Error("Failed to register GetStatus query handler", "error", err)
		return err
	}

	// Wait for seat selection
	updateSeatsChan := workflow.GetSignalChannel(ctx, UpdateSeatsSignal)

	// Block until seats are selected for the first time
	// For subsequent updates, we'll use a selector inside the main loop
	var seats []string
	updateSeatsChan.Receive(ctx, &seats)

	// Hold the selected seats using activity
	ao := workflow.ActivityOptions{
		StartToCloseTimeout:    10 * time.Second,
		ScheduleToCloseTimeout: 1 * time.Minute,
		RetryPolicy:            &temporal.RetryPolicy{MaximumAttempts: 5},
	}
	ctxA := workflow.WithActivityOptions(ctx, ao)

	for _, seatID := range seats {
		cmd := seat.Command{Type: seat.CmdHold, OrderID: input.OrderID, TTL: 15 * time.Minute}
		_ = workflow.ExecuteActivity(ctxA, activities.SeatSignalActivity, activities.SeatSignalInput{
			FlightID: input.FlightID, SeatID: seatID, Cmd: cmd,
		}).Get(ctx, nil)
	}

	state.Seats = seats
	state.State = "SEATS_SELECTED"
	state.HoldExpiresAt = workflow.Now(ctx).Add(15 * time.Minute)
	logger.Info("Seats selected, hold timer started.", "Seats", state.Seats, "ExpiresAt", state.HoldExpiresAt)

	// Set up payment signal channel
	paymentChan := workflow.GetSignalChannel(ctx, SubmitPaymentSignal)
	state.AttemptsLeft = 3

	// Main loop for handling signals or timer expiry
	for state.State != "CONFIRMED" && state.State != "FAILED" && state.State != "EXPIRED" {
		timerCtx, cancelTimer := workflow.WithCancel(ctx)
		holdTimer := workflow.NewTimer(timerCtx, time.Until(state.HoldExpiresAt))

		selector := workflow.NewSelector(ctx)
		selector.AddFuture(holdTimer, func(f workflow.Future) {
			state.State = "EXPIRED"
			logger.Warn("Hold timer expired.")
		})

		selector.AddReceive(updateSeatsChan, func(c workflow.ReceiveChannel, more bool) {
			var newSeats []string
			c.Receive(ctx, &newSeats)

			// Determine which seats to release and which to hold
			toRelease, toHold := diffSeats(state.Seats, newSeats)
			logger.Info("Updating seats", "ToRelease", toRelease, "ToHold", toHold)

			// Release old seats
			for _, seatID := range toRelease {
				cmd := seat.Command{Type: seat.CmdRelease, OrderID: input.OrderID}
				_ = workflow.ExecuteActivity(ctxA, activities.SeatSignalActivity, activities.SeatSignalInput{
					FlightID: input.FlightID, SeatID: seatID, Cmd: cmd,
				}).Get(ctx, nil)
			}

			// Hold new seats
			for _, seatID := range toHold {
				cmd := seat.Command{Type: seat.CmdHold, OrderID: input.OrderID, TTL: 15 * time.Minute}
				_ = workflow.ExecuteActivity(ctxA, activities.SeatSignalActivity, activities.SeatSignalInput{
					FlightID: input.FlightID, SeatID: seatID, Cmd: cmd,
				}).Get(ctx, nil)
			}

			state.Seats = newSeats
			state.HoldExpiresAt = workflow.Now(ctx).Add(15 * time.Minute)
			logger.Info("Hold timer has been refreshed.", "ExpiresAt", state.HoldExpiresAt)
		})

		selector.AddReceive(paymentChan, func(c workflow.ReceiveChannel, more bool) {
			var paymentCode string
			c.Receive(ctx, &paymentCode)

			if state.AttemptsLeft <= 0 {
				logger.Warn("No payment attempts left.")
				return
			}
			state.AttemptsLeft--

			// Set up activity options with built-in retry policy
			activityOpts := workflow.ActivityOptions{
				StartToCloseTimeout: 10 * time.Second,
				RetryPolicy: &temporal.RetryPolicy{
					MaximumAttempts:    3, // Use Temporal's built-in retry mechanism
					InitialInterval:    1 * time.Second,
					MaximumInterval:    5 * time.Second,
					BackoffCoefficient: 2.0,
				},
			}
			ctx = workflow.WithActivityOptions(ctx, activityOpts)

			// Execute the payment activity with built-in retries
			var result string
			err := workflow.ExecuteActivity(ctx, activities.ValidatePaymentActivity, input.OrderID, paymentCode).Get(ctx, &result)

			if err != nil {
				logger.Error("Payment activity failed after all retries", "error", err)
				state.LastPaymentErr = err.Error()
				state.State = "FAILED"
				logger.Error("Order failed after maximum payment attempts.")
			} else {
				logger.Info("Payment successful")
				state.State = "CONFIRMED"
			}
		})

		selector.Select(ctx)
		cancelTimer() // clean up the timer
	}

	// Post-loop logic based on final state
	activityOpts := workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Second}
	ctx = workflow.WithActivityOptions(ctx, activityOpts)

	// Ensure we have the same activity context for seat signals
	ao = workflow.ActivityOptions{
		StartToCloseTimeout:    10 * time.Second,
		ScheduleToCloseTimeout: 1 * time.Minute,
		RetryPolicy:            &temporal.RetryPolicy{MaximumAttempts: 5},
	}
	ctxA = workflow.WithActivityOptions(ctx, ao)

	switch state.State {
	case "CONFIRMED":
		_ = workflow.ExecuteActivity(ctx, activities.ConfirmOrderActivity, input.OrderID).Get(ctx, nil)

		// Send CONFIRM signals to all seats to permanently lock them
		logger.Info("Payment confirmed, permanently locking seats", "Seats", state.Seats)
		for _, seatID := range state.Seats {
			cmd := seat.Command{Type: seat.CmdConfirm, OrderID: input.OrderID}
			_ = workflow.ExecuteActivity(ctxA, activities.SeatSignalActivity, activities.SeatSignalInput{
				FlightID: input.FlightID, SeatID: seatID, Cmd: cmd,
			}).Get(ctx, nil)
		}
	case "FAILED", "EXPIRED":
		// Best-effort attempt to release seats
		dCtx, cancel := workflow.NewDisconnectedContext(ctx)
		defer cancel()
		for _, seatID := range state.Seats {
			cmd := seat.Command{Type: seat.CmdRelease, OrderID: input.OrderID}
			_ = workflow.ExecuteActivity(dCtx, activities.SeatSignalActivity, activities.SeatSignalInput{
				FlightID: input.FlightID, SeatID: seatID, Cmd: cmd,
			}).Get(dCtx, nil)
		}
		_ = workflow.ExecuteActivity(ctx, activities.FailOrderActivity, input.OrderID).Get(ctx, nil)
	}

	logger.Info("Order workflow completed.", "FinalState", state.State)

	// Keep the workflow running to handle queries and maintain state
	// This ensures the workflow doesn't complete and die
	for {
		// Wait for any additional signals or just keep running
		// The workflow will be terminated by Temporal when appropriate
		workflow.Sleep(ctx, time.Hour)
	}
}

// diffSeats calculates which seats to release and which to hold.
func diffSeats(oldSeats, newSeats []string) (toRelease, toHold []string) {
	oldSet := make(map[string]bool)
	for _, s := range oldSeats {
		oldSet[s] = true
	}

	newSet := make(map[string]bool)
	for _, s := range newSeats {
		newSet[s] = true
	}

	for s := range oldSet {
		if !newSet[s] {
			toRelease = append(toRelease, s)
		}
	}

	for s := range newSet {
		if !oldSet[s] {
			toHold = append(toHold, s)
		}
	}

	return toRelease, toHold
}
