package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Client represents a registered client in the database
type Client struct {
	ID           string
	Name         string
	Version      string
	Token        string // Auth token for API calls
	Capabilities []string
	Config       ClientConfig
	CreatedAt    time.Time
	LastSeenAt   *time.Time
}

// ClientConfig holds client-specific configuration
type ClientConfig struct {
	DefaultModel     string   `json:"default_model,omitempty"`
	ThinkingModel    string   `json:"thinking_model,omitempty"`
	MaxOutputTokens  int      `json:"max_output_tokens,omitempty"`
	TimeoutMs        int      `json:"timeout_ms,omitempty"`
	ProviderPriority []string `json:"provider_priority,omitempty"`
}

// ClientRepository provides CRUD operations for clients
type ClientRepository struct {
	db *DB
}

// NewClientRepository creates a new ClientRepository
func NewClientRepository(db *DB) *ClientRepository {
	return &ClientRepository{db: db}
}

// Create inserts a new client
func (r *ClientRepository) Create(c *Client) error {
	capabilitiesJSON, err := json.Marshal(c.Capabilities)
	if err != nil {
		return fmt.Errorf("failed to marshal capabilities: %w", err)
	}

	configJSON, err := json.Marshal(c.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	query := `
		INSERT INTO clients (id, name, version, token, capabilities, config, created_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = r.db.conn.Exec(query,
		c.ID, c.Name, c.Version, c.Token, string(capabilitiesJSON), string(configJSON),
		c.CreatedAt, c.LastSeenAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	return nil
}

// Get retrieves a client by ID
func (r *ClientRepository) Get(id string) (*Client, error) {
	query := `
		SELECT id, name, version, token, capabilities, config, created_at, last_seen_at
		FROM clients WHERE id = ?
	`
	c := &Client{}
	var capabilitiesJSON, configJSON string
	err := r.db.conn.QueryRow(query, id).Scan(
		&c.ID, &c.Name, &c.Version, &c.Token,
		&capabilitiesJSON, &configJSON,
		&c.CreatedAt, &c.LastSeenAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}

	if err := json.Unmarshal([]byte(capabilitiesJSON), &c.Capabilities); err != nil {
		return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
	}
	if err := json.Unmarshal([]byte(configJSON), &c.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return c, nil
}

// GetByToken retrieves a client by auth token
func (r *ClientRepository) GetByToken(token string) (*Client, error) {
	query := `
		SELECT id, name, version, token, capabilities, config, created_at, last_seen_at
		FROM clients WHERE token = ?
	`
	c := &Client{}
	var capabilitiesJSON, configJSON string
	err := r.db.conn.QueryRow(query, token).Scan(
		&c.ID, &c.Name, &c.Version, &c.Token,
		&capabilitiesJSON, &configJSON,
		&c.CreatedAt, &c.LastSeenAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get client by token: %w", err)
	}

	if err := json.Unmarshal([]byte(capabilitiesJSON), &c.Capabilities); err != nil {
		return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
	}
	if err := json.Unmarshal([]byte(configJSON), &c.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return c, nil
}

// List retrieves all clients
func (r *ClientRepository) List() ([]*Client, error) {
	query := `
		SELECT id, name, version, token, capabilities, config, created_at, last_seen_at
		FROM clients ORDER BY name
	`
	rows, err := r.db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list clients: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var clients []*Client
	for rows.Next() {
		c := &Client{}
		var capabilitiesJSON, configJSON string
		err := rows.Scan(
			&c.ID, &c.Name, &c.Version, &c.Token,
			&capabilitiesJSON, &configJSON,
			&c.CreatedAt, &c.LastSeenAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan client: %w", err)
		}

		if err := json.Unmarshal([]byte(capabilitiesJSON), &c.Capabilities); err != nil {
			return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
		}
		if err := json.Unmarshal([]byte(configJSON), &c.Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}

		clients = append(clients, c)
	}
	return clients, rows.Err()
}

// Update updates a client
func (r *ClientRepository) Update(c *Client) error {
	capabilitiesJSON, err := json.Marshal(c.Capabilities)
	if err != nil {
		return fmt.Errorf("failed to marshal capabilities: %w", err)
	}

	configJSON, err := json.Marshal(c.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	query := `
		UPDATE clients
		SET name = ?, version = ?, token = ?, capabilities = ?, config = ?, last_seen_at = ?
		WHERE id = ?
	`
	result, err := r.db.conn.Exec(query,
		c.Name, c.Version, c.Token, string(capabilitiesJSON), string(configJSON),
		c.LastSeenAt, c.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update client: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("client not found: %s", c.ID)
	}

	return nil
}

// UpdateConfig updates only the client config
func (r *ClientRepository) UpdateConfig(id string, config ClientConfig) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	query := `UPDATE clients SET config = ? WHERE id = ?`
	result, err := r.db.conn.Exec(query, string(configJSON), id)
	if err != nil {
		return fmt.Errorf("failed to update client config: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("client not found: %s", id)
	}

	return nil
}

// UpdateLastSeen updates the last_seen_at timestamp for a client
func (r *ClientRepository) UpdateLastSeen(id string) error {
	query := `UPDATE clients SET last_seen_at = ? WHERE id = ?`
	now := time.Now()
	result, err := r.db.conn.Exec(query, now, id)
	if err != nil {
		return fmt.Errorf("failed to update last_seen_at: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("client not found: %s", id)
	}

	return nil
}

// Delete removes a client by ID
func (r *ClientRepository) Delete(id string) error {
	query := `DELETE FROM clients WHERE id = ?`
	result, err := r.db.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete client: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("client not found: %s", id)
	}

	return nil
}

// Exists checks if a client exists by ID
func (r *ClientRepository) Exists(id string) (bool, error) {
	query := `SELECT 1 FROM clients WHERE id = ? LIMIT 1`
	var exists int
	err := r.db.conn.QueryRow(query, id).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check client existence: %w", err)
	}
	return true, nil
}

// TokenExists checks if a token is already in use
func (r *ClientRepository) TokenExists(token string) (bool, error) {
	query := `SELECT 1 FROM clients WHERE token = ? LIMIT 1`
	var exists int
	err := r.db.conn.QueryRow(query, token).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check token existence: %w", err)
	}
	return true, nil
}

// Count returns the total number of registered clients
func (r *ClientRepository) Count() (int, error) {
	query := `SELECT COUNT(*) FROM clients`
	var count int
	err := r.db.conn.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count clients: %w", err)
	}
	return count, nil
}
