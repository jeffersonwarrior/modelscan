package discovery

import (
	"context"
	"testing"
	"time"
)

func TestNewAgent(t *testing.T) {
	cfg := Config{
		Model:         "claude-sonnet-4-5",
		ParallelBatch: 5,
		CacheDays:     7,
		MaxRetries:    3,
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	if agent.model != "claude-sonnet-4-5" {
		t.Errorf("expected model claude-sonnet-4-5, got %s", agent.model)
	}
	if agent.parallelBatch != 5 {
		t.Errorf("expected parallel batch 5, got %d", agent.parallelBatch)
	}
	if agent.maxRetries != 3 {
		t.Errorf("expected max retries 3, got %d", agent.maxRetries)
	}
}

func TestCacheBasics(t *testing.T) {
	cache := NewCache(1 * time.Second)

	// Test Set and Get
	result := &DiscoveryResult{
		Provider: ProviderInfo{
			ID:   "test-provider",
			Name: "Test Provider",
		},
	}

	cache.Set("test", result)

	retrieved, ok := cache.Get("test")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if retrieved.Provider.ID != "test-provider" {
		t.Errorf("expected provider ID test-provider, got %s", retrieved.Provider.ID)
	}

	// Test cache miss
	_, ok = cache.Get("nonexistent")
	if ok {
		t.Error("expected cache miss")
	}

	// Test expiration
	time.Sleep(2 * time.Second)
	_, ok = cache.Get("test")
	if ok {
		t.Error("expected cache miss after expiration")
	}
}

func TestCacheDelete(t *testing.T) {
	cache := NewCache(1 * time.Hour)

	result := &DiscoveryResult{
		Provider: ProviderInfo{ID: "test"},
	}

	cache.Set("test", result)
	cache.Delete("test")

	_, ok := cache.Get("test")
	if ok {
		t.Error("expected cache miss after delete")
	}
}

func TestCacheClear(t *testing.T) {
	cache := NewCache(1 * time.Hour)

	cache.Set("test1", &DiscoveryResult{Provider: ProviderInfo{ID: "1"}})
	cache.Set("test2", &DiscoveryResult{Provider: ProviderInfo{ID: "2"}})

	if cache.Size() != 2 {
		t.Errorf("expected size 2, got %d", cache.Size())
	}

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", cache.Size())
	}
}

func TestExtractProvider(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"openai/gpt-4", "openai"},
		{"anthropic/claude-sonnet-4-5", "anthropic"},
		{"deepseek-coder", "deepseek-coder"},
	}

	for _, tt := range tests {
		result := extractProvider(tt.input)
		if result != tt.expected {
			t.Errorf("extractProvider(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestValidator(t *testing.T) {
	validator := NewValidator(3)

	result := &DiscoveryResult{
		Provider: ProviderInfo{
			ID:      "test-provider",
			BaseURL: "https://api.test.com",
		},
		SDK: SDKInfo{
			Endpoints: []EndpointInfo{
				{Path: "/v1/chat/completions", Method: "POST", Purpose: "chat"},
			},
		},
	}

	ctx := context.Background()
	success, log := validator.Validate(ctx, result, "test-key")

	// Should succeed (mocked validation)
	if !success {
		t.Errorf("validation failed unexpectedly:\n%s", log)
	}

	if log == "" {
		t.Error("expected non-empty validation log")
	}
}

func TestModelsDevSource(t *testing.T) {
	source := NewModelsDevSource()

	if source.Name() != "models.dev" {
		t.Errorf("expected name models.dev, got %s", source.Name())
	}

	// Note: Actual fetching is skipped in tests to avoid external dependencies
	// Integration tests would test actual API calls
}
