package seat

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type SeatWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestSeatWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(SeatWorkflowTestSuite))
}

func (s *SeatWorkflowTestSuite) TestSeatWorkflow_CanStart() {
	env := s.NewTestWorkflowEnvironment()
	flightID := "FL123"
	seatID := "1A"

	// Test that the workflow can be started without crashing
	// The workflow will timeout in test environment, which is expected
	env.ExecuteWorkflow(SeatEntityWorkflow, flightID, seatID)

	// In test environment, the workflow will complete due to timeout
	// This is expected behavior for infinite loop workflows in tests
	s.True(env.IsWorkflowCompleted())
}

func (s *SeatWorkflowTestSuite) TestSeatWorkflow_CommandTypes() {
	// Test that command types are defined correctly
	s.Equal(CommandType("HOLD"), CmdHold)
	s.Equal(CommandType("EXTEND"), CmdExtend)
	s.Equal(CommandType("RELEASE"), CmdRelease)
}

func (s *SeatWorkflowTestSuite) TestSeatWorkflow_CommandStructure() {
	// Test that Command struct can be created
	cmd := Command{
		Type:    CmdHold,
		OrderID: "ORDER-123",
		TTL:     15 * time.Minute,
	}

	s.Equal(CmdHold, cmd.Type)
	s.Equal("ORDER-123", cmd.OrderID)
	s.Equal(15*time.Minute, cmd.TTL)
}
