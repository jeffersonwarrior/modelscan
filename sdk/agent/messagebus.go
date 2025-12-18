package agent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TeamMessage represents a message between agents in a team
type TeamMessage struct {
	ID        string                 `json:"id"`
	Type      MessageType            `json:"type"`
	From      string                 `json:"from"`
	To        string                 `json:"to,omitempty"` // Empty for broadcast
	Content   string                 `json:"content"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// MessageType defines the type of team message
type MessageType int

const (
	MessageTypeText MessageType = iota
	MessageTypeTask
	MessageTypeResult
	MessageTypeStatus
	MessageTypeError
	MessageTypeHandoff
)

// String returns the string representation of MessageType
func (mt MessageType) String() string {
	switch mt {
	case MessageTypeText:
		return "text"
	case MessageTypeTask:
		return "task"
	case MessageTypeResult:
		return "result"
	case MessageTypeStatus:
		return "status"
	case MessageTypeError:
		return "error"
	case MessageTypeHandoff:
		return "handoff"
	default:
		return "unknown"
	}
}

// MessageHandler defines the function signature for handling team messages
type MessageHandler func(TeamMessage) error

// MessageBus provides inter-agent communication capabilities
type MessageBus interface {
	// Subscribe registers a message handler for an agent
	Subscribe(id string, handler MessageHandler) error
	
	// Unsubscribe removes a message handler
	Unsubscribe(id string) error
	
	// Send sends a message to a specific agent
	Send(ctx context.Context, msg TeamMessage) error
	
	// Broadcast sends a message to all agents except sender
	Broadcast(ctx context.Context, msg TeamMessage) error
	
	// GetStats returns message bus statistics
	GetStats() MessageBusStats
	
	// Shutdown gracefully shuts down the message bus
	Shutdown()
}

// MessageBusStats contains statistics about the message bus
type MessageBusStats struct {
	Subscribers       int
	MessagesSent      int
	MessagesDelivered int
	MessagesFailed    int
	Uptime            time.Duration
}

// InMemoryMessageBus implements MessageBus with in-memory storage
type InMemoryMessageBus struct {
	mu           sync.RWMutex
	subscribers  map[string]MessageHandler
	stats        MessageBusStats
	startTime    time.Time
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	shutdownOnce sync.Once
}

// NewInMemoryMessageBus creates a new in-memory message bus
func NewInMemoryMessageBus() *InMemoryMessageBus {
	ctx, cancel := context.WithCancel(context.Background())
	mb := &InMemoryMessageBus{
		subscribers: make(map[string]MessageHandler),
		startTime:   time.Now(),
		ctx:         ctx,
		cancel:      cancel,
	}
	
	// Start maintenance goroutine
	mb.wg.Add(1)
	go mb.maintenanceLoop()
	
	return mb
}

// Subscribe registers a message handler for an agent
func (mb *InMemoryMessageBus) Subscribe(id string, handler MessageHandler) error {
	if id == "" {
		return fmt.Errorf("subscriber ID cannot be empty")
	}
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}
	
	mb.mu.Lock()
	defer mb.mu.Unlock()
	
	if _, exists := mb.subscribers[id]; exists {
		return fmt.Errorf("subscriber %s already exists", id)
	}
	
	mb.subscribers[id] = handler
	mb.stats.Subscribers = len(mb.subscribers)
	
	return nil
}

// Unsubscribe removes a message handler
func (mb *InMemoryMessageBus) Unsubscribe(id string) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	
	if _, exists := mb.subscribers[id]; !exists {
		return fmt.Errorf("subscriber %s not found", id)
	}
	
	delete(mb.subscribers, id)
	mb.stats.Subscribers = len(mb.subscribers)
	
	return nil
}

// Send sends a message to a specific agent
func (mb *InMemoryMessageBus) Send(ctx context.Context, msg TeamMessage) error {
	if msg.From == "" {
		return fmt.Errorf("message sender cannot be empty")
	}
	if msg.To == "" {
		return fmt.Errorf("message recipient cannot be empty for direct message")
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	
	// Generate message ID if not provided
	if msg.ID == "" {
		msg.ID = fmt.Sprintf("%s-%s-%d", msg.From, msg.To, time.Now().UnixNano())
	}
	
	// Increment sent counter regardless of outcome
	mb.mu.Lock()
	mb.stats.MessagesSent++
	mb.mu.Unlock()
	
	mb.mu.RLock()
	handler, exists := mb.subscribers[msg.To]
	mb.mu.RUnlock()
	
	if !exists {
		mb.mu.Lock()
		mb.stats.MessagesFailed++
		mb.mu.Unlock()
		return fmt.Errorf("recipient %s not found", msg.To)
	}
	
	// Deliver message asynchronously
	mb.wg.Add(1)
	go func() {
		defer mb.wg.Done()
		
		select {
		case <-ctx.Done():
			mb.mu.Lock()
			mb.stats.MessagesFailed++
			mb.mu.Unlock()
			return
		default:
			if err := handler(msg); err != nil {
				mb.mu.Lock()
				mb.stats.MessagesFailed++
				mb.mu.Unlock()
			} else {
				mb.mu.Lock()
				mb.stats.MessagesDelivered++
				mb.mu.Unlock()
			}
		}
	}()
	
	return nil
}

// Broadcast sends a message to all agents except sender
func (mb *InMemoryMessageBus) Broadcast(ctx context.Context, msg TeamMessage) error {
	if msg.From == "" {
		return fmt.Errorf("message sender cannot be empty")
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	
	// Generate message ID if not provided
	if msg.ID == "" {
		msg.ID = fmt.Sprintf("broadcast-%s-%d", msg.From, time.Now().UnixNano())
	}
	
	mb.mu.RLock()
	handlers := make(map[string]MessageHandler)
	for id, handler := range mb.subscribers {
		if id != msg.From { // Don't send to sender
			handlers[id] = handler
		}
	}
	mb.mu.RUnlock()
	
	if len(handlers) == 0 {
		return fmt.Errorf("no recipients available for broadcast")
	}
	
	// Deliver messages asynchronously to all recipients
	for recipientID, handler := range handlers {
		mb.wg.Add(1)
		go func(recipient string, h MessageHandler) {
			defer mb.wg.Done()
			
			select {
			case <-ctx.Done():
				mb.mu.Lock()
				mb.stats.MessagesFailed++
				mb.mu.Unlock()
				return
			default:
				recipientMsg := msg
				recipientMsg.To = recipient
				
				if err := h(recipientMsg); err != nil {
					mb.mu.Lock()
					mb.stats.MessagesFailed++
					mb.mu.Unlock()
				} else {
					mb.mu.Lock()
					mb.stats.MessagesDelivered++
					mb.mu.Unlock()
				}
			}
		}(recipientID, handler)
	}
	
	mb.mu.Lock()
	mb.stats.MessagesSent += len(handlers)
	mb.mu.Unlock()
	
	return nil
}

// GetStats returns message bus statistics
func (mb *InMemoryMessageBus) GetStats() MessageBusStats {
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	
	stats := mb.stats
	stats.Uptime = time.Since(mb.startTime)
	return stats
}

// Shutdown gracefully shuts down the message bus
func (mb *InMemoryMessageBus) Shutdown() {
	mb.shutdownOnce.Do(func() {
		mb.cancel()
		
		// Wait for all message deliveries to complete
		done := make(chan struct{})
		go func() {
			mb.wg.Wait()
			close(done)
		}()
		
		select {
		case <-done:
			// All deliveries completed
		case <-time.After(5 * time.Second):
			// Timeout - force shutdown
		}
		
		mb.mu.Lock()
		mb.subscribers = make(map[string]MessageHandler)
		mb.stats.Subscribers = 0  // Reset subscriber count
		mb.mu.Unlock()
	})
}

// maintenanceLoop performs periodic maintenance tasks
func (mb *InMemoryMessageBus) maintenanceLoop() {
	defer mb.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-mb.ctx.Done():
			return
		case <-ticker.C:
			// Perform maintenance tasks (could include cleanup, stats updates, etc.)
			// For now, just update uptime
			mb.mu.RLock()
			_ = mb.stats.Uptime
			mb.mu.RUnlock()
		}
	}
}

// NewTeamMessage creates a new team message with proper initialization
func NewTeamMessage(msgType MessageType, from, to, content string) TeamMessage {
	return TeamMessage{
		ID:        fmt.Sprintf("%s-%d", from, time.Now().UnixNano()),
		Type:      msgType,
		From:      from,
		To:        to,
		Content:   content,
		Data:      make(map[string]interface{}),
		Timestamp: time.Now(),
	}
}

// AddData adds a key-value pair to the message data
func (tm *TeamMessage) AddData(key string, value interface{}) {
	if tm.Data == nil {
		tm.Data = make(map[string]interface{})
	}
	tm.Data[key] = value
}

// GetData retrieves a value from the message data
func (tm *TeamMessage) GetData(key string) (interface{}, bool) {
	if tm.Data == nil {
		return nil, false
	}
	value, exists := tm.Data[key]
	return value, exists
}