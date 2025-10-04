package seat

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

type CommandType string

const (
	CmdHold    CommandType = "HOLD"
	CmdExtend  CommandType = "EXTEND"
	CmdRelease CommandType = "RELEASE"
	CmdConfirm CommandType = "CONFIRM" // NEW - permanent lock after payment
)

type Command struct {
	Type    CommandType
	OrderID string
	TTL     time.Duration
}

type seatState struct {
	isHeld      bool
	isConfirmed bool   // NEW - permanently reserved after payment
	heldBy      string // which orderID holds it
	confirmedBy string // NEW - which orderID confirmed it
	expiresAt   time.Time
}

// SeatState represents the public state of a seat
type SeatState struct {
	IsHeld      bool   `json:"isHeld"`
	IsConfirmed bool   `json:"isConfirmed"`
	HeldBy      string `json:"heldBy"`
	ConfirmedBy string `json:"confirmedBy"`
	ExpiresAt   string `json:"expiresAt"` // ISO string for JSON serialization
}

// SeatEntityWorkflow manages the state of a single seat.
// Its workflow ID should be "seat::<flightID>::<seatID>".
func SeatEntityWorkflow(ctx workflow.Context, flightID, seatID string) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting SeatEntityWorkflow", "FlightID", flightID, "SeatID", seatID)

	state := seatState{isHeld: false}
	cmdChan := workflow.GetSignalChannel(ctx, "cmd")
	processed := 0

	// Register query handler to expose seat state
	workflow.SetQueryHandler(ctx, "GetState", func() (SeatState, error) {
		return SeatState{
			IsHeld:      state.isHeld,
			IsConfirmed: state.isConfirmed,
			HeldBy:      state.heldBy,
			ConfirmedBy: state.confirmedBy,
			ExpiresAt:   state.expiresAt.Format(time.RFC3339),
		}, nil
	})

	for {
		// Block until a command is received.
		var cmd Command
		cmdChan.Receive(ctx, &cmd)
		logger.Info("Received command", "Type", cmd.Type, "OrderID", cmd.OrderID)

		switch cmd.Type {
		case CmdHold:
			// Reject hold if seat is already confirmed by another order
			if state.isConfirmed {
				logger.Warn("Cannot hold - seat is confirmed by another order", "ConfirmedBy", state.confirmedBy)
				continue
			}
			// Reject hold if seat is already held and not expired
			if state.isHeld && !workflow.Now(ctx).After(state.expiresAt) {
				logger.Warn("Seat is already held and not expired", "HeldBy", state.heldBy)
				continue
			}
			state.isHeld = true
			state.heldBy = cmd.OrderID
			state.expiresAt = workflow.Now(ctx).Add(cmd.TTL)
			logger.Info("Seat is now HELD", "HeldBy", state.heldBy, "ExpiresAt", state.expiresAt)

			// Start a timer to auto-release the seat
			timerCtx, cancelTimer := workflow.WithCancel(ctx)
			timer := workflow.NewTimer(timerCtx, cmd.TTL)
			selector := workflow.NewSelector(ctx)
			selector.AddFuture(timer, func(f workflow.Future) {
				logger.Info("Hold expired, releasing seat.", "HeldBy", state.heldBy)
				state.isHeld = false
				state.heldBy = ""
			})

			// While waiting for the timer, listen for other commands (e.g. RELEASE)
			selector.AddReceive(cmdChan, func(c workflow.ReceiveChannel, more bool) {
				c.Receive(ctx, &cmd) // process this new command in the next loop iteration
				cancelTimer()        // cancel the auto-release timer
			})

			selector.Select(ctx)

		case CmdRelease:
			if state.isHeld && state.heldBy == cmd.OrderID {
				state.isHeld = false
				state.heldBy = ""
				logger.Info("Seat released by order.", "OrderID", cmd.OrderID)
			}

		case CmdConfirm:
			// Reject confirm if seat is already confirmed by another order
			if state.isConfirmed {
				logger.Warn("Seat already confirmed by another order", "ConfirmedBy", state.confirmedBy)
				continue
			}
			// Confirm the seat permanently
			state.isConfirmed = true
			state.confirmedBy = cmd.OrderID
			state.isHeld = false // Clear hold since it's now confirmed
			state.heldBy = ""    // Clear heldBy since it's now confirmed
			logger.Info("Seat PERMANENTLY CONFIRMED", "ConfirmedBy", state.confirmedBy)
		}

		processed++
		if processed%1000 == 0 {
			// Keep running forever; trim history via ContinueAsNew
			logger.Info("Continuing as new to trim history", "Processed", processed)
			return workflow.NewContinueAsNewError(ctx, SeatEntityWorkflow, flightID, seatID)
		}
	}
}
