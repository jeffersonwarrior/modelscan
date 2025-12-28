package keymanager

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// KeyManager manages API keys with round-robin selection and degradation tracking
type KeyManager struct {
	mu       sync.RWMutex
	db       Database
	cache    map[string][]*APIKey // provider -> sorted keys
	cacheTTL time.Duration
}

// Database interface for key storage
type Database interface {
	ListActiveAPIKeys(providerID string) ([]*APIKey, error)
	IncrementKeyUsage(keyID int, tokens int) error
	MarkKeyDegraded(keyID int, until time.Time) error
	ResetKeyLimits(keyID int) error
	GetAPIKey(id int) (*APIKey, error)
}

// APIKey represents an API key
type APIKey struct {
	ID            int
	ProviderID    string
	KeyHash       string
	KeyPrefix     *string
	Tier          string
	RPMLimit      *int
	TPMLimit      *int
	DailyLimit    *int
	ResetInterval *string
	LastReset     time.Time
	RequestsCount int
	TokensCount   int
	Active        bool
	Degraded      bool
	DegradedUntil *time.Time
	CreatedAt     time.Time
	actualKey     string // Stored temporarily, not persisted
}

// Config holds key manager configuration
type Config struct {
	CacheTTL        time.Duration
	DegradeDuration time.Duration // How long to mark key as degraded on error
}

// NewKeyManager creates a new key manager
func NewKeyManager(db Database, cfg Config) *KeyManager {
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 5 * time.Minute
	}
	if cfg.DegradeDuration == 0 {
		cfg.DegradeDuration = 15 * time.Minute
	}

	km := &KeyManager{
		db:       db,
		cache:    make(map[string][]*APIKey),
		cacheTTL: cfg.CacheTTL,
	}

	// Start background refresh
	go km.refreshLoop()

	return km
}

// GetKey selects the best API key for a provider using round-robin (lowest usage)
func (km *KeyManager) GetKey(ctx context.Context, providerID string) (*APIKey, error) {
	km.mu.RLock()
	keys, ok := km.cache[providerID]
	km.mu.RUnlock()

	if !ok || len(keys) == 0 {
		// Load from database
		if err := km.refreshCache(providerID); err != nil {
			return nil, fmt.Errorf("failed to load keys: %w", err)
		}

		km.mu.RLock()
		keys = km.cache[providerID]
		km.mu.RUnlock()

		if len(keys) == 0 {
			return nil, fmt.Errorf("no active keys for provider %s", providerID)
		}
	}

	// Check for degraded keys that can be re-enabled
	now := time.Now()
	for _, key := range keys {
		if key.Degraded && key.DegradedUntil != nil && now.After(*key.DegradedUntil) {
			// Re-enable key
			key.Degraded = false
			key.DegradedUntil = nil
		}
	}

	// Find key with lowest usage (round-robin)
	var bestKey *APIKey
	minUsage := int(^uint(0) >> 1) // Max int

	for _, key := range keys {
		if key.Degraded {
			continue
		}

		// Check rate limits
		if key.RPMLimit != nil && key.RequestsCount >= *key.RPMLimit {
			continue
		}
		if key.TPMLimit != nil && key.TokensCount >= *key.TPMLimit {
			continue
		}
		if key.DailyLimit != nil && key.RequestsCount >= *key.DailyLimit {
			continue
		}

		// Select key with lowest combined usage
		usage := key.RequestsCount + (key.TokensCount / 1000) // Weight tokens less
		if usage < minUsage {
			minUsage = usage
			bestKey = key
		}
	}

	if bestKey == nil {
		return nil, fmt.Errorf("all keys for %s are rate limited or degraded", providerID)
	}

	return bestKey, nil
}

// RecordUsage records API key usage
func (km *KeyManager) RecordUsage(ctx context.Context, keyID int, tokens int) error {
	return km.db.IncrementKeyUsage(keyID, tokens)
}

// MarkDegraded marks a key as degraded after an error
func (km *KeyManager) MarkDegraded(ctx context.Context, keyID int, duration time.Duration) error {
	until := time.Now().Add(duration)
	if err := km.db.MarkKeyDegraded(keyID, until); err != nil {
		return err
	}

	// Update cache
	key, err := km.db.GetAPIKey(keyID)
	if err != nil {
		return err
	}

	km.mu.Lock()
	defer km.mu.Unlock()

	if keys, ok := km.cache[key.ProviderID]; ok {
		for i, k := range keys {
			if k.ID == keyID {
				km.cache[key.ProviderID][i] = key
				break
			}
		}
	}

	return nil
}

// ResetLimits resets rate limits for a key (called on interval)
func (km *KeyManager) ResetLimits(ctx context.Context, keyID int) error {
	return km.db.ResetKeyLimits(keyID)
}

// refreshCache refreshes the key cache for a provider
func (km *KeyManager) refreshCache(providerID string) error {
	keys, err := km.db.ListActiveAPIKeys(providerID)
	if err != nil {
		return err
	}

	km.mu.Lock()
	km.cache[providerID] = keys
	km.mu.Unlock()

	return nil
}

// refreshLoop periodically refreshes the key cache
func (km *KeyManager) refreshLoop() {
	ticker := time.NewTicker(km.cacheTTL)
	defer ticker.Stop()

	for range ticker.C {
		km.mu.RLock()
		providers := make([]string, 0, len(km.cache))
		for provider := range km.cache {
			providers = append(providers, provider)
		}
		km.mu.RUnlock()

		// Refresh each provider's keys
		for _, provider := range providers {
			km.refreshCache(provider)
		}
	}
}

// ListKeys lists all keys for a provider
func (km *KeyManager) ListKeys(ctx context.Context, providerID string) ([]*APIKey, error) {
	return km.db.ListActiveAPIKeys(providerID)
}

// Close cleans up resources
func (km *KeyManager) Close() error {
	return nil
}
