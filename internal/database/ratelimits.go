package database

import (
	"database/sql"
	"fmt"
	"time"
)

// ClientRateLimitRepository provides CRUD operations for client rate limits
type ClientRateLimitRepository struct {
	db *DB
}

// NewClientRateLimitRepository creates a new ClientRateLimitRepository
func NewClientRateLimitRepository(db *DB) *ClientRateLimitRepository {
	return &ClientRateLimitRepository{db: db}
}

// Create inserts a new client rate limit configuration
func (r *ClientRateLimitRepository) Create(rl *ClientRateLimit) error {
	query := `
		INSERT INTO client_rate_limits (client_id, rpm_limit, tpm_limit, daily_limit, current_rpm, current_tpm, current_daily, last_reset)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.conn.Exec(query,
		rl.ClientID, rl.RPMLimit, rl.TPMLimit, rl.DailyLimit,
		rl.CurrentRPM, rl.CurrentTPM, rl.CurrentDaily, rl.LastReset,
	)
	if err != nil {
		return fmt.Errorf("failed to create client rate limit: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	rl.ID = int(id)
	return nil
}

// Get retrieves a client rate limit by client ID
func (r *ClientRateLimitRepository) Get(clientID string) (*ClientRateLimit, error) {
	query := `
		SELECT id, client_id, rpm_limit, tpm_limit, daily_limit, current_rpm, current_tpm, current_daily, last_reset
		FROM client_rate_limits WHERE client_id = ?
	`
	rl := &ClientRateLimit{}
	err := r.db.conn.QueryRow(query, clientID).Scan(
		&rl.ID, &rl.ClientID, &rl.RPMLimit, &rl.TPMLimit, &rl.DailyLimit,
		&rl.CurrentRPM, &rl.CurrentTPM, &rl.CurrentDaily, &rl.LastReset,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get client rate limit: %w", err)
	}
	return rl, nil
}

// GetByID retrieves a client rate limit by its ID
func (r *ClientRateLimitRepository) GetByID(id int) (*ClientRateLimit, error) {
	query := `
		SELECT id, client_id, rpm_limit, tpm_limit, daily_limit, current_rpm, current_tpm, current_daily, last_reset
		FROM client_rate_limits WHERE id = ?
	`
	rl := &ClientRateLimit{}
	err := r.db.conn.QueryRow(query, id).Scan(
		&rl.ID, &rl.ClientID, &rl.RPMLimit, &rl.TPMLimit, &rl.DailyLimit,
		&rl.CurrentRPM, &rl.CurrentTPM, &rl.CurrentDaily, &rl.LastReset,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get client rate limit by ID: %w", err)
	}
	return rl, nil
}

// List retrieves all client rate limits
func (r *ClientRateLimitRepository) List() ([]*ClientRateLimit, error) {
	query := `
		SELECT id, client_id, rpm_limit, tpm_limit, daily_limit, current_rpm, current_tpm, current_daily, last_reset
		FROM client_rate_limits ORDER BY client_id
	`
	rows, err := r.db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list client rate limits: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var limits []*ClientRateLimit
	for rows.Next() {
		rl := &ClientRateLimit{}
		err := rows.Scan(
			&rl.ID, &rl.ClientID, &rl.RPMLimit, &rl.TPMLimit, &rl.DailyLimit,
			&rl.CurrentRPM, &rl.CurrentTPM, &rl.CurrentDaily, &rl.LastReset,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan client rate limit: %w", err)
		}
		limits = append(limits, rl)
	}
	return limits, rows.Err()
}

// Update updates a client rate limit configuration
func (r *ClientRateLimitRepository) Update(rl *ClientRateLimit) error {
	query := `
		UPDATE client_rate_limits
		SET rpm_limit = ?, tpm_limit = ?, daily_limit = ?, current_rpm = ?, current_tpm = ?, current_daily = ?, last_reset = ?
		WHERE client_id = ?
	`
	result, err := r.db.conn.Exec(query,
		rl.RPMLimit, rl.TPMLimit, rl.DailyLimit,
		rl.CurrentRPM, rl.CurrentTPM, rl.CurrentDaily, rl.LastReset,
		rl.ClientID,
	)
	if err != nil {
		return fmt.Errorf("failed to update client rate limit: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("client rate limit not found for client: %s", rl.ClientID)
	}
	return nil
}

// UpdateLimits updates only the limit values (not current counters)
func (r *ClientRateLimitRepository) UpdateLimits(clientID string, rpmLimit, tpmLimit, dailyLimit *int) error {
	query := `
		UPDATE client_rate_limits
		SET rpm_limit = COALESCE(?, rpm_limit),
		    tpm_limit = COALESCE(?, tpm_limit),
		    daily_limit = COALESCE(?, daily_limit)
		WHERE client_id = ?
	`
	result, err := r.db.conn.Exec(query, rpmLimit, tpmLimit, dailyLimit, clientID)
	if err != nil {
		return fmt.Errorf("failed to update client rate limits: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("client rate limit not found for client: %s", clientID)
	}
	return nil
}

// IncrementUsage increments the current usage counters for a client
func (r *ClientRateLimitRepository) IncrementUsage(clientID string, requests, tokens int) error {
	query := `
		UPDATE client_rate_limits
		SET current_rpm = current_rpm + ?,
		    current_tpm = current_tpm + ?,
		    current_daily = current_daily + ?
		WHERE client_id = ?
	`
	_, err := r.db.conn.Exec(query, requests, tokens, requests, clientID)
	if err != nil {
		return fmt.Errorf("failed to increment usage: %w", err)
	}
	return nil
}

// ResetMinuteCounters resets RPM and TPM counters for all clients (called every minute)
func (r *ClientRateLimitRepository) ResetMinuteCounters() error {
	query := `
		UPDATE client_rate_limits
		SET current_rpm = 0, current_tpm = 0, last_reset = ?
	`
	_, err := r.db.conn.Exec(query, time.Now())
	if err != nil {
		return fmt.Errorf("failed to reset minute counters: %w", err)
	}
	return nil
}

// ResetDailyCounters resets daily counters for all clients (called daily)
func (r *ClientRateLimitRepository) ResetDailyCounters() error {
	query := `
		UPDATE client_rate_limits
		SET current_daily = 0, last_reset = ?
	`
	_, err := r.db.conn.Exec(query, time.Now())
	if err != nil {
		return fmt.Errorf("failed to reset daily counters: %w", err)
	}
	return nil
}

// Delete removes a client rate limit by client ID
func (r *ClientRateLimitRepository) Delete(clientID string) error {
	query := `DELETE FROM client_rate_limits WHERE client_id = ?`
	result, err := r.db.conn.Exec(query, clientID)
	if err != nil {
		return fmt.Errorf("failed to delete client rate limit: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("client rate limit not found for client: %s", clientID)
	}
	return nil
}

// Exists checks if a rate limit exists for a client
func (r *ClientRateLimitRepository) Exists(clientID string) (bool, error) {
	query := `SELECT 1 FROM client_rate_limits WHERE client_id = ? LIMIT 1`
	var exists int
	err := r.db.conn.QueryRow(query, clientID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check rate limit existence: %w", err)
	}
	return true, nil
}

// CheckLimits checks if a client is within their rate limits
// Returns (withinLimits, limitType, error)
// limitType can be "rpm", "tpm", "daily", or "" if within limits
func (r *ClientRateLimitRepository) CheckLimits(clientID string) (bool, string, error) {
	rl, err := r.Get(clientID)
	if err != nil {
		return false, "", err
	}
	if rl == nil {
		// No rate limit configured, allow request
		return true, "", nil
	}

	// Check RPM limit
	if rl.RPMLimit != nil && rl.CurrentRPM >= *rl.RPMLimit {
		return false, "rpm", nil
	}

	// Check TPM limit
	if rl.TPMLimit != nil && rl.CurrentTPM >= *rl.TPMLimit {
		return false, "tpm", nil
	}

	// Check daily limit
	if rl.DailyLimit != nil && rl.CurrentDaily >= *rl.DailyLimit {
		return false, "daily", nil
	}

	return true, "", nil
}

// GetOrCreate gets an existing rate limit or creates a new one with defaults
func (r *ClientRateLimitRepository) GetOrCreate(clientID string) (*ClientRateLimit, error) {
	rl, err := r.Get(clientID)
	if err != nil {
		return nil, err
	}
	if rl != nil {
		return rl, nil
	}

	// Create new rate limit with no limits (unlimited)
	rl = &ClientRateLimit{
		ClientID:     clientID,
		RPMLimit:     nil,
		TPMLimit:     nil,
		DailyLimit:   nil,
		CurrentRPM:   0,
		CurrentTPM:   0,
		CurrentDaily: 0,
		LastReset:    time.Now(),
	}
	if err := r.Create(rl); err != nil {
		return nil, err
	}
	return rl, nil
}
