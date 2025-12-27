package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ToolExecution represents a tool execution in the database
type ToolExecution struct {
	ID          string                 `json:"id"`
	TaskID      string                 `json:"task_id"`
	AgentID     string                 `json:"agent_id"`
	ToolName    string                 `json:"tool_name"`
	ToolType    string                 `json:"tool_type"`
	Input       string                 `json:"input"`
	Output      string                 `json:"output"`
	Error       string                 `json:"error"`
	Status      string                 `json:"status"`
	Duration    int64                  `json:"duration"`
	Metadata    map[string]interface{} `json:"metadata"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
}

// ToolExecutionRepository handles tool execution database operations
type ToolExecutionRepository struct {
	db *sql.DB
}

// NewToolExecutionRepository creates a new tool execution repository
func NewToolExecutionRepository(db *sql.DB) *ToolExecutionRepository {
	return &ToolExecutionRepository{db: db}
}

// Create creates a new tool execution
func (r *ToolExecutionRepository) Create(ctx context.Context, execution *ToolExecution) error {
	metadataJSON, _ := json.Marshal(execution.Metadata)

	query := `
		INSERT INTO tool_executions (id, task_id, agent_id, tool_name, tool_type, input, output, error, status, duration, metadata, started_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		execution.ID, execution.TaskID, execution.AgentID, execution.ToolName, execution.ToolType,
		execution.Input, execution.Output, execution.Error, execution.Status, execution.Duration,
		metadataJSON, execution.StartedAt, execution.CompletedAt)
	if err != nil {
		return fmt.Errorf("failed to create tool execution: %w", err)
	}

	return nil
}

// Get retrieves a tool execution by ID
func (r *ToolExecutionRepository) Get(ctx context.Context, id string) (*ToolExecution, error) {
	query := `
		SELECT id, task_id, agent_id, tool_name, tool_type, input, output, error, status, duration, metadata, started_at, completed_at
		FROM tool_executions WHERE id = ?
	`

	execution := &ToolExecution{}
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&execution.ID, &execution.TaskID, &execution.AgentID, &execution.ToolName, &execution.ToolType,
		&execution.Input, &execution.Output, &execution.Error, &execution.Status, &execution.Duration,
		&metadataJSON, &execution.StartedAt, &execution.CompletedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tool execution not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get tool execution: %w", err)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &execution.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return execution, nil
}

// Update updates a tool execution
func (r *ToolExecutionRepository) Update(ctx context.Context, execution *ToolExecution) error {
	metadataJSON, _ := json.Marshal(execution.Metadata)

	query := `
		UPDATE tool_executions 
		SET task_id = ?, agent_id = ?, tool_name = ?, tool_type = ?, input = ?, output = ?, 
		    error = ?, status = ?, duration = ?, metadata = ?, started_at = ?, completed_at = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		execution.TaskID, execution.AgentID, execution.ToolName, execution.ToolType,
		execution.Input, execution.Output, execution.Error, execution.Status, execution.Duration,
		metadataJSON, execution.StartedAt, execution.CompletedAt, execution.ID)
	if err != nil {
		return fmt.Errorf("failed to update tool execution: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("tool execution not found: %s", execution.ID)
	}

	return nil
}

// MarkCompleted updates execution with completion details
func (r *ToolExecutionRepository) MarkCompleted(ctx context.Context, id, output, status string, duration int64) error {
	query := `
		UPDATE tool_executions 
		SET output = ?, status = ?, duration = ?, completed_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query, output, status, duration, id)
	if err != nil {
		return fmt.Errorf("failed to mark tool execution completed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("tool execution not found: %s", id)
	}

	return nil
}

// MarkFailed updates execution with error details
func (r *ToolExecutionRepository) MarkFailed(ctx context.Context, id, errorMsg string, duration int64) error {
	query := `
		UPDATE tool_executions 
		SET error = ?, status = 'failed', duration = ?, completed_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query, errorMsg, duration, id)
	if err != nil {
		return fmt.Errorf("failed to mark tool execution failed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("tool execution not found: %s", id)
	}

	return nil
}

// ListByTask retrieves tool executions for a specific task
func (r *ToolExecutionRepository) ListByTask(ctx context.Context, taskID string, limit, offset int) ([]*ToolExecution, error) {
	query := `
		SELECT id, task_id, agent_id, tool_name, tool_type, input, output, error, status, duration, metadata, started_at, completed_at
		FROM tool_executions 
		WHERE task_id = ?
		ORDER BY started_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, taskID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list tool executions by task: %w", err)
	}
	defer rows.Close()

	var executions []*ToolExecution
	for rows.Next() {
		execution := &ToolExecution{}
		var metadataJSON []byte

		err := rows.Scan(
			&execution.ID, &execution.TaskID, &execution.AgentID, &execution.ToolName, &execution.ToolType,
			&execution.Input, &execution.Output, &execution.Error, &execution.Status, &execution.Duration,
			&metadataJSON, &execution.StartedAt, &execution.CompletedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tool execution: %w", err)
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &execution.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		executions = append(executions, execution)
	}

	return executions, nil
}

// ListByAgent retrieves tool executions for a specific agent
func (r *ToolExecutionRepository) ListByAgent(ctx context.Context, agentID string, limit, offset int) ([]*ToolExecution, error) {
	query := `
		SELECT id, task_id, agent_id, tool_name, tool_type, input, output, error, status, duration, metadata, started_at, completed_at
		FROM tool_executions 
		WHERE agent_id = ?
		ORDER BY started_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, agentID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list tool executions by agent: %w", err)
	}
	defer rows.Close()

	var executions []*ToolExecution
	for rows.Next() {
		execution := &ToolExecution{}
		var metadataJSON []byte

		err := rows.Scan(
			&execution.ID, &execution.TaskID, &execution.AgentID, &execution.ToolName, &execution.ToolType,
			&execution.Input, &execution.Output, &execution.Error, &execution.Status, &execution.Duration,
			&metadataJSON, &execution.StartedAt, &execution.CompletedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tool execution: %w", err)
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &execution.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		executions = append(executions, execution)
	}

	return executions, nil
}

// ListByTool retrieves executions for a specific tool
func (r *ToolExecutionRepository) ListByTool(ctx context.Context, toolName string, limit, offset int) ([]*ToolExecution, error) {
	query := `
		SELECT id, task_id, agent_id, tool_name, tool_type, input, output, error, status, duration, metadata, started_at, completed_at
		FROM tool_executions 
		WHERE tool_name = ?
		ORDER BY started_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, toolName, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list tool executions by tool: %w", err)
	}
	defer rows.Close()

	var executions []*ToolExecution
	for rows.Next() {
		execution := &ToolExecution{}
		var metadataJSON []byte

		err := rows.Scan(
			&execution.ID, &execution.TaskID, &execution.AgentID, &execution.ToolName, &execution.ToolType,
			&execution.Input, &execution.Output, &execution.Error, &execution.Status, &execution.Duration,
			&metadataJSON, &execution.StartedAt, &execution.CompletedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tool execution: %w", err)
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &execution.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		executions = append(executions, execution)
	}

	return executions, nil
}

// DeleteByTask deletes all tool executions for a task
func (r *ToolExecutionRepository) DeleteByTask(ctx context.Context, taskID string) error {
	query := `DELETE FROM tool_executions WHERE task_id = ?`

	_, err := r.db.ExecContext(ctx, query, taskID)
	if err != nil {
		return fmt.Errorf("failed to delete tool executions by task: %w", err)
	}

	return nil
}

// GetUsageStats retrieves usage statistics for tools
func (r *ToolExecutionRepository) GetUsageStats(ctx context.Context, since time.Time) ([]map[string]interface{}, error) {
	query := `
		SELECT 
			tool_name,
			COUNT(*) as execution_count,
			AVG(duration) as avg_duration,
			COUNT(CASE WHEN status = 'completed' THEN 1 END) as success_count,
			COUNT(CASE WHEN status = 'failed' THEN 1 END) as failure_count
		FROM tool_executions 
		WHERE started_at >= ?
		GROUP BY tool_name
		ORDER BY execution_count DESC
	`

	rows, err := r.db.QueryContext(ctx, query, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool usage stats: %w", err)
	}
	defer rows.Close()

	var stats []map[string]interface{}
	for rows.Next() {
		var toolName string
		var executionCount, successCount, failureCount int
		var avgDuration float64

		err := rows.Scan(&toolName, &executionCount, &avgDuration, &successCount, &failureCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan usage stats: %w", err)
		}

		stat := map[string]interface{}{
			"tool_name":       toolName,
			"execution_count": executionCount,
			"avg_duration":    avgDuration,
			"success_count":   successCount,
			"failure_count":   failureCount,
		}
		stats = append(stats, stat)
	}

	return stats, nil
}
