package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// Storage provides a unified interface to all repositories
type Storage struct {
	db             *sql.DB
	Agents         *AgentRepository
	Tasks          *TaskRepository
	Messages       *MessageRepository
	Teams          *TeamRepository
	ToolExecutions *ToolExecutionRepository
	dataRetention  time.Duration
}

// NewStorage creates a new storage instance with all repositories
func NewStorage(db *sql.DB, dataRetention time.Duration) *Storage {
	return &Storage{
		db:             db,
		Agents:         NewAgentRepository(db),
		Tasks:          NewTaskRepository(db),
		Messages:       NewMessageRepository(db),
		Teams:          NewTeamRepository(db),
		ToolExecutions: NewToolExecutionRepository(db),
		dataRetention:  dataRetention,
	}
}

// NewAgentWithDefaults creates a new agent with default values
func (s *Storage) NewAgentWithDefaults(name, agentType string, capabilities []string) *Agent {
	return &Agent{
		ID:           uuid.New().String(),
		Name:         name,
		Capabilities: capabilities,
		Config:       fmt.Sprintf(`{"type": "%s", "version": "1.0"}`, agentType),
		Status:       "idle",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// NewTaskWithDefaults creates a new task with default values
func (s *Storage) NewTaskWithDefaults(agentID, taskType, input string, priority int) *Task {
	return &Task{
		ID:        uuid.New().String(),
		AgentID:   agentID,
		Type:      taskType,
		Status:    "pending",
		Priority:  priority,
		Input:     input,
		Metadata:  make(map[string]interface{}),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// NewMessageWithDefaults creates a new message with default values
func (s *Storage) NewMessageWithDefaults(taskID, agentID, messageType, content string) *Message {
	return &Message{
		ID:        uuid.New().String(),
		TaskID:    taskID,
		AgentID:   agentID,
		Type:      messageType,
		Content:   content,
		Metadata:  make(map[string]interface{}),
		CreatedAt: time.Now(),
	}
}

// NewTeamWithDefaults creates a new team with default values
func (s *Storage) NewTeamWithDefaults(name, description string) *Team {
	return &Team{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Config:      `{"version": "1.0"}`,
		Metadata:    make(map[string]interface{}),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// NewToolExecutionWithDefaults creates a new tool execution with default values
func (s *Storage) NewToolExecutionWithDefaults(taskID, agentID, toolName, toolType, input string) *ToolExecution {
	return &ToolExecution{
		ID:        uuid.New().String(),
		TaskID:    taskID,
		AgentID:   agentID,
		ToolName:  toolName,
		ToolType:  toolType,
		Input:     input,
		Status:    "running",
		Duration:  0,
		Metadata:  make(map[string]interface{}),
		StartedAt: time.Now(),
	}
}

// CleanupOldData removes old data based on retention policy
func (s *Storage) CleanupOldData(ctx context.Context) error {
	cutoffDate := time.Now().Add(-s.dataRetention)

	// Clean up old tasks
	_, err := s.db.ExecContext(ctx, "DELETE FROM tasks WHERE created_at < ?", cutoffDate)
	if err != nil {
		return fmt.Errorf("failed to cleanup old tasks: %w", err)
	}

	// Clean up old messages
	_, err = s.db.ExecContext(ctx, "DELETE FROM messages WHERE created_at < ?", cutoffDate)
	if err != nil {
		return fmt.Errorf("failed to cleanup old messages: %w", err)
	}

	// Clean up old tool executions
	_, err = s.db.ExecContext(ctx, "DELETE FROM tool_executions WHERE started_at < ?", cutoffDate)
	if err != nil {
		return fmt.Errorf("failed to cleanup old tool executions: %w", err)
	}

	return nil
}

// GetStorageStats returns statistics about the storage
func (s *Storage) GetStorageStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Count agents
	var agentCount int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM agents").Scan(&agentCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count agents: %w", err)
	}
	stats["agents"] = agentCount

	// Count tasks
	var taskCount int
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks").Scan(&taskCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count tasks: %w", err)
	}
	stats["tasks"] = taskCount

	// Count messages
	var messageCount int
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM messages").Scan(&messageCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count messages: %w", err)
	}
	stats["messages"] = messageCount

	// Count teams
	var teamCount int
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM teams").Scan(&teamCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count teams: %w", err)
	}
	stats["teams"] = teamCount

	// Count tool executions
	var toolExecCount int
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tool_executions").Scan(&toolExecCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count tool executions: %w", err)
	}
	stats["tool_executions"] = toolExecCount

	// Database size (SQLite specific)
	var dbSize int64
	err = s.db.QueryRowContext(ctx, "SELECT page_count * page_size as size FROM pragma_page_count(), pragma_page_size()").Scan(&dbSize)
	if err != nil {
		// Fallback if pragma queries fail
		dbSize = -1
	}
	stats["database_size_bytes"] = dbSize

	return stats, nil
}

// PerformHealthCheck performs a health check on the storage
func (s *Storage) PerformHealthCheck(ctx context.Context) error {
	// Test database connection
	err := s.db.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Test a simple query
	var result int
	err = s.db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("database query failed: %w", err)
	}

	if result != 1 {
		return fmt.Errorf("unexpected query result: %d", result)
	}

	return nil
}

// Close closes the database connection
func (s *Storage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// SetAllAgentsIdle sets all agents to idle status (for startup zero-state)
func (s *Storage) SetAllAgentsIdle(ctx context.Context) error {
	query := `UPDATE agents SET status = 'idle', updated_at = CURRENT_TIMESTAMP WHERE status != 'idle'`

	_, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to set all agents idle: %w", err)
	}

	return nil
}

// CancelAllPendingTasks cancels all pending tasks (for startup zero-state)
func (s *Storage) CancelAllPendingTasks(ctx context.Context) error {
	query := `UPDATE tasks SET status = 'cancelled', updated_at = CURRENT_TIMESTAMP WHERE status = 'pending' OR status = 'running'`

	_, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to cancel all pending tasks: %w", err)
	}

	return nil
}

// InitializeZeroState performs startup zero-state initialization
func (s *Storage) InitializeZeroState(ctx context.Context) error {
	// Set all agents to idle
	if err := s.SetAllAgentsIdle(ctx); err != nil {
		return fmt.Errorf("failed to set agents idle: %w", err)
	}

	// Cancel all pending tasks
	if err := s.CancelAllPendingTasks(ctx); err != nil {
		return fmt.Errorf("failed to cancel pending tasks: %w", err)
	}

	return nil
}
