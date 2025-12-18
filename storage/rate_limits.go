package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var rateLimitDB *sql.DB

// RateLimit represents a rate limit configuration for a provider
type RateLimit struct {
	ID                 int64
	ProviderName       string
	PlanType           string // free, pay_per_go, pro, enterprise
	LimitType          string // rpm, tpm, rph, rpd, concurrent
	LimitValue         int64
	BurstAllowance     int64
	ResetWindowSeconds int64
	AppliesTo          string         // account, model, endpoint
	ModelID            sql.NullString // if applies_to=model
	EndpointPath       sql.NullString // if applies_to=endpoint
	SourceURL          string
	LastVerified       time.Time
}

// PlanMetadata stores metadata about provider pricing plans
type PlanMetadata struct {
	ProviderName     string
	PlanType         string
	OfficialName     string
	CostPerMonth     sql.NullFloat64
	HasFreeTier      bool
	DocumentationURL string
}

// ProviderPricing stores pricing information per model and plan
type ProviderPricing struct {
	ProviderName  string
	ModelID       string
	PlanType      string
	InputCost     float64
	OutputCost    float64
	UnitType      string // "1M tokens", "per character", "per second"
	Currency      string
	IncludedUnits sql.NullInt64 // free tier included units
}

// PricingHistory tracks pricing changes over time
type PricingHistory struct {
	ID            int64
	ProviderName  string
	ModelID       string
	PlanType      string
	OldInputCost  float64
	OldOutputCost float64
	NewInputCost  float64
	NewOutputCost float64
	ChangeDate    time.Time
	ChangeReason  string
}

// InitRateLimitDB initializes the rate limit database with WAL mode
func InitRateLimitDB(dbPath string) error {
	var err error
	rateLimitDB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open rate limit database: %w", err)
	}

	// Enable WAL mode for concurrent reads during writes
	if _, err := rateLimitDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := rateLimitDB.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	if err := createRateLimitTables(); err != nil {
		return fmt.Errorf("failed to create rate limit tables: %w", err)
	}

	return nil
}

// CloseRateLimitDB closes the rate limit database connection
func CloseRateLimitDB() error {
	if rateLimitDB != nil {
		return rateLimitDB.Close()
	}
	return nil
}

// GetRateLimitDB returns the rate limit database connection
func GetRateLimitDB() *sql.DB {
	return rateLimitDB
}

// createRateLimitTables creates all rate limit related tables
func createRateLimitTables() error {
	queries := []string{
		// Rate limits table
		`CREATE TABLE IF NOT EXISTS rate_limits (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider_name TEXT NOT NULL,
			plan_type TEXT NOT NULL,
			limit_type TEXT NOT NULL,
			limit_value INTEGER NOT NULL,
			burst_allowance INTEGER DEFAULT 0,
			reset_window_seconds INTEGER NOT NULL,
			applies_to TEXT NOT NULL,
			model_id TEXT DEFAULT '',
			endpoint_path TEXT DEFAULT '',
			source_url TEXT,
			last_verified DATETIME NOT NULL,
			UNIQUE(provider_name, plan_type, limit_type, model_id, endpoint_path)
		)`,

		// Plan metadata table
		`CREATE TABLE IF NOT EXISTS plan_metadata (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider_name TEXT NOT NULL,
			plan_type TEXT NOT NULL,
			official_name TEXT NOT NULL,
			cost_per_month REAL,
			has_free_tier BOOLEAN DEFAULT 0,
			documentation_url TEXT,
			UNIQUE(provider_name, plan_type)
		)`,

		// Provider pricing table
		`CREATE TABLE IF NOT EXISTS provider_pricing (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider_name TEXT NOT NULL,
			model_id TEXT NOT NULL,
			plan_type TEXT NOT NULL,
			input_cost REAL NOT NULL,
			output_cost REAL NOT NULL,
			unit_type TEXT NOT NULL,
			currency TEXT NOT NULL DEFAULT 'USD',
			included_units INTEGER,
			UNIQUE(provider_name, model_id, plan_type)
		)`,

		// Pricing history table (for auditing)
		`CREATE TABLE IF NOT EXISTS pricing_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider_name TEXT NOT NULL,
			model_id TEXT NOT NULL,
			plan_type TEXT NOT NULL,
			old_input_cost REAL,
			old_output_cost REAL,
			new_input_cost REAL NOT NULL,
			new_output_cost REAL NOT NULL,
			change_date DATETIME NOT NULL,
			change_reason TEXT
		)`,

		// Indexes for performance
		`CREATE INDEX IF NOT EXISTS idx_rate_limits_provider_plan 
		 ON rate_limits(provider_name, plan_type, limit_type)`,

		`CREATE INDEX IF NOT EXISTS idx_rate_limits_model 
		 ON rate_limits(model_id) WHERE model_id IS NOT NULL`,

		`CREATE INDEX IF NOT EXISTS idx_plan_metadata_provider 
		 ON plan_metadata(provider_name)`,

		`CREATE INDEX IF NOT EXISTS idx_provider_pricing_model 
		 ON provider_pricing(provider_name, model_id)`,

		`CREATE INDEX IF NOT EXISTS idx_pricing_history_date 
		 ON pricing_history(change_date DESC)`,
	}

	for _, query := range queries {
		if _, err := rateLimitDB.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	return nil
}

// InsertRateLimit inserts or updates a rate limit (upsert)
func InsertRateLimit(rl RateLimit) error {
	// Convert NULL values to empty strings for UNIQUE constraint
	modelID := ""
	if rl.ModelID.Valid {
		modelID = rl.ModelID.String
	}
	endpointPath := ""
	if rl.EndpointPath.Valid {
		endpointPath = rl.EndpointPath.String
	}

	query := `
		INSERT INTO rate_limits (
			provider_name, plan_type, limit_type, limit_value, 
			burst_allowance, reset_window_seconds, applies_to, 
			model_id, endpoint_path, source_url, last_verified
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider_name, plan_type, limit_type, model_id, endpoint_path)
		DO UPDATE SET
			limit_value = excluded.limit_value,
			burst_allowance = excluded.burst_allowance,
			reset_window_seconds = excluded.reset_window_seconds,
			applies_to = excluded.applies_to,
			source_url = excluded.source_url,
			last_verified = excluded.last_verified
	`

	_, err := rateLimitDB.Exec(query,
		rl.ProviderName, rl.PlanType, rl.LimitType, rl.LimitValue,
		rl.BurstAllowance, rl.ResetWindowSeconds, rl.AppliesTo,
		modelID, endpointPath, rl.SourceURL, rl.LastVerified,
	)

	return err
}

// QueryRateLimit retrieves rate limits matching the criteria
func QueryRateLimit(providerName, planType, limitType, modelID, endpointPath string) ([]RateLimit, error) {
	query := `
		SELECT id, provider_name, plan_type, limit_type, limit_value,
		       burst_allowance, reset_window_seconds, applies_to,
		       model_id, endpoint_path, source_url, last_verified
		FROM rate_limits
		WHERE provider_name = ? AND plan_type = ? AND limit_type = ?
	`
	args := []interface{}{providerName, planType, limitType}

	if modelID != "" {
		query += " AND (model_id = ? OR model_id = '')"
		args = append(args, modelID)
	}
	if endpointPath != "" {
		query += " AND (endpoint_path = ? OR endpoint_path = '')"
		args = append(args, endpointPath)
	}

	query += " ORDER BY applies_to DESC" // prioritize specific over general

	rows, err := rateLimitDB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []RateLimit
	for rows.Next() {
		var rl RateLimit
		var modelIDStr, endpointPathStr string
		err := rows.Scan(
			&rl.ID, &rl.ProviderName, &rl.PlanType, &rl.LimitType, &rl.LimitValue,
			&rl.BurstAllowance, &rl.ResetWindowSeconds, &rl.AppliesTo,
			&modelIDStr, &endpointPathStr, &rl.SourceURL, &rl.LastVerified,
		)
		if err != nil {
			return nil, err
		}
		if modelIDStr != "" {
			rl.ModelID = sql.NullString{String: modelIDStr, Valid: true}
		}
		if endpointPathStr != "" {
			rl.EndpointPath = sql.NullString{String: endpointPathStr, Valid: true}
		}
		results = append(results, rl)
	}

	return results, nil
}

// InsertPlanMetadata inserts or updates plan metadata
func InsertPlanMetadata(pm PlanMetadata) error {
	query := `
		INSERT INTO plan_metadata (
			provider_name, plan_type, official_name, cost_per_month,
			has_free_tier, documentation_url
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider_name, plan_type)
		DO UPDATE SET
			official_name = excluded.official_name,
			cost_per_month = excluded.cost_per_month,
			has_free_tier = excluded.has_free_tier,
			documentation_url = excluded.documentation_url
	`

	_, err := rateLimitDB.Exec(query,
		pm.ProviderName, pm.PlanType, pm.OfficialName, pm.CostPerMonth,
		pm.HasFreeTier, pm.DocumentationURL,
	)

	return err
}

// InsertProviderPricing inserts or updates provider pricing
func InsertProviderPricing(pp ProviderPricing) error {
	query := `
		INSERT INTO provider_pricing (
			provider_name, model_id, plan_type, input_cost, output_cost,
			unit_type, currency, included_units
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider_name, model_id, plan_type)
		DO UPDATE SET
			input_cost = excluded.input_cost,
			output_cost = excluded.output_cost,
			unit_type = excluded.unit_type,
			currency = excluded.currency,
			included_units = excluded.included_units
	`

	_, err := rateLimitDB.Exec(query,
		pp.ProviderName, pp.ModelID, pp.PlanType, pp.InputCost, pp.OutputCost,
		pp.UnitType, pp.Currency, pp.IncludedUnits,
	)

	return err
}

// InsertPricingHistory records a pricing change
func InsertPricingHistory(ph PricingHistory) error {
	query := `
		INSERT INTO pricing_history (
			provider_name, model_id, plan_type, old_input_cost, old_output_cost,
			new_input_cost, new_output_cost, change_date, change_reason
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := rateLimitDB.Exec(query,
		ph.ProviderName, ph.ModelID, ph.PlanType, ph.OldInputCost, ph.OldOutputCost,
		ph.NewInputCost, ph.NewOutputCost, ph.ChangeDate, ph.ChangeReason,
	)

	return err
}

// GetProviderPricing retrieves pricing for a specific model and plan
func GetProviderPricing(providerName, modelID, planType string) (*ProviderPricing, error) {
	query := `
		SELECT provider_name, model_id, plan_type, input_cost, output_cost,
		       unit_type, currency, included_units
		FROM provider_pricing
		WHERE provider_name = ? AND model_id = ? AND plan_type = ?
	`

	var pp ProviderPricing
	err := rateLimitDB.QueryRow(query, providerName, modelID, planType).Scan(
		&pp.ProviderName, &pp.ModelID, &pp.PlanType, &pp.InputCost, &pp.OutputCost,
		&pp.UnitType, &pp.Currency, &pp.IncludedUnits,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &pp, nil
}

// GetAllRateLimitsForProvider retrieves all rate limits for a provider and plan
func GetAllRateLimitsForProvider(providerName, planType string) ([]RateLimit, error) {
	query := `
		SELECT id, provider_name, plan_type, limit_type, limit_value,
		       burst_allowance, reset_window_seconds, applies_to,
		       model_id, endpoint_path, source_url, last_verified
		FROM rate_limits
		WHERE provider_name = ? AND plan_type = ?
		ORDER BY limit_type, applies_to
	`

	rows, err := rateLimitDB.Query(query, providerName, planType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []RateLimit
	for rows.Next() {
		var rl RateLimit
		var modelIDStr, endpointPathStr string
		err := rows.Scan(
			&rl.ID, &rl.ProviderName, &rl.PlanType, &rl.LimitType, &rl.LimitValue,
			&rl.BurstAllowance, &rl.ResetWindowSeconds, &rl.AppliesTo,
			&modelIDStr, &endpointPathStr, &rl.SourceURL, &rl.LastVerified,
		)
		if err != nil {
			return nil, err
		}
		if modelIDStr != "" {
			rl.ModelID = sql.NullString{String: modelIDStr, Valid: true}
		}
		if endpointPathStr != "" {
			rl.EndpointPath = sql.NullString{String: endpointPathStr, Valid: true}
		}
		results = append(results, rl)
	}

	return results, nil
}
