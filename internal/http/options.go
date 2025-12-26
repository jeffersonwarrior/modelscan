package http

import (
	"log"
	"time"
)

// Config configures the HTTP client behavior.
type Config struct {
	// BaseURL is the base URL for all requests (e.g., "https://api.openai.com/v1").
	BaseURL string

	// APIKey is the authentication key for the provider.
	APIKey string

	// Timeout is the maximum time to wait for a request to complete (default: 30s).
	Timeout time.Duration

	// Connection pool configuration
	MaxIdleConns        int           // Maximum idle connections across all hosts (default: 100)
	MaxIdleConnsPerHost int           // Maximum idle connections per host (default: 10)
	MaxConnsPerHost     int           // Maximum total connections per host (default: 10)
	IdleConnTimeout     time.Duration // How long idle connections stay open (default: 90s)

	// Retry configuration
	Retry RetryConfig

	// Hooks for request/response interception
	BeforeRequest BeforeRequestHook // Called before each request attempt
	AfterResponse AfterResponseHook // Called after each successful response
	OnError       OnErrorHook       // Called when an error occurs
	OnRetry       OnRetryHook       // Called before each retry attempt

	// Logger for debug output (optional)
	// If set, the client will log request/response details
	// API keys are automatically sanitized in logs
	Logger *log.Logger
}

// setDefaults fills in default values for zero-valued fields.
func (c *Config) setDefaults() {
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = 100
	}
	if c.MaxIdleConnsPerHost == 0 {
		c.MaxIdleConnsPerHost = 10
	}
	if c.MaxConnsPerHost == 0 {
		c.MaxConnsPerHost = 10
	}
	if c.IdleConnTimeout == 0 {
		c.IdleConnTimeout = 90 * time.Second
	}

	// Set retry defaults
	c.Retry.setDefaults()
}
