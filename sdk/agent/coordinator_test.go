package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoordinator_NewCoordinator(t *testing.T) {
	team := NewTeam("test-team")
	coordinator := NewCoordinator(team)
	
	assert.NotNil(t, coordinator)
	assert.Equal(t, team, coordinator.team)
	assert.Equal(t, DistributionStrategyRoundRobin, coordinator.strategy)
	assert.NotNil(t, coordinator.capability)
}

func TestCoordinator_NewCoordinatorWithStrategy(t *testing.T) {
	team := NewTeam("test-team")
	coordinator := NewCoordinatorWithStrategy(team, DistributionStrategyCapabilityBased)
	
	assert.Equal(t, DistributionStrategyCapabilityBased, coordinator.strategy)
}

func TestCoordinator_SubmitTask(t *testing.T) {
	team := NewTeam("test-team")
	coordinator := NewCoordinator(team)
	
	// Create a task
	task := NewTask("Test task", 5)
	task.CreatedBy = "test"
	
	// Submit task
	ctx := context.Background()
	err := coordinator.SubmitTask(ctx, task)
	require.NoError(t, err)
	
	// Verify task is stored
	storedTask, exists := coordinator.GetTask(task.ID)
	assert.True(t, exists)
	assert.Equal(t, task.ID, storedTask.ID)
	assert.Equal(t, task.Description, storedTask.Description)
}

func TestCoordinator_ExecuteTask_RoundRobin(t *testing.T) {
	team := NewTeam("test-team")
	coordinator := NewCoordinatorWithStrategy(team, DistributionStrategyRoundRobin)
	
	// Add agents to team
	memory1 := &MockMemory{}
	memory2 := &MockMemory{}
	
	tool1 := NewMockTool("calculator", "Math calculations", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"result": "42"}, nil
	})
	tool2 := NewMockTool("echo", "Echo tool", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"result": "echoed"}, nil
	})
	
	agent1 := NewAgent(WithMemory(memory1), WithTools(tool1))
	agent2 := NewAgent(WithMemory(memory2), WithTools(tool2))
	
	err := team.AddAgent("agent1", agent1)
	require.NoError(t, err)
	err = team.AddAgent("agent2", agent2)
	require.NoError(t, err)
	
	// Execute multiple tasks and verify round-robin distribution
	ctx := context.Background()
	
	// First task should go to agent1
	task1 := NewTask("Task 1", 1)
	task1.CreatedBy = "test"
	result1, err := coordinator.ExecuteTask(ctx, *task1)
	require.NoError(t, err)
	assert.Equal(t, "agent1", result1.AgentID)
	
	// Second task should go to agent2
	task2 := NewTask("Task 2", 1)
	task2.CreatedBy = "test"
	result2, err := coordinator.ExecuteTask(ctx, *task2)
	require.NoError(t, err)
	assert.Equal(t, "agent2", result2.AgentID)
	
	// Third task should go back to agent1
	task3 := NewTask("Task 3", 1)
	task3.CreatedBy = "test"
	result3, err := coordinator.ExecuteTask(ctx, *task3)
	require.NoError(t, err)
	assert.Equal(t, "agent1", result3.AgentID)
}

func TestCoordinator_ExecuteTask_CapabilityBased(t *testing.T) {
	team := NewTeam("test-team")
	coordinator := NewCoordinatorWithStrategy(team, DistributionStrategyCapabilityBased)
	
	// Add agents with different capabilities
	memory1 := &MockMemory{}
	memory2 := &MockMemory{}
	
	calculatorTool := NewMockTool("calculator", "Math calculations", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"result": "42"}, nil
	})
	echoTool := NewMockTool("echo", "Echo tool", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"result": "echoed"}, nil
	})
	
	agent1 := NewAgent(WithMemory(memory1), WithTools(calculatorTool))
	agent2 := NewAgent(WithMemory(memory2), WithTools(echoTool))
	
	err := team.AddAgent("calculator_agent", agent1)
	require.NoError(t, err)
	err = team.AddAgent("echo_agent", agent2)
	require.NoError(t, err)
	
	// Create capability matcher
	matcher := NewCapabilityMatcher()
	
	// Register agent capabilities
	matcher.RegisterCapability("calculator_agent", []string{"calculator", "math"})
	matcher.RegisterCapability("echo_agent", []string{"echo", "text"})
	
	coordinator.SetCapabilityMatcher(matcher)
	
	ctx := context.Background()
	
	// Math task should go to calculator agent
	mathTask := NewTask("Calculate 5 * 7", 5)
	mathTask.CreatedBy = "test"
	mathTask.SetData("required_capabilities", []string{"calculator"})
	
	result, err := coordinator.ExecuteTask(ctx, *mathTask)
	require.NoError(t, err)
	assert.Equal(t, "calculator_agent", result.AgentID)
	
	// Echo task should go to echo agent
	echoTask := NewTask("Echo hello world", 3)
	echoTask.CreatedBy = "test"
	echoTask.SetData("required_capabilities", []string{"echo"})
	
	result2, err := coordinator.ExecuteTask(ctx, *echoTask)
	require.NoError(t, err)
	assert.Equal(t, "echo_agent", result2.AgentID)
}

func TestCoordinator_ExecuteTask_LoadBalance(t *testing.T) {
	team := NewTeam("test-team")
	coordinator := NewCoordinatorWithStrategy(team, DistributionStrategyLoadBalance)
	
	// Add agents to team
	memory1 := &MockMemory{}
	memory2 := &MockMemory{}
	
	tool1 := NewMockTool("tool1", "Tool 1", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"result": "1"}, nil
	})
	tool2 := NewMockTool("tool2", "Tool 2", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"result": "2"}, nil
	})
	
	agent1 := NewAgent(WithMemory(memory1), WithTools(tool1))
	agent2 := NewAgent(WithMemory(memory2), WithTools(tool2))
	
	err := team.AddAgent("agent1", agent1)
	require.NoError(t, err)
	err = team.AddAgent("agent2", agent2)
	require.NoError(t, err)
	
	ctx := context.Background()
	
	// First task should go to either agent (equal load)
	task1 := NewTask("Task 1", 1)
	task1.CreatedBy = "test"
	result1, err := coordinator.ExecuteTask(ctx, *task1)
	require.NoError(t, err)
	
	// Second task should go to the other agent
	task2 := NewTask("Task 2", 1)
	task2.CreatedBy = "test"
	result2, err := coordinator.ExecuteTask(ctx, *task2)
	require.NoError(t, err)
	
	// Results should be from different agents
	assert.NotEqual(t, result1.AgentID, result2.AgentID)
}

func TestCoordinator_ExecuteTask_Priority(t *testing.T) {
	team := NewTeam("test-team")
	coordinator := NewCoordinatorWithStrategy(team, DistributionStrategyPriority)
	
	// Add agents to team
	memory1 := &MockMemory{}
	memory2 := &MockMemory{}
	
	tool1 := NewMockTool("tool1", "Tool 1", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"result": "1"}, nil
	})
	tool2 := NewMockTool("tool2", "Tool 2", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"result": "2"}, nil
	})
	
	agent1 := NewAgent(WithMemory(memory1), WithTools(tool1))
	agent2 := NewAgent(WithMemory(memory2), WithTools(tool2))
	
	err := team.AddAgent("agent1", agent1)
	require.NoError(t, err)
	err = team.AddAgent("agent2", agent2)
	require.NoError(t, err)
	
	ctx := context.Background()
	
	// Submit high priority task first
	highPriorityTask := NewTask("High priority task", 10)
	highPriorityTask.CreatedBy = "test"
	
	// Submit low priority task second
	lowPriorityTask := NewTask("Low priority task", 1)
	lowPriorityTask.CreatedBy = "test"
	
	// Submit both tasks
	err = coordinator.SubmitTask(ctx, highPriorityTask)
	require.NoError(t, err)
	err = coordinator.SubmitTask(ctx, lowPriorityTask)
	require.NoError(t, err)
	
	// Execute both - high priority should be executed first
	result1, err := coordinator.ExecuteTask(ctx, *highPriorityTask)
	require.NoError(t, err)
	
	result2, err := coordinator.ExecuteTask(ctx, *lowPriorityTask)
	require.NoError(t, err)
	
	// Verify tasks were executed
	assert.NotNil(t, result1)
	assert.NotNil(t, result2)
}

func TestCoordinator_GetTask(t *testing.T) {
	team := NewTeam("test-team")
	coordinator := NewCoordinator(team)
	
	// Create and submit a task
	task := NewTask("Test task", 5)
	task.CreatedBy = "test"
	
	ctx := context.Background()
	err := coordinator.SubmitTask(ctx, task)
	require.NoError(t, err)
	
	// Get the task
	storedTask, exists := coordinator.GetTask(task.ID)
	assert.True(t, exists)
	assert.Equal(t, task.Description, storedTask.Description)
	assert.Equal(t, task.Priority, storedTask.Priority)
	
	// Try to get non-existent task
	_, exists = coordinator.GetTask("non-existent")
	assert.False(t, exists)
}

func TestCoordinator_ListTasks(t *testing.T) {
	team := NewTeam("test-team")
	coordinator := NewCoordinator(team)
	
	ctx := context.Background()
	
	// Initially empty
	tasks := coordinator.ListTasks()
	assert.Empty(t, tasks)
	
	// Add some tasks
	task1 := NewTask("Task 1", 1)
	task1.CreatedBy = "test"
	err := coordinator.SubmitTask(ctx, task1)
	require.NoError(t, err)
	
	task2 := NewTask("Task 2", 5)
	task2.CreatedBy = "test"
	err = coordinator.SubmitTask(ctx, task2)
	require.NoError(t, err)
	
	// List tasks
	tasks = coordinator.ListTasks()
	assert.Len(t, tasks, 2)
	
	taskIDs := make(map[string]bool)
	for _, task := range tasks {
		taskIDs[task.ID] = true
	}
	assert.True(t, taskIDs[task1.ID])
	assert.True(t, taskIDs[task2.ID])
}

func TestCoordinator_CancelTask(t *testing.T) {
	team := NewTeam("test-team")
	coordinator := NewCoordinator(team)
	
	// Create and submit a task
	task := NewTask("Test task", 5)
	task.CreatedBy = "test"
	
	ctx := context.Background()
	err := coordinator.SubmitTask(ctx, task)
	require.NoError(t, err)
	
	// Cancel the task
	err = coordinator.CancelTask(task.ID)
	require.NoError(t, err)
	
	// Verify task is cancelled
	storedTask, exists := coordinator.GetTask(task.ID)
	assert.True(t, exists)
	assert.Equal(t, TaskStatusCancelled, storedTask.Status)
}

func TestCoordinator_CancelNonExistentTask(t *testing.T) {
	team := NewTeam("test-team")
	coordinator := NewCoordinator(team)
	
	// Try to cancel non-existent task
	err := coordinator.CancelTask("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCapabilityMatcher_NewCapabilityMatcher(t *testing.T) {
	matcher := NewCapabilityMatcher()
	
	assert.NotNil(t, matcher)
	assert.Empty(t, matcher.agentCapabilities)
}

func TestCapabilityMatcher_RegisterCapability(t *testing.T) {
	matcher := NewCapabilityMatcher()
	
	// Register capabilities for an agent
	capabilities := []string{"calculator", "math", "analysis"}
	matcher.RegisterCapability("agent1", capabilities)
	
	// Verify capabilities are stored
	assert.Equal(t, capabilities, matcher.GetAgentCapabilities("agent1"))
}

func TestCapabilityMatcher_FindBestAgent_Simple(t *testing.T) {
	matcher := NewCapabilityMatcher()
	
	// Register capabilities for agents
	matcher.RegisterCapability("agent1", []string{"calculator", "math"})
	matcher.RegisterCapability("agent2", []string{"echo", "text"})
	
	// Find agent for calculator task
	required := []string{"calculator"}
	agentID := matcher.FindBestAgent(required, []string{"agent1", "agent2"})
	assert.Equal(t, "agent1", agentID)
	
	// Find agent for echo task
	required = []string{"echo"}
	agentID = matcher.FindBestAgent(required, []string{"agent1", "agent2"})
	assert.Equal(t, "agent2", agentID)
}

func TestCapabilityMatcher_FindBestAgent_MultipleCapabilities(t *testing.T) {
	matcher := NewCapabilityMatcher()
	
	// Register capabilities for agents
	matcher.RegisterCapability("agent1", []string{"calculator", "math"})
	matcher.RegisterCapability("agent2", []string{"calculator", "echo", "text"})
	matcher.RegisterCapability("agent3", []string{"analysis"})
	
	// Find agent for calculator + echo task
	required := []string{"calculator", "echo"}
	agentID := matcher.FindBestAgent(required, []string{"agent1", "agent2", "agent3"})
	assert.Equal(t, "agent2", agentID) // agent2 has both capabilities
	
	// No agent has all required capabilities
	required = []string{"calculator", "nonexistent"}
	agentID = matcher.FindBestAgent(required, []string{"agent1", "agent2", "agent3"})
	assert.Empty(t, agentID) // Should return empty string
}

func TestCapabilityMatcher_FindBestAgent_PartialMatch(t *testing.T) {
	matcher := NewCapabilityMatcher()
	
	// Register capabilities for agents
	matcher.RegisterCapability("agent1", []string{"calculator"})
	matcher.RegisterCapability("agent2", []string{"echo"})
	
	// Find agent for calculator task (perfect match)
	required := []string{"calculator"}
	agentID := matcher.FindBestAgent(required, []string{"agent1", "agent2"})
	assert.Equal(t, "agent1", agentID)
	
	// Find agent for math task (partial match with calculator)
	required = []string{"math"}
	agentID = matcher.FindBestAgent(required, []string{"agent1", "agent2"})
	assert.Equal(t, "agent1", agentID) // agent1 has calculator which is related
}

func TestCapabilityMatcher_GetAgentCapabilities(t *testing.T) {
	matcher := NewCapabilityMatcher()
	
	// Non-existent agent should return empty
	capabilities := matcher.GetAgentCapabilities("nonexistent")
	assert.Empty(t, capabilities)
	
	// Register capabilities and verify retrieval
	expected := []string{"calculator", "math"}
	matcher.RegisterCapability("agent1", expected)
	
	actual := matcher.GetAgentCapabilities("agent1")
	assert.Equal(t, expected, actual)
}

func TestCoordinator_SetStrategy(t *testing.T) {
	team := NewTeam("test-team")
	coordinator := NewCoordinator(team)
	
	// Default should be round-robin
	assert.Equal(t, DistributionStrategyRoundRobin, coordinator.strategy)
	
	// Change strategy
	coordinator.SetStrategy(DistributionStrategyCapabilityBased)
	assert.Equal(t, DistributionStrategyCapabilityBased, coordinator.strategy)
	
	coordinator.SetStrategy(DistributionStrategyLoadBalance)
	assert.Equal(t, DistributionStrategyLoadBalance, coordinator.strategy)
	
	coordinator.SetStrategy(DistributionStrategyPriority)
	assert.Equal(t, DistributionStrategyPriority, coordinator.strategy)
}

func TestCoordinator_SetCapabilityMatcher(t *testing.T) {
	team := NewTeam("test-team")
	coordinator := NewCoordinator(team)
	
	// Create custom matcher
	matcher := NewCapabilityMatcher()
	matcher.RegisterCapability("special_agent", []string{"special"})
	
	// Set custom matcher
	coordinator.SetCapabilityMatcher(matcher)
	assert.Equal(t, matcher, coordinator.capability)
}