package agent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeam_NewTeam(t *testing.T) {
	team := NewTeam("test-team")
	
	assert.NotNil(t, team)
	assert.Equal(t, "test-team", team.name)
	assert.Empty(t, team.agents)
	assert.NotNil(t, team.messageBus)
	assert.NotNil(t, team.coordinator)
}

func TestTeam_NewTeamWithOptions(t *testing.T) {
	team := NewTeam("test-team", 
		WithTeamTimeout(5*time.Second),
		WithMaxParallel(10),
	)
	
	assert.Equal(t, 5*time.Second, team.timeout)
	assert.Equal(t, 10, team.maxParallel)
}

func TestTeam_AddAgent(t *testing.T) {
	team := NewTeam("test-team")
	
	// Create a test agent with memory
	memory := &MockMemory{}
	agent := NewAgent(WithMemory(memory))
	
	err := team.AddAgent("agent1", agent)
	require.NoError(t, err)
	
	assert.Equal(t, 1, team.Size())
	assert.Contains(t, team.ListMembers(), "agent1")
	
	// Verify agent is linked to team
	assert.Equal(t, team, agent.GetTeamContext())
}

func TestTeam_AddDuplicateAgent(t *testing.T) {
	team := NewTeam("test-team")
	
	memory := &MockMemory{}
	agent1 := NewAgent(WithMemory(memory))
	agent2 := NewAgent(WithMemory(memory))
	
	// Add first agent
	err := team.AddAgent("agent1", agent1)
	require.NoError(t, err)
	
	// Try to add duplicate
	err = team.AddAgent("agent1", agent2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	
	// Should still only have one agent
	assert.Equal(t, 1, team.Size())
}

func TestTeam_RemoveAgent(t *testing.T) {
	team := NewTeam("test-team")
	
	memory := &MockMemory{}
	agent := NewAgent(WithMemory(memory))
	
	// Add agent
	err := team.AddAgent("agent1", agent)
	require.NoError(t, err)
	assert.Equal(t, 1, team.Size())
	
	// Remove agent
	team.RemoveAgent("agent1")
	assert.Equal(t, 0, team.Size())
	
	// Verify agent team context is cleared
	assert.Nil(t, agent.GetTeamContext())
}

func TestTeam_GetMember(t *testing.T) {
	team := NewTeam("test-team")
	
	memory := &MockMemory{}
	agent := NewAgent(WithMemory(memory))
	
	// Add agent
	err := team.AddAgent("agent1", agent)
	require.NoError(t, err)
	
	// Get existing agent
	retrievedAgent, exists := team.GetMember("agent1")
	assert.True(t, exists)
	assert.Equal(t, agent, retrievedAgent)
	
	// Get non-existing agent
	_, exists = team.GetMember("nonexistent")
	assert.False(t, exists)
}

func TestTeam_ListMembers(t *testing.T) {
	team := NewTeam("test-team")
	
	memory := &MockMemory{}
	
	// Initially empty
	assert.Empty(t, team.ListMembers())
	
	// Add some agents
	agent1 := NewAgent(WithMemory(memory))
	agent2 := NewAgent(WithMemory(memory))
	
	err := team.AddAgent("agent1", agent1)
	require.NoError(t, err)
	err = team.AddAgent("agent2", agent2)
	require.NoError(t, err)
	
	members := team.ListMembers()
	assert.Len(t, members, 2)
	assert.Contains(t, members, "agent1")
	assert.Contains(t, members, "agent2")
}

func TestTeam_Broadcast(t *testing.T) {
	team := NewTeam("test-team")
	
	memory1 := &MockMemory{}
	memory2 := &MockMemory{}
	
	agent1 := NewAgent(WithMemory(memory1))
	agent2 := NewAgent(WithMemory(memory2))
	
	err := team.AddAgent("agent1", agent1)
	require.NoError(t, err)
	err = team.AddAgent("agent2", agent2)
	require.NoError(t, err)
	
	// Broadcast message
	ctx := context.Background()
	err = team.Broadcast(ctx, "agent1", "Hello team!", MessageTypeText)
	require.NoError(t, err)
	
	// Give some time for async message delivery
	time.Sleep(10 * time.Millisecond)
	
	// Verify other agent received the message
	msgs, err := memory2.Retrieve(ctx, "", 10)
	require.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Contains(t, msgs[0].Content, "[agent1] Hello team!")
}

func TestTeam_SendToAgent(t *testing.T) {
	team := NewTeam("test-team")
	
	memory1 := &MockMemory{}
	memory2 := &MockMemory{}
	
	agent1 := NewAgent(WithMemory(memory1))
	agent2 := NewAgent(WithMemory(memory2))
	
	err := team.AddAgent("agent1", agent1)
	require.NoError(t, err)
	err = team.AddAgent("agent2", agent2)
	require.NoError(t, err)
	
	// Send direct message
	ctx := context.Background()
	err = team.SendToAgent(ctx, "agent1", "agent2", "Hello agent2!", MessageTypeText)
	require.NoError(t, err)
	
	// Give some time for async message delivery
	time.Sleep(10 * time.Millisecond)
	
	// Verify target agent received the message
	msgs, err := memory2.Retrieve(ctx, "", 10)
	require.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Contains(t, msgs[0].Content, "[agent1] Hello agent2!")
	
	// Verify sender didn't receive their own message
	senderMsgs, err := memory1.Retrieve(ctx, "", 10)
	require.NoError(t, err)
	assert.Len(t, senderMsgs, 0)
}

func TestTeam_SendToNonExistentAgent(t *testing.T) {
	team := NewTeam("test-team")
	
	memory := &MockMemory{}
	agent := NewAgent(WithMemory(memory))
	
	err := team.AddAgent("agent1", agent)
	require.NoError(t, err)
	
	// Try to send to non-existent agent
	ctx := context.Background()
	err = team.SendToAgent(ctx, "agent1", "nonexistent", "Hello!", MessageTypeText)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestTeam_Shutdown(t *testing.T) {
	team := NewTeam("test-team")
	
	memory := &MockMemory{}
	agent := NewAgent(WithMemory(memory))
	
	err := team.AddAgent("agent1", agent)
	require.NoError(t, err)
	
	// Shutdown team
	err = team.Shutdown()
	require.NoError(t, err)
	
	// Verify message bus is shut down by trying to send a message
	ctx := context.Background()
	err = team.Broadcast(ctx, "agent1", "Test message", MessageTypeText)
	assert.Error(t, err)
}

func TestNewTask(t *testing.T) {
	task := NewTask("Test task", 5)
	
	assert.NotEmpty(t, task.ID)
	assert.Contains(t, task.ID, "task-")
	assert.Equal(t, "Test task", task.Description)
	assert.Equal(t, 5, task.Priority)
	assert.Equal(t, TaskStatusPending, task.Status)
	assert.NotNil(t, task.Data)
	assert.Empty(t, task.Data)
}

func TestTask_Assign(t *testing.T) {
	task := NewTask("Test task", 0)
	
	task.Assign("agent1")
	assert.Equal(t, "agent1", task.AssignedTo)
	assert.Equal(t, TaskStatusPending, task.Status)
}

func TestTask_Activate(t *testing.T) {
	task := NewTask("Test task", 0)
	
	task.Activate()
	assert.Equal(t, TaskStatusActive, task.Status)
}

func TestTask_Complete(t *testing.T) {
	task := NewTask("Test task", 0)
	
	task.Complete()
	assert.Equal(t, TaskStatusCompleted, task.Status)
}

func TestTask_Fail(t *testing.T) {
	task := NewTask("Test task", 0)
	
	task.Fail(assert.AnError)
	assert.Equal(t, TaskStatusFailed, task.Status)
	assert.Equal(t, assert.AnError.Error(), task.Data["error"])
}

func TestTask_Cancel(t *testing.T) {
	task := NewTask("Test task", 0)
	
	task.Cancel()
	assert.Equal(t, TaskStatusCancelled, task.Status)
}

func TestTask_IsCompleted(t *testing.T) {
	pendingTask := NewTask("Pending task", 0)
	assert.False(t, pendingTask.IsCompleted())
	
	completedTask := NewTask("Completed task", 0)
	completedTask.Complete()
	assert.True(t, completedTask.IsCompleted())
	
	failedTask := NewTask("Failed task", 0)
	failedTask.Fail(assert.AnError)
	assert.True(t, failedTask.IsCompleted())
	
	cancelledTask := NewTask("Cancelled task", 0)
	cancelledTask.Cancel()
	assert.True(t, cancelledTask.IsCompleted())
}

func TestTask_SetData_GetData(t *testing.T) {
	task := NewTask("Test task", 0)
	
	// Test setting data
	task.SetData("key1", "value1")
	task.SetData("key2", 42)
	
	// Test getting data
	value1, exists := task.GetData("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", value1)
	
	value2, exists := task.GetData("key2")
	assert.True(t, exists)
	assert.Equal(t, 42, value2)
	
	// Test getting non-existent key
	_, exists = task.GetData("nonexistent")
	assert.False(t, exists)
}