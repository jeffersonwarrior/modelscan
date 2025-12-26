package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/jeffersonwarrior/modelscan/providers"
)

var db *sql.DB

// InitDB initializes the SQLite database
func InitDB(dbPath string) error {
	var err error
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if err = createTables(); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	return nil
}

// createTables creates the necessary database tables
func createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS providers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			capabilities TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS models (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider_name TEXT NOT NULL,
			model_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			cost_per_1m_in REAL,
			cost_per_1m_out REAL,
			context_window INTEGER,
			max_tokens INTEGER,
			supports_images BOOLEAN,
			supports_tools BOOLEAN,
			can_reason BOOLEAN,
			can_stream BOOLEAN,
			categories TEXT,
			capabilities TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(provider_name, model_id),
			FOREIGN KEY(provider_name) REFERENCES providers(name)
		)`,
		`CREATE TABLE IF NOT EXISTS endpoints (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider_name TEXT NOT NULL,
			path TEXT NOT NULL,
			method TEXT NOT NULL,
			description TEXT,
			status TEXT,
			latency_ms INTEGER,
			error_message TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(provider_name, path, method),
			FOREIGN KEY(provider_name) REFERENCES providers(name)
		)`,
		`CREATE TABLE IF NOT EXISTS validation_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider_name TEXT NOT NULL,
			run_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			success_count INTEGER,
			failure_count INTEGER,
			total_latency_ms INTEGER
		)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query %s: %w", query, err)
		}
	}

	return nil
}

// StoreProviderInfo saves provider information to the database
func StoreProviderInfo(name string, models []providers.Model, capabilities providers.ProviderCapabilities) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Store provider and capabilities
	capsJSON, _ := json.Marshal(capabilities)
	_, err := db.Exec(`
		INSERT OR REPLACE INTO providers (name, capabilities) 
		VALUES (?, ?)
	`, name, string(capsJSON))
	if err != nil {
		return fmt.Errorf("failed to insert provider: %w", err)
	}

	// Store models
	for _, model := range models {
		categoriesJSON, _ := json.Marshal(model.Categories)
		modelCapsJSON, _ := json.Marshal(model.Capabilities)

		_, err := db.Exec(`
			INSERT OR REPLACE INTO models 
			(provider_name, model_id, name, description, cost_per_1m_in, cost_per_1m_out,
			 context_window, max_tokens, supports_images, supports_tools, can_reason, can_stream,
			 categories, capabilities)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			name, model.ID, model.Name, model.Description,
			model.CostPer1MIn, model.CostPer1MOut,
			model.ContextWindow, model.MaxTokens,
			model.SupportsImages, model.SupportsTools,
			model.CanReason, model.CanStream,
			string(categoriesJSON), string(modelCapsJSON),
		)
		if err != nil {
			return fmt.Errorf("failed to insert model %s: %w", model.ID, err)
		}
	}

	return nil
}

// StoreEndpointResults saves endpoint validation results
func StoreEndpointResults(name string, endpoints []providers.Endpoint) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	var successCount, failureCount, totalLatency int

	for _, endpoint := range endpoints {
		latencyMs := 0
		if endpoint.Latency > 0 {
			latencyMs = int(endpoint.Latency.Milliseconds())
		}

		_, err := db.Exec(`
			INSERT OR REPLACE INTO endpoints 
			(provider_name, path, method, description, status, latency_ms, error_message)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`,
			name, endpoint.Path, endpoint.Method,
			endpoint.Description, string(endpoint.Status),
			latencyMs, endpoint.Error,
		)
		if err != nil {
			return fmt.Errorf("failed to insert endpoint %s: %w", endpoint.Path, err)
		}

		totalLatency += latencyMs
		if endpoint.Status == providers.StatusWorking {
			successCount++
		} else if endpoint.Status == providers.StatusFailed {
			failureCount++
		}
	}

	// Record validation run
	_, err := db.Exec(`
		INSERT INTO validation_runs 
		(provider_name, success_count, failure_count, total_latency_ms)
		VALUES (?, ?, ?, ?)
	`,
		name, successCount, failureCount, totalLatency)
	if err != nil {
		return fmt.Errorf("failed to record validation run: %w", err)
	}

	return nil
}

// GetProviderModels retrieves all models for a provider
func GetProviderModels(providerName string) ([]providers.Model, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	rows, err := db.Query(`
		SELECT model_id, name, description, cost_per_1m_in, cost_per_1m_out,
			   context_window, max_tokens, supports_images, supports_tools, can_reason, can_stream,
			   categories, capabilities
		FROM models 
		WHERE provider_name = ?
		ORDER BY name
	`, providerName)
	if err != nil {
		return nil, fmt.Errorf("failed to query models: %w", err)
	}
	defer rows.Close()

	var models []providers.Model
	for rows.Next() {
		var model providers.Model
		var categoriesJSON, capsJSON string

		err := rows.Scan(
			&model.ID, &model.Name, &model.Description,
			&model.CostPer1MIn, &model.CostPer1MOut,
			&model.ContextWindow, &model.MaxTokens,
			&model.SupportsImages, &model.SupportsTools,
			&model.CanReason, &model.CanStream,
			&categoriesJSON, &capsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan model row: %w", err)
		}

		json.Unmarshal([]byte(categoriesJSON), &model.Categories)
		json.Unmarshal([]byte(capsJSON), &model.Capabilities)

		models = append(models, model)
	}

	return models, nil
}

// GetProviderEndpoints retrieves validation results for a provider
func GetProviderEndpoints(providerName string) ([]providers.Endpoint, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	rows, err := db.Query(`
		SELECT path, method, description, status, latency_ms, error_message
		FROM endpoints 
		WHERE provider_name = ?
		ORDER BY path, method
	`, providerName)
	if err != nil {
		return nil, fmt.Errorf("failed to query endpoints: %w", err)
	}
	defer rows.Close()

	var endpoints []providers.Endpoint
	for rows.Next() {
		var endpoint providers.Endpoint
		var latencyMs int
		var statusStr string

		err := rows.Scan(
			&endpoint.Path, &endpoint.Method, &endpoint.Description,
			&statusStr, &latencyMs, &endpoint.Error,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan endpoint row: %w", err)
		}

		endpoint.Status = providers.EndpointStatus(statusStr)
		if latencyMs > 0 {
			endpoint.Latency = time.Duration(latencyMs) * time.Millisecond
		}

		endpoints = append(endpoints, endpoint)
	}

	return endpoints, nil
}

// CloseDB closes the database connection
func CloseDB() error {
	if db != nil {
		err := db.Close()
		db = nil
		return err
	}
	return nil
}

// ExportToSQLite creates the SQLite database and exports all stored data
func ExportToSQLite(dbPath string) error {
	if err := InitDB(dbPath); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	return db.Ping()
}
