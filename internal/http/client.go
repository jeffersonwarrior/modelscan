package http

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a production-grade HTTP client with retry logic, rate limiting,
// and connection pooling.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	config     Config
}

// NewClient creates a new HTTP client with the given configuration.
// Default values are applied to zero-valued config fields.
func NewClient(cfg Config) *Client {
	cfg.setDefaults()

	// Configure transport with connection pooling
	transport := &http.Transport{
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		MaxConnsPerHost:     cfg.MaxConnsPerHost,
		IdleConnTimeout:     cfg.IdleConnTimeout,
	}

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   cfg.Timeout,
		},
		baseURL: cfg.BaseURL,
		apiKey:  cfg.APIKey,
		config:  cfg,
	}
}

// Do executes an HTTP request with automatic retry logic, rate limit parsing,
// and hook execution.
//
// The request is automatically enriched with:
//   - Authorization header (Bearer token)
//   - Content-Type header (if not set and body is present)
//
// Retry behavior:
//   - Retries on 429, 500, 502, 503, 504
//   - Does NOT retry on 4xx client errors (except 429)
//   - Does NOT retry on context cancellation
//   - Uses exponential backoff with jitter
//
// Hooks are executed in this order:
//  1. BeforeRequest (before each attempt, including retries)
//  2. Do HTTP request
//  3. AfterResponse (on success) OR OnError (on failure)
//  4. OnRetry (before retry delay, if retrying)
//
// Returns a Response with parsed rate limit information.
func (c *Client) Do(req *http.Request) (*Response, error) {
	var lastResp *http.Response
	var lastErr error

	// Preserve request body for retries
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, lastErr = io.ReadAll(req.Body)
		if lastErr != nil {
			return nil, fmt.Errorf("failed to read request body: %w", lastErr)
		}
		req.Body.Close()
	}

	// Execute request with retries
	for attempt := 0; attempt < c.config.Retry.MaxAttempts; attempt++ {
		// Restore body for this attempt
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// Add Authorization header if not present
		if c.apiKey != "" && req.Header.Get("Authorization") == "" {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}

		// Execute BeforeRequest hook
		if c.config.BeforeRequest != nil {
			if err := c.config.BeforeRequest(req); err != nil {
				return nil, err
			}
		}

		// Log request if logger is set
		if c.config.Logger != nil {
			c.logRequest(req, attempt)
		}

		// Execute the HTTP request
		resp, err := c.httpClient.Do(req)

		// Handle errors
		if err != nil {
			lastErr = err
			lastResp = nil

			// Execute OnError hook
			if c.config.OnError != nil {
				c.config.OnError(req, err)
			}

			// Check if we should retry
			if !shouldRetry(resp, err) {
				return nil, err
			}

			// Retry if not the last attempt
			if attempt < c.config.Retry.MaxAttempts-1 {
				delay := calculateBackoff(&c.config.Retry, attempt)

				// Execute OnRetry hook
				if c.config.OnRetry != nil {
					if hookErr := c.config.OnRetry(req, attempt+1, delay); hookErr != nil {
						return nil, hookErr
					}
				}

				time.Sleep(delay)
			}
			continue
		}

		// Success - we have a response
		lastResp = resp
		lastErr = nil

		// Execute AfterResponse hook
		if c.config.AfterResponse != nil {
			c.config.AfterResponse(req, resp)
		}

		// Log response if logger is set
		if c.config.Logger != nil {
			c.logResponse(resp, attempt)
		}

		// Check if we should retry based on status code
		if shouldRetry(resp, nil) && attempt < c.config.Retry.MaxAttempts-1 {
			// Close the response body before retrying
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			delay := calculateBackoff(&c.config.Retry, attempt)

			// Execute OnRetry hook
			if c.config.OnRetry != nil {
				if hookErr := c.config.OnRetry(req, attempt+1, delay); hookErr != nil {
					return nil, hookErr
				}
			}

			time.Sleep(delay)
			continue
		}

		// Parse rate limit headers
		rateLimit := ParseRateLimitHeaders(resp.Header)

		// Return wrapped response
		return &Response{
			Response:  resp,
			RateLimit: rateLimit,
			Attempt:   attempt,
		}, nil
	}

	// All retries exhausted
	if lastErr != nil {
		return nil, lastErr
	}

	// Return the last response (even if it's an error status code)
	rateLimit := ParseRateLimitHeaders(lastResp.Header)
	return &Response{
		Response:  lastResp,
		RateLimit: rateLimit,
		Attempt:   c.config.Retry.MaxAttempts - 1,
	}, nil
}

// logRequest logs the outgoing request with sanitized API key.
func (c *Client) logRequest(req *http.Request, attempt int) {
	auth := req.Header.Get("Authorization")
	if auth != "" && c.apiKey != "" {
		auth = "Bearer " + sanitizeAPIKey(c.apiKey)
	}

	c.config.Logger.Printf("[HTTP] Request (attempt %d): %s %s [auth=%s]",
		attempt+1, req.Method, req.URL.Path, auth)
}

// logResponse logs the response with rate limit information.
func (c *Client) logResponse(resp *http.Response, attempt int) {
	rateLimit := ParseRateLimitHeaders(resp.Header)
	rateLimitStr := ""
	if rateLimit != nil {
		rateLimitStr = fmt.Sprintf(" [%s]", rateLimit.String())
	}

	c.config.Logger.Printf("[HTTP] Response (attempt %d): %d %s%s",
		attempt+1, resp.StatusCode, resp.Status, rateLimitStr)
}
