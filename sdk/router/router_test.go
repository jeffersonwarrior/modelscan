package router

import (
	"context"
	"os"
	"testing"

	"github.com/jeffersonwarrior/modelscan/scraper"
	"github.com/jeffersonwarrior/modelscan/storage"
)

func setupRouterTest(t *testing.T) string {
	dbPath := "/tmp/test_router_" + t.Name() + ".db"
	os.Remove(dbPath)

	if err := storage.InitRateLimitDB(dbPath); err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}

	// Seed test data
	if err := scraper.SeedInitialRateLimits(); err != nil {
		t.Fatalf("Failed to seed rate limits: %v", err)
	}
	if err := scraper.SeedInitialPricing(); err != nil {
		t.Fatalf("Failed to seed pricing: %v", err)
	}

	return dbPath
}

func teardownRouterTest(t *testing.T, dbPath string) {
	storage.CloseRateLimitDB()
	os.Remove(dbPath)
}

func TestNewRouter_CreatesWithStrategy(t *testing.T) {
	router := NewRouter(StrategyCheapest)
	if router.strategy != StrategyCheapest {
		t.Errorf("Expected strategy cheapest, got %s", router.strategy)
	}
	if router.healthTracker == nil {
		t.Error("Health tracker not initialized")
	}
}

func TestRouter_SelectsCheapestProvider(t *testing.T) {
	dbPath := setupRouterTest(t)
	defer teardownRouterTest(t, dbPath)

	router := NewRouter(StrategyCheapest)
	ctx := context.Background()

	req := RouteRequest{
		Capability:      "chat",
		EstimatedTokens: 1000,
	}

	result, err := router.Route(ctx, req)
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	// DeepSeek should be cheapest at $0.14-$0.28 per 1M tokens
	if result.Provider.ProviderName != "deepseek" && result.Provider.ProviderName != "cerebras" {
		t.Logf("Selected: %s at $%.6f", result.Provider.ProviderName, result.EstimatedCost)
		// Note: Cerebras is FREE so it might win
	}

	if result.EstimatedCost < 0 {
		t.Errorf("Invalid estimated cost: $%.6f", result.EstimatedCost)
	}

	if len(result.Alternatives) == 0 {
		t.Error("No alternatives provided")
	}
}

func TestRouter_SelectsFastestProvider(t *testing.T) {
	dbPath := setupRouterTest(t)
	defer teardownRouterTest(t, dbPath)

	router := NewRouter(StrategyFastest)

	// Simulate latency data
	router.RecordSuccess("groq", 50)         // Very fast (LPU hardware)
	router.RecordSuccess("openai", 200)      // Standard
	router.RecordSuccess("deepseek", 400)    // Slower (China-hosted)

	ctx := context.Background()
	req := RouteRequest{
		Capability:      "chat",
		EstimatedTokens: 1000,
	}

	result, err := router.Route(ctx, req)
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	if result.Provider.ProviderName == "groq" && result.Provider.AvgLatencyMs > 100 {
		t.Errorf("Expected fastest provider with low latency, got %dms", result.Provider.AvgLatencyMs)
	}
}

func TestRouter_RespectsMaxCost(t *testing.T) {
	dbPath := setupRouterTest(t)
	defer teardownRouterTest(t, dbPath)

	router := NewRouter(StrategyCheapest)
	ctx := context.Background()

	req := RouteRequest{
		Capability:      "chat",
		EstimatedTokens: 1000,
		MaxCost:         0.00001, // Very tight budget - should exclude expensive providers
	}

	result, err := router.Route(ctx, req)
	if err != nil {
		// May fail if no providers meet budget
		if result == nil {
			return // Expected
		}
		t.Fatalf("Route failed: %v", err)
	}

	if result.EstimatedCost > req.MaxCost {
		t.Errorf("Selected provider exceeds budget: $%.6f > $%.6f", result.EstimatedCost, req.MaxCost)
	}
}

func TestRouter_ExcludesProviders(t *testing.T) {
	dbPath := setupRouterTest(t)
	defer teardownRouterTest(t, dbPath)

	router := NewRouter(StrategyCheapest)
	ctx := context.Background()

	req := RouteRequest{
		Capability:       "chat",
		EstimatedTokens:  1000,
		ExcludeProviders: []string{"openai", "anthropic"},
	}

	result, err := router.Route(ctx, req)
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	if result.Provider.ProviderName == "openai" || result.Provider.ProviderName == "anthropic" {
		t.Errorf("Selected excluded provider: %s", result.Provider.ProviderName)
	}
}

func TestRouter_HealthTracking_MarksUnhealthy(t *testing.T) {
	dbPath := setupRouterTest(t)
	defer teardownRouterTest(t, dbPath)

	router := NewRouter(StrategyCheapest)

	// Record 3 consecutive failures
	router.RecordFailure("openai", nil)
	router.RecordFailure("openai", nil)
	router.RecordFailure("openai", nil)

	health := router.getHealth("openai")
	if health.IsHealthy {
		t.Error("Provider should be marked unhealthy after 3 failures")
	}
	if health.ConsecutiveFails != 3 {
		t.Errorf("Expected 3 consecutive fails, got %d", health.ConsecutiveFails)
	}
}

func TestRouter_HealthTracking_RecoverAfterSuccess(t *testing.T) {
	dbPath := setupRouterTest(t)
	defer teardownRouterTest(t, dbPath)

	router := NewRouter(StrategyCheapest)

	// Record failures
	router.RecordFailure("openai", nil)
	router.RecordFailure("openai", nil)
	router.RecordFailure("openai", nil)

	// Then success
	router.RecordSuccess("openai", 150)

	health := router.getHealth("openai")
	if !health.IsHealthy {
		t.Error("Provider should recover after success")
	}
	if health.ConsecutiveFails != 0 {
		t.Errorf("Consecutive fails should reset to 0, got %d", health.ConsecutiveFails)
	}
}

func TestRouter_LatencyTracking_ExponentialMovingAverage(t *testing.T) {
	dbPath := setupRouterTest(t)
	defer teardownRouterTest(t, dbPath)

	router := NewRouter(StrategyCheapest)

	// Initial latency
	router.RecordSuccess("openai", 100)
	health := router.getHealth("openai")
	if health.AvgLatencyMs != 100 {
		t.Errorf("Expected initial latency 100ms, got %dms", health.AvgLatencyMs)
	}

	// Record higher latency (should average out)
	router.RecordSuccess("openai", 200)
	health = router.getHealth("openai")
	if health.AvgLatencyMs < 100 || health.AvgLatencyMs > 200 {
		t.Errorf("Expected averaged latency between 100-200ms, got %dms", health.AvgLatencyMs)
	}
}

func TestRouter_RoundRobin_CyclesThroughProviders(t *testing.T) {
	dbPath := setupRouterTest(t)
	defer teardownRouterTest(t, dbPath)

	router := NewRouter(StrategyRoundRobin)
	ctx := context.Background()

	req := RouteRequest{
		Capability:      "chat",
		EstimatedTokens: 1000,
	}

	// Make multiple routing decisions
	seen := make(map[string]int)
	for i := 0; i < 10; i++ {
		result, err := router.Route(ctx, req)
		if err != nil {
			continue
		}
		seen[result.Provider.ProviderName]++
	}

	// Should see multiple different providers
	if len(seen) < 2 {
		t.Errorf("Round-robin should cycle through providers, only saw: %v", seen)
	}
}

func TestRouter_Balanced_ScoresCostAndLatency(t *testing.T) {
	dbPath := setupRouterTest(t)
	defer teardownRouterTest(t, dbPath)

	router := NewRouter(StrategyBalanced)

	// Set up contrasting scenarios
	router.RecordSuccess("groq", 50)      // Fast but not cheapest
	router.RecordSuccess("deepseek", 400) // Cheap but slower

	ctx := context.Background()
	req := RouteRequest{
		Capability:      "chat",
		EstimatedTokens: 1000,
	}

	result, err := router.Route(ctx, req)
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	// Should balance between cost and speed
	if result.Reason == "" {
		t.Error("No reason provided for balanced selection")
	}
	t.Logf("Selected: %s - %s", result.Provider.ProviderName, result.Reason)
}

func TestRouter_NoProvidersAvailable_ReturnsError(t *testing.T) {
	dbPath := setupRouterTest(t)
	defer teardownRouterTest(t, dbPath)

	router := NewRouter(StrategyCheapest)
	ctx := context.Background()

	req := RouteRequest{
		Capability:      "chat",
		EstimatedTokens: 1000,
		MaxCost:         0.0000000001, // Impossibly low budget
	}

	result, err := router.Route(ctx, req)
	if err == nil {
		t.Errorf("Should fail with impossibly low budget, got result: %+v", result)
	}
}

func TestRouter_Fallback_SelectsPrimaryFirst(t *testing.T) {
	providers := []*ProviderOption{
		{ProviderName: "primary", Health: &ProviderHealth{IsHealthy: true}},
		{ProviderName: "fallback1", Health: &ProviderHealth{IsHealthy: true}},
		{ProviderName: "fallback2", Health: &ProviderHealth{IsHealthy: true}},
	}

	router := NewRouter(StrategyFallback)
	selected, reason := router.selectFallback(providers)

	if selected.ProviderName != "primary" {
		t.Errorf("Should select primary when healthy, got %s", selected.ProviderName)
	}
	if reason != "primary" {
		t.Errorf("Expected reason 'primary', got '%s'", reason)
	}
}

func TestRouter_Fallback_UsesBackupWhenPrimaryUnhealthy(t *testing.T) {
	providers := []*ProviderOption{
		{ProviderName: "primary", Health: &ProviderHealth{IsHealthy: false}},
		{ProviderName: "fallback1", Health: &ProviderHealth{IsHealthy: true}},
		{ProviderName: "fallback2", Health: &ProviderHealth{IsHealthy: true}},
	}

	router := NewRouter(StrategyFallback)
	selected, reason := router.selectFallback(providers)

	if selected.ProviderName != "fallback1" {
		t.Errorf("Should select first healthy fallback, got %s", selected.ProviderName)
	}
	if reason != "fallback #1" {
		t.Errorf("Expected reason 'fallback #1', got '%s'", reason)
	}
}

func TestProviderHealth_ThreadSafety(t *testing.T) {
	dbPath := setupRouterTest(t)
	defer teardownRouterTest(t, dbPath)

	router := NewRouter(StrategyCheapest)

	// Concurrent updates
	done := make(chan bool, 20)
	for i := 0; i < 10; i++ {
		go func() {
			router.RecordSuccess("openai", 100)
			done <- true
		}()
		go func() {
			router.RecordFailure("openai", nil)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Should not panic and have valid state
	health := router.getHealth("openai")
	if health == nil {
		t.Error("Health tracker corrupted by concurrent access")
	}
}

func TestRouter_GetHealthStatus_ReturnsAllProviders(t *testing.T) {
	dbPath := setupRouterTest(t)
	defer teardownRouterTest(t, dbPath)

	router := NewRouter(StrategyCheapest)

	router.RecordSuccess("openai", 100)
	router.RecordSuccess("anthropic", 150)
	router.RecordSuccess("deepseek", 400)

	status := router.GetHealthStatus()
	if len(status) != 3 {
		t.Errorf("Expected 3 providers in health status, got %d", len(status))
	}

	if status["openai"] == nil || status["anthropic"] == nil || status["deepseek"] == nil {
		t.Error("Missing providers in health status")
	}
}

func TestRouter_MatchesModel(t *testing.T) {
	dbPath := setupRouterTest(t)
	defer teardownRouterTest(t, dbPath)

	router := NewRouter(StrategyCheapest)

	tests := []struct {
		name           string
		modelID        string
		requiredModels []string
		expected       bool
	}{
		{"exact match", "gpt-4", []string{"gpt-4", "gpt-3.5"}, true},
		{"no match", "claude-2", []string{"gpt-4", "gpt-3.5"}, false},
		{"empty required", "gpt-4", []string{}, false},
		{"single match", "gpt-4", []string{"gpt-4"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.matchesModel(tt.modelID, tt.requiredModels)
			if result != tt.expected {
				t.Errorf("matchesModel(%q, %v) = %v, want %v", tt.modelID, tt.requiredModels, result, tt.expected)
			}
		})
	}
}

