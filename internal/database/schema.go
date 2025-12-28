package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	CurrentSchemaVersion = 1
)

// DB wraps the SQLite database
type DB struct {
	conn *sql.DB
	path string
}

// Open opens or creates the SQLite database
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	db := &DB{
		conn: conn,
		path: path,
	}

	// Run migrations
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// migrate runs database migrations
func (db *DB) migrate() error {
	// Create schema_version table if not exists
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema_version table: %w", err)
	}

	// Check current version
	var currentVersion int
	err = db.conn.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	// Run migrations
	for version := currentVersion + 1; version <= CurrentSchemaVersion; version++ {
		if err := db.runMigration(version); err != nil {
			return fmt.Errorf("migration %d failed: %w", version, err)
		}
	}

	return nil
}

// runMigration runs a specific migration version
func (db *DB) runMigration(version int) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	switch version {
	case 1:
		if err := db.migration1(tx); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown migration version: %d", version)
	}

	// Record migration
	_, err = tx.Exec("INSERT INTO schema_version (version) VALUES (?)", version)
	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return tx.Commit()
}

// migration1 creates the initial schema
func (db *DB) migration1(tx *sql.Tx) error {
	schema := `
	-- Providers table
	CREATE TABLE providers (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		base_url TEXT NOT NULL,
		auth_method TEXT NOT NULL,
		auth_header TEXT,
		pricing_model TEXT NOT NULL,
		subscription_tiers JSON,
		discovered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		last_validated TIMESTAMP,
		sdk_path TEXT,
		sdk_hash TEXT,
		sdk_version TEXT,
		status TEXT DEFAULT 'offline',
		last_error TEXT,
		UNIQUE(base_url)
	);

	-- Model families table
	CREATE TABLE model_families (
		id TEXT PRIMARY KEY,
		provider_id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE
	);

	-- Models table
	CREATE TABLE models (
		id TEXT PRIMARY KEY,
		family_id TEXT NOT NULL,
		name TEXT NOT NULL,
		cost_per_1m_in REAL,
		cost_per_1m_out REAL,
		cost_per_1m_reasoning REAL,
		context_window INTEGER,
		max_tokens INTEGER,
		capabilities JSON,
		status TEXT DEFAULT 'offline',
		last_tested TIMESTAMP,
		last_error TEXT,
		FOREIGN KEY (family_id) REFERENCES model_families(id) ON DELETE CASCADE
	);

	-- API keys table
	CREATE TABLE api_keys (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		provider_id TEXT NOT NULL,
		key_hash TEXT NOT NULL,
		key_prefix TEXT,
		tier TEXT DEFAULT 'unknown',
		rpm_limit INTEGER,
		tpm_limit INTEGER,
		daily_limit INTEGER,
		reset_interval TEXT,
		last_reset TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		requests_count INTEGER DEFAULT 0,
		tokens_count INTEGER DEFAULT 0,
		active BOOLEAN DEFAULT 1,
		degraded BOOLEAN DEFAULT 0,
		degraded_until TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE,
		UNIQUE(provider_id, key_hash)
	);

	-- Usage tracking table
	CREATE TABLE usage_tracking (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		model_id TEXT NOT NULL,
		api_key_id INTEGER,
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		tokens_in INTEGER DEFAULT 0,
		tokens_out INTEGER DEFAULT 0,
		tokens_reasoning INTEGER DEFAULT 0,
		requests INTEGER DEFAULT 1,
		cost REAL DEFAULT 0,
		latency_ms INTEGER,
		success BOOLEAN DEFAULT 1,
		error TEXT,
		FOREIGN KEY (model_id) REFERENCES models(id) ON DELETE CASCADE,
		FOREIGN KEY (api_key_id) REFERENCES api_keys(id) ON DELETE SET NULL
	);

	-- Discovery logs table
	CREATE TABLE discovery_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		provider_id TEXT,
		agent_model TEXT,
		started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		completed_at TIMESTAMP,
		status TEXT,
		retry_count INTEGER DEFAULT 0,
		total_cost REAL DEFAULT 0,
		sources_scraped JSON,
		findings JSON,
		error TEXT,
		FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE
	);

	-- SDK versions table
	CREATE TABLE sdk_versions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		provider_id TEXT NOT NULL,
		version TEXT NOT NULL,
		sdk_path TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		deprecated_at TIMESTAMP,
		FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE,
		UNIQUE(provider_id, version)
	);

	-- Settings table
	CREATE TABLE settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Indexes for performance
	CREATE INDEX idx_usage_timestamp ON usage_tracking(timestamp);
	CREATE INDEX idx_usage_model ON usage_tracking(model_id);
	CREATE INDEX idx_usage_key ON usage_tracking(api_key_id);
	CREATE INDEX idx_models_status ON models(status);
	CREATE INDEX idx_providers_status ON providers(status);
	CREATE INDEX idx_api_keys_provider ON api_keys(provider_id);
	CREATE INDEX idx_api_keys_active ON api_keys(active, degraded);
	`

	_, err := tx.Exec(schema)
	return err
}

// Provider represents a provider in the database
type Provider struct {
	ID                string
	Name              string
	BaseURL           string
	AuthMethod        string
	AuthHeader        *string
	PricingModel      string
	SubscriptionTiers *string // JSON
	DiscoveredAt      time.Time
	LastValidated     *time.Time
	SDKPath           *string
	SDKHash           *string
	SDKVersion        *string
	Status            string
	LastError         *string
}

// ModelFamily represents a model family in the database
type ModelFamily struct {
	ID          string
	ProviderID  string
	Name        string
	Description *string
}

// Model represents a model in the database
type Model struct {
	ID                 string
	FamilyID           string
	Name               string
	CostPer1MIn        *float64
	CostPer1MOut       *float64
	CostPer1MReasoning *float64
	ContextWindow      *int
	MaxTokens          *int
	Capabilities       *string // JSON
	Status             string
	LastTested         *time.Time
	LastError          *string
}

// APIKey represents an API key in the database
type APIKey struct {
	ID            int
	ProviderID    string
	KeyHash       string
	KeyPrefix     *string
	Tier          string
	RPMLimit      *int
	TPMLimit      *int
	DailyLimit    *int
	ResetInterval *string
	LastReset     time.Time
	RequestsCount int
	TokensCount   int
	Active        bool
	Degraded      bool
	DegradedUntil *time.Time
	CreatedAt     time.Time
}

// UsageRecord represents a usage record in the database
type UsageRecord struct {
	ID              int
	ModelID         string
	APIKeyID        *int
	Timestamp       time.Time
	TokensIn        int
	TokensOut       int
	TokensReasoning int
	Requests        int
	Cost            float64
	LatencyMS       *int
	Success         bool
	Error           *string
}

// DiscoveryLog represents a discovery log in the database
type DiscoveryLog struct {
	ID             int
	ProviderID     *string
	AgentModel     *string
	StartedAt      time.Time
	CompletedAt    *time.Time
	Status         *string
	RetryCount     int
	TotalCost      float64
	SourcesScraped *string // JSON
	Findings       *string // JSON
	Error          *string
}

// SDKVersion represents an SDK version in the database
type SDKVersion struct {
	ID           int
	ProviderID   string
	Version      string
	SDKPath      string
	CreatedAt    time.Time
	DeprecatedAt *time.Time
}
