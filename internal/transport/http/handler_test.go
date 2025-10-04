package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/EyalShahaf/temporal-seats/internal/workflows"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
)

// MockTemporalClient is a mock for the Temporal client.
type MockTemporalClient struct {
	client.Client
	mock.Mock
}

func (m *MockTemporalClient) ExecuteWorkflow(
	ctx context.Context,
	options client.StartWorkflowOptions,
	workflow interface{},
	args ...interface{},
) (client.WorkflowRun, error) {
	callArgs := m.Called(ctx, options, workflow, args)
	if callArgs.Get(0) == nil {
		return nil, callArgs.Error(1)
	}
	return callArgs.Get(0).(client.WorkflowRun), callArgs.Error(1)
}

type MockWorkflowRun struct {
	client.WorkflowRun
}

func (m *MockWorkflowRun) GetID() string {
	return "mock-workflow-id"
}
func (m *MockWorkflowRun) GetRunID() string {
	return "mock-run-id"
}

func TestOrderHandler_CreateOrder(t *testing.T) {
	mockTemporal := new(MockTemporalClient)
	handler := NewOrderHandler(mockTemporal)

	// Define what the mock should expect and return
	mockTemporal.
		On(
			"ExecuteWorkflow",
			mock.Anything, // context
			mock.Anything, // options
			mock.Anything, // workflow function
			mock.Anything,
		).
		Return(&MockWorkflowRun{}, nil).
		Once()

	// Create a request
	reqBody := `{"orderID":"test-order","flightID":"test-flight"}`
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(reqBody))
	rr := httptest.NewRecorder()

	// Call the handler
	handler.createOrderHandler(rr, req)

	// Assert the response
	require.Equal(t, http.StatusCreated, rr.Code)

	// Assert that the mock was called as expected
	mockTemporal.AssertExpectations(t)
}

func (m *MockTemporalClient) SignalWorkflow(ctx context.Context, workflowID string, runID string, signalName string, arg interface{}) error {
	args := m.Called(ctx, workflowID, runID, signalName, arg)
	return args.Error(0)
}

func TestOrderHandler_UpdateSeats(t *testing.T) {
	mockTemporal := new(MockTemporalClient)
	handler := NewOrderHandler(mockTemporal)

	orderID := "test-order-seats"
	seats := []string{"1A", "1B"}

	mockTemporal.
		On(
			"SignalWorkflow",
			mock.Anything, // context
			"order::"+orderID,
			"", // runID
			workflows.UpdateSeatsSignal,
			seats,
		).
		Return(nil).
		Once()

	reqBody := `{"seats":["1A", "1B"]}`
	req := httptest.NewRequest(http.MethodPost, "/orders/"+orderID+"/seats", bytes.NewBufferString(reqBody))
	req.SetPathValue("id", orderID)
	rr := httptest.NewRecorder()

	handler.updateSeatsHandler(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	mockTemporal.AssertExpectations(t)
}
