package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Set environment variables for testing
	os.Setenv("MISTRAL_API_KEY", "test-mistral-key")
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	defer func() {
		os.Unsetenv("MISTRAL_API_KEY")
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("ANTHROPIC_API_KEY")
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("LoadConfig() returned nil config")
	}

	// Check that providers were loaded from environment
	if _, exists := cfg.Providers["mistral"]; !exists {
		t.Error("Expected mistral provider to be loaded from environment")
	}
	if _, exists := cfg.Providers["openai"]; !exists {
		t.Error("Expected openai provider to be loaded from environment")
	}
	if _, exists := cfg.Providers["anthropic"]; !exists {
		t.Error("Expected anthropic provider to be loaded from environment")
	}
}

func TestGetAPIKey(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderConfig{
			"test": {
				APIKey:      "test-api-key",
				Endpoint:    "https://api.test.com",
				Description: "Test provider",
			},
		},
	}

	tests := []struct {
		name     string
		provider string
		wantErr  bool
	}{
		{"Valid provider", "test", false},
		{"Invalid provider", "nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := cfg.GetAPIKey(tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAPIKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && key == "" {
				t.Error("GetAPIKey() returned empty key for valid provider")
			}
		})
	}
}

func TestHasProvider(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderConfig{
			"test": {
				APIKey: "test-key",
			},
		},
	}

	tests := []struct {
		name     string
		provider string
		want     bool
	}{
		{"Existing provider", "test", true},
		{"Non-existing provider", "nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cfg.HasProvider(tt.provider); got != tt.want {
				t.Errorf("HasProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListProviders(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderConfig{
			"provider1": {APIKey: "key1"},
			"provider2": {APIKey: "key2"},
			"provider3": {APIKey: "key3"},
		},
	}

	providers := cfg.ListProviders()

	if len(providers) != 3 {
		t.Errorf("ListProviders() returned %d providers, want 3", len(providers))
	}

	// Check that all providers are present
	providerMap := make(map[string]bool)
	for _, p := range providers {
		providerMap[p] = true
	}

	expected := []string{"provider1", "provider2", "provider3"}
	for _, exp := range expected {
		if !providerMap[exp] {
			t.Errorf("ListProviders() missing provider %q", exp)
		}
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	// Set override environment variables
	os.Setenv("MODELSCAN_MISTRAL_KEY", "override-mistral")
	os.Setenv("MODELSCAN_OPENAI_KEY", "override-openai")
	defer func() {
		os.Unsetenv("MODELSCAN_MISTRAL_KEY")
		os.Unsetenv("MODELSCAN_OPENAI_KEY")
	}()

	cfg := &Config{
		Providers: make(map[string]ProviderConfig),
	}

	loadFromEnvironment(cfg)

	// Check that override variables were loaded
	if mistral, exists := cfg.Providers["mistral"]; !exists || mistral.APIKey != "override-mistral" {
		t.Error("MODELSCAN_MISTRAL_KEY should override mistral API key")
	}
	if openai, exists := cfg.Providers["openai"]; !exists || openai.APIKey != "override-openai" {
		t.Error("MODELSCAN_OPENAI_KEY should override openai API key")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	// Create a test config
	testConfig := &Config{
		Providers: map[string]ProviderConfig{
			"test-provider": {
				APIKey:      "test-key-123",
				Endpoint:    "https://api.test.com",
				Description: "Test provider for unit testing",
			},
		},
	}

	// Save the config
	if err := saveConfig(testConfig, configPath); err != nil {
		t.Fatalf("saveConfig() failed: %v", err)
	}

	// Check that file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}
}

func TestProviderConfigStructure(t *testing.T) {
	config := ProviderConfig{
		APIKey:      "test-key",
		Endpoint:    "https://api.test.com",
		Description: "Test description",
	}

	if config.APIKey == "" {
		t.Error("APIKey should not be empty")
	}
	if config.Endpoint == "" {
		t.Error("Endpoint should not be empty")
	}
	if config.Description == "" {
		t.Error("Description should not be empty")
	}
}

func TestConfigMerging(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderConfig{
			"existing": {
				APIKey:      "existing-key",
				Description: "Existing provider",
			},
		},
	}

	// Simulate loading from environment
	os.Setenv("MISTRAL_API_KEY", "env-mistral-key")
	defer os.Unsetenv("MISTRAL_API_KEY")

	loadFromNexoraConfig(cfg)

	// Check that existing provider is not overwritten
	if cfg.Providers["existing"].APIKey != "existing-key" {
		t.Error("Existing provider should not be overwritten")
	}

	// Check that new provider from environment was added
	if mistral, exists := cfg.Providers["mistral"]; !exists {
		t.Error("Mistral provider should be added from environment")
	} else if mistral.APIKey != "env-mistral-key" {
		t.Errorf("Mistral API key = %q, want %q", mistral.APIKey, "env-mistral-key")
	}
}

func BenchmarkLoadConfig(b *testing.B) {
	// Set up environment
	os.Setenv("MISTRAL_API_KEY", "test-key")
	defer os.Unsetenv("MISTRAL_API_KEY")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = LoadConfig()
	}
}
