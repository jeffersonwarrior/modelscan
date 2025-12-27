package agent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToolRegistry_Register_AddsTool tests registering tools
func TestToolRegistry_Register_AddsTool(t *testing.T) {
	registry := NewToolRegistry()
	tool := NewMockTool("test-tool", "A test tool", nil)

	err := registry.Register(tool)
	require.NoError(t, err)

	// Verify tool is registered
	retrievedTool, err := registry.Get("test-tool")
	require.NoError(t, err)
	assert.Equal(t, "test-tool", retrievedTool.Name())
	assert.Equal(t, "A test tool", retrievedTool.Description())
}

// TestToolRegistry_Register_DuplicateName_ReturnsError tests duplicate registration
func TestToolRegistry_Register_DuplicateName_ReturnsError(t *testing.T) {
	registry := NewToolRegistry()
	tool1 := NewMockTool("duplicate", "First tool", nil)
	tool2 := NewMockTool("duplicate", "Second tool", nil)

	err := registry.Register(tool1)
	require.NoError(t, err)

	err = registry.Register(tool2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

// TestToolRegistry_Get_UnknownTool_ReturnsError tests getting unknown tool
func TestToolRegistry_Get_UnknownTool_ReturnsError(t *testing.T) {
	registry := NewToolRegistry()

	_, err := registry.Get("unknown-tool")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestToolRegistry_List_ReturnsAllTools tests listing all tools
func TestToolRegistry_List_ReturnsAllTools(t *testing.T) {
	registry := NewToolRegistry()

	tools := []*MockTool{
		NewMockTool("tool1", "First tool", nil),
		NewMockTool("tool2", "Second tool", nil),
		NewMockTool("tool3", "Third tool", nil),
	}

	for _, tool := range tools {
		err := registry.Register(tool)
		require.NoError(t, err)
	}

	allTools := registry.List()
	assert.Len(t, allTools, 3)

	// registry.List() returns []string (tool names)
	toolNames := make(map[string]bool)
	for _, toolName := range allTools {
		toolNames[toolName] = true
	}

	assert.True(t, toolNames["tool1"])
	assert.True(t, toolNames["tool2"])
	assert.True(t, toolNames["tool3"])
}

// TestToolExecutor_Execute_WithValidInput tests tool execution
func TestToolExecutor_Execute_WithValidInput(t *testing.T) {
	registry := NewToolRegistry()
	executor := NewToolExecutor(registry)

	tool := NewMockTool("echo", "Echoes input", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{
			"echo": input["message"],
		}, nil
	})

	err := registry.Register(tool)
	require.NoError(t, err)

	result, err := executor.Execute(context.Background(), "echo", map[string]interface{}{
		"message": "Hello, World!",
	})

	require.NoError(t, err)
	assert.Equal(t, "Hello, World!", result["echo"])
}

// TestToolExecutor_Execute_UnknownTool_ReturnsError tests executing unknown tool
func TestToolExecutor_Execute_UnknownTool_ReturnsError(t *testing.T) {
	registry := NewToolRegistry()
	executor := NewToolExecutor(registry)

	_, err := executor.Execute(context.Background(), "unknown-tool", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestToolExecutor_Execute_WithTimeout_RespectsTimeout tests timeout handling
func TestToolExecutor_Execute_WithTimeout_RespectsTimeout(t *testing.T) {
	registry := NewToolRegistry()
	executor := NewToolExecutor(registry)

	// Tool that waits longer than the context timeout
	tool := NewMockTool("hang", "Hanging tool", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			return map[string]interface{}{"result": "done"}, nil
		}
	})

	err := registry.Register(tool)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = executor.Execute(ctx, "hang", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "deadline exceeded")
}

// TestEchoTool_Execute_ReturnsInput tests echo tool functionality
func TestEchoTool_Execute_ReturnsInput(t *testing.T) {
	tool := &EchoTool{}

	input := map[string]interface{}{
		"message": "Test message",
		"number":  42,
		"array":   []interface{}{1, 2, 3},
	}

	result, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)

	echoed := result["echoed"].(map[string]interface{})
	assert.Equal(t, "Test message", echoed["message"])
	assert.Equal(t, 42, echoed["number"])
	assert.Equal(t, []interface{}{1, 2, 3}, echoed["array"])
}

// TestCalculatorTool_Execute_Addition tests calculator addition
func TestCalculatorTool_Execute_Addition(t *testing.T) {
	tool := &CalculatorTool{}

	input := map[string]interface{}{
		"a":         15.0,
		"b":         27.0,
		"operation": "+",
	}

	result, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)

	assert.Equal(t, 42.0, result["result"])
}

// TestCalculatorTool_Execute_Subtraction tests calculator subtraction
func TestCalculatorTool_Execute_Subtraction(t *testing.T) {
	tool := &CalculatorTool{}

	input := map[string]interface{}{
		"a":         100.0,
		"b":         57.0,
		"operation": "-",
	}

	result, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)

	assert.Equal(t, 43.0, result["result"])
}

// TestCalculatorTool_Execute_Multiplication tests calculator multiplication
func TestCalculatorTool_Execute_Multiplication(t *testing.T) {
	tool := &CalculatorTool{}

	input := map[string]interface{}{
		"a":         6.0,
		"b":         7.0,
		"operation": "*",
	}

	result, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)

	assert.Equal(t, 42.0, result["result"])
}

// TestCalculatorTool_Execute_Division tests calculator division
func TestCalculatorTool_Execute_Division(t *testing.T) {
	tool := &CalculatorTool{}

	input := map[string]interface{}{
		"a":         84.0,
		"b":         2.0,
		"operation": "/",
	}

	result, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)

	assert.Equal(t, 42.0, result["result"])
}

// TestCalculatorTool_Execute_DivisionByZero_ReturnsError tests division by zero
func TestCalculatorTool_Execute_DivisionByZero_ReturnsError(t *testing.T) {
	tool := &CalculatorTool{}

	input := map[string]interface{}{
		"a":         42.0,
		"b":         0.0,
		"operation": "/",
	}

	_, err := tool.Execute(context.Background(), input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "division by zero")
}

// TestCalculatorTool_Execute_InvalidOperation_ReturnsError tests invalid operation
func TestCalculatorTool_Execute_InvalidOperation_ReturnsError(t *testing.T) {
	tool := &CalculatorTool{}

	input := map[string]interface{}{
		"a":         42.0,
		"b":         1.0,
		"operation": "invalid",
	}

	_, err := tool.Execute(context.Background(), input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported operation")
}

// TestCalculatorTool_Execute_MissingParameters_ReturnsError tests missing parameters
func TestCalculatorTool_Execute_MissingParameters_ReturnsError(t *testing.T) {
	tool := &CalculatorTool{}

	// Missing 'operation' parameter
	input := map[string]interface{}{
		"a": 1.0,
		"b": 1.0,
	}

	_, err := tool.Execute(context.Background(), input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing 'operation' parameter")
}

// TestToolUtility_RegisterCommonTools_AddsCommonTools tests default tool registration
func TestToolUtility_RegisterCommonTools_AddsCommonTools(t *testing.T) {
	registry := NewToolRegistry()

	err := RegisterCommonTools(registry)
	require.NoError(t, err)

	// Verify default tools are registered
	tools := registry.List()
	assert.Len(t, tools, 2)

	// registry.List() returns []string (tool names)
	toolNames := make(map[string]bool)
	for _, toolName := range tools {
		toolNames[toolName] = true
	}

	assert.True(t, toolNames["echo"])
	assert.True(t, toolNames["calculator"])
}

// TestToolRegistry_ConcurrentAccess tests concurrent registry access
func TestToolRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewToolRegistry()
	done := make(chan bool, 10)

	// Register tools concurrently
	for i := 0; i < 5; i++ {
		go func(index int) {
			tool := NewMockTool("tool-"+string(rune('A'+index)), "Concurrent tool", nil)
			err := registry.Register(tool)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Get tools concurrently
	for i := 0; i < 5; i++ {
		go func(index int) {
			_, _ = registry.Get("tool-" + string(rune('A'+index)))
			// May not exist yet, that's OK
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all tools are registered
	assert.Len(t, registry.List(), 5)
}
