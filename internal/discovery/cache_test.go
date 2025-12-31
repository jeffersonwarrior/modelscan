package discovery

import (
	"testing"
	"time"
)

func TestCacheStatistics(t *testing.T) {
	cache := NewCache(1 * time.Hour)

	// Test miss tracking
	_, ok := cache.Get("nonexistent")
	if ok {
		t.Error("Expected cache miss")
	}

	stats := cache.GetStats()
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}
	if stats.Hits != 0 {
		t.Errorf("Expected 0 hits, got %d", stats.Hits)
	}

	// Add entry and test hit tracking
	result := &DiscoveryResult{
		Provider: ProviderInfo{ID: "test"},
	}
	cache.Set("test", result)

	// Test multiple hits
	for i := 0; i < 5; i++ {
		_, ok := cache.Get("test")
		if !ok {
			t.Error("Expected cache hit")
		}
	}

	stats = cache.GetStats()
	if stats.Hits != 5 {
		t.Errorf("Expected 5 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss (from earlier), got %d", stats.Misses)
	}

	// Test hit rate calculation
	expectedHitRate := 5.0 / 6.0 * 100 // 5 hits out of 6 total attempts
	if stats.HitRate < expectedHitRate-0.1 || stats.HitRate > expectedHitRate+0.1 {
		t.Errorf("Expected hit rate ~%.2f%%, got %.2f%%", expectedHitRate, stats.HitRate)
	}

	// Test size tracking
	if stats.TotalSize != 1 {
		t.Errorf("Expected cache size 1, got %d", stats.TotalSize)
	}

	// Add more entries
	for i := 0; i < 10; i++ {
		cache.Set(string(rune('a'+i)), &DiscoveryResult{
			Provider: ProviderInfo{ID: string(rune('a' + i))},
		})
	}

	stats = cache.GetStats()
	if stats.TotalSize != 11 {
		t.Errorf("Expected cache size 11, got %d", stats.TotalSize)
	}
}

func TestCacheExpiration(t *testing.T) {
	cache := NewCache(100 * time.Millisecond)

	result := &DiscoveryResult{
		Provider: ProviderInfo{ID: "test"},
	}
	cache.Set("test", result)

	// Immediate get should hit
	_, ok := cache.Get("test")
	if !ok {
		t.Error("Expected immediate cache hit")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should miss after expiration
	_, ok = cache.Get("test")
	if ok {
		t.Error("Expected cache miss after expiration")
	}

	stats := cache.GetStats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss (after expiration), got %d", stats.Misses)
	}
}

func TestCacheEviction(t *testing.T) {
	cache := NewCache(50 * time.Millisecond)

	// Add multiple entries
	for i := 0; i < 5; i++ {
		cache.Set(string(rune('a'+i)), &DiscoveryResult{
			Provider: ProviderInfo{ID: string(rune('a' + i))},
		})
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Trigger manual cleanup
	cache.removeExpired()

	stats := cache.GetStats()
	if stats.Evictions != 5 {
		t.Errorf("Expected 5 evictions, got %d", stats.Evictions)
	}
	if stats.TotalSize != 0 {
		t.Errorf("Expected cache size 0 after eviction, got %d", stats.TotalSize)
	}
}

func TestCacheResetStats(t *testing.T) {
	cache := NewCache(1 * time.Hour)

	// Generate some activity
	cache.Set("test", &DiscoveryResult{Provider: ProviderInfo{ID: "test"}})
	cache.Get("test")    // hit
	cache.Get("missing") // miss

	stats := cache.GetStats()
	if stats.Hits == 0 || stats.Misses == 0 {
		t.Error("Expected some hits and misses before reset")
	}

	// Reset stats
	cache.ResetStats()

	stats = cache.GetStats()
	if stats.Hits != 0 {
		t.Errorf("Expected 0 hits after reset, got %d", stats.Hits)
	}
	if stats.Misses != 0 {
		t.Errorf("Expected 0 misses after reset, got %d", stats.Misses)
	}
	if stats.Evictions != 0 {
		t.Errorf("Expected 0 evictions after reset, got %d", stats.Evictions)
	}

	// Cache entries should still exist
	if stats.TotalSize != 1 {
		t.Errorf("Expected cache size 1 (reset doesn't clear entries), got %d", stats.TotalSize)
	}
}

func TestCacheConcurrentAccess(t *testing.T) {
	cache := NewCache(1 * time.Hour)

	// Add initial entry
	cache.Set("shared", &DiscoveryResult{Provider: ProviderInfo{ID: "shared"}})

	// Launch concurrent readers and writers
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				// Read
				cache.Get("shared")
				cache.Get("missing")

				// Write
				cache.Set(string(rune('a'+id)), &DiscoveryResult{
					Provider: ProviderInfo{ID: string(rune('a' + id))},
				})
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify stats were tracked (should be 10*100*2 = 2000 gets)
	stats := cache.GetStats()
	total := stats.Hits + stats.Misses
	if total != 2000 {
		t.Errorf("Expected 2000 total cache accesses, got %d", total)
	}

	// Should have some cache entries
	if stats.TotalSize == 0 {
		t.Error("Expected cache to have entries after concurrent writes")
	}
}

func TestCacheHitRateCalculation(t *testing.T) {
	cache := NewCache(1 * time.Hour)

	// Test 0% hit rate (all misses)
	for i := 0; i < 10; i++ {
		cache.Get("missing")
	}

	stats := cache.GetStats()
	if stats.HitRate != 0.0 {
		t.Errorf("Expected 0%% hit rate with all misses, got %.2f%%", stats.HitRate)
	}

	// Add entry
	cache.Set("test", &DiscoveryResult{Provider: ProviderInfo{ID: "test"}})

	// Test 50% hit rate
	for i := 0; i < 10; i++ {
		cache.Get("test") // hit
	}

	stats = cache.GetStats()
	expectedHitRate := 10.0 / 20.0 * 100 // 10 hits out of 20 total
	if stats.HitRate < expectedHitRate-0.1 || stats.HitRate > expectedHitRate+0.1 {
		t.Errorf("Expected ~%.2f%% hit rate, got %.2f%%", expectedHitRate, stats.HitRate)
	}

	// Test 100% hit rate (reset and only hits)
	cache.ResetStats()
	for i := 0; i < 10; i++ {
		cache.Get("test") // all hits
	}

	stats = cache.GetStats()
	if stats.HitRate != 100.0 {
		t.Errorf("Expected 100%% hit rate with all hits, got %.2f%%", stats.HitRate)
	}
}
