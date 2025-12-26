package http

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		err        error
		want       bool
	}{
		// Should retry
		{name: "429 Too Many Requests", statusCode: 429, want: true},
		{name: "500 Internal Server Error", statusCode: 500, want: true},
		{name: "502 Bad Gateway", statusCode: 502, want: true},
		{name: "503 Service Unavailable", statusCode: 503, want: true},
		{name: "504 Gateway Timeout", statusCode: 504, want: true},

		// Should NOT retry
		{name: "200 OK", statusCode: 200, want: false},
		{name: "201 Created", statusCode: 201, want: false},
		{name: "400 Bad Request", statusCode: 400, want: false},
		{name: "401 Unauthorized", statusCode: 401, want: false},
		{name: "403 Forbidden", statusCode: 403, want: false},
		{name: "404 Not Found", statusCode: 404, want: false},
		{name: "422 Unprocessable Entity", statusCode: 422, want: false},

		// Edge cases
		{name: "0 status with error", statusCode: 0, err: errors.New("network error"), want: false},
		{name: "Context canceled", statusCode: 0, err: context.Canceled, want: false},
		{name: "Context deadline exceeded", statusCode: 0, err: context.DeadlineExceeded, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: tt.statusCode}
			got := shouldRetry(resp, tt.err)
			if got != tt.want {
				t.Errorf("shouldRetry(status=%d, err=%v) = %v, want %v",
					tt.statusCode, tt.err, got, tt.want)
			}
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	cfg := &RetryConfig{
		BaseDelay:     1 * time.Second,
		MaxDelay:      60 * time.Second,
		Multiplier:    2.0,
		JitterPercent: 0.1,
	}

	tests := []struct {
		name    string
		attempt int
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name:    "First retry (attempt 0)",
			attempt: 0,
			wantMin: 900 * time.Millisecond,  // 1s - 10% jitter
			wantMax: 1100 * time.Millisecond, // 1s + 10% jitter
		},
		{
			name:    "Second retry (attempt 1)",
			attempt: 1,
			wantMin: 1800 * time.Millisecond, // 2s - 10% jitter
			wantMax: 2200 * time.Millisecond, // 2s + 10% jitter
		},
		{
			name:    "Third retry (attempt 2)",
			attempt: 2,
			wantMin: 3600 * time.Millisecond, // 4s - 10% jitter
			wantMax: 4400 * time.Millisecond, // 4s + 10% jitter
		},
		{
			name:    "Large attempt - should cap at MaxDelay",
			attempt: 10,
			wantMin: 54 * time.Second, // 60s - 10% jitter
			wantMax: 60 * time.Second, // capped at MaxDelay
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run multiple times to verify jitter is working
			for i := 0; i < 10; i++ {
				got := calculateBackoff(cfg, tt.attempt)

				if got < tt.wantMin || got > tt.wantMax {
					t.Errorf("calculateBackoff(attempt=%d) = %v, want between %v and %v",
						tt.attempt, got, tt.wantMin, tt.wantMax)
				}
			}
		})
	}
}

func TestCalculateBackoffNoJitter(t *testing.T) {
	cfg := &RetryConfig{
		BaseDelay:     1 * time.Second,
		MaxDelay:      60 * time.Second,
		Multiplier:    2.0,
		JitterPercent: 0.0, // No jitter
	}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 0, want: 1 * time.Second},
		{attempt: 1, want: 2 * time.Second},
		{attempt: 2, want: 4 * time.Second},
		{attempt: 3, want: 8 * time.Second},
		{attempt: 4, want: 16 * time.Second},
		{attempt: 5, want: 32 * time.Second},
		{attempt: 6, want: 60 * time.Second},  // Capped at MaxDelay
		{attempt: 10, want: 60 * time.Second}, // Still capped
	}

	for _, tt := range tests {
		got := calculateBackoff(cfg, tt.attempt)
		if got != tt.want {
			t.Errorf("calculateBackoff(attempt=%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestCalculateBackoffDifferentMultiplier(t *testing.T) {
	cfg := &RetryConfig{
		BaseDelay:     100 * time.Millisecond,
		MaxDelay:      10 * time.Second,
		Multiplier:    3.0, // Triple each time
		JitterPercent: 0.0,
	}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 0, want: 100 * time.Millisecond},
		{attempt: 1, want: 300 * time.Millisecond},
		{attempt: 2, want: 900 * time.Millisecond},
		{attempt: 3, want: 2700 * time.Millisecond},
		{attempt: 4, want: 8100 * time.Millisecond},
		{attempt: 5, want: 10 * time.Second}, // Capped
	}

	for _, tt := range tests {
		got := calculateBackoff(cfg, tt.attempt)
		if got != tt.want {
			t.Errorf("calculateBackoff(attempt=%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestJitterDistribution(t *testing.T) {
	cfg := &RetryConfig{
		BaseDelay:     1 * time.Second,
		MaxDelay:      60 * time.Second,
		Multiplier:    2.0,
		JitterPercent: 0.2, // 20% jitter
	}

	// Collect many samples
	samples := make([]time.Duration, 1000)
	for i := 0; i < 1000; i++ {
		samples[i] = calculateBackoff(cfg, 0)
	}

	// Verify all samples are within expected range
	minExpected := 800 * time.Millisecond  // 1s - 20%
	maxExpected := 1200 * time.Millisecond // 1s + 20%

	for i, sample := range samples {
		if sample < minExpected || sample > maxExpected {
			t.Errorf("sample[%d] = %v, want between %v and %v", i, sample, minExpected, maxExpected)
		}
	}

	// Verify we got some variety (not all the same value)
	allSame := true
	first := samples[0]
	for _, sample := range samples[1:] {
		if sample != first {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("All backoff samples are identical - jitter not working")
	}
}

func TestRetryConfigDefaults(t *testing.T) {
	cfg := &RetryConfig{}
	cfg.setDefaults()

	if cfg.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", cfg.MaxAttempts)
	}
	if cfg.BaseDelay != 1*time.Second {
		t.Errorf("BaseDelay = %v, want 1s", cfg.BaseDelay)
	}
	if cfg.MaxDelay != 60*time.Second {
		t.Errorf("MaxDelay = %v, want 60s", cfg.MaxDelay)
	}
	if cfg.Multiplier != 2.0 {
		t.Errorf("Multiplier = %f, want 2.0", cfg.Multiplier)
	}
	if cfg.JitterPercent != 0.1 {
		t.Errorf("JitterPercent = %f, want 0.1", cfg.JitterPercent)
	}
}

func TestRetryConfigDefaultsPartial(t *testing.T) {
	cfg := &RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   500 * time.Millisecond,
		// Other fields should get defaults
	}
	cfg.setDefaults()

	// User values preserved
	if cfg.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", cfg.MaxAttempts)
	}
	if cfg.BaseDelay != 500*time.Millisecond {
		t.Errorf("BaseDelay = %v, want 500ms", cfg.BaseDelay)
	}

	// Defaults applied
	if cfg.MaxDelay != 60*time.Second {
		t.Errorf("MaxDelay = %v, want 60s", cfg.MaxDelay)
	}
	if cfg.Multiplier != 2.0 {
		t.Errorf("Multiplier = %f, want 2.0", cfg.Multiplier)
	}
}

func TestShouldRetryEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		resp *http.Response
		err  error
		want bool
	}{
		{
			name: "Nil response with context.Canceled",
			resp: nil,
			err:  context.Canceled,
			want: false,
		},
		{
			name: "Nil response with context.DeadlineExceeded",
			resp: nil,
			err:  context.DeadlineExceeded,
			want: false,
		},
		{
			name: "Nil response with network error",
			resp: nil,
			err:  errors.New("dial tcp: connection refused"),
			want: false,
		},
		{
			name: "Nil response no error",
			resp: nil,
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRetry(tt.resp, tt.err)
			if got != tt.want {
				t.Errorf("shouldRetry(resp=%v, err=%v) = %v, want %v",
					tt.resp, tt.err, got, tt.want)
			}
		})
	}
}

func TestCalculateBackoffZeroValues(t *testing.T) {
	cfg := &RetryConfig{
		BaseDelay:     0,
		MaxDelay:      0,
		Multiplier:    0,
		JitterPercent: 0,
	}

	// Should not panic, should return 0
	got := calculateBackoff(cfg, 0)
	if got != 0 {
		t.Errorf("calculateBackoff with zero config = %v, want 0", got)
	}
}

// TestCalculateBackoffJitterNegative tests the edge case where jitter
// could produce a negative delay (though statistically rare).
// This covers line 109-110 in retry.go
func TestCalculateBackoffJitterNegative(t *testing.T) {
	// Create a config with very small base delay and large jitter
	// to increase chance of negative result
	cfg := &RetryConfig{
		BaseDelay:     1 * time.Nanosecond,
		MaxDelay:      1 * time.Second,
		Multiplier:    1.0,
		JitterPercent: 0.99, // ±99% jitter
	}

	// Run multiple times to hit the edge case
	for i := 0; i < 100; i++ {
		delay := calculateBackoff(cfg, 0)

		// Should never be negative
		if delay < 0 {
			t.Errorf("calculateBackoff() produced negative delay: %v", delay)
		}

		// Should be >= 0 (clamped)
		if delay < 0 || delay > cfg.MaxDelay {
			t.Errorf("calculateBackoff() = %v, want [0, %v]", delay, cfg.MaxDelay)
		}
	}
}

// TestCalculateBackoffJitterExceedsMax tests the edge case where jitter
// pushes delay above MaxDelay. This covers line 112-114 in retry.go
func TestCalculateBackoffJitterExceedsMax(t *testing.T) {
	cfg := &RetryConfig{
		BaseDelay:     500 * time.Millisecond,
		MaxDelay:      600 * time.Millisecond,
		Multiplier:    2.0,
		JitterPercent: 0.5, // ±50% jitter
	}

	// With multiplier 2.0, attempt 3 gives: 500ms * 2^3 = 4000ms
	// This is already > MaxDelay (600ms), but jitter could push it higher
	for i := 0; i < 50; i++ {
		delay := calculateBackoff(cfg, 3)

		// Should never exceed MaxDelay even with jitter
		if delay > cfg.MaxDelay {
			t.Errorf("calculateBackoff() = %v, want <= %v (clamped to MaxDelay)", delay, cfg.MaxDelay)
		}

		// Should be positive
		if delay < 0 {
			t.Errorf("calculateBackoff() = %v, want >= 0", delay)
		}
	}
}
