package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	CurrentSchemaVersion = 5
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
		if err = db.migration1(tx); err != nil {
			return err
		}
	case 2:
		if err = db.migration2(tx); err != nil {
			return err
		}
	case 3:
		if err = db.migration3(tx); err != nil {
			return err
		}
	case 4:
		if err = db.migration4(tx); err != nil {
			return err
		}
	case 5:
		if err = db.migration5(tx); err != nil {
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

// migration2 adds discovery_results table
func (db *DB) migration2(tx *sql.Tx) error {
	schema := `
	-- Discovery results table (stores complete discovery data)
	CREATE TABLE discovery_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		identifier TEXT NOT NULL UNIQUE,
		provider_data JSON NOT NULL,
		model_families JSON,
		models JSON,
		sdk_data JSON,
		validated BOOLEAN DEFAULT 0,
		validation_log TEXT,
		sources JSON,
		discovered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		ttl_expires_at TIMESTAMP
	);

	-- Index for cache lookups
	CREATE INDEX idx_discovery_identifier ON discovery_results(identifier);
	CREATE INDEX idx_discovery_ttl ON discovery_results(ttl_expires_at);
	`

	_, err := tx.Exec(schema)
	return err
}

// migration3 creates clients and request_logs tables for MClaude integration
func (db *DB) migration3(tx *sql.Tx) error {
	schema := `
	-- Clients table for MClaude integration
	CREATE TABLE clients (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		version TEXT NOT NULL,
		token TEXT NOT NULL UNIQUE,
		capabilities JSON NOT NULL DEFAULT '[]',
		config JSON NOT NULL DEFAULT '{}',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		last_seen_at TIMESTAMP
	);

	-- Index for client token lookups
	CREATE INDEX idx_clients_token ON clients(token);

	-- Request logs table for observability
	CREATE TABLE request_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		client_id TEXT,
		provider TEXT NOT NULL,
		model TEXT NOT NULL,
		endpoint TEXT NOT NULL,
		request_tokens INTEGER DEFAULT 0,
		response_tokens INTEGER DEFAULT 0,
		latency_ms INTEGER DEFAULT 0,
		status_code INTEGER,
		error_message TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE SET NULL
	);

	-- Indexes for request log queries
	CREATE INDEX idx_request_logs_client ON request_logs(client_id);
	CREATE INDEX idx_request_logs_created ON request_logs(created_at);
	CREATE INDEX idx_request_logs_provider ON request_logs(provider);
	`

	_, err := tx.Exec(schema)
	return err
}

// migration4 creates aliases table for model name aliases
func (db *DB) migration4(tx *sql.Tx) error {
	schema := `
	-- Aliases table for model name aliases
	-- Uses unique constraint on (name, client_id) with a workaround for NULL client_id
	CREATE TABLE aliases (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		model_id TEXT NOT NULL,
		client_id TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE CASCADE
	);

	-- Unique index for alias lookups (NULL client_id means global alias)
	CREATE UNIQUE INDEX idx_aliases_name_client ON aliases(name, client_id);
	CREATE INDEX idx_aliases_client ON aliases(client_id);
	CREATE INDEX idx_aliases_name ON aliases(name);
	`

	_, err := tx.Exec(schema)
	return err
}

// migration5 creates remap_rules, client_rate_limits tables and default aliases
func (db *DB) migration5(tx *sql.Tx) error {
	schema := `
	-- Remap rules table for model remapping
	CREATE TABLE remap_rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		client_id TEXT NOT NULL,
		from_model TEXT NOT NULL,
		to_model TEXT NOT NULL,
		to_provider TEXT NOT NULL,
		priority INTEGER DEFAULT 0,
		enabled BOOLEAN DEFAULT 1,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE CASCADE
	);

	-- Indexes for remap rule lookups
	CREATE INDEX idx_remap_rules_client ON remap_rules(client_id);
	CREATE INDEX idx_remap_rules_enabled ON remap_rules(client_id, enabled);
	CREATE INDEX idx_remap_rules_priority ON remap_rules(client_id, priority DESC);

	-- Client rate limits table
	CREATE TABLE client_rate_limits (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		client_id TEXT NOT NULL UNIQUE,
		rpm_limit INTEGER,
		tpm_limit INTEGER,
		daily_limit INTEGER,
		current_rpm INTEGER DEFAULT 0,
		current_tpm INTEGER DEFAULT 0,
		current_daily INTEGER DEFAULT 0,
		last_reset TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE CASCADE
	);

	-- Index for rate limit lookups
	CREATE INDEX idx_client_rate_limits_client ON client_rate_limits(client_id);
	`

	_, err := tx.Exec(schema)
	if err != nil {
		return err
	}

	// Insert default global aliases (client_id = NULL for global)
	defaultAliases := `
	INSERT OR IGNORE INTO aliases (name, model_id, client_id) VALUES
		('sonnet', 'claude-sonnet-4-5-20250929', NULL),
		('opus', 'claude-opus-4-5-20250929', NULL),
		('haiku', 'claude-3-5-haiku-20241022', NULL),
		('gpt4', 'gpt-4o', NULL),
		('gemini', 'gemini-1.5-pro', NULL);
	`

	_, err = tx.Exec(defaultAliases)
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

// NOTE: Client, Alias, RemapRule, and RequestLog types are defined in their respective files:
// - clients.go
// - aliases.go
// - remaps.go
// - requests.go

// ClientRateLimit represents a client's rate limit configuration
type ClientRateLimit struct {
	ID           int
	ClientID     string
	RPMLimit     *int
	TPMLimit     *int
	DailyLimit   *int
	CurrentRPM   int
	CurrentTPM   int
	CurrentDaily int
	LastReset    time.Time
}
