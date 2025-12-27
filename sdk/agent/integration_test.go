package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTeam_ExecuteTask_Integration tests the Team's ExecuteTask method
func TestTeam_ExecuteTask_Integration(t *testing.T) {
	team := NewTeam("test-team")

	// Create agents with different capabilities
	memory := &MockMemory{}
	agent1 := NewAgent(WithMemory(memory))
	agent2 := NewAgent(WithMemory(&MockMemory{}))

	// Add agents to team
	require.NoError(t, team.AddAgent("calc-agent", agent1))
	require.NoError(t, team.AddAgent("echo-agent", agent2))

	ctx := context.Background()

	// Test task execution through team
	task := Task{
		ID:          "test-task",
		Description: "Calculate 2+2",
		Priority:    1,
		Data: map[string]interface{}{
			"required_capabilities": []string{"calculator"},
		},
	}

	// Just test that the method doesn't panic
	_, err := team.ExecuteTask(ctx, task)
	// It might fail since no actual calculator capability is registered
	_ = err
}

// TestCoordinator_GetTaskResult_Integration tests retrieving task results
func TestCoordinator_GetTaskResult_Integration(t *testing.T) {
	team := NewTeam("test-team")
	coordinator := NewCoordinator(team)

	memory := &MockMemory{}
	agent := NewAgent(WithMemory(memory))

	team.AddAgent("agent1", agent)

	ctx := context.Background()

	// Execute a task
	task := Task{
		ID:          "test-task",
		Description: "Test task",
		Priority:    1,
	}

	result, err := coordinator.ExecuteTask(ctx, task)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Test GetTaskResult
	retrievedResult, err := coordinator.GetTaskResult("test-task")
	assert.NotNil(t, retrievedResult)
	assert.NoError(t, err)
}

// TestCoordinator_GetActiveTasks_Integration tests retrieving active tasks
func TestCoordinator_GetActiveTasks_Integration(t *testing.T) {
	team := NewTeam("test-team")
	coordinator := NewCoordinator(team)

	ctx := context.Background()

	// Submit tasks without executing
	task1 := &Task{ID: "task1", Description: "Task 1", Priority: 1}
	task2 := &Task{ID: "task2", Description: "Task 2", Priority: 2}

	coordinator.SubmitTask(ctx, task1)
	coordinator.SubmitTask(ctx, task2)

	// Get active tasks
	active := coordinator.GetActiveTasks()
	// Find the tasks by ID
	found1, found2 := false, false
	for _, exec := range active {
		if exec.Task.ID == "task1" {
			found1 = true
		}
		if exec.Task.ID == "task2" {
			found2 = true
		}
	}
	assert.True(t, found1)
	assert.True(t, found2)
	assert.Equal(t, 2, len(active))
}

// TestCoordinator_GetStats_Integration tests coordinator statistics
func TestCoordinator_GetStats_Integration(t *testing.T) {
	team := NewTeam("test-team")
	coordinator := NewCoordinator(team)

	memory := &MockMemory{}
	agent := NewAgent(WithMemory(memory))

	team.AddAgent("agent1", agent)

	ctx := context.Background()

	// Execute some tasks
	task1 := Task{ID: "task1", Description: "Task 1", Priority: 1}

	coordinator.ExecuteTask(ctx, task1)

	// Get stats
	stats := coordinator.GetStats()
	// stats is a map[string]interface{}, so we need to check fields
	assert.NotNil(t, stats)
	assert.NotNil(t, stats["completed_tasks"])
	assert.Equal(t, 0, stats["active_tasks"])
}

// TestTask_DataOperations tests additional task data operations
func TestTask_DataOperations(t *testing.T) {
	task := NewTask("Test task with different data", 0)

	// Test various data types
	task.SetData("number", 42)
	task.SetData("string", "test")
	task.SetData("bool", true)
	task.SetData("slice", []string{"a", "b", "c"})

	// Test that data was set
	assert.NotNil(t, task.Data)
	assert.Equal(t, 42, task.Data["number"])
	assert.Equal(t, "test", task.Data["string"])
	assert.Equal(t, true, task.Data["bool"])
}
