package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// RemapRule represents a model remapping rule in the database
type RemapRule struct {
	ID         int
	ClientID   string
	FromModel  string // Supports glob patterns like "claude-*"
	ToModel    string
	ToProvider string
	Priority   int
	Enabled    bool
	CreatedAt  time.Time
}

// RemapRuleRepository provides CRUD operations for remap rules
type RemapRuleRepository struct {
	db *DB
}

// NewRemapRuleRepository creates a new RemapRuleRepository
func NewRemapRuleRepository(db *DB) *RemapRuleRepository {
	return &RemapRuleRepository{db: db}
}

// Create inserts a new remap rule
func (r *RemapRuleRepository) Create(rule *RemapRule) error {
	query := `
		INSERT INTO remap_rules (client_id, from_model, to_model, to_provider, priority, enabled)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.conn.Exec(query,
		rule.ClientID, rule.FromModel, rule.ToModel, rule.ToProvider, rule.Priority, rule.Enabled,
	)
	if err != nil {
		return fmt.Errorf("failed to create remap rule: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	rule.ID = int(id)

	return nil
}

// Get retrieves a remap rule by ID
func (r *RemapRuleRepository) Get(id int) (*RemapRule, error) {
	query := `
		SELECT id, client_id, from_model, to_model, to_provider, priority, enabled, created_at
		FROM remap_rules WHERE id = ?
	`
	rule := &RemapRule{}
	err := r.db.conn.QueryRow(query, id).Scan(
		&rule.ID, &rule.ClientID, &rule.FromModel, &rule.ToModel,
		&rule.ToProvider, &rule.Priority, &rule.Enabled, &rule.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get remap rule: %w", err)
	}
	return rule, nil
}

// List retrieves all remap rules, optionally filtered by client ID
func (r *RemapRuleRepository) List(clientID *string) ([]*RemapRule, error) {
	var query string
	var args []interface{}

	if clientID == nil {
		query = `
			SELECT id, client_id, from_model, to_model, to_provider, priority, enabled, created_at
			FROM remap_rules ORDER BY priority DESC, id ASC
		`
	} else {
		query = `
			SELECT id, client_id, from_model, to_model, to_provider, priority, enabled, created_at
			FROM remap_rules WHERE client_id = ?
			ORDER BY priority DESC, id ASC
		`
		args = []interface{}{*clientID}
	}

	rows, err := r.db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list remap rules: %w", err)
	}
	defer rows.Close()

	var rules []*RemapRule
	for rows.Next() {
		rule := &RemapRule{}
		err := rows.Scan(
			&rule.ID, &rule.ClientID, &rule.FromModel, &rule.ToModel,
			&rule.ToProvider, &rule.Priority, &rule.Enabled, &rule.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan remap rule: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// ListByClientID retrieves all remap rules for a specific client
func (r *RemapRuleRepository) ListByClientID(clientID string) ([]*RemapRule, error) {
	return r.List(&clientID)
}

// Update updates an existing remap rule
func (r *RemapRuleRepository) Update(rule *RemapRule) error {
	query := `
		UPDATE remap_rules
		SET client_id = ?, from_model = ?, to_model = ?, to_provider = ?, priority = ?, enabled = ?
		WHERE id = ?
	`
	result, err := r.db.conn.Exec(query,
		rule.ClientID, rule.FromModel, rule.ToModel, rule.ToProvider,
		rule.Priority, rule.Enabled, rule.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update remap rule: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("remap rule not found: %d", rule.ID)
	}

	return nil
}

// Delete removes a remap rule by ID
func (r *RemapRuleRepository) Delete(id int) error {
	query := `DELETE FROM remap_rules WHERE id = ?`
	result, err := r.db.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete remap rule: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("remap rule not found: %d", id)
	}

	return nil
}

// FindMatching finds the highest-priority enabled remap rule that matches the given model
// for the specified client. Returns nil if no matching rule is found.
func (r *RemapRuleRepository) FindMatching(model string, clientID string) (*RemapRule, error) {
	// Fetch all enabled rules for this client, ordered by priority descending
	query := `
		SELECT id, client_id, from_model, to_model, to_provider, priority, enabled, created_at
		FROM remap_rules
		WHERE client_id = ? AND enabled = 1
		ORDER BY priority DESC, id ASC
	`
	rows, err := r.db.conn.Query(query, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to query remap rules: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		rule := &RemapRule{}
		err := rows.Scan(
			&rule.ID, &rule.ClientID, &rule.FromModel, &rule.ToModel,
			&rule.ToProvider, &rule.Priority, &rule.Enabled, &rule.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan remap rule: %w", err)
		}

		if matchGlob(rule.FromModel, model) {
			return rule, nil
		}
	}

	return nil, rows.Err()
}

// matchGlob performs glob pattern matching with support for:
// - '*' matches any sequence of characters
// - '?' matches any single character
// - Exact string matching when no wildcards present
func matchGlob(pattern, s string) bool {
	// Handle simple cases
	if pattern == "*" {
		return true
	}
	if pattern == s {
		return true
	}
	if !strings.ContainsAny(pattern, "*?") {
		return pattern == s
	}

	return matchGlobRecursive(pattern, s)
}

// matchGlobRecursive implements glob matching using dynamic programming approach
func matchGlobRecursive(pattern, s string) bool {
	// Use indices for efficient string traversal
	pi := 0 // pattern index
	si := 0 // string index
	starIdx := -1
	matchIdx := 0

	for si < len(s) {
		if pi < len(pattern) && (pattern[pi] == '?' || pattern[pi] == s[si]) {
			// Current characters match, or pattern has '?'
			pi++
			si++
		} else if pi < len(pattern) && pattern[pi] == '*' {
			// Star found, record position
			starIdx = pi
			matchIdx = si
			pi++
		} else if starIdx != -1 {
			// Mismatch, but we have a star to fall back to
			pi = starIdx + 1
			matchIdx++
			si = matchIdx
		} else {
			// No match
			return false
		}
	}

	// Check remaining pattern characters - must all be stars
	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}

	return pi == len(pattern)
}

// SetEnabled enables or disables a remap rule
func (r *RemapRuleRepository) SetEnabled(id int, enabled bool) error {
	query := `UPDATE remap_rules SET enabled = ? WHERE id = ?`
	result, err := r.db.conn.Exec(query, enabled, id)
	if err != nil {
		return fmt.Errorf("failed to set remap rule enabled: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("remap rule not found: %d", id)
	}

	return nil
}

// Exists checks if a remap rule exists by ID
func (r *RemapRuleRepository) Exists(id int) (bool, error) {
	query := `SELECT 1 FROM remap_rules WHERE id = ? LIMIT 1`
	var exists int
	err := r.db.conn.QueryRow(query, id).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check remap rule existence: %w", err)
	}
	return true, nil
}
