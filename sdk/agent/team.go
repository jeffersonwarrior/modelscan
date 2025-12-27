package agent

import (
	"context"
	"fmt"
	"time"
)

// Team represents a collection of agents working together
type Team struct {
	name        string
	agents      map[string]*Agent
	coordinator *Coordinator
	messageBus  MessageBus
	maxParallel int
	timeout     time.Duration
	ctx         context.Context
	cancel      context.CancelFunc
}

// TeamOption configures a team
type TeamOption func(*Team)

// WithTeamTimeout sets the timeout for team operations
func WithTeamTimeout(timeout time.Duration) TeamOption {
	return func(t *Team) {
		t.timeout = timeout
	}
}

// WithMaxParallel sets the maximum number of parallel agents
func WithMaxParallel(max int) TeamOption {
	return func(t *Team) {
		t.maxParallel = max
	}
}

// NewTeam creates a new team of agents
func NewTeam(name string, opts ...TeamOption) *Team {
	ctx, cancel := context.WithCancel(context.Background())
	team := &Team{
		name:        name,
		agents:      make(map[string]*Agent),
		messageBus:  NewInMemoryMessageBus(),
		maxParallel: 5,
		timeout:     30 * time.Second,
		ctx:         ctx,
		cancel:      cancel,
	}

	team.coordinator = NewCoordinator(team)

	for _, opt := range opts {
		opt(team)
	}

	return team
}

// AddAgent adds an agent to the team
func (t *Team) AddAgent(id string, agent *Agent) error {
	if _, exists := t.agents[id]; exists {
		return fmt.Errorf("agent with ID %s already exists", id)
	}

	t.agents[id] = agent
	agent.teamContext = t // Link agent to team

	// Subscribe to team messages
	t.messageBus.Subscribe(id, func(msg TeamMessage) error {
		if agent.memory != nil {
			return agent.memory.Store(context.Background(), MemoryMessage{
				Role:      "team",
				Content:   fmt.Sprintf("[%s] %s", msg.From, msg.Content),
				Timestamp: time.Now().UnixNano(),
				Metadata: map[string]interface{}{
					"from":    msg.From,
					"to":      msg.To,
					"msgType": msg.Type,
				},
			})
		}
		return nil
	})

	return nil
}

// RemoveAgent removes an agent from the team
func (t *Team) RemoveAgent(id string) {
	if agent, exists := t.agents[id]; exists {
		agent.teamContext = nil // Clear agent's team context
	}
	delete(t.agents, id)
	t.messageBus.Unsubscribe(id)
}

// ExecuteTask executes a task using the team
func (t *Team) ExecuteTask(ctx context.Context, task Task) (*TaskResult, error) {
	// Apply team timeout if set
	if t.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.timeout)
		defer cancel()
	}

	return t.coordinator.ExecuteTask(ctx, task)
}

// Broadcast sends a message to all agents
func (t *Team) Broadcast(ctx context.Context, from, content string, msgType MessageType) error {
	msg := NewTeamMessage(msgType, from, "", content)
	return t.messageBus.Broadcast(ctx, msg)
}

// SendToAgent sends a message to a specific agent
func (t *Team) SendToAgent(ctx context.Context, from, to, content string, msgType MessageType) error {
	msg := NewTeamMessage(msgType, from, to, content)
	return t.messageBus.Send(ctx, msg)
}

// GetMember returns an agent by ID
func (t *Team) GetMember(id string) (*Agent, bool) {
	agent, exists := t.agents[id]
	return agent, exists
}

// ListMembers returns all agent IDs
func (t *Team) ListMembers() []string {
	ids := make([]string, 0, len(t.agents))
	for id := range t.agents {
		ids = append(ids, id)
	}
	return ids
}

// Size returns the number of agents in the team
func (t *Team) Size() int {
	return len(t.agents)
}

// Shutdown gracefully shuts down the team
func (t *Team) Shutdown() error {
	t.cancel()
	t.messageBus.Shutdown()
	return nil
}

// Task represents a task to be executed by the team
type Task struct {
	ID          string                 `json:"id"`
	Description string                 `json:"description"`
	Priority    int                    `json:"priority"`
	AssignedTo  string                 `json:"assigned_to"` // Agent ID, "team", or empty
	CreatedBy   string                 `json:"created_by"`
	Data        map[string]interface{} `json:"data"`
	Deadline    time.Time              `json:"deadline,omitempty"`
	Status      TaskStatus             `json:"status"`
}

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusActive    TaskStatus = "active"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// TaskResult represents the result of a task execution
type TaskResult struct {
	TaskID    string                 `json:"task_id"`
	Status    TaskStatus             `json:"status"`
	Result    string                 `json:"result"`
	Error     string                 `json:"error,omitempty"`
	AgentID   string                 `json:"agent_id,omitempty"`
	StartTime time.Time              `json:"start_time"`
	EndTime   time.Time              `json:"end_time"`
	Duration  time.Duration          `json:"duration"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	SubTasks  []*TaskResult          `json:"sub_tasks,omitempty"`
}

// NewTask creates a new task
func NewTask(description string, priority int) *Task {
	return &Task{
		ID:          fmt.Sprintf("task-%d", time.Now().UnixNano()),
		Description: description,
		Priority:    priority,
		Status:      TaskStatusPending,
		Data:        make(map[string]interface{}),
	}
}

// Assign assigns the task to an agent
func (t *Task) Assign(agentID string) {
	t.AssignedTo = agentID
	t.Status = TaskStatusPending
}

// Activate marks the task as active
func (t *Task) Activate() {
	t.Status = TaskStatusActive
}

// Complete marks the task as completed
func (t *Task) Complete() {
	t.Status = TaskStatusCompleted
}

// Fail marks the task as failed
func (t *Task) Fail(err error) {
	t.Status = TaskStatusFailed
	if err != nil {
		t.Data["error"] = err.Error()
	}
}

// Cancel marks the task as cancelled
func (t *Task) Cancel() {
	t.Status = TaskStatusCancelled
}

// IsCompleted returns true if the task is in a terminal state
func (t *Task) IsCompleted() bool {
	return t.Status == TaskStatusCompleted ||
		t.Status == TaskStatusFailed ||
		t.Status == TaskStatusCancelled
}

// SetData sets a key-value pair in the task data
func (t *Task) SetData(key string, value interface{}) {
	if t.Data == nil {
		t.Data = make(map[string]interface{})
	}
	t.Data[key] = value
}

// GetData gets a value from the task data
func (t *Task) GetData(key string) (interface{}, bool) {
	if t.Data == nil {
		return nil, false
	}
	value, exists := t.Data[key]
	return value, exists
}
