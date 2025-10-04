package activities

import (
	"context"
	"os"
	"time"

	"go.temporal.io/sdk/client"

	seat "github.com/EyalShahaf/temporal-seats/internal/entities/seat"
)

type SeatSignalInput struct {
	FlightID      string
	SeatID        string
	Cmd           seat.Command
	SeatTaskQueue string // Optional: override task queue
}

func SeatSignalActivity(ctx context.Context, in SeatSignalInput) error {
	host := os.Getenv("TEMPORAL_HOSTPORT")
	if host == "" {
		host = "localhost:7233"
	}
	ns := os.Getenv("TEMPORAL_NAMESPACE")
	if ns == "" {
		ns = "default"
	}
	tq := in.SeatTaskQueue
	if tq == "" {
		tq = "seat-tq"
	}

	c, err := client.Dial(client.Options{HostPort: host, Namespace: ns})
	if err != nil {
		return err
	}
	defer c.Close()

	wfID := "seat::" + in.FlightID + "::" + in.SeatID

	// Start if not running, else just signal.
	_, err = c.SignalWithStartWorkflow(
		ctx,
		wfID,   // WorkflowID
		"cmd",  // signal name
		in.Cmd, // signal payload
		client.StartWorkflowOptions{
			ID:        wfID,
			TaskQueue: tq,
			// very long-lived entity
			WorkflowExecutionTimeout: 3650 * 24 * time.Hour, // ~10y
			WorkflowRunTimeout:       365 * 24 * time.Hour,  // rotate via ContinueAsNew
		},
		seat.SeatEntityWorkflow,
		in.FlightID, in.SeatID, (*seat.SeatPersistedState)(nil), // entity workflow args
	)
	return err
}
