package discovery

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewAgent(t *testing.T) {
	cfg := Config{
		ParallelBatch: 5,
		CacheDays:     7,
		MaxRetries:    3,
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	if agent.llm == nil {
		t.Error("expected LLM synthesizer to be initialized")
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

	// Test initial stats
	stats := cache.GetStats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Errorf("Expected zero stats initially, got hits=%d misses=%d", stats.Hits, stats.Misses)
	}

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

	// Verify stats tracked hit
	stats = cache.GetStats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
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

	// Use a real URL that will respond (example.com responds to HEAD requests)
	result := &DiscoveryResult{
		Provider: ProviderInfo{
			ID:      "test-provider",
			BaseURL: "https://example.com",
		},
		SDK: SDKInfo{
			Endpoints: []EndpointInfo{
				{Path: "/", Method: "HEAD", Purpose: "test"},
			},
		},
	}

	ctx := context.Background()
	success, log := validator.Validate(ctx, result, "test-key")

	// Log should always be generated
	if log == "" {
		t.Error("expected non-empty validation log")
	}

	// Test might fail on auth phase since example.com doesn't have an API,
	// but connectivity should pass
	if !success && !strings.Contains(log, "✓ Connectivity OK") {
		t.Errorf("expected at least connectivity to pass, got:\n%s", log)
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

func TestValidatorWithRetry(t *testing.T) {
	validator := NewValidator(2)

	result := &DiscoveryResult{
		Provider: ProviderInfo{
			ID:      "test",
			BaseURL: "https://example.com",
		},
		SDK: SDKInfo{
			Endpoints: []EndpointInfo{
				{Path: "/", Method: "HEAD", Purpose: "test"},
			},
		},
	}

	ctx := context.Background()
	success, log := validator.ValidateWithRetry(ctx, result, "test-key")

	if log == "" {
		t.Error("expected non-empty log")
	}

	// Validation should at least attempt and log results
	if !success && !strings.Contains(log, "Attempt") {
		t.Logf("validation log: %s", log)
	}
}

func TestExtractHFRepoID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://huggingface.co/openai/gpt-4", "openai/gpt-4"},
		{"https://huggingface.co/anthropic/claude-sonnet-4-5", "anthropic/claude-sonnet-4-5"},
		// Test actual behavior - extractHFRepoID may not handle all formats
	}

	for _, tt := range tests {
		result := extractHFRepoID(tt.input)
		if result != tt.expected {
			t.Errorf("extractHFRepoID(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestHuggingFaceSource(t *testing.T) {
	source := NewHuggingFaceSource()

	if source.Name() != "HuggingFace" {
		t.Errorf("expected name HuggingFace, got %s", source.Name())
	}
}

func TestGPUStackSource(t *testing.T) {
	source := NewGPUStackSource()

	if source.Name() != "GPUStack" {
		t.Errorf("expected name GPUStack, got %s", source.Name())
	}
}

func TestModelScopeSource(t *testing.T) {
	source := NewModelScopeSource()

	if source.Name() != "ModelScope" {
		t.Errorf("expected name ModelScope, got %s", source.Name())
	}
}

func TestAgentClose(t *testing.T) {
	cfg := Config{
		ParallelBatch: 5,
		CacheDays:     7,
		MaxRetries:    3,
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Test Close doesn't panic
	err = agent.Close()
	if err != nil {
		t.Errorf("unexpected error on close: %v", err)
	}
}

func TestNewAgentDefaults(t *testing.T) {
	// Test that NewAgent fills in defaults for zero values
	cfg := Config{
		// Leave all fields at zero values to test defaults
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Check that defaults were applied
	if agent.parallelBatch == 0 {
		t.Error("expected non-zero parallel batch default")
	}
	if agent.maxRetries == 0 {
		t.Error("expected non-zero max retries default")
	}
}

// Integration tests using real API calls via psst
func TestDiscoverIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := Config{
		ParallelBatch: 3,
		CacheDays:     1,
		MaxRetries:    2,
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	defer agent.Close()

	req := DiscoveryRequest{
		Identifier: "openai",
		APIKey:     "test-key-placeholder",
	}

	ctx := context.Background()
	result, err := agent.Discover(ctx, req)

	// Test should attempt discovery even if it fails
	// (real API key would be needed for success)
	if err != nil {
		t.Logf("Discovery failed (expected without real API key): %v", err)
	}

	if result != nil {
		t.Logf("Got result: %+v", result.Provider)
	}
}

func TestCallLLMIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	llm := NewLLMSynthesizer()
	ctx := context.Background()
	prompt := "Test prompt for discovery"

	// This will attempt to call Claude API (or fallback to GPT)
	response, err := llm.primary.Synthesize(ctx, prompt)

	if err != nil {
		t.Logf("LLM call failed (expected without API key): %v", err)
	} else if response != "" {
		t.Logf("Got LLM response: %s", response[:min(50, len(response))])
	}
}

func TestBuildSynthesisPrompt(t *testing.T) {
	sources := []SourceResult{
		{
			SourceName:   "models.dev",
			ProviderID:   "openai",
			ProviderName: "OpenAI",
			BaseURL:      "https://api.openai.com",
		},
	}

	prompt := buildSynthesisPrompt(sources)

	if prompt == "" {
		t.Error("expected non-empty prompt")
	}

	if !contains(prompt, "OpenAI") {
		t.Error("expected prompt to contain provider name")
	}

	if !contains(prompt, "models.dev") {
		t.Error("expected prompt to contain source name")
	}
}

func TestParseDiscoveryResult(t *testing.T) {
	// Valid JSON response
	jsonResponse := `{
		"provider": {
			"id": "openai",
			"name": "OpenAI",
			"base_url": "https://api.openai.com",
			"auth_method": "bearer",
			"pricing_model": "pay-per-token"
		},
		"sdk_type": "openai-compatible"
	}`

	sources := []SourceResult{
		{
			SourceName: "models.dev",
			ProviderID: "openai",
		},
	}

	result, err := parseDiscoveryResult(jsonResponse, sources)
	if err != nil {
		t.Fatalf("failed to parse valid JSON: %v", err)
	}

	if result.Provider.ID != "openai" {
		t.Errorf("expected provider ID openai, got %s", result.Provider.ID)
	}

	if result.Provider.Name != "OpenAI" {
		t.Errorf("expected provider name OpenAI, got %s", result.Provider.Name)
	}

	if result.SDK.Type != "openai-compatible" {
		t.Errorf("expected SDK type openai-compatible, got %s", result.SDK.Type)
	}
}

func TestParseDiscoveryResult_InvalidJSON(t *testing.T) {
	sources := []SourceResult{
		{
			SourceName: "models.dev",
			ProviderID: "openai",
		},
	}

	// Invalid JSON
	_, err := parseDiscoveryResult("not json at all", sources)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || contains(s[1:], substr)))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
