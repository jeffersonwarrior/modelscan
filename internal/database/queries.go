package database

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// CreateProvider inserts a new provider
func (db *DB) CreateProvider(p *Provider) error {
	query := `
		INSERT INTO providers (
			id, name, base_url, auth_method, auth_header,
			pricing_model, subscription_tiers, sdk_path, sdk_hash, sdk_version, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.conn.Exec(query,
		p.ID, p.Name, p.BaseURL, p.AuthMethod, p.AuthHeader,
		p.PricingModel, p.SubscriptionTiers, p.SDKPath, p.SDKHash, p.SDKVersion, p.Status,
	)
	return err
}

// GetProvider retrieves a provider by ID
func (db *DB) GetProvider(id string) (*Provider, error) {
	query := `SELECT * FROM providers WHERE id = ?`
	p := &Provider{}
	err := db.conn.QueryRow(query, id).Scan(
		&p.ID, &p.Name, &p.BaseURL, &p.AuthMethod, &p.AuthHeader,
		&p.PricingModel, &p.SubscriptionTiers, &p.DiscoveredAt, &p.LastValidated,
		&p.SDKPath, &p.SDKHash, &p.SDKVersion, &p.Status, &p.LastError,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

// ListProviders lists all providers
func (db *DB) ListProviders() ([]*Provider, error) {
	query := `SELECT * FROM providers ORDER BY name`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []*Provider
	for rows.Next() {
		p := &Provider{}
		err := rows.Scan(
			&p.ID, &p.Name, &p.BaseURL, &p.AuthMethod, &p.AuthHeader,
			&p.PricingModel, &p.SubscriptionTiers, &p.DiscoveredAt, &p.LastValidated,
			&p.SDKPath, &p.SDKHash, &p.SDKVersion, &p.Status, &p.LastError,
		)
		if err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	return providers, rows.Err()
}

// UpdateProviderStatus updates provider status
func (db *DB) UpdateProviderStatus(id, status string, lastError *string) error {
	query := `UPDATE providers SET status = ?, last_error = ?, last_validated = ? WHERE id = ?`
	_, err := db.conn.Exec(query, status, lastError, time.Now(), id)
	return err
}

// CreateModelFamily inserts a new model family
func (db *DB) CreateModelFamily(f *ModelFamily) error {
	query := `INSERT INTO model_families (id, provider_id, name, description) VALUES (?, ?, ?, ?)`
	_, err := db.conn.Exec(query, f.ID, f.ProviderID, f.Name, f.Description)
	return err
}

// CreateModel inserts a new model
func (db *DB) CreateModel(m *Model) error {
	query := `
		INSERT INTO models (
			id, family_id, name, cost_per_1m_in, cost_per_1m_out, cost_per_1m_reasoning,
			context_window, max_tokens, capabilities, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.conn.Exec(query,
		m.ID, m.FamilyID, m.Name, m.CostPer1MIn, m.CostPer1MOut, m.CostPer1MReasoning,
		m.ContextWindow, m.MaxTokens, m.Capabilities, m.Status,
	)
	return err
}

// ListModels lists all models
func (db *DB) ListModels() ([]*Model, error) {
	query := `SELECT * FROM models ORDER BY name`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var models []*Model
	for rows.Next() {
		m := &Model{}
		err := rows.Scan(
			&m.ID, &m.FamilyID, &m.Name, &m.CostPer1MIn, &m.CostPer1MOut, &m.CostPer1MReasoning,
			&m.ContextWindow, &m.MaxTokens, &m.Capabilities, &m.Status, &m.LastTested, &m.LastError,
		)
		if err != nil {
			return nil, err
		}
		models = append(models, m)
	}
	return models, rows.Err()
}

// ListModelsByStatus lists models by status
func (db *DB) ListModelsByStatus(status string) ([]*Model, error) {
	query := `SELECT * FROM models WHERE status = ? ORDER BY name`
	rows, err := db.conn.Query(query, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var models []*Model
	for rows.Next() {
		m := &Model{}
		err := rows.Scan(
			&m.ID, &m.FamilyID, &m.Name, &m.CostPer1MIn, &m.CostPer1MOut, &m.CostPer1MReasoning,
			&m.ContextWindow, &m.MaxTokens, &m.Capabilities, &m.Status, &m.LastTested, &m.LastError,
		)
		if err != nil {
			return nil, err
		}
		models = append(models, m)
	}
	return models, rows.Err()
}

// UpdateModelStatus updates model status
func (db *DB) UpdateModelStatus(id, status string, lastError *string) error {
	query := `UPDATE models SET status = ?, last_error = ?, last_tested = ? WHERE id = ?`
	_, err := db.conn.Exec(query, status, lastError, time.Now(), id)
	return err
}

// HashAPIKey creates a SHA256 hash of an API key
func HashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// CreateAPIKey inserts a new API key
func (db *DB) CreateAPIKey(providerID, apiKey string) (*APIKey, error) {
	keyHash := HashAPIKey(apiKey)
	var keyPrefix *string
	if len(apiKey) >= 10 {
		prefix := apiKey[:10] + "..."
		keyPrefix = &prefix
	}

	query := `
		INSERT INTO api_keys (provider_id, key_hash, key_prefix)
		VALUES (?, ?, ?)
		RETURNING id
	`
	var id int
	err := db.conn.QueryRow(query, providerID, keyHash, keyPrefix).Scan(&id)
	if err != nil {
		return nil, err
	}

	return db.GetAPIKey(id)
}

// GetAPIKey retrieves an API key by ID
func (db *DB) GetAPIKey(id int) (*APIKey, error) {
	query := `SELECT * FROM api_keys WHERE id = ?`
	k := &APIKey{}
	err := db.conn.QueryRow(query, id).Scan(
		&k.ID, &k.ProviderID, &k.KeyHash, &k.KeyPrefix, &k.Tier,
		&k.RPMLimit, &k.TPMLimit, &k.DailyLimit, &k.ResetInterval,
		&k.LastReset, &k.RequestsCount, &k.TokensCount,
		&k.Active, &k.Degraded, &k.DegradedUntil, &k.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return k, err
}

// ListActiveAPIKeys lists active, non-degraded API keys for a provider
func (db *DB) ListActiveAPIKeys(providerID string) ([]*APIKey, error) {
	query := `
		SELECT * FROM api_keys
		WHERE provider_id = ? AND active = 1 AND degraded = 0
		ORDER BY requests_count ASC, tokens_count ASC
	`
	rows, err := db.conn.Query(query, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*APIKey
	for rows.Next() {
		k := &APIKey{}
		err := rows.Scan(
			&k.ID, &k.ProviderID, &k.KeyHash, &k.KeyPrefix, &k.Tier,
			&k.RPMLimit, &k.TPMLimit, &k.DailyLimit, &k.ResetInterval,
			&k.LastReset, &k.RequestsCount, &k.TokensCount,
			&k.Active, &k.Degraded, &k.DegradedUntil, &k.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// IncrementKeyUsage increments request and token counts for an API key
func (db *DB) IncrementKeyUsage(keyID int, tokens int) error {
	query := `
		UPDATE api_keys
		SET requests_count = requests_count + 1,
		    tokens_count = tokens_count + ?
		WHERE id = ?
	`
	_, err := db.conn.Exec(query, tokens, keyID)
	return err
}

// MarkKeyDegraded marks an API key as degraded until a specific time
func (db *DB) MarkKeyDegraded(keyID int, until time.Time) error {
	query := `UPDATE api_keys SET degraded = 1, degraded_until = ? WHERE id = ?`
	_, err := db.conn.Exec(query, until, keyID)
	return err
}

// ResetKeyLimits resets request and token counts for an API key
func (db *DB) ResetKeyLimits(keyID int) error {
	query := `
		UPDATE api_keys
		SET requests_count = 0,
		    tokens_count = 0,
		    degraded = 0,
		    degraded_until = NULL,
		    last_reset = ?
		WHERE id = ?
	`
	_, err := db.conn.Exec(query, time.Now(), keyID)
	return err
}

// RecordUsage records a usage event
func (db *DB) RecordUsage(u *UsageRecord) error {
	query := `
		INSERT INTO usage_tracking (
			model_id, api_key_id, timestamp, tokens_in, tokens_out, tokens_reasoning,
			requests, cost, latency_ms, success, error
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.conn.Exec(query,
		u.ModelID, u.APIKeyID, u.Timestamp, u.TokensIn, u.TokensOut, u.TokensReasoning,
		u.Requests, u.Cost, u.LatencyMS, u.Success, u.Error,
	)
	return err
}

// CreateDiscoveryLog creates a new discovery log
func (db *DB) CreateDiscoveryLog(providerID, agentModel string) (int, error) {
	query := `
		INSERT INTO discovery_logs (provider_id, agent_model, status)
		VALUES (?, ?, 'started')
		RETURNING id
	`
	var id int
	err := db.conn.QueryRow(query, providerID, agentModel).Scan(&id)
	return id, err
}

// UpdateDiscoveryLog updates a discovery log
func (db *DB) UpdateDiscoveryLog(id int, status string, findings, sourcesScraped *string, totalCost float64, err *string) error {
	query := `
		UPDATE discovery_logs
		SET completed_at = ?,
		    status = ?,
		    findings = ?,
		    sources_scraped = ?,
		    total_cost = ?,
		    error = ?
		WHERE id = ?
	`
	_, execErr := db.conn.Exec(query, time.Now(), status, findings, sourcesScraped, totalCost, err, id)
	return execErr
}

// IncrementDiscoveryRetry increments the retry count for a discovery log
func (db *DB) IncrementDiscoveryRetry(id int) error {
	query := `UPDATE discovery_logs SET retry_count = retry_count + 1 WHERE id = ?`
	_, err := db.conn.Exec(query, id)
	return err
}

// CreateSDKVersion creates a new SDK version
func (db *DB) CreateSDKVersion(providerID, version, sdkPath string) error {
	query := `INSERT INTO sdk_versions (provider_id, version, sdk_path) VALUES (?, ?, ?)`
	_, err := db.conn.Exec(query, providerID, version, sdkPath)
	return err
}

// ListSDKVersions lists SDK versions for a provider
func (db *DB) ListSDKVersions(providerID string) ([]*SDKVersion, error) {
	query := `
		SELECT * FROM sdk_versions
		WHERE provider_id = ? AND deprecated_at IS NULL
		ORDER BY created_at DESC
	`
	rows, err := db.conn.Query(query, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []*SDKVersion
	for rows.Next() {
		v := &SDKVersion{}
		err := rows.Scan(&v.ID, &v.ProviderID, &v.Version, &v.SDKPath, &v.CreatedAt, &v.DeprecatedAt)
		if err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// DeprecateOldSDKVersions deprecates old SDK versions, keeping only the last N
func (db *DB) DeprecateOldSDKVersions(providerID string, keepLast int) error {
	query := `
		UPDATE sdk_versions
		SET deprecated_at = ?
		WHERE provider_id = ?
		  AND deprecated_at IS NULL
		  AND id NOT IN (
			SELECT id FROM sdk_versions
			WHERE provider_id = ? AND deprecated_at IS NULL
			ORDER BY created_at DESC
			LIMIT ?
		  )
	`
	_, err := db.conn.Exec(query, time.Now(), providerID, providerID, keepLast)
	return err
}

// GetSetting retrieves a setting value
func (db *DB) GetSetting(key string) (string, error) {
	query := `SELECT value FROM settings WHERE key = ?`
	var value string
	err := db.conn.QueryRow(query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetSetting sets a setting value
func (db *DB) SetSetting(key, value string) error {
	query := `
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = ?
	`
	_, err := db.conn.Exec(query, key, value, value, time.Now())
	return err
}

// GetUsageStats retrieves usage statistics for a time range
func (db *DB) GetUsageStats(modelID string, since time.Time) (map[string]interface{}, error) {
	query := `
		SELECT
			COUNT(*) as total_requests,
			SUM(tokens_in) as total_tokens_in,
			SUM(tokens_out) as total_tokens_out,
			SUM(cost) as total_cost,
			AVG(latency_ms) as avg_latency_ms,
			SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) as successful_requests
		FROM usage_tracking
		WHERE model_id = ? AND timestamp >= ?
	`

	var (
		totalRequests      int
		totalTokensIn      sql.NullInt64
		totalTokensOut     sql.NullInt64
		totalCost          sql.NullFloat64
		avgLatencyMS       sql.NullFloat64
		successfulRequests sql.NullInt64
	)

	err := db.conn.QueryRow(query, modelID, since).Scan(
		&totalRequests, &totalTokensIn, &totalTokensOut,
		&totalCost, &avgLatencyMS, &successfulRequests,
	)
	if err != nil {
		return nil, err
	}

	successRate := 0.0
	if totalRequests > 0 {
		successRate = float64(successfulRequests.Int64) / float64(totalRequests)
	}

	stats := map[string]interface{}{
		"total_requests":      totalRequests,
		"total_tokens_in":     int(totalTokensIn.Int64),
		"total_tokens_out":    int(totalTokensOut.Int64),
		"total_cost":          totalCost.Float64,
		"avg_latency_ms":      avgLatencyMS.Float64,
		"successful_requests": int(successfulRequests.Int64),
		"success_rate":        successRate,
	}

	return stats, nil
}

// SaveDiscoveryResult saves a discovery result to the database
func (db *DB) SaveDiscoveryResult(identifier string, result interface{}, ttl time.Duration) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal discovery result: %w", err)
	}

	var ttlExpires *time.Time
	if ttl > 0 {
		expires := time.Now().Add(ttl)
		ttlExpires = &expires
	}

	query := `
		INSERT INTO discovery_results (identifier, provider_data, ttl_expires_at)
		VALUES (?, ?, ?)
		ON CONFLICT(identifier) DO UPDATE SET
			provider_data = ?,
			ttl_expires_at = ?,
			discovered_at = CURRENT_TIMESTAMP
	`
	_, err = db.conn.Exec(query, identifier, resultJSON, ttlExpires, resultJSON, ttlExpires)
	return err
}

// GetDiscoveryResult retrieves a discovery result from the database
func (db *DB) GetDiscoveryResult(identifier string) (map[string]interface{}, bool, error) {
	query := `
		SELECT provider_data, ttl_expires_at
		FROM discovery_results
		WHERE identifier = ?
	`

	var resultJSON string
	var ttlExpires *time.Time
	err := db.conn.QueryRow(query, identifier).Scan(&resultJSON, &ttlExpires)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	// Check TTL
	if ttlExpires != nil && time.Now().After(*ttlExpires) {
		// Expired, delete and return not found
		_, _ = db.conn.Exec("DELETE FROM discovery_results WHERE identifier = ?", identifier)
		return nil, false, nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal discovery result: %w", err)
	}

	return result, true, nil
}

// DeleteExpiredDiscoveryResults removes expired cache entries
func (db *DB) DeleteExpiredDiscoveryResults() (int64, error) {
	result, err := db.conn.Exec("DELETE FROM discovery_results WHERE ttl_expires_at IS NOT NULL AND ttl_expires_at < ?", time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
