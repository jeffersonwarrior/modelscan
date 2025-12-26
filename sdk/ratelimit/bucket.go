package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jeffersonwarrior/modelscan/storage"
)

// TokenBucket implements the token bucket rate limiting algorithm
type TokenBucket struct {
	capacity       int64         // Maximum tokens in bucket
	tokens         int64         // Current tokens available
	refillRate     int64         // Tokens added per refill interval
	refillInterval time.Duration // How often to refill
	lastRefill     time.Time     // Last refill timestamp
	mu             sync.Mutex
}

// RateLimiter manages multiple token buckets for different limit types
type RateLimiter struct {
	providerName string
	planType     string
	buckets      map[string]*TokenBucket // key: limit_type (rpm, tpm, etc.)
	mu           sync.RWMutex
}

// NewRateLimiter creates a rate limiter from database configuration
func NewRateLimiter(providerName, planType string) (*RateLimiter, error) {
	limits, err := storage.GetAllRateLimitsForProvider(providerName, planType)
	if err != nil {
		return nil, fmt.Errorf("failed to load rate limits: %w", err)
	}

	if len(limits) == 0 {
		return nil, fmt.Errorf("no rate limits found for provider=%s plan=%s", providerName, planType)
	}

	rl := &RateLimiter{
		providerName: providerName,
		planType:     planType,
		buckets:      make(map[string]*TokenBucket),
	}

	for _, limit := range limits {
		bucket := &TokenBucket{
			capacity:       limit.LimitValue + limit.BurstAllowance,
			tokens:         limit.LimitValue + limit.BurstAllowance,
			refillRate:     limit.LimitValue,
			refillInterval: time.Duration(limit.ResetWindowSeconds) * time.Second,
			lastRefill:     time.Now(),
		}
		rl.buckets[limit.LimitType] = bucket
	}

	return rl, nil
}

// Acquire attempts to acquire n tokens from the specified bucket
func (rl *RateLimiter) Acquire(ctx context.Context, limitType string, tokens int64) error {
	rl.mu.RLock()
	bucket, exists := rl.buckets[limitType]
	rl.mu.RUnlock()

	if !exists {
		// No rate limit for this type, allow immediately
		return nil
	}

	return bucket.Acquire(ctx, tokens)
}

// Acquire attempts to acquire n tokens from the bucket
func (tb *TokenBucket) Acquire(ctx context.Context, n int64) error {
	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		tb.mu.Lock()
		tb.refill()

		if tb.tokens >= n {
			tb.tokens -= n
			tb.mu.Unlock()
			return nil
		}

		// Calculate wait time for next refill
		waitTime := tb.refillInterval - time.Since(tb.lastRefill)
		tb.mu.Unlock()

		if waitTime <= 0 {
			waitTime = 10 * time.Millisecond
		}

		// Wait for refill or context cancellation
		timer := time.NewTimer(waitTime)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			// Retry after wait
		}
	}
}

// refill adds tokens to the bucket based on elapsed time
// Must be called with tb.mu locked
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)

	if elapsed >= tb.refillInterval && tb.refillInterval > 0 {
		// Calculate how many full refill periods have elapsed
		periods := elapsed / tb.refillInterval
		tokensToAdd := int64(periods) * tb.refillRate
		tb.tokens = min(tb.capacity, tb.tokens+tokensToAdd)
		tb.lastRefill = now
	}
}

// GetAvailableTokens returns current available tokens (thread-safe)
func (tb *TokenBucket) GetAvailableTokens() int64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	return tb.tokens
}

// MultiLimitCoordinator manages multiple rate limiters that must all succeed
type MultiLimitCoordinator struct {
	limiters []*RateLimiter
}

// NewMultiLimitCoordinator creates a coordinator for multiple limiters
func NewMultiLimitCoordinator(limiters ...*RateLimiter) *MultiLimitCoordinator {
	return &MultiLimitCoordinator{limiters: limiters}
}

// AcquireAll attempts to acquire tokens from all limiters (RPM + TPM)
func (mlc *MultiLimitCoordinator) AcquireAll(ctx context.Context, rpm, tpm int64) error {
	// Try to acquire from all limiters
	acquired := make([]struct {
		limiter   *RateLimiter
		limitType string
		tokens    int64
	}, 0, len(mlc.limiters)*2)

	for _, limiter := range mlc.limiters {
		// Acquire RPM
		if rpm > 0 {
			if err := limiter.Acquire(ctx, "rpm", rpm); err != nil {
				// Rollback already acquired tokens
				mlc.rollback(acquired)
				return fmt.Errorf("rpm limit exceeded for %s: %w", limiter.providerName, err)
			}
			acquired = append(acquired, struct {
				limiter   *RateLimiter
				limitType string
				tokens    int64
			}{limiter, "rpm", rpm})
		}

		// Acquire TPM
		if tpm > 0 {
			if err := limiter.Acquire(ctx, "tpm", tpm); err != nil {
				// Rollback already acquired tokens
				mlc.rollback(acquired)
				return fmt.Errorf("tpm limit exceeded for %s: %w", limiter.providerName, err)
			}
			acquired = append(acquired, struct {
				limiter   *RateLimiter
				limitType string
				tokens    int64
			}{limiter, "tpm", tpm})
		}
	}

	return nil
}

// rollback returns tokens to buckets
func (mlc *MultiLimitCoordinator) rollback(acquired []struct {
	limiter   *RateLimiter
	limitType string
	tokens    int64
}) {
	for _, acq := range acquired {
		acq.limiter.mu.RLock()
		if bucket, exists := acq.limiter.buckets[acq.limitType]; exists {
			bucket.mu.Lock()
			bucket.tokens += acq.tokens
			if bucket.tokens > bucket.capacity {
				bucket.tokens = bucket.capacity
			}
			bucket.mu.Unlock()
		}
		acq.limiter.mu.RUnlock()
	}
}

// EstimateTokens estimates tokens for a text string (rough approximation)
func EstimateTokens(text string) int64 {
	// Rough estimate: 1 token ≈ 4 characters for English text
	return int64(len(text) / 4)
}

// GetRateLimitInfo returns current status of all buckets
func (rl *RateLimiter) GetRateLimitInfo() map[string]map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	info := make(map[string]map[string]interface{})
	for limitType, bucket := range rl.buckets {
		info[limitType] = map[string]interface{}{
			"capacity":  bucket.capacity,
			"available": bucket.GetAvailableTokens(),
			"refill":    bucket.refillRate,
			"interval":  bucket.refillInterval.String(),
		}
	}
	return info
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
