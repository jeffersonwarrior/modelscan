package agent

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryOption configures the in-memory implementation
type MemoryOption func(*InMemoryMemory)

// WithCapacity sets the maximum number of messages to store
func WithCapacity(capacity int) MemoryOption {
	return func(m *InMemoryMemory) {
		m.capacity = capacity
	}
}

// WithExpiration sets the expiration duration for messages
func WithExpiration(expiration time.Duration) MemoryOption {
	return func(m *InMemoryMemory) {
		m.expiration = expiration
	}
}

// InMemoryMemory provides an in-memory implementation of the Memory interface
type InMemoryMemory struct {
	mu         sync.RWMutex
	messages   []MemoryMessage // Messages in chronological order (oldest first)
	capacity   int             // Maximum number of messages to store (0 = unlimited)
	expiration time.Duration   // Message expiration time (0 = no expiration)
}

// NewInMemoryMemory creates a new in-memory storage
func NewInMemoryMemory(opts ...MemoryOption) *InMemoryMemory {
	memory := &InMemoryMemory{
		capacity:   0,        // Unlimited by default
		expiration: 0,        // No expiration by default
		messages:   make([]MemoryMessage, 0),
	}
	
	for _, opt := range opts {
		opt(memory)
	}
	
	return memory
}

// Store stores a message in memory
func (m *InMemoryMemory) Store(ctx context.Context, message MemoryMessage) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Assign ID if not provided
	if message.ID == "" {
		message.ID = uuid.New().String()
	}
	
	// Set timestamp if not provided
	if message.Timestamp == 0 {
		message.Timestamp = time.Now().UnixNano()
	}
	
	// Initialize metadata if nil
	if message.Metadata == nil {
		message.Metadata = make(map[string]interface{})
	}
	
	// Append message
	m.messages = append(m.messages, message)
	
	// Enforce capacity limit
	if m.capacity > 0 && len(m.messages) > m.capacity {
		// Remove oldest messages to maintain capacity
		overflow := len(m.messages) - m.capacity
		m.messages = m.messages[overflow:]
	}
	
	return nil
}

// Retrieve retrieves messages from memory
func (m *InMemoryMemory) Retrieve(ctx context.Context, query string, limit int) ([]MemoryMessage, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Filter expires messages
	now := time.Now().UnixNano()
	validMessages := make([]MemoryMessage, 0, len(m.messages))
	
	for _, msg := range m.messages {
		// Skip expired messages
		if m.expiration > 0 && now-msg.Timestamp > m.expiration.Nanoseconds() {
			continue
		}
		
		// Filter by query if provided
		if query != "" && !strings.Contains(strings.ToLower(msg.Content), strings.ToLower(query)) {
			continue
		}
		
		validMessages = append(validMessages, msg)
	}
	
	// Sort messages in reverse chronological order (newest first)
	sort.Slice(validMessages, func(i, j int) bool {
		return validMessages[i].Timestamp > validMessages[j].Timestamp
	})
	
	// Apply limit
	if limit > 0 && len(validMessages) > limit {
		validMessages = validMessages[:limit]
	}
	
	return validMessages, nil
}

// Search searches for messages matching the pattern
func (m *InMemoryMemory) Search(ctx context.Context, pattern string) ([]MemoryMessage, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Filter expires messages
	now := time.Now().UnixNano()
	var results []MemoryMessage
	
	patternLower := strings.ToLower(pattern)
	
	for _, msg := range m.messages {
		// Skip expired messages
		if m.expiration > 0 && now-msg.Timestamp > m.expiration.Nanoseconds() {
			continue
		}
		
		// Check if pattern matches content or metadata
		contentMatches := strings.Contains(strings.ToLower(msg.Content), patternLower)
		roleMatches := strings.Contains(strings.ToLower(msg.Role), patternLower)
		
		// Check metadata string values
		var metadataMatches bool
		for _, v := range msg.Metadata {
			if strVal, ok := v.(string); ok && strings.Contains(strings.ToLower(strVal), patternLower) {
				metadataMatches = true
				break
			}
		}
		
		if contentMatches || roleMatches || metadataMatches {
			results = append(results, msg)
		}
	}
	
	// Sort results by timestamp (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp > results[j].Timestamp
	})
	
	return results, nil
}

// Clear removes all messages from memory
func (m *InMemoryMemory) Clear(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.messages = make([]MemoryMessage, 0)
	return nil
}

// Cleanup removes expired messages from memory
func (m *InMemoryMemory) Cleanup(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	if m.expiration == 0 {
		return nil // No expiration, nothing to clean up
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now().UnixNano()
	validMessages := make([]MemoryMessage, 0, len(m.messages))
	
	for _, msg := range m.messages {
		if now-msg.Timestamp <= m.expiration.Nanoseconds() {
			validMessages = append(validMessages, msg)
		}
	}
	
	m.messages = validMessages
	return nil
}

// Len returns the number of messages currently stored (non-expired)
func (m *InMemoryMemory) Len(ctx context.Context) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.expiration == 0 {
		return len(m.messages), nil
	}
	
	now := time.Now().UnixNano()
	count := 0
	for _, msg := range m.messages {
		if now-msg.Timestamp <= m.expiration.Nanoseconds() {
			count++
		}
	}
	
	return count, nil
}

// Stats returns memory statistics
type MemoryStats struct {
	TotalMessages int     `json:"total_messages"`
	Capacity      int     `json:"capacity"`
	UsagePercent  float64 `json:"usage_percent"` // 0-1 range
	HasExpiration bool    `json:"has_expiration"`
	Expiration    string  `json:"expiration"`
}

// Stats returns memory statistics
func (m *InMemoryMemory) Stats(ctx context.Context) (MemoryStats, error) {
	select {
	case <-ctx.Done():
		return MemoryStats{}, ctx.Err()
	default:
	}
	
	count, err := m.Len(ctx)
	if err != nil {
		return MemoryStats{}, err
	}
	
	stats := MemoryStats{
		TotalMessages: count,
		Capacity:      m.capacity,
		HasExpiration: m.expiration > 0,
	}
	
	if m.expiration > 0 {
		stats.Expiration = m.expiration.String()
	}
	
	if m.capacity > 0 {
		stats.UsagePercent = float64(count) / float64(m.capacity)
	}
	
	return stats, nil
}

// Mock memory for testing
type MockMemory struct {
	messages []MemoryMessage
	mu       sync.RWMutex
}

// NewMockMemory creates a mock memory implementation
func NewMockMemory() *MockMemory {
	return &MockMemory{
		messages: make([]MemoryMessage, 0),
	}
}

// Store stores a message in mock memory
func (m *MockMemory) Store(ctx context.Context, message MemoryMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if message.ID == "" {
		message.ID = fmt.Sprintf("mock-%d", rand.Intn(10000))
	}
	
	if message.Timestamp == 0 {
		message.Timestamp = time.Now().UnixNano()
	}
	
	m.messages = append(m.messages, message)
	return nil
}

// Retrieve retrieves messages from mock memory
func (m *MockMemory) Retrieve(ctx context.Context, query string, limit int) ([]MemoryMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var results []MemoryMessage
	
	for _, msg := range m.messages {
		if query == "" || strings.Contains(strings.ToLower(msg.Content), strings.ToLower(query)) {
			results = append(results, msg)
		}
	}
	
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	
	return results, nil
}

// Search searches messages in mock memory
func (m *MockMemory) Search(ctx context.Context, pattern string) ([]MemoryMessage, error) {
	return m.Retrieve(ctx, pattern, 0)
}

// Clear clears mock memory
func (m *MockMemory) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.messages = make([]MemoryMessage, 0)
	return nil
}