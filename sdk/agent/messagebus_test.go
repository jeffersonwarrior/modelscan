package agent

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryMessageBus_NewInMemoryMessageBus(t *testing.T) {
	bus := NewInMemoryMessageBus()

	assert.NotNil(t, bus)
	assert.Empty(t, bus.GetStats().Subscribers)
	assert.Equal(t, 0, bus.GetStats().MessagesSent)
	assert.Equal(t, 0, bus.GetStats().MessagesDelivered)
	assert.Equal(t, 0, bus.GetStats().MessagesFailed)
}

func TestInMemoryMessageBus_Subscribe_Unsubscribe(t *testing.T) {
	bus := NewInMemoryMessageBus()

	// Subscribe with valid parameters
	handler := func(msg TeamMessage) error { return nil }
	err := bus.Subscribe("agent1", handler)
	require.NoError(t, err)

	stats := bus.GetStats()
	assert.Equal(t, 1, stats.Subscribers)

	// Try to subscribe again with same ID
	err = bus.Subscribe("agent1", handler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Unsubscribe
	err = bus.Unsubscribe("agent1")
	require.NoError(t, err)

	stats = bus.GetStats()
	assert.Equal(t, 0, stats.Subscribers)

	// Try to unsubscribe non-existent subscriber
	err = bus.Unsubscribe("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInMemoryMessageBus_SubscribeValidation(t *testing.T) {
	bus := NewInMemoryMessageBus()
	handler := func(msg TeamMessage) error { return nil }

	// Empty ID should fail
	err := bus.Subscribe("", handler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "subscriber ID cannot be empty")

	// Nil handler should fail
	err = bus.Subscribe("agent1", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "handler cannot be nil")
}

func TestInMemoryMessageBus_Send_Success(t *testing.T) {
	bus := NewInMemoryMessageBus()

	// Setup receiver with proper synchronization
	var receivedMsg TeamMessage
	var mu sync.Mutex
	done := make(chan struct{})
	var receivedOnce sync.Once

	handler := func(msg TeamMessage) error {
		receivedOnce.Do(func() {
			mu.Lock()
			receivedMsg = msg
			mu.Unlock()
			close(done)
		})
		return nil
	}

	err := bus.Subscribe("receiver", handler)
	require.NoError(t, err)

	// Send message
	ctx := context.Background()
	msg := NewTeamMessage(MessageTypeText, "sender", "receiver", "Hello!")

	err = bus.Send(ctx, msg)
	require.NoError(t, err)

	// Wait for async delivery with timeout
	select {
	case <-done:
		// Message received
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for message delivery")
	}

	// Verify message was received (protected by mutex)
	mu.Lock()
	assert.Equal(t, msg.ID, receivedMsg.ID)
	assert.Equal(t, msg.From, receivedMsg.From)
	assert.Equal(t, msg.To, receivedMsg.To)
	assert.Equal(t, msg.Content, receivedMsg.Content)
	assert.Equal(t, msg.Type, receivedMsg.Type)
	mu.Unlock()

	// Verify stats
	stats := bus.GetStats()
	assert.Equal(t, 1, stats.MessagesSent)
	assert.Equal(t, 1, stats.MessagesDelivered)
	assert.Equal(t, 0, stats.MessagesFailed)
}

func TestInMemoryMessageBus_SendRecipientNotFound(t *testing.T) {
	bus := NewInMemoryMessageBus()

	// Send message to non-existent recipient
	ctx := context.Background()
	msg := NewTeamMessage(MessageTypeText, "sender", "nonexistent", "Hello!")

	err := bus.Send(ctx, msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Verify stats
	stats := bus.GetStats()
	assert.Equal(t, 1, stats.MessagesSent)
	assert.Equal(t, 0, stats.MessagesDelivered)
	assert.Equal(t, 1, stats.MessagesFailed)
}

func TestInMemoryMessageBus_SendValidation(t *testing.T) {
	bus := NewInMemoryMessageBus()
	ctx := context.Background()

	// Empty sender should fail
	msg := NewTeamMessage(MessageTypeText, "", "receiver", "Hello!")
	err := bus.Send(ctx, msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sender cannot be empty")

	// Empty recipient for direct message should fail
	msg = NewTeamMessage(MessageTypeText, "sender", "", "Hello!")
	err = bus.Send(ctx, msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "recipient cannot be empty")
}

func TestInMemoryMessageBus_Broadcast_Success(t *testing.T) {
	bus := NewInMemoryMessageBus()

	// Setup multiple receivers
	var receivedMsgs []TeamMessage
	var mu sync.Mutex

	handler := func(msg TeamMessage) error {
		mu.Lock()
		defer mu.Unlock()
		receivedMsgs = append(receivedMsgs, msg)
		return nil
	}

	// Subscribe three agents (including the sender)
	err := bus.Subscribe("sender", handler)
	require.NoError(t, err)
	err = bus.Subscribe("receiver1", handler)
	require.NoError(t, err)
	err = bus.Subscribe("receiver2", handler)
	require.NoError(t, err)

	// Broadcast message
	ctx := context.Background()
	msg := NewTeamMessage(MessageTypeText, "sender", "", "Broadcast message")

	err = bus.Broadcast(ctx, msg)
	require.NoError(t, err)

	// Wait for async delivery
	time.Sleep(20 * time.Millisecond)

	// Verify message was received by non-senders only
	mu.Lock()
	assert.Len(t, receivedMsgs, 2) // Should not be received by sender
	mu.Unlock()

	// Verify stats
	stats := bus.GetStats()
	assert.Equal(t, 2, stats.MessagesSent) // Sent to 2 non-senders
	assert.Equal(t, 2, stats.MessagesDelivered)
	assert.Equal(t, 0, stats.MessagesFailed)
}

func TestInMemoryMessageBus_Broadcast_NoRecipients(t *testing.T) {
	bus := NewInMemoryMessageBus()

	// Broadcast with no subscribers
	ctx := context.Background()
	msg := NewTeamMessage(MessageTypeText, "sender", "", "Broadcast message")

	err := bus.Broadcast(ctx, msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no recipients available")
}

func TestInMemoryMessageBus_Broadcast_OnlySender(t *testing.T) {
	bus := NewInMemoryMessageBus()

	// Subscribe only the sender
	handler := func(msg TeamMessage) error { return nil }
	err := bus.Subscribe("sender", handler)
	require.NoError(t, err)

	// Broadcast should fail since only sender is subscribed
	ctx := context.Background()
	msg := NewTeamMessage(MessageTypeText, "sender", "", "Broadcast message")

	err = bus.Broadcast(ctx, msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no recipients available")
}

func TestInMemoryMessageBus_ContextCancellation(t *testing.T) {
	bus := NewInMemoryMessageBus()

	// Setup slow handler that checks context
	blockingHandler := func(msg TeamMessage) error {
		// Just simulate a slow operation
		time.Sleep(50 * time.Millisecond)
		return nil
	}

	err := bus.Subscribe("receiver", blockingHandler)
	require.NoError(t, err)

	// Send with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	msg := NewTeamMessage(MessageTypeText, "sender", "receiver", "Hello!")
	err = bus.Send(ctx, msg)
	require.NoError(t, err) // Send itself won't fail, but delivery will

	// Wait for async delivery
	time.Sleep(50 * time.Millisecond)

	// Verify stats show failure
	stats := bus.GetStats()
	assert.Equal(t, 1, stats.MessagesSent)
	assert.Equal(t, 0, stats.MessagesDelivered)
	assert.Equal(t, 1, stats.MessagesFailed)
}

func TestInMemoryMessageBus_HandlerError(t *testing.T) {
	bus := NewInMemoryMessageBus()

	// Setup handler that returns an error
	errorHandler := func(msg TeamMessage) error {
		return assert.AnError
	}

	err := bus.Subscribe("receiver", errorHandler)
	require.NoError(t, err)

	// Send message
	ctx := context.Background()
	msg := NewTeamMessage(MessageTypeText, "sender", "receiver", "Hello!")

	err = bus.Send(ctx, msg)
	require.NoError(t, err)

	// Wait for async delivery
	time.Sleep(10 * time.Millisecond)

	// Verify stats show failure
	stats := bus.GetStats()
	assert.Equal(t, 1, stats.MessagesSent)
	assert.Equal(t, 0, stats.MessagesDelivered)
	assert.Equal(t, 1, stats.MessagesFailed)
}

func TestInMemoryMessageBus_Shutdown(t *testing.T) {
	bus := NewInMemoryMessageBus()

	// Subscribe some handlers
	handler := func(msg TeamMessage) error { return nil }

	err := bus.Subscribe("agent1", handler)
	require.NoError(t, err)
	err = bus.Subscribe("agent2", handler)
	require.NoError(t, err)

	// Verify subscribers
	stats := bus.GetStats()
	assert.Equal(t, 2, stats.Subscribers)

	// Shutdown
	bus.Shutdown()

	// Verify subscribers are cleared
	stats = bus.GetStats()
	assert.Equal(t, 0, stats.Subscribers)

	// Verify subsequent operations fail
	ctx := context.Background()
	msg := NewTeamMessage(MessageTypeText, "sender", "receiver", "Hello!")

	err = bus.Send(ctx, msg)
	assert.Error(t, err)

	err = bus.Broadcast(ctx, msg)
	assert.Error(t, err)
}

func TestInMemoryMessageBus_ConcurrentOperations(t *testing.T) {
	bus := NewInMemoryMessageBus()

	// Setup multiple concurrent subscribers
	numSubscribers := 10
	numMessages := 5

	var wg sync.WaitGroup

	// Subscribe concurrently
	for i := 0; i < numSubscribers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			handler := func(msg TeamMessage) error { return nil }
			agentID := fmt.Sprintf("agent%d", id)
			bus.Subscribe(agentID, handler)
		}(i)
	}
	wg.Wait()

	// Verify all subscribers are registered
	stats := bus.GetStats()
	assert.Equal(t, numSubscribers, stats.Subscribers)

	// Send concurrent messages
	ctx := context.Background()
	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			from := fmt.Sprintf("sender%d", i)
			to := fmt.Sprintf("agent%d", i%numSubscribers)
			msg := NewTeamMessage(MessageTypeText, from, to, fmt.Sprintf("Message %d", i))
			bus.Send(ctx, msg)
		}(i)
	}
	wg.Wait()

	// Wait for async delivery
	time.Sleep(50 * time.Millisecond)

	// Verify messages were processed
	stats = bus.GetStats()
	assert.Equal(t, numMessages, stats.MessagesSent)
}

func TestInMemoryMessageBus_GetStats(t *testing.T) {
	bus := NewInMemoryMessageBus()

	// Initial stats
	stats := bus.GetStats()
	assert.Equal(t, 0, stats.Subscribers)
	assert.Equal(t, 0, stats.MessagesSent)
	assert.Equal(t, 0, stats.MessagesDelivered)
	assert.Equal(t, 0, stats.MessagesFailed)
	assert.True(t, stats.Uptime >= 0)

	// Add subscriber
	handler := func(msg TeamMessage) error { return nil }
	err := bus.Subscribe("agent1", handler)
	require.NoError(t, err)

	stats = bus.GetStats()
	assert.Equal(t, 1, stats.Subscribers)

	// Send message
	ctx := context.Background()
	msg := NewTeamMessage(MessageTypeText, "sender", "agent1", "Hello!")
	err = bus.Send(ctx, msg)
	require.NoError(t, err)

	// Wait for async delivery
	time.Sleep(10 * time.Millisecond)

	stats = bus.GetStats()
	assert.Equal(t, 1, stats.MessagesSent)
	assert.Equal(t, 1, stats.MessagesDelivered)
	assert.Equal(t, 0, stats.MessagesFailed)
}

func TestNewTeamMessage(t *testing.T) {
	msg := NewTeamMessage(MessageTypeText, "agent1", "agent2", "Hello")

	assert.NotEmpty(t, msg.ID)
	assert.Contains(t, msg.ID, "agent1")
	assert.Equal(t, MessageTypeText, msg.Type)
	assert.Equal(t, "agent1", msg.From)
	assert.Equal(t, "agent2", msg.To)
	assert.Equal(t, "Hello", msg.Content)
	assert.NotNil(t, msg.Data)
	assert.NotZero(t, msg.Timestamp)
}

func TestTeamMessage_AddData_GetData(t *testing.T) {
	msg := NewTeamMessage(MessageTypeText, "agent1", "agent2", "Hello")

	// Test adding data
	msg.AddData("key1", "value1")
	msg.AddData("key2", 42)

	// Test getting data
	value1, exists := msg.GetData("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", value1)

	value2, exists := msg.GetData("key2")
	assert.True(t, exists)
	assert.Equal(t, 42, value2)

	// Test getting non-existent key
	_, exists = msg.GetData("nonexistent")
	assert.False(t, exists)
}

func TestMessageType_String(t *testing.T) {
	assert.Equal(t, "text", MessageTypeText.String())
	assert.Equal(t, "task", MessageTypeTask.String())
	assert.Equal(t, "result", MessageTypeResult.String())
	assert.Equal(t, "status", MessageTypeStatus.String())
	assert.Equal(t, "error", MessageTypeError.String())
	assert.Equal(t, "handoff", MessageTypeHandoff.String())
	assert.Equal(t, "unknown", MessageType(999).String())
}
