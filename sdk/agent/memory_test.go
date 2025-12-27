package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMemoryMemoryMessage tests the MemoryMessage structure
func TestMemoryMemoryMessage(t *testing.T) {
	now := time.Now().UnixNano()
	msg := MemoryMessage{
		ID:        "test-id",
		Role:      "user",
		Content:   "Hello, world!",
		Metadata:  map[string]interface{}{"source": "test"},
		Timestamp: now,
	}

	assert.Equal(t, "test-id", msg.ID)
	assert.Equal(t, "user", msg.Role)
	assert.Equal(t, "Hello, world!", msg.Content)
	assert.Equal(t, "test", msg.Metadata["source"])
	assert.Equal(t, now, msg.Timestamp)
}

// TestInMemoryMemory_Store_StoresMessage tests storing messages
func TestInMemoryMemory_Store_StoresMessage(t *testing.T) {
	memory := NewInMemoryMemory()
	ctx := context.Background()

	msg := MemoryMessage{
		Role:    "user",
		Content: "Test message",
	}

	err := memory.Store(ctx, msg)
	require.NoError(t, err)

	// Retrieve the message
	messages, err := memory.Retrieve(ctx, "", 10)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "Test message", messages[0].Content)
	assert.NotEmpty(t, messages[0].ID)
	assert.NotEmpty(t, messages[0].Timestamp)
}

// TestInMemoryMemory_Store_WithID_RespectsID tests that storing with ID preserves it
func TestInMemoryMemory_Store_WithID_RespectsID(t *testing.T) {
	memory := NewInMemoryMemory()
	ctx := context.Background()

	msg := MemoryMessage{
		ID:      "custom-id",
		Role:    "user",
		Content: "Test message",
	}

	err := memory.Store(ctx, msg)
	require.NoError(t, err)

	// Retrieve the message
	messages, err := memory.Retrieve(ctx, "", 10)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, "custom-id", messages[0].ID)
}

// TestInMemoryMemory_Retrieve_WithLimit_RespectsLimit tests retrieval with limit
func TestInMemoryMemory_Retrieve_WithLimit_RespectsLimit(t *testing.T) {
	memory := NewInMemoryMemory()
	ctx := context.Background()

	// Store 5 messages
	for i := 0; i < 5; i++ {
		msg := MemoryMessage{
			Role:    "user",
			Content: "Message " + string(rune('A'+i)),
		}
		err := memory.Store(ctx, msg)
		require.NoError(t, err)
	}

	// Retrieve with limit 3
	messages, err := memory.Retrieve(ctx, "", 3)
	require.NoError(t, err)
	require.Len(t, messages, 3)

	// Should get the most recent 3 messages
	assert.Equal(t, "Message E", messages[0].Content)
	assert.Equal(t, "Message D", messages[1].Content)
	assert.Equal(t, "Message C", messages[2].Content)
}

// TestInMemoryMemory_Retrieve_WithQuery_FiltersByContent tests retrieval with query
func TestInMemoryMemory_Retrieve_WithQuery_FiltersByContent(t *testing.T) {
	memory := NewInMemoryMemory()
	ctx := context.Background()

	// Store messages with different content
	messages := []MemoryMessage{
		{Role: "user", Content: "The weather is nice today"},
		{Role: "assistant", Content: "Yes, it's perfect for a walk"},
		{Role: "user", Content: "What about tomorrow?"},
		{Role: "assistant", Content: "Tomorrow will be rainy"},
	}

	for _, msg := range messages {
		err := memory.Store(ctx, msg)
		require.NoError(t, err)
	}

	// Search for "weather"
	weatherMessages, err := memory.Retrieve(ctx, "weather", 10)
	require.NoError(t, err)
	require.Len(t, weatherMessages, 1)
	assert.Contains(t, weatherMessages[0].Content, "weather")

	// Search for "tomorrow" - both messages contain "tomorrow"
	tomorrowMessages, err := memory.Retrieve(ctx, "tomorrow", 10)
	require.NoError(t, err)
	require.Len(t, tomorrowMessages, 2)
	assert.Contains(t, strings.ToLower(tomorrowMessages[0].Content), "tomorrow")
}

// TestInMemoryMemory_Search_FindsMatchingMessages tests search functionality
func TestInMemoryMemory_Search_FindsMatchingMessages(t *testing.T) {
	memory := NewInMemoryMemory()
	ctx := context.Background()

	// Store messages
	messages := []MemoryMessage{
		{Role: "user", Content: "How do I create a function in Python?"},
		{Role: "assistant", Content: "To create a function in Python, use def"},
		{Role: "user", Content: "What about JavaScript functions?"},
		{Role: "assistant", Content: "JavaScript functions can be declared with function"},
	}

	for _, msg := range messages {
		err := memory.Store(ctx, msg)
		require.NoError(t, err)
	}

	// Search for "function"
	results, err := memory.Search(ctx, "function")
	require.NoError(t, err)
	require.Len(t, results, 4) // All messages containing "function"

	// Search for "Python"
	pythonResults, err := memory.Search(ctx, "Python")
	require.NoError(t, err)
	require.Len(t, pythonResults, 2) // User question and assistant answer about Python
}

// TestInMemoryMemory_Clear_RemovesAllMessages tests clearing memory
func TestInMemoryMemory_Clear_RemovesAllMessages(t *testing.T) {
	memory := NewInMemoryMemory()
	ctx := context.Background()

	// Store some messages
	for i := 0; i < 3; i++ {
		msg := MemoryMessage{
			Role:    "user",
			Content: "Message " + string(rune('A'+i)),
		}
		err := memory.Store(ctx, msg)
		require.NoError(t, err)
	}

	// Verify messages exist
	messages, err := memory.Retrieve(ctx, "", 10)
	require.NoError(t, err)
	require.NotEmpty(t, messages)

	// Clear memory
	err = memory.Clear(ctx)
	require.NoError(t, err)

	// Verify messages are gone
	messages, err = memory.Retrieve(ctx, "", 10)
	require.NoError(t, err)
	require.Empty(t, messages)
}

// TestInMemoryMemory_Capacity_LimitsStorage tests capacity limits
func TestInMemoryMemory_Capacity_LimitsStorage(t *testing.T) {
	// Create memory with capacity of 3
	memory := NewInMemoryMemory(WithCapacity(3))
	ctx := context.Background()

	// Store 5 messages
	for i := 0; i < 5; i++ {
		msg := MemoryMessage{
			Role:    "user",
			Content: "Message " + string(rune('A'+i)),
		}
		err := memory.Store(ctx, msg)
		require.NoError(t, err)
	}

	// Should only have the most recent 3 messages
	messages, err := memory.Retrieve(ctx, "", 10)
	require.NoError(t, err)
	require.Len(t, messages, 3)

	// Should have messages C, D, E (A and B were evicted)
	assert.Equal(t, "Message E", messages[0].Content)
	assert.Equal(t, "Message D", messages[1].Content)
	assert.Equal(t, "Message C", messages[2].Content)
}

// TestInMemoryMemory_Expiration_RemovesOldMessages tests message expiration
func TestInMemoryMemory_Expiration_RemovesOldMessages(t *testing.T) {
	// Create memory with 1 hour expiration
	memory := NewInMemoryMemory(WithExpiration(time.Hour))
	ctx := context.Background()

	// Store a message
	msg := MemoryMessage{
		Role:    "user",
		Content: "Recent message",
	}
	err := memory.Store(ctx, msg)
	require.NoError(t, err)

	// Should have the message
	messages, err := memory.Retrieve(ctx, "", 10)
	require.NoError(t, err)
	require.Len(t, messages, 1)

	// Create memory with 1 nanosecond expiration (already expired)
	expiredMemory := NewInMemoryMemory(WithExpiration(time.Nanosecond))

	// Store a message
	err = expiredMemory.Store(ctx, MemoryMessage{
		Role:    "user",
		Content: "Expired message",
	})
	require.NoError(t, err)

	// Should not have the message due to expiration
	messages, err = expiredMemory.Retrieve(ctx, "", 10)
	require.NoError(t, err)
	require.Empty(t, messages)
}

// TestInMemoryMemory_ConcurrentAccess tests concurrent access to memory
func TestInMemoryMemory_ConcurrentAccess(t *testing.T) {
	memory := NewInMemoryMemory()
	ctx := context.Background()

	// Store messages concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(index int) {
			msg := MemoryMessage{
				Role:    "user",
				Content: "Concurrent message " + string(rune('A'+index)),
			}
			err := memory.Store(ctx, msg)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have all 10 messages
	messages, err := memory.Retrieve(ctx, "", 20)
	require.NoError(t, err)
	require.Len(t, messages, 10)
}
