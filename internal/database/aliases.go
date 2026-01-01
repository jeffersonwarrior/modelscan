package database

import (
	"database/sql"
	"fmt"
	"time"
)

// Alias represents a model alias in the database
type Alias struct {
	Name      string
	ModelID   string
	ClientID  *string // nil = global alias
	CreatedAt time.Time
}

// DefaultAliases contains the default global aliases
var DefaultAliases = []Alias{
	{Name: "sonnet", ModelID: "claude-sonnet-4-5-20250929"},
	{Name: "opus", ModelID: "claude-opus-4-5-20250929"},
	{Name: "haiku", ModelID: "claude-3-5-haiku-20241022"},
	{Name: "gpt4", ModelID: "gpt-4o"},
	{Name: "gemini", ModelID: "gemini-1.5-pro"},
}

// CreateAlias creates a new alias
func (db *DB) CreateAlias(alias *Alias) error {
	query := `
		INSERT INTO aliases (name, model_id, client_id)
		VALUES (?, ?, ?)
	`
	_, err := db.conn.Exec(query, alias.Name, alias.ModelID, alias.ClientID)
	if err != nil {
		return fmt.Errorf("failed to create alias: %w", err)
	}
	return nil
}

// GetAlias retrieves an alias by name and optional client ID
// If clientID is nil, returns the global alias
func (db *DB) GetAlias(name string, clientID *string) (*Alias, error) {
	var query string
	var args []interface{}

	if clientID == nil {
		query = `SELECT name, model_id, client_id, created_at FROM aliases WHERE name = ? AND client_id IS NULL`
		args = []interface{}{name}
	} else {
		query = `SELECT name, model_id, client_id, created_at FROM aliases WHERE name = ? AND client_id = ?`
		args = []interface{}{name, *clientID}
	}

	alias := &Alias{}
	err := db.conn.QueryRow(query, args...).Scan(&alias.Name, &alias.ModelID, &alias.ClientID, &alias.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get alias: %w", err)
	}
	return alias, nil
}

// ListAliases lists all aliases, optionally filtered by client ID
// If clientID is nil, returns only global aliases
// If clientID is provided, returns both client-specific and global aliases
func (db *DB) ListAliases(clientID *string) ([]*Alias, error) {
	var query string
	var args []interface{}

	if clientID == nil {
		// Return only global aliases
		query = `SELECT name, model_id, client_id, created_at FROM aliases WHERE client_id IS NULL ORDER BY name`
	} else {
		// Return both client-specific and global aliases, with client-specific taking precedence
		query = `
			SELECT name, model_id, client_id, created_at FROM aliases
			WHERE client_id = ? OR client_id IS NULL
			ORDER BY name, client_id DESC
		`
		args = []interface{}{*clientID}
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list aliases: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var aliases []*Alias
	for rows.Next() {
		alias := &Alias{}
		if err := rows.Scan(&alias.Name, &alias.ModelID, &alias.ClientID, &alias.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan alias: %w", err)
		}
		aliases = append(aliases, alias)
	}
	return aliases, rows.Err()
}

// ListAllAliases returns all aliases in the database
func (db *DB) ListAllAliases() ([]*Alias, error) {
	query := `SELECT name, model_id, client_id, created_at FROM aliases ORDER BY name, client_id`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list all aliases: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var aliases []*Alias
	for rows.Next() {
		alias := &Alias{}
		if err := rows.Scan(&alias.Name, &alias.ModelID, &alias.ClientID, &alias.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan alias: %w", err)
		}
		aliases = append(aliases, alias)
	}
	return aliases, rows.Err()
}

// DeleteAlias deletes an alias by name and optional client ID
func (db *DB) DeleteAlias(name string, clientID *string) error {
	var query string
	var args []interface{}

	if clientID == nil {
		query = `DELETE FROM aliases WHERE name = ? AND client_id IS NULL`
		args = []interface{}{name}
	} else {
		query = `DELETE FROM aliases WHERE name = ? AND client_id = ?`
		args = []interface{}{name, *clientID}
	}

	result, err := db.conn.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete alias: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("alias not found: %s", name)
	}
	return nil
}

// ResolveAlias resolves an alias to a model ID
// First checks for client-specific alias, then falls back to global
// Returns the input name unchanged if no alias exists
func (db *DB) ResolveAlias(name string, clientID *string) (string, error) {
	// First try client-specific alias if clientID is provided
	if clientID != nil {
		alias, err := db.GetAlias(name, clientID)
		if err != nil {
			return "", err
		}
		if alias != nil {
			return alias.ModelID, nil
		}
	}

	// Fall back to global alias
	alias, err := db.GetAlias(name, nil)
	if err != nil {
		return "", err
	}
	if alias != nil {
		return alias.ModelID, nil
	}

	// No alias found, return the input unchanged
	return name, nil
}

// UpdateAlias updates an existing alias
func (db *DB) UpdateAlias(name string, clientID *string, newModelID string) error {
	var query string
	var args []interface{}

	if clientID == nil {
		query = `UPDATE aliases SET model_id = ? WHERE name = ? AND client_id IS NULL`
		args = []interface{}{newModelID, name}
	} else {
		query = `UPDATE aliases SET model_id = ? WHERE name = ? AND client_id = ?`
		args = []interface{}{newModelID, name, *clientID}
	}

	result, err := db.conn.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update alias: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("alias not found: %s", name)
	}
	return nil
}

// SeedDefaultAliases inserts the default global aliases if they don't exist
func (db *DB) SeedDefaultAliases() error {
	for _, alias := range DefaultAliases {
		existing, err := db.GetAlias(alias.Name, nil)
		if err != nil {
			return fmt.Errorf("failed to check existing alias %s: %w", alias.Name, err)
		}
		if existing == nil {
			if err := db.CreateAlias(&alias); err != nil {
				return fmt.Errorf("failed to seed alias %s: %w", alias.Name, err)
			}
		}
	}
	return nil
}
