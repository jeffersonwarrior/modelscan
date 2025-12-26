package ratelimit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jeffersonwarrior/modelscan/scraper"
	"github.com/jeffersonwarrior/modelscan/storage"
)

func setupTestDB(t *testing.T) string {
	dbPath := "/tmp/test_ratelimit_" + t.Name() + ".db"
	os.Remove(dbPath)
	
	if err := storage.InitRateLimitDB(dbPath); err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
	
	// Seed test data
	if err := scraper.SeedInitialRateLimits(); err != nil {
		t.Fatalf("Failed to seed test data: %v", err)
	}
	
	return dbPath
}

func teardownTestDB(t *testing.T, dbPath string) {
	storage.CloseRateLimitDB()
	os.Remove(dbPath)
}

func TestNewRateLimiter_LoadsFromDatabase(t *testing.T) {
	dbPath := setupTestDB(t)
	defer teardownTestDB(t, dbPath)

	// Act
	limiter, err := NewRateLimiter("openai", "tier-1")
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}

	// Assert
	if limiter.providerName != "openai" {
		t.Errorf("Expected provider openai, got %s", limiter.providerName)
	}
	if len(limiter.buckets) == 0 {
		t.Error("No buckets loaded from database")
	}

	// Check RPM bucket exists
	bucket, exists := limiter.buckets["rpm"]
	if !exists {
		t.Fatal("RPM bucket not loaded")
	}
	if bucket.capacity != 500 { // tier-1 rpm = 500
		t.Errorf("Expected RPM capacity 500, got %d", bucket.capacity)
	}
}

func TestTokenBucket_Acquire_Success(t *testing.T) {
	bucket := &TokenBucket{
		capacity:       100,
		tokens:         100,
		refillRate:     100,
		refillInterval: time.Minute,
		lastRefill:     time.Now(),
	}

	ctx := context.Background()
	err := bucket.Acquire(ctx, 10)
	if err != nil {
		t.Errorf("Acquire failed: %v", err)
	}

	available := bucket.GetAvailableTokens()
	if available != 90 {
		t.Errorf("Expected 90 tokens, got %d", available)
	}
}

func TestTokenBucket_Acquire_Exhaustion(t *testing.T) {
	bucket := &TokenBucket{
		capacity:       10,
		tokens:         10,
		refillRate:     10,
		refillInterval: 100 * time.Millisecond,
		lastRefill:     time.Now(),
	}

	ctx := context.Background()

	// Exhaust all tokens
	if err := bucket.Acquire(ctx, 10); err != nil {
		t.Fatalf("First acquire failed: %v", err)
	}

	// Try to acquire more immediately (should wait)
	start := time.Now()
	if err := bucket.Acquire(ctx, 5); err != nil {
		t.Fatalf("Second acquire failed: %v", err)
	}
	elapsed := time.Since(start)

	// Should have waited for refill (at least 80ms, allowing 20ms tolerance)
	if elapsed < 80*time.Millisecond {
		t.Errorf("Expected wait time ~100ms, got %v", elapsed)
	}
}

func TestTokenBucket_Acquire_ContextCancellation(t *testing.T) {
	bucket := &TokenBucket{
		capacity:       10,
		tokens:         0, // Empty bucket
		refillRate:     10,
		refillInterval: time.Hour, // Very slow refill
		lastRefill:     time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := bucket.Acquire(ctx, 5)
	if err == nil {
		t.Error("Expected context deadline exceeded error")
	}
	if ctx.Err() == nil {
		t.Error("Context should be cancelled")
	}
}

func TestTokenBucket_Refill_AddsTokens(t *testing.T) {
	bucket := &TokenBucket{
		capacity:       100,
		tokens:         50,
		refillRate:     100,
		refillInterval: 100 * time.Millisecond,
		lastRefill:     time.Now().Add(-100 * time.Millisecond), // Last refill was 100ms ago
	}

	bucket.mu.Lock()
	bucket.refill()
	bucket.mu.Unlock()

	available := bucket.GetAvailableTokens()
	if available < 100 {
		t.Errorf("Expected full refill to 100 tokens, got %d", available)
	}
}

func TestRateLimiter_Acquire_MultipleTypes(t *testing.T) {
	dbPath := setupTestDB(t)
	defer teardownTestDB(t, dbPath)

	limiter, err := NewRateLimiter("openai", "tier-1")
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}

	ctx := context.Background()

	// Acquire RPM
	if err := limiter.Acquire(ctx, "rpm", 1); err != nil {
		t.Errorf("RPM acquire failed: %v", err)
	}

	// Acquire TPM
	if err := limiter.Acquire(ctx, "tpm", 1000); err != nil {
		t.Errorf("TPM acquire failed: %v", err)
	}

	// Check info
	info := limiter.GetRateLimitInfo()
	if info["rpm"]["available"].(int64) >= 500 {
		t.Error("RPM tokens should have decreased")
	}
	if info["tpm"]["available"].(int64) >= 200000 {
		t.Error("TPM tokens should have decreased")
	}
}

func TestMultiLimitCoordinator_AcquireAll_Success(t *testing.T) {
	dbPath := setupTestDB(t)
	defer teardownTestDB(t, dbPath)

	limiter1, _ := NewRateLimiter("openai", "tier-1")
	limiter2, _ := NewRateLimiter("anthropic", "tier-1")

	coordinator := NewMultiLimitCoordinator(limiter1, limiter2)

	ctx := context.Background()
	err := coordinator.AcquireAll(ctx, 1, 1000)
	if err != nil {
		t.Errorf("AcquireAll failed: %v", err)
	}

	// Check both limiters consumed tokens
	info1 := limiter1.GetRateLimitInfo()
	info2 := limiter2.GetRateLimitInfo()

	if info1["rpm"]["available"].(int64) >= 500 {
		t.Error("Limiter1 RPM should have decreased")
	}
	if info2["rpm"]["available"].(int64) >= 50 {
		t.Error("Limiter2 RPM should have decreased")
	}
}

func TestMultiLimitCoordinator_AcquireAll_Rollback(t *testing.T) {
	dbPath := setupTestDB(t)
	defer teardownTestDB(t, dbPath)

	// Create limiter with very low RPM
	storage.InsertRateLimit(storage.RateLimit{
		ProviderName:       "test-provider",
		PlanType:           "test",
		LimitType:          "rpm",
		LimitValue:         2,
		ResetWindowSeconds: 60,
		AppliesTo:          "account",
		LastVerified:       time.Now(),
	})

	limiter1, _ := NewRateLimiter("openai", "tier-1")
	limiter2, _ := NewRateLimiter("test-provider", "test")

	coordinator := NewMultiLimitCoordinator(limiter1, limiter2)

	// Exhaust test-provider's RPM
	ctx := context.Background()
	limiter2.Acquire(ctx, "rpm", 2)

	// Get initial state
	info1Before := limiter1.GetRateLimitInfo()
	rpm1Before := info1Before["rpm"]["available"].(int64)

	// Try to acquire more (should fail and rollback)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	err := coordinator.AcquireAll(ctx, 1, 0)
	if err == nil {
		t.Error("Expected AcquireAll to fail due to test-provider exhaustion")
	}

	// Check that limiter1 tokens were rolled back
	time.Sleep(10 * time.Millisecond) // Allow rollback to complete
	info1After := limiter1.GetRateLimitInfo()
	rpm1After := info1After["rpm"]["available"].(int64)

	if rpm1After < rpm1Before-5 {
		t.Errorf("Limiter1 tokens not rolled back properly: before=%d, after=%d", rpm1Before, rpm1After)
	}
}

func TestEstimateTokens_ReasonableApproximation(t *testing.T) {
	tests := []struct {
		text     string
		expected int64
	}{
		{"Hello world", 2},                           // 11 chars / 4 = 2
		{"The quick brown fox", 4},                   // 19 chars / 4 = 4
		{"A much longer piece of text here", 8},      // 34 chars / 4 = 8
		{"", 0},                                       // Empty string
	}

	for _, test := range tests {
		result := EstimateTokens(test.text)
		if result != test.expected {
			t.Errorf("EstimateTokens(%q) = %d, expected %d", test.text, result, test.expected)
		}
	}
}

func TestRateLimiter_BurstAllowance(t *testing.T) {
	// Insert rate limit with burst allowance
	dbPath := setupTestDB(t)
	defer teardownTestDB(t, dbPath)

	storage.InsertRateLimit(storage.RateLimit{
		ProviderName:       "test-burst",
		PlanType:           "test",
		LimitType:          "rpm",
		LimitValue:         100,
		BurstAllowance:     50, // 50% burst
		ResetWindowSeconds: 60,
		AppliesTo:          "account",
		LastVerified:       time.Now(),
	})

	limiter, err := NewRateLimiter("test-burst", "test")
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}

	bucket := limiter.buckets["rpm"]
	if bucket.capacity != 150 { // 100 + 50 burst
		t.Errorf("Expected capacity 150 with burst, got %d", bucket.capacity)
	}
	if bucket.tokens != 150 {
		t.Errorf("Expected initial tokens 150, got %d", bucket.tokens)
	}
}

func TestRateLimiter_NoLimitType_AllowsImmediate(t *testing.T) {
	dbPath := setupTestDB(t)
	defer teardownTestDB(t, dbPath)

	limiter, _ := NewRateLimiter("openai", "tier-1")

	ctx := context.Background()
	// Request a limit type that doesn't exist
	err := limiter.Acquire(ctx, "nonexistent", 9999999)
	if err != nil {
		t.Errorf("Should allow requests for non-existent limit types, got error: %v", err)
	}
}
