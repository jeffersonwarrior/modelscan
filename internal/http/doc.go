// Package http provides a production-grade HTTP client for ModelScan providers.
//
// This package implements connection pooling, retry logic with exponential backoff,
// rate limit header parsing, timeout management, context propagation, and logging hooks.
//
// All functionality is built using only Go standard library with zero external dependencies.
//
// Key features:
//   - Connection pooling (configurable idle connections and per-host limits)
//   - Retry logic with exponential backoff and jitter (429, 500, 502, 503, 504)
//   - Rate limit header parsing (OpenAI, Anthropic, Google formats)
//   - Context propagation and cancellation support
//   - API key sanitization in logs
//   - Request/response hooks for interception
//   - Thread-safe operations verified by race detector
//
// Example usage:
//
//	client := http.NewClient(http.Config{
//	    BaseURL:     "https://api.openai.com/v1",
//	    APIKey:      "sk-...",
//	    Timeout:     30 * time.Second,
//	    MaxAttempts: 3,
//	})
//
//	req, _ := http.NewRequest(ctx, "POST", "/chat/completions", body)
//	resp, err := client.Do(req)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer resp.Body.Close()
package http
