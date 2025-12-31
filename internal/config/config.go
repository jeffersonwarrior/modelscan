package config

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config represents the minimal bootstrap configuration
type Config struct {
	Database  DatabaseConfig      `yaml:"database"`
	Server    ServerConfig        `yaml:"server"`
	APIKeys   map[string][]string `yaml:"api_keys"` // provider -> keys
	Discovery DiscoveryConfig     `yaml:"discovery"`
}

// DatabaseConfig holds database settings
type DatabaseConfig struct {
	Path string `yaml:"path"`
}

// ServerConfig holds server settings
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// DiscoveryConfig holds discovery agent settings
type DiscoveryConfig struct {
	AgentModel    string `yaml:"agent_model"`    // claude-sonnet-4-5, gpt-4o, etc.
	ParallelBatch int    `yaml:"parallel_batch"` // concurrent discovery tasks
	CacheDays     int    `yaml:"cache_days"`     // cache scraped data
	OutputDir     string `yaml:"output_dir"`     // directory for generated SDKs
	RoutingMode   string `yaml:"routing_mode"`   // routing mode: direct, proxy, embedded
}

// Load reads config from YAML file with graceful fallback
// Returns default config if file doesn't exist or is malformed
func Load(path string) (*Config, error) {
	// Try to read file
	data, err := os.ReadFile(path)
	if err != nil {
		// File doesn't exist - use defaults
		return DefaultConfig(), nil
	}

	var cfg Config
	// Try to parse YAML, but be resilient to bad formatting
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		// YAML parsing failed - use defaults
		return DefaultConfig(), nil
	}

	// Apply environment variable overrides
	cfg.applyEnvOverrides()

	// Apply defaults for missing values
	cfg.applyDefaults()

	return &cfg, nil
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	cfg := &Config{
		Database: DatabaseConfig{
			Path: getEnv("MODELSCAN_DB_PATH", "modelscan.db"),
		},
		Server: ServerConfig{
			Host: getEnv("MODELSCAN_HOST", "127.0.0.1"),
			Port: getEnvInt("MODELSCAN_PORT", 8080),
		},
		APIKeys: make(map[string][]string),
		Discovery: DiscoveryConfig{
			AgentModel:    getEnv("MODELSCAN_AGENT_MODEL", "claude-sonnet-4-5"),
			ParallelBatch: getEnvInt("MODELSCAN_PARALLEL_BATCH", 5),
			CacheDays:     getEnvInt("MODELSCAN_CACHE_DAYS", 7),
			OutputDir:     getEnv("MODELSCAN_OUTPUT_DIR", "generated"),
			RoutingMode:   getEnv("MODELSCAN_ROUTING_MODE", "direct"),
		},
	}
	return cfg
}

// applyEnvOverrides applies environment variable overrides
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("MODELSCAN_DB_PATH"); v != "" {
		c.Database.Path = v
	}
	if v := os.Getenv("MODELSCAN_HOST"); v != "" {
		c.Server.Host = v
	}
	if v := os.Getenv("MODELSCAN_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Server.Port = port
		}
	}
	if v := os.Getenv("MODELSCAN_AGENT_MODEL"); v != "" {
		c.Discovery.AgentModel = v
	}
	if v := os.Getenv("MODELSCAN_PARALLEL_BATCH"); v != "" {
		if batch, err := strconv.Atoi(v); err == nil {
			c.Discovery.ParallelBatch = batch
		}
	}
	if v := os.Getenv("MODELSCAN_CACHE_DAYS"); v != "" {
		if days, err := strconv.Atoi(v); err == nil {
			c.Discovery.CacheDays = days
		}
	}
	if v := os.Getenv("MODELSCAN_OUTPUT_DIR"); v != "" {
		c.Discovery.OutputDir = v
	}
	if v := os.Getenv("MODELSCAN_ROUTING_MODE"); v != "" {
		c.Discovery.RoutingMode = v
	}
}

// applyDefaults fills in missing values with defaults
func (c *Config) applyDefaults() {
	if c.Database.Path == "" {
		c.Database.Path = "modelscan.db"
	}
	if c.Server.Host == "" {
		c.Server.Host = "127.0.0.1"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Discovery.AgentModel == "" {
		c.Discovery.AgentModel = "claude-sonnet-4-5"
	}
	if c.Discovery.ParallelBatch == 0 {
		c.Discovery.ParallelBatch = 5
	}
	if c.Discovery.CacheDays == 0 {
		c.Discovery.CacheDays = 7
	}
	if c.Discovery.OutputDir == "" {
		c.Discovery.OutputDir = "generated"
	}
	if c.Discovery.RoutingMode == "" {
		c.Discovery.RoutingMode = "direct"
	}
	if c.APIKeys == nil {
		c.APIKeys = make(map[string][]string)
	}
}

// getEnv gets environment variable or returns default
func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// getEnvInt gets environment variable as int or returns default
func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}
