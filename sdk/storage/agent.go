package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Agent represents an agent in the database
type Agent struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Capabilities []string  `json:"capabilities"`
	Config       string    `json:"config"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AgentRepository handles agent database operations
type AgentRepository struct {
	db *sql.DB
}

// NewAgentRepository creates a new agent repository
func NewAgentRepository(db *sql.DB) *AgentRepository {
	return &AgentRepository{db: db}
}

// Create creates a new agent
func (r *AgentRepository) Create(ctx context.Context, agent *Agent) error {
	capabilitiesJSON, _ := json.Marshal(agent.Capabilities)

	query := `
		INSERT INTO agents (id, name, capabilities, config, status)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		agent.ID, agent.Name, capabilitiesJSON, agent.Config, agent.Status)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	return nil
}

// Get retrieves an agent by ID
func (r *AgentRepository) Get(ctx context.Context, id string) (*Agent, error) {
	query := `
		SELECT id, name, capabilities, config, status, created_at, updated_at
		FROM agents WHERE id = ?
	`

	agent := &Agent{}
	var capabilitiesJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&agent.ID, &agent.Name, &capabilitiesJSON, &agent.Config,
		&agent.Status, &agent.CreatedAt, &agent.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("agent not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	if len(capabilitiesJSON) > 0 {
		if err := json.Unmarshal(capabilitiesJSON, &agent.Capabilities); err != nil {
			return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
		}
	}

	return agent, nil
}

// Update updates an agent
func (r *AgentRepository) Update(ctx context.Context, agent *Agent) error {
	capabilitiesJSON, _ := json.Marshal(agent.Capabilities)

	query := `
		UPDATE agents 
		SET name = ?, capabilities = ?, config = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		agent.Name, capabilitiesJSON, agent.Config, agent.Status, agent.ID)
	if err != nil {
		return fmt.Errorf("failed to update agent: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("agent not found: %s", agent.ID)
	}

	return nil
}

// Delete deletes an agent
func (r *AgentRepository) Delete(ctx context.Context, id string) error {
	query := "DELETE FROM agents WHERE id = ?"

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("agent not found: %s", id)
	}

	return nil
}

// List retrieves all agents
func (r *AgentRepository) List(ctx context.Context) ([]*Agent, error) {
	query := `
		SELECT id, name, capabilities, config, status, created_at, updated_at
		FROM agents ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}
	defer rows.Close()

	var agents []*Agent
	for rows.Next() {
		agent := &Agent{}
		var capabilitiesJSON []byte

		err := rows.Scan(
			&agent.ID, &agent.Name, &capabilitiesJSON, &agent.Config,
			&agent.Status, &agent.CreatedAt, &agent.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent row: %w", err)
		}

		if len(capabilitiesJSON) > 0 {
			if err := json.Unmarshal(capabilitiesJSON, &agent.Capabilities); err != nil {
				return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
			}
		}

		agents = append(agents, agent)
	}

	return agents, nil
}

// UpdateStatus updates an agent's status
func (r *AgentRepository) UpdateStatus(ctx context.Context, id, status string) error {
	query := "UPDATE agents SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?"

	result, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update agent status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("agent not found: %s", id)
	}

	return nil
}

// ListByStatus retrieves agents by status
func (r *AgentRepository) ListByStatus(ctx context.Context, status string) ([]*Agent, error) {
	query := `
		SELECT id, name, capabilities, config, status, created_at, updated_at
		FROM agents WHERE status = ? ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents by status: %w", err)
	}
	defer rows.Close()

	var agents []*Agent
	for rows.Next() {
		agent := &Agent{}
		var capabilitiesJSON []byte

		err := rows.Scan(
			&agent.ID, &agent.Name, &capabilitiesJSON, &agent.Config,
			&agent.Status, &agent.CreatedAt, &agent.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent row: %w", err)
		}

		if len(capabilitiesJSON) > 0 {
			if err := json.Unmarshal(capabilitiesJSON, &agent.Capabilities); err != nil {
				return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
			}
		}

		agents = append(agents, agent)
	}

	return agents, nil
}

// ListActive retrieves active agents (includes active and idle agents)
func (r *AgentRepository) ListActive(ctx context.Context, limit, offset int) ([]*Agent, error) {
	query := `
		SELECT id, name, capabilities, config, status, created_at, updated_at
		FROM agents 
		WHERE status IN ('active', 'idle')
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list active agents: %w", err)
	}
	defer rows.Close()

	var agents []*Agent
	for rows.Next() {
		agent := &Agent{}
		var capabilitiesJSON []byte

		err := rows.Scan(
			&agent.ID, &agent.Name, &capabilitiesJSON, &agent.Config,
			&agent.Status, &agent.CreatedAt, &agent.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent row: %w", err)
		}

		if len(capabilitiesJSON) > 0 {
			if err := json.Unmarshal(capabilitiesJSON, &agent.Capabilities); err != nil {
				return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
			}
		}

		agents = append(agents, agent)
	}

	return agents, nil
}

// SetActive marks all agents as inactive and activates specific agents (zero-state on startup)
func (r *AgentRepository) SetActive(ctx context.Context, activeIDs []string) error {
	// First, set all agents to inactive
	if _, err := r.db.ExecContext(ctx, "UPDATE agents SET status = 'inactive'"); err != nil {
		return fmt.Errorf("failed to deactivate all agents: %w", err)
	}

	// Then activate the specified agents
	if len(activeIDs) == 0 {
		return nil // No agents to activate
	}

	// Build placeholder query for activation
	query := "UPDATE agents SET status = 'active' WHERE id = ?"
	for i := 1; i < len(activeIDs); i++ {
		query += " OR id = ?"
	}

	args := make([]interface{}, len(activeIDs))
	for i, id := range activeIDs {
		args[i] = id
	}

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("failed to activate agents: %w", err)
	}

	return nil
}
