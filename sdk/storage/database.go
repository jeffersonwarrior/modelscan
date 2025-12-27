package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// AgentDB manages the SQLite database for agent framework
type AgentDB struct {
	db *sql.DB
}

// NewAgentDB creates a new agent database instance
func NewAgentDB(dbPath string) (*AgentDB, error) {
	// Ensure directory exists
	if dir := filepath.Dir(dbPath); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_fk=true")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	agentDB := &AgentDB{db: db}
	if err := agentDB.init(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return agentDB, nil
}

// init creates tables and runs migrations
func (adb *AgentDB) init() error {
	if err := adb.createTables(); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	if err := adb.runMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// createTables creates the initial database schema
func (adb *AgentDB) createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Agents table
		`CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			capabilities TEXT,
			config TEXT,
			status TEXT DEFAULT 'inactive',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Teams table
		`CREATE TABLE IF NOT EXISTS teams (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			config TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Team membership
		`CREATE TABLE IF NOT EXISTS team_members (
			team_id TEXT,
			agent_id TEXT,
			role TEXT DEFAULT 'member',
			joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (team_id, agent_id),
			FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE,
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
		)`,

		// Tasks table
		`CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			team_id TEXT,
			type TEXT NOT NULL,
			status TEXT DEFAULT 'pending',
			priority INTEGER DEFAULT 1,
			input TEXT,
			output TEXT,
			metadata TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			started_at DATETIME,
			completed_at DATETIME,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
			FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE SET NULL
		)`,

		// Messages table
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			from_agent TEXT,
			to_agent TEXT,
			team_id TEXT,
			message_type TEXT,
			content TEXT,
			data TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (from_agent) REFERENCES agents(id),
			FOREIGN KEY (to_agent) REFERENCES agents(id),
			FOREIGN KEY (team_id) REFERENCES teams(id)
		)`,

		// Tool executions
		`CREATE TABLE IF NOT EXISTS tool_executions (
			id TEXT PRIMARY KEY,
			task_id TEXT,
			agent_id TEXT,
			tool_name TEXT NOT NULL,
			tool_type TEXT,
			input TEXT,
			output TEXT,
			error TEXT,
			status TEXT DEFAULT 'pending',
			duration INTEGER DEFAULT 0,
			metadata TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (agent_id) REFERENCES agents(id)
		)`,

		// Agent statistics
		`CREATE TABLE IF NOT EXISTS agent_stats (
			agent_id TEXT PRIMARY KEY,
			tasks_completed INTEGER DEFAULT 0,
			tasks_failed INTEGER DEFAULT 0,
			total_execution_time_ms INTEGER DEFAULT 0,
			last_activity DATETIME,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
		)`,
	}

	for _, query := range queries {
		if _, err := adb.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %s: %w", query, err)
		}
	}

	// Create indexes for common queries
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_agent_id ON tasks(agent_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_between_agents ON messages(from_agent, to_agent)`,
		`CREATE INDEX IF NOT EXISTS idx_tool_executions_agent ON tool_executions(agent_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status)`,
	}

	for _, query := range indexes {
		if _, err := adb.db.Exec(query); err != nil {
			return fmt.Errorf("failed to create index: %s: %w", query, err)
		}
	}

	return nil
}

// runMigrations handles database schema versioning
func (adb *AgentDB) runMigrations() error {
	// Get current migration version
	var version int
	err := adb.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	// Apply migrations in order
	migrations := []func(*sql.DB) error{
		migrationV1InitialSchema,
		migrationV2AddTeamDescription,
		migrationV3UpdateToolExecutions,
	}

	for i, migration := range migrations {
		migrationVersion := i + 1
		if version >= migrationVersion {
			continue // Skip already applied migrations
		}

		log.Printf("Applying migration v%d", migrationVersion)
		if err := migration(adb.db); err != nil {
			return fmt.Errorf("migration v%d failed: %w", migrationVersion, err)
		}

		// Mark migration as applied
		if _, err := adb.db.Exec(
			"INSERT INTO schema_migrations (version) VALUES (?)",
			migrationVersion,
		); err != nil {
			return fmt.Errorf("failed to mark migration v%d: %w", migrationVersion, err)
		}
	}

	return nil
}

// migrationV1InitialSchema is the initial schema migration
func migrationV1InitialSchema(db *sql.DB) error {
	// Initial schema is already created in createTables()
	// This migration is a placeholder for future schema changes
	return nil
}

// migrationV2AddTeamDescription adds the description column to teams table
func migrationV2AddTeamDescription(db *sql.DB) error {
	// Add description column to teams table
	_, err := db.Exec("ALTER TABLE teams ADD COLUMN description TEXT")
	if err != nil {
		// Check if column already exists (SQLite doesn't support IF NOT EXISTS for columns)
		// If the error indicates the column already exists, that's okay
		if !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("failed to add description column to teams: %w", err)
		}
	}

	// Add metadata column to teams table
	_, err = db.Exec("ALTER TABLE teams ADD COLUMN metadata TEXT")
	if err != nil {
		if !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("failed to add metadata column to teams: %w", err)
		}
	}

	// Update schema for tasks table to match Task struct
	// Add agent_id column (if assigned_to doesn't exist, use it)
	_, err = db.Exec("ALTER TABLE tasks ADD COLUMN agent_id TEXT")
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return fmt.Errorf("failed to add agent_id column to tasks: %w", err)
	}

	// Add team_id column
	_, err = db.Exec("ALTER TABLE tasks ADD COLUMN team_id TEXT")
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return fmt.Errorf("failed to add team_id column to tasks: %w", err)
	}

	// Add type column
	_, err = db.Exec("ALTER TABLE tasks ADD COLUMN type TEXT")
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return fmt.Errorf("failed to add type column to tasks: %w", err)
	}

	// Add input column
	_, err = db.Exec("ALTER TABLE tasks ADD COLUMN input TEXT")
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return fmt.Errorf("failed to add input column to tasks: %w", err)
	}

	// Add output column
	_, err = db.Exec("ALTER TABLE tasks ADD COLUMN output TEXT")
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return fmt.Errorf("failed to add output column to tasks: %w", err)
	}

	// Add metadata column
	_, err = db.Exec("ALTER TABLE tasks ADD COLUMN metadata TEXT")
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return fmt.Errorf("failed to add metadata column to tasks: %w", err)
	}

	// Add started_at column
	_, err = db.Exec("ALTER TABLE tasks ADD COLUMN started_at DATETIME")
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return fmt.Errorf("failed to add started_at column to tasks: %w", err)
	}

	// Add updated_at column to team_members table
	_, err = db.Exec("ALTER TABLE team_members ADD COLUMN updated_at DATETIME DEFAULT CURRENT_TIMESTAMP")
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return fmt.Errorf("failed to add updated_at column to team_members: %w", err)
	}

	return nil
}

// migrationV3UpdateToolExecutions updates the tool_executions table schema
func migrationV3UpdateToolExecutions(db *sql.DB) error {
	// Add missing columns to tool_executions table
	columns := []struct {
		name, sqlType string
	}{
		{"task_id", "TEXT"},
		{"tool_type", "TEXT"},
		{"status", "TEXT DEFAULT 'pending'"},
		{"duration", "INTEGER DEFAULT 0"},
		{"metadata", "TEXT"},
		{"started_at", "DATETIME"},
		{"completed_at", "DATETIME"},
	}

	for _, col := range columns {
		_, err := db.Exec(fmt.Sprintf("ALTER TABLE tool_executions ADD COLUMN %s %s", col.name, col.sqlType))
		if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("failed to add %s column to tool_executions: %w", col.name, err)
		}
	}

	// Rename error_message to error if it exists
	_, err := db.Exec("ALTER TABLE tool_executions RENAME COLUMN error_message TO error")
	if err != nil && !strings.Contains(err.Error(), "no such column") {
		// Column might not exist or rename failed, that's okay for this migration
	}

	return nil
}

// Close closes the database connection
func (adb *AgentDB) Close() error {
	if adb.db != nil {
		return adb.db.Close()
	}
	return nil
}

// GetDB returns the underlying database connection
func (adb *AgentDB) GetDB() *sql.DB {
	return adb.db
}

// CleanupOldData removes records older than the specified duration
func (adb *AgentDB) CleanupOldData(ctx context.Context, olderThan time.Duration) error {
	cutoffTime := time.Now().Add(-olderThan)

	queries := []string{
		"DELETE FROM messages WHERE created_at < ?",
		"DELETE FROM tool_executions WHERE started_at < ? OR (started_at IS NULL AND created_at < ?)",
		"DELETE FROM tasks WHERE status IN ('completed', 'failed') AND completed_at < ?",
	}

	for _, query := range queries {
		var args []interface{}
		if strings.Contains(query, "tool_executions") {
			args = []interface{}{cutoffTime, cutoffTime}
		} else {
			args = []interface{}{cutoffTime}
		}

		result, err := adb.db.ExecContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("cleanup query failed: %s: %w", query, err)
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			log.Printf("Cleaned up %d rows with query: %s", rowsAffected, query)
		}
	}

	return nil
}

// StartCleanupScheduler starts a goroutine that periodically cleans up old data
func (adb *AgentDB) StartCleanupScheduler(ctx context.Context, interval time.Duration, retention time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := adb.CleanupOldData(ctx, retention); err != nil {
					log.Printf("Cleanup failed: %v", err)
				}
			}
		}
	}()
}
