package admin

import (
	"testing"
	"time"

	"github.com/jeffersonwarrior/modelscan/internal/database"
	"github.com/jeffersonwarrior/modelscan/internal/discovery"
	"github.com/jeffersonwarrior/modelscan/internal/generator"
	"github.com/jeffersonwarrior/modelscan/internal/keymanager"
)

// Tests for DatabaseAdapter
func TestNewDatabaseAdapter(t *testing.T) {
	db := &database.DB{}
	adapter := NewDatabaseAdapter(db)
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}
	if adapter.db != db {
		t.Error("adapter db not set correctly")
	}
}

func TestDatabaseAdapter_CreateProvider(t *testing.T) {
	// Test with in-memory database
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	defer db.Close()

	adapter := NewDatabaseAdapter(db)

	provider := &Provider{
		ID:           "openai",
		Name:         "OpenAI",
		BaseURL:      "https://api.openai.com",
		AuthMethod:   "bearer",
		PricingModel: "pay-per-token",
		Status:       "online",
	}

	err = adapter.CreateProvider(provider)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify provider was created
	retrieved, err := adapter.GetProvider("openai")
	if err != nil {
		t.Fatalf("failed to retrieve provider: %v", err)
	}
	if retrieved.ID != "openai" {
		t.Errorf("expected ID openai, got %s", retrieved.ID)
	}
}

func TestDatabaseAdapter_GetProvider(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	defer db.Close()

	adapter := NewDatabaseAdapter(db)

	// Create a provider first
	provider := &Provider{
		ID:           "openai",
		Name:         "OpenAI",
		BaseURL:      "https://api.openai.com",
		AuthMethod:   "bearer",
		PricingModel: "pay-per-token",
	}
	err = adapter.CreateProvider(provider)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	// Test retrieval
	retrieved, err := adapter.GetProvider("openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved == nil {
		t.Fatal("expected non-nil provider")
	}
	if retrieved.ID != "openai" {
		t.Errorf("expected ID openai, got %s", retrieved.ID)
	}
	if retrieved.Name != "OpenAI" {
		t.Errorf("expected Name OpenAI, got %s", retrieved.Name)
	}
}

func TestDatabaseAdapter_GetProvider_NotFound(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	defer db.Close()

	adapter := NewDatabaseAdapter(db)

	retrieved, err := adapter.GetProvider("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retrieved != nil {
		t.Error("expected nil for nonexistent provider")
	}
}

func TestDatabaseAdapter_ListProviders(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	defer db.Close()

	adapter := NewDatabaseAdapter(db)

	// Create multiple providers
	providers := []*Provider{
		{
			ID:           "openai",
			Name:         "OpenAI",
			BaseURL:      "https://api.openai.com",
			AuthMethod:   "bearer",
			PricingModel: "pay-per-token",
		},
		{
			ID:           "anthropic",
			Name:         "Anthropic",
			BaseURL:      "https://api.anthropic.com",
			AuthMethod:   "x-api-key",
			PricingModel: "pay-per-token",
		},
	}

	for _, p := range providers {
		err = adapter.CreateProvider(p)
		if err != nil {
			t.Fatalf("failed to create provider %s: %v", p.ID, err)
		}
	}

	// Test listing
	list, err := adapter.ListProviders()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(list) != 2 {
		t.Errorf("expected 2 providers, got %d", len(list))
	}
}

func TestDatabaseAdapter_CreateAPIKey(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	defer db.Close()

	adapter := NewDatabaseAdapter(db)

	// Create a provider first
	provider := &Provider{
		ID:           "openai",
		Name:         "OpenAI",
		BaseURL:      "https://api.openai.com",
		AuthMethod:   "bearer",
		PricingModel: "pay-per-token",
	}
	err = adapter.CreateProvider(provider)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	// Create API key
	key, err := adapter.CreateAPIKey("openai", "sk-test-key-12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if key == nil {
		t.Fatal("expected non-nil key")
	}
	if key.ProviderID != "openai" {
		t.Errorf("expected provider ID openai, got %s", key.ProviderID)
	}
}

func TestDatabaseAdapter_ListActiveAPIKeys(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	defer db.Close()

	adapter := NewDatabaseAdapter(db)

	// Create providers
	for _, providerID := range []string{"openai", "anthropic"} {
		provider := &Provider{
			ID:           providerID,
			Name:         providerID,
			BaseURL:      "https://api." + providerID + ".com",
			AuthMethod:   "bearer",
			PricingModel: "pay-per-token",
		}
		err = adapter.CreateProvider(provider)
		if err != nil {
			t.Fatalf("failed to create provider: %v", err)
		}
	}

	// Create API keys
	_, err = adapter.CreateAPIKey("openai", "sk-openai-1")
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}
	_, err = adapter.CreateAPIKey("openai", "sk-openai-2")
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}
	_, err = adapter.CreateAPIKey("anthropic", "sk-anthropic-1")
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	// Test listing for openai
	keys, err := adapter.ListActiveAPIKeys("openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("expected 2 active keys for openai, got %d", len(keys))
	}
}

func TestDatabaseAdapter_GetUsageStats(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	defer db.Close()

	adapter := NewDatabaseAdapter(db)

	stats, err := adapter.GetUsageStats("gpt-4", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Stats should be empty for new database
	if stats == nil {
		t.Error("expected non-nil stats map")
	}
}

// Tests for DiscoveryAdapter
func TestNewDiscoveryAdapter(t *testing.T) {
	agent := &discovery.Agent{}
	adapter := NewDiscoveryAdapter(agent)
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}
	if adapter.agent != agent {
		t.Error("adapter agent not set correctly")
	}
}

// Tests for GeneratorAdapter
func TestNewGeneratorAdapter(t *testing.T) {
	gen := &generator.Generator{}
	adapter := NewGeneratorAdapter(gen)
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}
	if adapter.gen != gen {
		t.Error("adapter generator not set correctly")
	}
}

// Tests for KeyManagerAdapter
func TestNewKeyManagerAdapter(t *testing.T) {
	km := &keymanager.KeyManager{}
	adapter := NewKeyManagerAdapter(km, nil) // db can be nil for this simple test
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}
	if adapter.km != km {
		t.Error("adapter manager not set correctly")
	}
}
