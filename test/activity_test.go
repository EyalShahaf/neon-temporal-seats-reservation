package test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/EyalShahaf/temporal-seats/internal/activities"
	"github.com/EyalShahaf/temporal-seats/internal/entities/seat"
	"github.com/stretchr/testify/assert"
)

func TestSeatSignalActivity(t *testing.T) {
	t.Skip()
	// Set up environment variables for the activity
	os.Setenv("TEMPORAL_HOSTPORT", "localhost:7233")
	os.Setenv("TEMPORAL_NAMESPACE", "default")
	os.Setenv("SEAT_TASK_QUEUE", "seat-tq")
	defer func() {
		os.Unsetenv("TEMPORAL_HOSTPORT")
		os.Unsetenv("TEMPORAL_NAMESPACE")
		os.Unsetenv("SEAT_TASK_QUEUE")
	}()

	ctx := context.Background()

	// Test input
	input := activities.SeatSignalInput{
		FlightID: "FL-001",
		SeatID:   "4A", // Use a real seat ID
		Cmd: seat.Command{
			Type:    seat.CmdHold,
			OrderID: "test-order-activity",
			TTL:     15 * time.Minute,
		},
	}

	// Execute the activity
	err := activities.SeatSignalActivity(ctx, input)

	// The activity should succeed (it will start the seat entity workflow)
	assert.NoError(t, err, "SeatSignalActivity should succeed")

	t.Log("✅ SeatSignalActivity test completed successfully!")
}
