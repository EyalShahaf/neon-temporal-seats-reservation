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

// SeatPersistedState is the exported DTO for Continue-As-New serialization
type SeatPersistedState struct {
	IsHeld      bool      `json:"isHeld"`
	IsConfirmed bool      `json:"isConfirmed"`
	HeldBy      string    `json:"heldBy"`
	ConfirmedBy string    `json:"confirmedBy"`
	ExpiresAt   time.Time `json:"expiresAt"`
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
func SeatEntityWorkflow(ctx workflow.Context, flightID, seatID string, initial *SeatPersistedState) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting SeatEntityWorkflow", "FlightID", flightID, "SeatID", seatID)

	state := seatState{}
	if initial != nil {
		state = seatState{
			isHeld:      initial.IsHeld,
			isConfirmed: initial.IsConfirmed,
			heldBy:      initial.HeldBy,
			confirmedBy: initial.ConfirmedBy,
			expiresAt:   initial.ExpiresAt,
		}
		logger.Info("Restored state from ContinueAsNew", "IsHeld", state.isHeld, "IsConfirmed", state.isConfirmed, "HeldBy", state.heldBy, "ConfirmedBy", state.confirmedBy)
	}

	cmdChan := workflow.GetSignalChannel(ctx, "cmd")
	var holdTimer workflow.Future
	var holdCancel workflow.CancelFunc
	processed := 0

	// Register query handler to expose seat state
	workflow.SetQueryHandler(ctx, "GetState", func() (SeatState, error) {
		var exp string
		if !state.expiresAt.IsZero() {
			exp = state.expiresAt.Format(time.RFC3339)
		}
		return SeatState{
			IsHeld:      state.isHeld,
			IsConfirmed: state.isConfirmed,
			HeldBy:      state.heldBy,
			ConfirmedBy: state.confirmedBy,
			ExpiresAt:   exp,
		}, nil
	})

	makeHoldTimer := func(ttl time.Duration) {
		// Clamp TTL defensively to prevent zero/negative values
		if ttl < time.Second {
			ttl = time.Second
		}

		// cancel previous timer (if any)
		if holdCancel != nil {
			holdCancel()
			holdCancel = nil
			holdTimer = nil
		}
		holdCtx, cancel := workflow.WithCancel(ctx)
		holdCancel = cancel
		holdTimer = workflow.NewTimer(holdCtx, ttl)
		state.expiresAt = workflow.Now(ctx).Add(ttl)
	}

	clearHold := func() {
		state.isHeld = false
		state.heldBy = ""
		state.expiresAt = time.Time{}
		if holdCancel != nil {
			holdCancel()
			holdCancel = nil
			holdTimer = nil
		}
	}

	// Restore timer if seat was held and not expired
	if state.isHeld && !state.expiresAt.IsZero() && workflow.Now(ctx).Before(state.expiresAt) {
		// Re-arm timer for remaining duration
		remaining := state.expiresAt.Sub(workflow.Now(ctx))
		holdCtx, cancel := workflow.WithCancel(ctx)
		holdCancel = cancel
		holdTimer = workflow.NewTimer(holdCtx, remaining)
		logger.Info("Restored hold timer", "Remaining", remaining, "HeldBy", state.heldBy)
	} else if state.isHeld {
		// Hold already expired between runs
		logger.Info("Hold expired during ContinueAsNew, clearing", "HeldBy", state.heldBy)
		state.isHeld = false
		state.heldBy = ""
		state.expiresAt = time.Time{}
	}

	for {
		sel := workflow.NewSelector(ctx)

		// Incoming commands
		sel.AddReceive(cmdChan, func(c workflow.ReceiveChannel, more bool) {
			var cmd Command
			c.Receive(ctx, &cmd)
			logger.Info("Received command", "Type", cmd.Type, "OrderID", cmd.OrderID)

			// Early guard: ignore all commands on confirmed seats except idempotent confirm
			if state.isConfirmed {
				if cmd.Type == CmdConfirm && cmd.OrderID == state.confirmedBy {
					logger.Info("Confirm idempotent - already confirmed", "OrderID", cmd.OrderID)
				} else {
					logger.Warn("Ignoring command on confirmed seat", "Type", cmd.Type, "ConfirmedBy", state.confirmedBy)
				}
				return
			}

			switch cmd.Type {

			case CmdHold:
				// If already held by someone else and not expired, reject
				if state.isHeld && state.heldBy != cmd.OrderID && workflow.Now(ctx).Before(state.expiresAt) {
					logger.Warn("Seat already held and not expired", "HeldBy", state.heldBy)
					return
				}
				// Grant/refresh hold for this order
				state.isHeld = true
				state.heldBy = cmd.OrderID
				makeHoldTimer(cmd.TTL)
				logger.Info("Seat HELD", "HeldBy", state.heldBy, "ExpiresAt", state.expiresAt)

			case CmdExtend:
				// Only the current holder may extend
				if state.isHeld && state.heldBy == cmd.OrderID {
					makeHoldTimer(cmd.TTL)
					logger.Info("Seat HOLD EXTENDED", "HeldBy", state.heldBy, "ExpiresAt", state.expiresAt)
				} else {
					logger.Warn("Extend ignored - not held by this order", "HeldBy", state.heldBy, "OrderID", cmd.OrderID)
				}

			case CmdRelease:
				if state.isHeld && state.heldBy == cmd.OrderID {
					clearHold()
					logger.Info("Seat RELEASED by order", "OrderID", cmd.OrderID)
				} else {
					logger.Warn("Release ignored - not held by this order", "HeldBy", state.heldBy, "OrderID", cmd.OrderID)
				}

			case CmdConfirm:
				// Only holder can confirm; once confirmed, make it permanent
				if !state.isHeld || state.heldBy != cmd.OrderID {
					logger.Warn("Confirm ignored - seat not held by this order", "HeldBy", state.heldBy, "OrderID", cmd.OrderID)
					return
				}
				state.isConfirmed = true
				state.confirmedBy = cmd.OrderID
				clearHold()
				logger.Info("Seat PERMANENTLY CONFIRMED", "ConfirmedBy", state.confirmedBy)
			}
		})

		// Hold expiry
		if holdTimer != nil {
			sel.AddFuture(holdTimer, func(f workflow.Future) {
				// Timer fired (not canceled) → release hold
				logger.Info("Hold EXPIRED, releasing seat", "HeldBy", state.heldBy)
				clearHold()
			})
		}

		sel.Select(ctx)

		processed++
		if processed%1000 == 0 {
			logger.Info("Continuing as new to trim history", "Processed", processed)
			return workflow.NewContinueAsNewError(ctx, SeatEntityWorkflow, flightID, seatID,
				&SeatPersistedState{
					IsHeld:      state.isHeld,
					IsConfirmed: state.isConfirmed,
					HeldBy:      state.heldBy,
					ConfirmedBy: state.confirmedBy,
					ExpiresAt:   state.expiresAt,
				})
		}
	}
}
