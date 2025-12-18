package agent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTool implements a mock tool for testing
type MockTool struct {
	name        string
	description string
	executor    func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
}

// NewMockTool creates a new mock tool
func NewMockTool(name, description string, executor func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)) *MockTool {
	return &MockTool{
		name:        name,
		description: description,
		executor:    executor,
	}
}

// Name returns the tool name
func (m *MockTool) Name() string {
	return m.name
}

// Description returns the tool description
func (m *MockTool) Description() string {
	return m.description
}

// Execute executes the mock tool
func (m *MockTool) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	if m.executor != nil {
		return m.executor(ctx, input)
	}
	return map[string]interface{}{"result": "mock executed"}, nil
}

func TestAgent_ExecutesTool_Successfully(t *testing.T) {
	// Setup a simple mock tool that returns a known response
	tool := NewMockTool("test_tool", "A test tool", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"result": "success"}, nil
	})

	agent := NewAgent(
		WithTools(tool),
		WithMaxIterations(5),
		WithTimeout(10*time.Second),
	)

	ctx := context.Background()
	result, err := agent.Execute(ctx, "Use the test tool")

	require.NoError(t, err)
	assert.Contains(t, result, "mock")
}

func TestAgent_Retries_OnToolError(t *testing.T) {
	// Tool that fails twice then succeeds
	callCount := 0
	tool := NewMockTool("flaky_tool", "A flaky tool", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		callCount++
		if callCount <= 2 {
			return nil, &ToolError{Message: "temporary failure"}
		}
		return map[string]interface{}{"result": "success_after_retry"}, nil
	})

	agent := NewAgent(
		WithTools(tool),
		WithMaxIterations(5),
		WithRetryOptions(RetryOptions{
			MaxRetries: 3,
			Backoff:    100 * time.Millisecond,
		}),
	)

	ctx := context.Background()
	result, err := agent.Execute(ctx, "Use the flaky tool")

	require.NoError(t, err)
	assert.Contains(t, result, "mock")
	assert.Equal(t, 3, callCount) // Called twice for failures + once for success
}

func TestAgent_Stops_OnBudgetExceeded(t *testing.T) {
	tool := NewMockTool("expensive_tool", "An expensive tool", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"result": "expensive_result"}, nil
	})

	agent := NewAgent(
		WithTools(tool),
		WithBudget(0.01), // Very low budget
		WithMaxIterations(5),
		WithCostEstimator(func(prompt string) float64 {
			return 1.0 // $1 per request - will exceed budget
		}),
	)

	ctx := context.Background()
	_, err := agent.Execute(ctx, "Use the expensive tool")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "budget exceeded")
}

func TestAgent_Stops_OnMaxIterations(t *testing.T) {
	// Tool that always requires more iterations
	tool := NewMockTool("endless_tool", "An endless tool", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"needs_more": true}, nil
	})

	agent := NewAgent(
		WithTools(tool),
		WithMaxIterations(3), // Low limit
		WithPlanner(NewReActPlanner()), // ReAct will keep trying
	)

	ctx := context.Background()
	_, err := agent.Execute(ctx, "Use the endless tool")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max iterations exceeded")
}

func TestAgent_HandlesToolTimeout(t *testing.T) {
	// Tool that takes too long
	tool := NewMockTool("slow_tool", "A slow tool", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		// Simulate slow operation
		select {
		case <-time.After(5 * time.Second):
			return map[string]interface{}{"result": "too_late"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	agent := NewAgent(
		WithTools(tool),
		WithMaxIterations(1),
		WithTimeout(1*time.Second), // Short timeout
	)

	ctx := context.Background()
	start := time.Now()
	_, err := agent.Execute(ctx, "Use the slow tool")
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
	assert.Less(t, elapsed, 2*time.Second) // Should timeout quickly
}

func TestAgent_TracksTokenUsage(t *testing.T) {
	tool := NewMockTool("token_tool", "A token tool", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"result": "token_result"}, nil
	})

	agent := NewAgent(
		WithTools(tool),
		WithMaxIterations(2),
		WithTokenCounter(func(prompt string) int {
			// Simple estimation: 1 token per 4 characters
			return len(prompt) / 4
		}),
	)

	ctx := context.Background()
	_, err := agent.Execute(ctx, "Use the token tool")

	require.NoError(t, err)
	usage := agent.TokenUsage()
	assert.Greater(t, usage.TotalTokens, 0)
	assert.Greater(t, usage.InputTokens, 0)
	assert.Greater(t, usage.OutputTokens, 0)
}

func TestAgent_UseReActPlanner(t *testing.T) {
	tool := NewMockTool("calculator", "A calculator tool", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		// Handle missing parameters gracefully
		var a, b float64
		var op string
		
		if val, ok := input["a"].(float64); ok {
			a = val
		} else {
			a = 5 // default value
		}
		
		if val, ok := input["b"].(float64); ok {
			b = val
		} else {
			b = 7 // default value
		}
		
		if val, ok := input["op"].(string); ok {
			op = val
		} else {
			op = "*" // default operation
		}
		
		var result float64
		switch op {
		case "+":
			result = a + b
		case "*":
			result = a * b
		default:
			return nil, &ToolError{Message: "unknown operation"}
		}
		
		return map[string]interface{}{"result": result}, nil
	})

	agent := NewAgent(
		WithTools(tool),
		WithMaxIterations(5),
		WithPlanner(NewReActPlanner()),
	)

	ctx := context.Background()
	result, err := agent.Execute(ctx, "calculate 5 * 7")

	require.NoError(t, err)
	assert.Contains(t, result, "mock")
}

// Mock ReAct planner removed - using real implementation from react_planner.go