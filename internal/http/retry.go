package http

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"net/http"
	"time"
)

// RetryConfig configures the retry behavior for HTTP requests.
type RetryConfig struct {
	MaxAttempts   int           // Maximum number of retry attempts (default: 3)
	BaseDelay     time.Duration // Initial delay before first retry (default: 1s)
	MaxDelay      time.Duration // Maximum delay between retries (default: 60s)
	Multiplier    float64       // Backoff multiplier (default: 2.0)
	JitterPercent float64       // Jitter as a percentage (default: 0.1 = 10%)
}

// setDefaults fills in default values for zero-valued fields.
func (r *RetryConfig) setDefaults() {
	if r.MaxAttempts == 0 {
		r.MaxAttempts = 3
	}
	if r.BaseDelay == 0 {
		r.BaseDelay = 1 * time.Second
	}
	if r.MaxDelay == 0 {
		r.MaxDelay = 60 * time.Second
	}
	if r.Multiplier == 0 {
		r.Multiplier = 2.0
	}
	if r.JitterPercent == 0 {
		r.JitterPercent = 0.1
	}
}

// shouldRetry determines if an HTTP request should be retried based on the
// response status code and error.
//
// Retry conditions:
//   - 429 Too Many Requests
//   - 500 Internal Server Error
//   - 502 Bad Gateway
//   - 503 Service Unavailable
//   - 504 Gateway Timeout
//
// Do NOT retry:
//   - 2xx Success responses
//   - 4xx Client errors (except 429)
//   - Context cancellation or deadline exceeded
//   - Network errors (to avoid retry loops on persistent network issues)
func shouldRetry(resp *http.Response, err error) bool {
	// Don't retry if context was canceled or deadline exceeded
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
	}

	// If we don't have a response, don't retry
	// (likely a network error that won't be fixed by retrying)
	if resp == nil {
		return false
	}

	// Retry on rate limits and server errors
	switch resp.StatusCode {
	case 429, 500, 502, 503, 504:
		return true
	default:
		return false
	}
}

// calculateBackoff computes the delay before the next retry attempt using
// exponential backoff with jitter.
//
// Formula: delay = min(baseDelay * multiplier^attempt, maxDelay)
// Jitter: delay *= (1 ± jitterPercent)
//
// The jitter helps prevent thundering herd problems when multiple clients
// retry simultaneously.
func calculateBackoff(cfg *RetryConfig, attempt int) time.Duration {
	// Handle zero/nil config gracefully
	if cfg.BaseDelay == 0 || cfg.Multiplier == 0 {
		return 0
	}

	// Calculate exponential backoff: baseDelay * multiplier^attempt
	delay := float64(cfg.BaseDelay) * math.Pow(cfg.Multiplier, float64(attempt))

	// Cap at maximum delay
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	// Apply jitter if configured
	if cfg.JitterPercent > 0 {
		// Generate random jitter: ±jitterPercent
		// rand.Float64() returns [0.0, 1.0)
		// We want [-jitterPercent, +jitterPercent]
		jitter := (rand.Float64()*2 - 1) * cfg.JitterPercent
		delay = delay * (1 + jitter)

		// Ensure we don't go negative or exceed MaxDelay after jitter
		if delay < 0 {
			delay = 0
		}
		if delay > float64(cfg.MaxDelay) {
			delay = float64(cfg.MaxDelay)
		}
	}

	return time.Duration(delay)
}
