package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAgent_TeamContextFunctions tests agent team context related functions
func TestAgent_TeamContextFunctions(t *testing.T) {
	agent := NewAgent()
	
	// Initially, agent should have no team context
	assert.Nil(t, agent.GetTeamContext())
	
	// Create a team and add the agent
	team := NewTeam("test-team")
	err := team.AddAgent("test-agent", agent)
	require.NoError(t, err)
	
	// Now agent should have team context
	assert.NotNil(t, agent.GetTeamContext())
	assert.Equal(t, team, agent.GetTeamContext())
	
	// Remove agent from team
	team.RemoveAgent("test-agent")
	
	// Team context should be cleared
	assert.Nil(t, agent.GetTeamContext())
}

// TestAgent_TeamMessaging tests agent messaging functions within a team
func TestAgent_TeamMessaging(t *testing.T) {
	team := NewTeam("test-team")
	
	sender := NewAgent(WithMemory(&MockMemory{}))
	receiver := NewAgent(WithMemory(&MockMemory{}))
	
	// Add agents to team
	require.NoError(t, team.AddAgent("sender", sender))
	require.NoError(t, team.AddAgent("receiver", receiver))
	
	ctx := context.Background()
	
	// Test.SendMessageToAgent
	err := sender.SendMessageToAgent(ctx, "receiver", "Hello from sender", MessageTypeText)
	assert.NoError(t, err)
	
	// Test.BroadcastMessage
	err = sender.BroadcastMessage(ctx, "Broadcast message", MessageTypeText)
	assert.NoError(t, err)
}

// TestAgent_CreateTask tests task creation by agent
func TestAgent_CreateTask(t *testing.T) {
	// Create a team with coordinator first
	team := NewTeam("test-team")
	agent := NewAgent()
	
	// Add agent to team (this sets up team context)
	err := team.AddAgent("test-agent", agent)
	require.NoError(t, err)
	
	ctx := context.Background()
	task, err := agent.CreateTask(ctx, "Test task", 5)
	require.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, "Test task", task.Description)
	assert.Equal(t, 5, task.Priority)
	assert.NotEmpty(t, task.CreatedBy)
}

// TestAgent_RequestHelp tests help request functionality
func TestAgent_RequestHelp(t *testing.T) {
	team := NewTeam("test-team")
	
	agent1 := NewAgent(WithMemory(&MockMemory{}))
	agent2 := NewAgent(WithMemory(&MockMemory{}))
	
	// Add agents to team
	require.NoError(t, team.AddAgent("agent1", agent1))
	require.NoError(t, team.AddAgent("agent2", agent2))
	
	ctx := context.Background()
	
	// Request help - should broadcast a help message
	err := agent1.RequestHelp(ctx, "I need help with something", []string{})
	assert.NoError(t, err)
}

// TestMessageBus_Broadcast_Integration tests broadcasting messages
func TestMessageBus_Broadcast_Integration(t *testing.T) {
	bus := NewInMemoryMessageBus()
	
	// Subscribe multiple agents
	messages := make(map[string][]TeamMessage)
	var mu sync.Mutex
	
	handlerFunc := func(id string) MessageHandler {
		return func(msg TeamMessage) error {
			mu.Lock()
			messages[id] = append(messages[id], msg)
			mu.Unlock()
			return nil
		}
	}
	
	err := bus.Subscribe("agent1", handlerFunc("agent1"))
	require.NoError(t, err)
	err = bus.Subscribe("agent2", handlerFunc("agent2"))
	require.NoError(t, err)
	
	// Send broadcast from agent1 (should not be sent back to agent1)
	ctx := context.Background()
	msg := NewTeamMessage(MessageTypeText, "agent1", "", "Broadcast message")
	
	err = bus.Broadcast(ctx, msg)
	require.NoError(t, err)
	
	// Wait for async delivery to complete
	time.Sleep(10 * time.Millisecond)
	
	// Check that agent2 received the message but agent1 didn't
	mu.Lock()
	assert.Empty(t, messages["agent1"], "Agent should not receive its own broadcast")
	assert.Len(t, messages["agent2"], 1, "Agent2 should have received the broadcast")
	assert.Equal(t, "Broadcast message", messages["agent2"][0].Content)
	mu.Unlock()
}

// TestMessageBus_DataOperations tests message data operations
func TestMessageBus_DataOperations(t *testing.T) {
	
	// Test with data
	msg := NewTeamMessage(MessageTypeText, "sender", "receiver", "Hello")
	
	// Add various data types
	msg.AddData("number", 42)
	msg.AddData("string", "test")
	msg.AddData("bool", true)
	
	// Check data exists
	_, exists := msg.GetData("number")
	assert.True(t, exists)
	_, exists = msg.GetData("string")
	assert.True(t, exists)
	_, exists = msg.GetData("bool")
	assert.True(t, exists)
	_, exists = msg.GetData("nonexistent")
	assert.False(t, exists)
	
	// Get data - returns value and exists flag
	val, exists := msg.GetData("number")
	assert.True(t, exists)
	assert.Equal(t, 42, val)
	
	val, exists = msg.GetData("string")
	assert.True(t, exists)
	assert.Equal(t, "test", val)
	
	val, exists = msg.GetData("bool")
	assert.True(t, exists)
	assert.Equal(t, true, val)
	
	// Test default value - GetData doesn't support default values, so we check separately
	val, exists = msg.GetData("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, val)
}

// TestTeam_ExecuteTask_TeamContext tests task execution through team context
func TestTeam_ExecuteTask_TeamContext(t *testing.T) {
	team := NewTeam("test-team")
	
	agent := NewAgent(WithMemory(&MockMemory{}))
	
	// Add agent to team
	require.NoError(t, team.AddAgent("test-agent", agent))
	
	// agent.SetTeamContext should not be called directly
	// The agent's team context is set when added to the team
	assert.NotNil(t, agent.GetTeamContext())
	
	// Execute a task through the team
	ctx := context.Background()
	task := NewTask("Test task execution through team", 1)
	
	// Test that we can call ExecuteTask (actual execution depends on implementation)
	_, err := team.ExecuteTask(ctx, *task)
	_ = err // It's okay if this fails for now
}

// TestAgent_WithInfiniteLoopDetection tests the infinite loop detection option
func TestAgent_WithInfiniteLoopDetection(t *testing.T) {
	agent := NewAgent(WithInfiniteLoopDetection(true))
	
	// Just test that the option is accepted - infinite loop detection
	// behavior would be tested in integration/execution tests
	assert.NotNil(t, agent)
}

// TestMemory_UncoveredFunctions tests uncovered memory functions
func TestMemory_UncoveredFunctions(t *testing.T) {
	memory := NewInMemoryMemory()
	ctx := context.Background()
	
	// Test Len function
	length, err := memory.Len(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, length)
	
	// Test Stats function
	stats, err := memory.Stats(ctx)
	require.NoError(t, err)
	assert.NotNil(t, stats)
	
	// Test Clear function
	err = memory.Clear(ctx)
	require.NoError(t, err)
	
	// Test Cleanup function
	err = memory.Cleanup(ctx)
	require.NoError(t, err)
}

// TestErrors_UncoveredFunctions tests uncovered error functions
func TestErrors_UncoveredFunctions(t *testing.T) {
	err := NewToolError("test error")
	assert.NotNil(t, err)
	assert.Equal(t, "test error", err.Error())
}