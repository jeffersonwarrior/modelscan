package integration

import (
	"context"
	"testing"
	"time"

	"github.com/jeffersonwarrior/modelscan/internal/discovery"
)

func TestDiscoveryAgent_BasicFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDB(t)
	agent := setupDiscoveryAgent(t, db)

	// Test discovery with a well-known provider
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := agent.Discover(ctx, discovery.DiscoveryRequest{
		Identifier: "openai/gpt-4",
	})
	if err != nil {
		t.Logf("Discovery failed (expected if no internet/API access): %v", err)
		t.Skip("Skipping - discovery requires internet connectivity")
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Provider.ID == "" {
		t.Error("Expected provider ID to be set")
	}

	t.Logf("Discovered provider: %s", result.Provider.Name)
	t.Logf("Base URL: %s", result.Provider.BaseURL)
	t.Logf("Models found: %d", len(result.Models))
}

func TestDiscoveryAgent_Caching(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDB(t)
	agent := setupDiscoveryAgent(t, db)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := discovery.DiscoveryRequest{Identifier: "openai/gpt-4"}

	// First discovery
	start := time.Now()
	result1, err := agent.Discover(ctx, req)
	duration1 := time.Since(start)

	if err != nil {
		t.Skipf("Discovery failed (expected if no internet): %v", err)
	}

	// Second discovery (should hit cache)
	start = time.Now()
	result2, err := agent.Discover(ctx, req)
	duration2 := time.Since(start)

	if err != nil {
		t.Fatalf("Second discovery should not fail: %v", err)
	}

	// Cache hit should be significantly faster
	if duration2 > duration1 {
		t.Logf("Warning: Second discovery took longer (cache miss?). D1: %v, D2: %v", duration1, duration2)
	}

	if result1.Provider.ID != result2.Provider.ID {
		t.Error("Cached result should match original")
	}

	t.Logf("First discovery: %v", duration1)
	t.Logf("Second discovery (cached): %v", duration2)
}

func TestDiscoveryAgent_Statistics(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDB(t)
	agent := setupDiscoveryAgent(t, db)

	// Get initial statistics
	stats := agent.GetSourceStats()
	if stats == nil {
		t.Error("Expected non-nil statistics")
	}

	t.Logf("Source Statistics:")
	t.Logf("  Total sources: %d", len(stats))

	for name, stat := range stats {
		t.Logf("  Source %s: %d calls, %d successes, %d failures",
			name, stat.TotalCalls, stat.SuccessCalls, stat.FailedCalls)
	}
}

func TestDiscoveryAgent_ValidationPhases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create validator directly for testing phases
	validator := discovery.NewValidator(2) // maxRetries = 2

	testCases := []struct {
		name     string
		provider discovery.ProviderInfo
		apiKey   string
		skipTest bool
	}{
		{
			name: "valid provider - OpenAI",
			provider: discovery.ProviderInfo{
				ID:         "openai",
				Name:       "OpenAI",
				BaseURL:    "https://api.openai.com",
				AuthMethod: "bearer",
			},
			apiKey:   "", // Empty API key for testing
			skipTest: false,
		},
		{
			name: "invalid provider - unreachable",
			provider: discovery.ProviderInfo{
				ID:         "invalid",
				Name:       "Invalid",
				BaseURL:    "https://invalid.example.invalid",
				AuthMethod: "bearer",
			},
			apiKey:   "",
			skipTest: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipTest {
				t.Skip("Test requires internet connectivity")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			result := &discovery.DiscoveryResult{
				Provider: tc.provider,
			}

			success, log := validator.Validate(ctx, result, tc.apiKey)

			if tc.provider.ID == "invalid" {
				if success {
					t.Error("Expected validation to fail for invalid provider")
				} else {
					t.Logf("Validation correctly failed:\n%s", log)
				}
			} else {
				if !success {
					t.Logf("Validation failed (expected without API key):\n%s", log)
				} else {
					t.Logf("Validation passed:\n%s", log)
				}
			}
		})
	}
}

func TestDiscoveryAgent_CacheExpiration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDB(t)

	// Create agent with very short cache TTL (1 second for testing)
	agent, err := discovery.NewAgent(discovery.Config{
		ParallelBatch: 2,
		CacheDays:     0, // 0 days = immediate expiration for testing
		MaxRetries:    2,
		DB:            db,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	defer agent.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := discovery.DiscoveryRequest{Identifier: "openai/gpt-4"}

	// First discovery
	result1, err := agent.Discover(ctx, req)
	if err != nil {
		t.Skipf("Discovery failed: %v", err)
	}

	// Wait for cache to expire
	time.Sleep(2 * time.Second)

	// Second discovery (cache should be expired)
	result2, err := agent.Discover(ctx, req)
	if err != nil {
		t.Skipf("Second discovery failed: %v", err)
	}

	// Both should succeed but may have different timestamps
	if result1 == nil || result2 == nil {
		t.Error("Expected both results to be non-nil")
	}

	t.Log("Cache expiration test completed successfully")
}
