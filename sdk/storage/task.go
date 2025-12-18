package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Task represents a task in the database
type Task struct {
	ID          string                 `json:"id"`
	AgentID     string                 `json:"agent_id"`
	TeamID      *string                `json:"team_id,omitempty"`
	Type        string                 `json:"type"`
	Status      string                 `json:"status"`
	Priority    int                    `json:"priority"`
	Input       string                 `json:"input"`
	Output      string                 `json:"output"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// TaskRepository handles task database operations
type TaskRepository struct {
	db *sql.DB
}

// NewTaskRepository creates a new task repository
func NewTaskRepository(db *sql.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

// Create creates a new task
func (r *TaskRepository) Create(ctx context.Context, task *Task) error {
	metadataJSON, _ := json.Marshal(task.Metadata)
	
	query := `
		INSERT INTO tasks (id, agent_id, team_id, type, status, priority, input, output, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	
	_, err := r.db.ExecContext(ctx, query,
		task.ID, task.AgentID, task.TeamID, task.Type, task.Status,
		task.Priority, task.Input, task.Output, metadataJSON)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}
	
	return nil
}

// Get retrieves a task by ID
func (r *TaskRepository) Get(ctx context.Context, id string) (*Task, error) {
	query := `
		SELECT id, agent_id, team_id, type, status, priority, input, output, metadata,
		       created_at, started_at, completed_at, updated_at
		FROM tasks WHERE id = ?
	`
	
	task := &Task{}
	var metadataJSON []byte
	
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&task.ID, &task.AgentID, &task.TeamID, &task.Type, &task.Status,
		&task.Priority, &task.Input, &task.Output, &metadataJSON,
		&task.CreatedAt, &task.StartedAt, &task.CompletedAt, &task.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &task.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}
	
	return task, nil
}

// Update updates a task
func (r *TaskRepository) Update(ctx context.Context, task *Task) error {
	metadataJSON, _ := json.Marshal(task.Metadata)
	
	query := `
		UPDATE tasks 
		SET agent_id = ?, team_id = ?, type = ?, status = ?, priority = ?,
		    input = ?, output = ?, metadata = ?, started_at = ?, completed_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	
	result, err := r.db.ExecContext(ctx, query,
		task.AgentID, task.TeamID, task.Type, task.Status, task.Priority,
		task.Input, task.Output, metadataJSON, task.StartedAt, task.CompletedAt, task.ID)
	if err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("task not found: %s", task.ID)
	}
	
	return nil
}

// Delete deletes a task
func (r *TaskRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM tasks WHERE id = ?`
	
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("task not found: %s", id)
	}
	
	return nil
}

// ListByAgent retrieves tasks for a specific agent
func (r *TaskRepository) ListByAgent(ctx context.Context, agentID string, limit, offset int) ([]*Task, error) {
	query := `
		SELECT id, agent_id, team_id, type, status, priority, input, output, metadata,
		       created_at, started_at, completed_at, updated_at
		FROM tasks 
		WHERE agent_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	
	rows, err := r.db.QueryContext(ctx, query, agentID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks by agent: %w", err)
	}
	defer rows.Close()
	
	var tasks []*Task
	for rows.Next() {
		task := &Task{}
		var metadataJSON []byte
		
		err := rows.Scan(
			&task.ID, &task.AgentID, &task.TeamID, &task.Type, &task.Status,
			&task.Priority, &task.Input, &task.Output, &metadataJSON,
			&task.CreatedAt, &task.StartedAt, &task.CompletedAt, &task.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &task.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}
		
		tasks = append(tasks, task)
	}
	
	return tasks, nil
}

// ListByTeam retrieves tasks for a specific team
func (r *TaskRepository) ListByTeam(ctx context.Context, teamID string, limit, offset int) ([]*Task, error) {
	query := `
		SELECT id, agent_id, team_id, type, status, priority, input, output, metadata,
		       created_at, started_at, completed_at, updated_at
		FROM tasks 
		WHERE team_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	
	rows, err := r.db.QueryContext(ctx, query, teamID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks by team: %w", err)
	}
	defer rows.Close()
	
	var tasks []*Task
	for rows.Next() {
		task := &Task{}
		var metadataJSON []byte
		
		err := rows.Scan(
			&task.ID, &task.AgentID, &task.TeamID, &task.Type, &task.Status,
			&task.Priority, &task.Input, &task.Output, &metadataJSON,
			&task.CreatedAt, &task.StartedAt, &task.CompletedAt, &task.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &task.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}
		
		tasks = append(tasks, task)
	}
	
	return tasks, nil
}

// ListByStatus retrieves tasks by status
func (r *TaskRepository) ListByStatus(ctx context.Context, status string, limit, offset int) ([]*Task, error) {
	query := `
		SELECT id, agent_id, team_id, type, status, priority, input, output, metadata,
		       created_at, started_at, completed_at, updated_at
		FROM tasks 
		WHERE status = ?
		ORDER BY priority DESC, created_at ASC
		LIMIT ? OFFSET ?
	`
	
	rows, err := r.db.QueryContext(ctx, query, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks by status: %w", err)
	}
	defer rows.Close()
	
	var tasks []*Task
	for rows.Next() {
		task := &Task{}
		var metadataJSON []byte
		
		err := rows.Scan(
			&task.ID, &task.AgentID, &task.TeamID, &task.Type, &task.Status,
			&task.Priority, &task.Input, &task.Output, &metadataJSON,
			&task.CreatedAt, &task.StartedAt, &task.CompletedAt, &task.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &task.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}
		
		tasks = append(tasks, task)
	}
	
	return tasks, nil
}

// UpdateStatus updates task status
func (r *TaskRepository) UpdateStatus(ctx context.Context, id, status string) error {
	query := `
		UPDATE tasks 
		SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	
	result, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("task not found: %s", id)
	}
	
	return nil
}