package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Database.Path != "modelscan.db" {
		t.Errorf("expected database path 'modelscan.db', got %s", cfg.Database.Path)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected host '127.0.0.1', got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Discovery.AgentModel != "claude-sonnet-4-5" {
		t.Errorf("expected agent model 'claude-sonnet-4-5', got %s", cfg.Discovery.AgentModel)
	}
	if cfg.Discovery.ParallelBatch != 5 {
		t.Errorf("expected parallel batch 5, got %d", cfg.Discovery.ParallelBatch)
	}
	if cfg.Discovery.CacheDays != 7 {
		t.Errorf("expected cache days 7, got %d", cfg.Discovery.CacheDays)
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	cfg, err := Load("nonexistent.yaml")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should return default config
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
}

func TestLoadValidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yaml := `
database:
  path: /tmp/test.db
server:
  host: 0.0.0.0
  port: 9090
api_keys:
  openai:
    - sk-test-key-1
    - sk-test-key-2
  anthropic:
    - sk-ant-test-1
discovery:
  agent_model: gpt-4o
  parallel_batch: 10
  cache_days: 14
`

	if err := os.WriteFile(configPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Database.Path != "/tmp/test.db" {
		t.Errorf("expected database path '/tmp/test.db', got %s", cfg.Database.Path)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected host '0.0.0.0', got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if len(cfg.APIKeys["openai"]) != 2 {
		t.Errorf("expected 2 openai keys, got %d", len(cfg.APIKeys["openai"]))
	}
	if len(cfg.APIKeys["anthropic"]) != 1 {
		t.Errorf("expected 1 anthropic key, got %d", len(cfg.APIKeys["anthropic"]))
	}
	if cfg.Discovery.AgentModel != "gpt-4o" {
		t.Errorf("expected agent model 'gpt-4o', got %s", cfg.Discovery.AgentModel)
	}
	if cfg.Discovery.ParallelBatch != 10 {
		t.Errorf("expected parallel batch 10, got %d", cfg.Discovery.ParallelBatch)
	}
	if cfg.Discovery.CacheDays != 14 {
		t.Errorf("expected cache days 14, got %d", cfg.Discovery.CacheDays)
	}
}

func TestLoadBadlyFormattedYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bad.yaml")

	// Badly formatted YAML (tabs instead of spaces, inconsistent indentation)
	badYAML := `
database:
	path: /tmp/test.db
  server:
host: 0.0.0.0
	port: not-a-number
`

	if err := os.WriteFile(configPath, []byte(badYAML), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("expected no error (graceful fallback), got %v", err)
	}

	// Should fall back to defaults
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
}

func TestEnvOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("MODELSCAN_DB_PATH", "/custom/db.db")
	os.Setenv("MODELSCAN_HOST", "192.168.1.1")
	os.Setenv("MODELSCAN_PORT", "3000")
	os.Setenv("MODELSCAN_AGENT_MODEL", "gpt-4o")
	os.Setenv("MODELSCAN_PARALLEL_BATCH", "20")
	os.Setenv("MODELSCAN_CACHE_DAYS", "30")

	defer func() {
		os.Unsetenv("MODELSCAN_DB_PATH")
		os.Unsetenv("MODELSCAN_HOST")
		os.Unsetenv("MODELSCAN_PORT")
		os.Unsetenv("MODELSCAN_AGENT_MODEL")
		os.Unsetenv("MODELSCAN_PARALLEL_BATCH")
		os.Unsetenv("MODELSCAN_CACHE_DAYS")
	}()

	cfg := DefaultConfig()

	if cfg.Database.Path != "/custom/db.db" {
		t.Errorf("expected database path '/custom/db.db', got %s", cfg.Database.Path)
	}
	if cfg.Server.Host != "192.168.1.1" {
		t.Errorf("expected host '192.168.1.1', got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 3000 {
		t.Errorf("expected port 3000, got %d", cfg.Server.Port)
	}
	if cfg.Discovery.AgentModel != "gpt-4o" {
		t.Errorf("expected agent model 'gpt-4o', got %s", cfg.Discovery.AgentModel)
	}
	if cfg.Discovery.ParallelBatch != 20 {
		t.Errorf("expected parallel batch 20, got %d", cfg.Discovery.ParallelBatch)
	}
	if cfg.Discovery.CacheDays != 30 {
		t.Errorf("expected cache days 30, got %d", cfg.Discovery.CacheDays)
	}
}

func TestEnvOverridesYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yaml := `
database:
  path: /tmp/test.db
server:
  host: 0.0.0.0
  port: 9090
`

	if err := os.WriteFile(configPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Environment variables should override YAML
	os.Setenv("MODELSCAN_PORT", "5000")
	defer os.Unsetenv("MODELSCAN_PORT")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Port should be from env, not YAML
	if cfg.Server.Port != 5000 {
		t.Errorf("expected port 5000 (from env), got %d", cfg.Server.Port)
	}
	// Path should be from YAML
	if cfg.Database.Path != "/tmp/test.db" {
		t.Errorf("expected database path '/tmp/test.db', got %s", cfg.Database.Path)
	}
}
