package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock tool for testing
type mockTool struct {
	name string
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{
		"result": fmt.Sprintf("mock result from %s", m.name),
	}, nil
}

func (m *mockTool) Description() string {
	return fmt.Sprintf("Mock tool %s for testing", m.name)
}

// Mock memory for testing
type mockMemory struct {
	messages []MemoryMessage
}

func (m *mockMemory) Store(ctx context.Context, msg MemoryMessage) error {
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockMemory) Retrieve(ctx context.Context, query string, limit int) ([]MemoryMessage, error) {
	if limit <= 0 || limit > len(m.messages) {
		return m.messages, nil
	}
	return m.messages[:limit], nil
}

func (m *mockMemory) Search(ctx context.Context, pattern string) ([]MemoryMessage, error) {
	var results []MemoryMessage
	// No limit specified, return all matches
	for _, msg := range m.messages {
		if contains(msg.Content, pattern) {
			results = append(results, msg)
		}
	}
	return results, nil
}

func (m *mockMemory) Clear(ctx context.Context) error {
	m.messages = []MemoryMessage{}
	return nil
}

// Helper function for string containment
func TestNewReActPlanner_CreatesPlannerWithDefaults(t *testing.T) {
	planner := NewReActPlanner()
	
	assert.Equal(t, 5, planner.maxThoughts)
	assert.Equal(t, 3, planner.maxActions)
	assert.NotNil(t, planner.toolSelector)
	assert.Equal(t, "react_history", planner.memoryKey)
}

func TestReActPlanner_WithMaxThoughts_SetsMaxThoughts(t *testing.T) {
	planner := NewReActPlanner().WithMaxThoughts(10)
	
	assert.Equal(t, 10, planner.maxThoughts)
}

func TestReActPlanner_WithMaxActions_SetsMaxActions(t *testing.T) {
	planner := NewReActPlanner().WithMaxActions(5)
	
	assert.Equal(t, 5, planner.maxActions)
}

func TestReActPlanner_WithToolSelector_SetsCustomSelector(t *testing.T) {
	customSelector := &DefaultToolSelector{}
	planner := NewReActPlanner().WithToolSelector(customSelector)
	
	assert.Equal(t, customSelector, planner.toolSelector)
}

func TestReActPlanner_Plan_GeneratesBasicPlan(t *testing.T) {
	planner := NewReActPlanner()
	state := State{
		Tools:    []Tool{},
		Messages: []MemoryMessage{},
		Context:  make(map[string]interface{}),
	}
	
	plan, err := planner.Plan(context.Background(), state, "test goal")
	
	require.NoError(t, err)
	assert.NotEmpty(t, plan.Steps)
	assert.Equal(t, "think", plan.Steps[0].Type)
	assert.Contains(t, plan.Steps[0].Thought, "I need to: test goal")
}

func TestReActPlanner_Plan_WithMemory_IncludesMemoryContext(t *testing.T) {
	planner := NewReActPlanner()
	memory := &mockMemory{
		messages: []MemoryMessage{
			{
				ID:        "1",
				Content:   "Previous thought: I need to use tools",
				Role:      "assistant",
				Timestamp: 1234567890,
			},
		},
	}
	
	// Store a message in memory first
	ctx := context.Background()
	memory.Store(ctx, MemoryMessage{
		ID:        "1",
		Content:   "Previous thought: I need to use tools",
		Role:      "assistant",
		Timestamp: 1234567890,
	})
	
	state := State{
		Tools:    []Tool{},
		Messages: memory.messages, // Use messages from memory
		Context:  make(map[string]interface{}),
	}
	
	plan, err := planner.Plan(ctx, state, "test goal")
	
	require.NoError(t, err)
	assert.NotEmpty(t, plan.Steps)
	assert.Contains(t, plan.Steps[0].Thought, "I will use my memory")
}

func TestReActPlanner_Plan_WithTools_SelectsAppropriateTool(t *testing.T) {
	planner := NewReActPlanner()
	tool := &mockTool{name: "EchoTool"}
	state := State{
		Tools:    []Tool{tool},
		Messages: []MemoryMessage{},
		Context:  make(map[string]interface{}),
	}
	
	plan, err := planner.Plan(context.Background(), state, "echo this message")
	
	require.NoError(t, err)
	assert.Len(t, plan.Steps, 3) // Initial think, tool action, final think
	
	// Check that a tool step was added
	hasToolStep := false
	for _, step := range plan.Steps {
		if step.Type == "tool" {
			hasToolStep = true
			assert.Equal(t, "EchoTool", step.ToolName)
			assert.Equal(t, "echo this message", step.Parameters["message"])
		}
	}
	assert.True(t, hasToolStep, "Plan should include a tool step")
}

func TestReActPlanner_Plan_MathGoal_SelectsCalculator(t *testing.T) {
	planner := NewReActPlanner()
	tools := []Tool{
		&mockTool{name: "EchoTool"},
		&mockTool{name: "CalculatorTool"},
	}
	state := State{
		Tools:   tools,
		Context: map[string]interface{}{},
	}
	
	plan, err := planner.Plan(context.Background(), state, "calculate 2 + 2")
	
	require.NoError(t, err)
	
	// Should select CalculatorTool for math goals
	hasCalculatorStep := false
	for _, step := range plan.Steps {
		if step.Type == "tool" && step.ToolName == "CalculatorTool" {
			hasCalculatorStep = true
			assert.Equal(t, "add", step.Parameters["operation"])
			assert.Equal(t, 1.0, step.Parameters["a"])
			assert.Equal(t, 1.0, step.Parameters["b"])
		}
	}
	assert.True(t, hasCalculatorStep, "Plan should include CalculatorTool for math goal")
}

func TestReActPlanner_Plan_NoTools_GeneratesThinkingOnly(t *testing.T) {
	planner := NewReActPlanner()
	state := State{
		Tools:   []Tool{},
		Context: map[string]interface{}{},
	}
	
	plan, err := planner.Plan(context.Background(), state, "think about something")
	
	require.NoError(t, err)
	assert.Len(t, plan.Steps, 2) // Initial think and final think
	
	// Should have no tool steps
	for _, step := range plan.Steps {
		assert.NotEqual(t, "tool", step.Type)
	}
}

func TestReActPlanner_generateInitialThought_IncludesAvailableTools(t *testing.T) {
	planner := NewReActPlanner()
	tools := []Tool{
		&mockTool{name: "Tool1"},
		&mockTool{name: "Tool2"},
	}
	state := State{
		Tools:   tools,
		Context: map[string]interface{}{},
	}
	
	plan, err := planner.Plan(context.Background(), state, "test goal")
	
	require.NoError(t, err)
	assert.Contains(t, plan.Steps[0].Thought, "Available tools: Tool1, Tool2")
}

func TestDefaultToolSelector_SelectTool_SelectsCalculatorForMath(t *testing.T) {
	selector := &DefaultToolSelector{}
	tools := []Tool{
		&mockTool{name: "EchoTool"},
		&mockTool{name: "CalculatorTool"},
	}
	
	tool, params, err := selector.SelectTool(context.Background(), tools, "calculate something", []string{})
	
	require.NoError(t, err)
	assert.Equal(t, "CalculatorTool", tool.Name())
	assert.Equal(t, "add", params["operation"])
}

func TestDefaultToolSelector_SelectTool_SelectsEchoForGeneral(t *testing.T) {
	selector := &DefaultToolSelector{}
	tools := []Tool{
		&mockTool{name: "EchoTool"},
		&mockTool{name: "CalculatorTool"},
	}
	
	tool, params, err := selector.SelectTool(context.Background(), tools, "tell me something", []string{})
	
	require.NoError(t, err)
	assert.Equal(t, "EchoTool", tool.Name())
	assert.Equal(t, "tell me something", params["message"])
}

func TestDefaultToolSelector_SelectTool_NoTools_ReturnsError(t *testing.T) {
	selector := &DefaultToolSelector{}
	
	_, _, err := selector.SelectTool(context.Background(), []Tool{}, "any goal", []string{})
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no tools available")
}