package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the configuration for ModelScan including API keys
type Config struct {
	Providers map[string]ProviderConfig `json:"providers"`
}

// ProviderConfig holds provider-specific configuration
type ProviderConfig struct {
	APIKey      string `json:"api_key"`
	Endpoint    string `json:"endpoint,omitempty"`
	Description string `json:"description,omitempty"`
}

// LoadConfig loads configuration from multiple sources in priority order:
// 1. Environment variables
// 2. ModelScan config file (~/.config/nexora/modelscan.config.json or local)
// 3. NEXORA config from the project
// 4. Agent environment variables
func LoadConfig() (*Config, error) {
	config := &Config{
		Providers: make(map[string]ProviderConfig),
	}

	// First try to load from NEXORA config
	if err := loadFromNexoraConfig(config); err != nil {
		// It's okay if we can't find NEXORA config, continue with other sources
	}

	// Then load from Agent environment (has many more providers)
	if err := LoadFromAgentEnv(config); err != nil {
		// Continue even if agent env fails
	}

	// Extract additional providers
	extractFromGamma(config)
	extractFromManus(config)
	extractFromLlamaIndex(config)
	extractFromNanoGPT(config)
	extractYouCom(config)
	extractMinimax(config)
	extractKimiForCoding(config)
	extractFromVibe(config)

	// Then load from ModelScan config file
	if err := loadFromModelScanConfig(config); err != nil {
		return nil, fmt.Errorf("failed to load ModelScan config: %w", err)
	}

	// Finally override with environment variables
	loadFromEnvironment(config)

	return config, nil
}

// loadFromNexoraConfig attempts to extract API keys from NEXORA's setup
func loadFromNexoraConfig(config *Config) error {
	// Try to get API key from environment (NEXORA's method)
	if mistralKey := os.Getenv("MISTRAL_API_KEY"); mistralKey != "" {
		config.Providers["mistral"] = ProviderConfig{
			APIKey:      mistralKey,
			Endpoint:    "https://api.mistral.ai/v1",
			Description: "NEXORA Mistral API Key",
		}
	}

	// Add other providers if available
	if openaiKey := os.Getenv("OPENAI_API_KEY"); openaiKey != "" {
		config.Providers["openai"] = ProviderConfig{
			APIKey:      openaiKey,
			Endpoint:    "https://api.openai.com/v1",
			Description: "NEXORA OpenAI API Key",
		}
	}

	if anthropicKey := os.Getenv("ANTHROPIC_API_KEY"); anthropicKey != "" {
		config.Providers["anthropic"] = ProviderConfig{
			APIKey:      anthropicKey,
			Endpoint:    "https://api.anthropic.com/v1",
			Description: "NEXORA Anthropic API Key",
		}
	}

	// Google/Gemini can use either env var
	googleKey := os.Getenv("GOOGLE_API_KEY")
	if googleKey == "" {
		googleKey = os.Getenv("GEMINI_API_KEY")
	}
	if googleKey != "" {
		config.Providers["google"] = ProviderConfig{
			APIKey:      googleKey,
			Endpoint:    "https://generativelanguage.googleapis.com/v1",
			Description: "NEXORA Google/Gemini API Key",
		}
	}

	return nil
}

// loadFromModelScanConfig loads from local config file
func loadFromModelScanConfig(config *Config) error {
	// Try multiple locations for the config file
	locations := []string{
		filepath.Join(os.Getenv("HOME"), ".config", "nexora", "modelscan.config.json"),
		"modelscan.config.json",
		"./config/modelscan.config.json",
		".modelscan.config.json",
	}

	var configData []byte
	var configPath string
	for _, path := range locations {
		if data, err := os.ReadFile(path); err == nil {
			configData = data
			configPath = path
			break
		}
	}

	if configData != nil {
		// Parse existing config
		var fileConfig Config
		if err := json.Unmarshal(configData, &fileConfig); err != nil {
			return fmt.Errorf("failed to parse config file %s: %w", configPath, err)
		}

		// Merge with existing config (NEXORA takes priority)
		for provider, providerConfig := range fileConfig.Providers {
			if _, exists := config.Providers[provider]; !exists {
				config.Providers[provider] = providerConfig
			}
		}
	}

	// Save the merged config back to the preferred location
	preferredPath := filepath.Join(os.Getenv("HOME"), ".config", "nexora", "modelscan.config.json")
	if err := saveConfig(config, preferredPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// loadFromEnvironment overrides config with environment variables
func loadFromEnvironment(config *Config) {
	// Override with environment variables if set
	if apiKey := os.Getenv("MODELSCAN_MISTRAL_KEY"); apiKey != "" {
		if config.Providers["mistral"].Description == "" {
			cfg := config.Providers["mistral"]
			cfg.Description = "Environment override"
			config.Providers["mistral"] = cfg
		}
		cfg := config.Providers["mistral"]
		cfg.APIKey = apiKey
		config.Providers["mistral"] = cfg
	}
	if apiKey := os.Getenv("MODELSCAN_OPENAI_KEY"); apiKey != "" {
		providerConfig := ProviderConfig{Description: "Environment override"}
		providerConfig.APIKey = apiKey
		config.Providers["openai"] = providerConfig
	}
	if apiKey := os.Getenv("MODELSCAN_ANTHROPIC_KEY"); apiKey != "" {
		providerConfig := ProviderConfig{Description: "Environment override"}
		providerConfig.APIKey = apiKey
		config.Providers["anthropic"] = providerConfig
	}
}

// saveConfig saves configuration to the specified path
func saveConfig(config *Config, path string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	// Create a copy without secret keys for saving
	safeConfig := &Config{
		Providers: make(map[string]ProviderConfig),
	}

	for provider, cfg := range config.Providers {
		// Only save if we have an API key
		if cfg.APIKey != "" {
			// Save API key in a way that can be restored
			if strings.HasPrefix(cfg.APIKey, "$") {
				// Already an environment reference
				safeConfig.Providers[provider] = ProviderConfig{
					Endpoint:    cfg.Endpoint,
					Description: cfg.Description,
					APIKey:      cfg.APIKey,
				}
			} else {
				// Save as empty, user can set manually if needed
				safeConfig.Providers[provider] = ProviderConfig{
					Endpoint:    cfg.Endpoint,
					Description: cfg.Description,
					APIKey:      "",
				}
			}
		}
	}

	data, err := json.MarshalIndent(safeConfig, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

// GetAPIKey returns the API key for a provider
func (c *Config) GetAPIKey(provider string) (string, error) {
	config, exists := c.Providers[provider]
	if !exists {
		return "", fmt.Errorf("no configuration found for provider: %s", provider)
	}

	if config.APIKey == "" {
		return "", fmt.Errorf("no API key configured for provider: %s", provider)
	}

	return config.APIKey, nil
}

// HasProvider checks if a provider is configured
func (c *Config) HasProvider(provider string) bool {
	_, exists := c.Providers[provider]
	return exists && c.Providers[provider].APIKey != ""
}

// ListProviders returns a list of configured providers
func (c *Config) ListProviders() []string {
	var providers []string
	for provider, config := range c.Providers {
		if config.APIKey != "" {
			providers = append(providers, provider)
		}
	}
	return providers
}
