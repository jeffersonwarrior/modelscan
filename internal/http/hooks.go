package http

import (
	"net/http"
)

// BeforeRequestHook is called before each HTTP request attempt.
// If the hook returns an error, the request is aborted and the error is returned to the caller.
//
// Use cases:
//   - Add custom headers
//   - Log request details
//   - Modify request body
//   - Implement custom authentication
//
// The request can be modified in place. The hook is called for every retry attempt.
type BeforeRequestHook func(req *http.Request) error

// AfterResponseHook is called after a successful HTTP response is received.
// The hook cannot modify the response, but can log or collect metrics.
// If the hook returns an error, it is ignored (the response is still returned).
//
// Use cases:
//   - Log response details
//   - Collect metrics
//   - Parse rate limit headers
//   - Update internal state
//
// This hook is NOT called if the request fails or is retried.
type AfterResponseHook func(req *http.Request, resp *http.Response) error

// OnErrorHook is called when an HTTP request fails with an error.
// The hook can log the error or collect metrics.
// If the hook returns an error, it is ignored (the original error is still returned).
//
// Use cases:
//   - Log errors
//   - Collect error metrics
//   - Trigger alerts
//
// This hook is called for each failed attempt, including retries.
type OnErrorHook func(req *http.Request, err error) error

// OnRetryHook is called before each retry attempt.
// The hook receives the attempt number (0-indexed) and the delay that will be applied.
// If the hook returns an error, the retry is aborted and the error is returned.
//
// Use cases:
//   - Log retry attempts
//   - Implement custom backoff logic
//   - Abort retries based on custom conditions
//   - Update metrics
//
// Example:
//
//	OnRetry: func(req *http.Request, attempt int, delay time.Duration) error {
//	    log.Printf("Retrying %s (attempt %d) after %v", req.URL, attempt, delay)
//	    return nil
//	}
type OnRetryHook func(req *http.Request, attempt int, delay time.Duration) error

// Response wraps http.Response with additional metadata.
type Response struct {
	*http.Response
	RateLimit *RateLimitInfo // Parsed rate limit information (nil if not available)
	Attempt   int            // Number of attempts made (0-indexed)
}
