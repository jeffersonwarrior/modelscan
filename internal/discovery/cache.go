package discovery

import (
	"sync"
	"time"
)

// CacheStats tracks cache performance metrics
type CacheStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
	TotalSize int
	HitRate   float64
}

// Cache stores discovery results with TTL
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration

	// Statistics
	hits      int64
	misses    int64
	evictions int64
}

// cacheEntry represents a cached result
type cacheEntry struct {
	result    *DiscoveryResult
	expiresAt time.Time
}

// NewCache creates a new cache with specified TTL
func NewCache(ttl time.Duration) *Cache {
	c := &Cache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
	}

	// Start background cleanup goroutine
	go c.cleanup()

	return c
}

// Get retrieves a cached result if not expired
func (c *Cache) Get(identifier string) (*DiscoveryResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[identifier]
	if !ok {
		c.misses++
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		c.misses++
		return nil, false
	}

	c.hits++
	return entry.result, true
}

// Set stores a result in the cache
func (c *Cache) Set(identifier string, result *DiscoveryResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[identifier] = &cacheEntry{
		result:    result,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Delete removes a result from the cache
func (c *Cache) Delete(identifier string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, identifier)
}

// Clear removes all cached results
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
}

// cleanup periodically removes expired entries
func (c *Cache) cleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		c.removeExpired()
	}
}

// removeExpired removes all expired entries
func (c *Cache) removeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for id, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, id)
			c.evictions++
		}
	}
}

// GetStats returns cache statistics
func (c *Cache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(c.hits) / float64(total) * 100
	}

	return CacheStats{
		Hits:      c.hits,
		Misses:    c.misses,
		Evictions: c.evictions,
		TotalSize: len(c.entries),
		HitRate:   hitRate,
	}
}

// ResetStats resets cache statistics
func (c *Cache) ResetStats() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.hits = 0
	c.misses = 0
	c.evictions = 0
}

// Size returns the number of cached entries
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.entries)
}
