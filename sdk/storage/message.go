package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Message represents a message in the database
type Message struct {
	ID        string                 `json:"id"`
	TaskID    string                 `json:"task_id"`
	AgentID   string                 `json:"agent_id"`
	TeamID    *string                `json:"team_id,omitempty"`
	Type      string                 `json:"type"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata"`
	CreatedAt time.Time              `json:"created_at"`
}

// MessageRepository handles message database operations
type MessageRepository struct {
	db *sql.DB
}

// NewMessageRepository creates a new message repository
func NewMessageRepository(db *sql.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// Create creates a new message
func (r *MessageRepository) Create(ctx context.Context, message *Message) error {
	metadataJSON, _ := json.Marshal(message.Metadata)
	
	query := `
		INSERT INTO messages (id, task_id, agent_id, team_id, type, content, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	
	_, err := r.db.ExecContext(ctx, query,
		message.ID, message.TaskID, message.AgentID, message.TeamID,
		message.Type, message.Content, metadataJSON)
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}
	
	return nil
}

// Get retrieves a message by ID
func (r *MessageRepository) Get(ctx context.Context, id string) (*Message, error) {
	query := `
		SELECT id, task_id, agent_id, team_id, type, content, metadata, created_at
		FROM messages WHERE id = ?
	`
	
	message := &Message{}
	var metadataJSON []byte
	
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&message.ID, &message.TaskID, &message.AgentID, &message.TeamID,
		&message.Type, &message.Content, &metadataJSON, &message.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("message not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get message: %w", err)
	}
	
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &message.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}
	
	return message, nil
}

// Delete deletes a message
func (r *MessageRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM messages WHERE id = ?`
	
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("message not found: %s", id)
	}
	
	return nil
}

// ListByTask retrieves messages for a specific task
func (r *MessageRepository) ListByTask(ctx context.Context, taskID string, limit, offset int) ([]*Message, error) {
	query := `
		SELECT id, task_id, agent_id, team_id, type, content, metadata, created_at
		FROM messages 
		WHERE task_id = ?
		ORDER BY created_at ASC
		LIMIT ? OFFSET ?
	`
	
	rows, err := r.db.QueryContext(ctx, query, taskID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages by task: %w", err)
	}
	defer rows.Close()
	
	var messages []*Message
	for rows.Next() {
		message := &Message{}
		var metadataJSON []byte
		
		err := rows.Scan(
			&message.ID, &message.TaskID, &message.AgentID, &message.TeamID,
			&message.Type, &message.Content, &metadataJSON, &message.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &message.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}
		
		messages = append(messages, message)
	}
	
	return messages, nil
}

// ListByAgent retrieves messages for a specific agent
func (r *MessageRepository) ListByAgent(ctx context.Context, agentID string, limit, offset int) ([]*Message, error) {
	query := `
		SELECT id, task_id, agent_id, team_id, type, content, metadata, created_at
		FROM messages 
		WHERE agent_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	
	rows, err := r.db.QueryContext(ctx, query, agentID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages by agent: %w", err)
	}
	defer rows.Close()
	
	var messages []*Message
	for rows.Next() {
		message := &Message{}
		var metadataJSON []byte
		
		err := rows.Scan(
			&message.ID, &message.TaskID, &message.AgentID, &message.TeamID,
			&message.Type, &message.Content, &metadataJSON, &message.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &message.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}
		
		messages = append(messages, message)
	}
	
	return messages, nil
}

// ListByTeam retrieves messages for a specific team
func (r *MessageRepository) ListByTeam(ctx context.Context, teamID string, limit, offset int) ([]*Message, error) {
	query := `
		SELECT id, task_id, agent_id, team_id, type, content, metadata, created_at
		FROM messages 
		WHERE team_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	
	rows, err := r.db.QueryContext(ctx, query, teamID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages by team: %w", err)
	}
	defer rows.Close()
	
	var messages []*Message
	for rows.Next() {
		message := &Message{}
		var metadataJSON []byte
		
		err := rows.Scan(
			&message.ID, &message.TaskID, &message.AgentID, &message.TeamID,
			&message.Type, &message.Content, &metadataJSON, &message.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &message.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}
		
		messages = append(messages, message)
	}
	
	return messages, nil
}

// DeleteByTask deletes all messages for a task
func (r *MessageRepository) DeleteByTask(ctx context.Context, taskID string) error {
	query := `DELETE FROM messages WHERE task_id = ?`
	
	_, err := r.db.ExecContext(ctx, query, taskID)
	if err != nil {
		return fmt.Errorf("failed to delete messages by task: %w", err)
	}
	
	return nil
}

// GetConversationThread retrieves the conversation thread for a task
func (r *MessageRepository) GetConversationThread(ctx context.Context, taskID string) ([]*Message, error) {
	query := `
		SELECT id, task_id, agent_id, team_id, type, content, metadata, created_at
		FROM messages 
		WHERE task_id = ?
		ORDER BY created_at ASC
	`
	
	rows, err := r.db.QueryContext(ctx, query, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation thread: %w", err)
	}
	defer rows.Close()
	
	var messages []*Message
	for rows.Next() {
		message := &Message{}
		var metadataJSON []byte
		
		err := rows.Scan(
			&message.ID, &message.TaskID, &message.AgentID, &message.TeamID,
			&message.Type, &message.Content, &metadataJSON, &message.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &message.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}
		
		messages = append(messages, message)
	}
	
	return messages, nil
}