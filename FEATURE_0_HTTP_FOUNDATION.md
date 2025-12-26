# Feature 0: HTTP Foundation Layer

## Objective
Build production-grade HTTP client infrastructure for all 18 providers. Zero external dependencies except Go stdlib.

## Context
Currently, providers use external SDKs (e.g., `github.com/sashabaranov/go-openai`). We need in-house HTTP layer to:
1. Eliminate dependency risks (maintenance, breaking changes)
2. Unify behavior across all providers
3. Control retry logic, rate limiting, logging
4. Enable precise cost tracking and monitoring

## Deliverables

### Files to Create

1. **`internal/http/client.go`** - Main HTTP client
2. **`internal/http/client_test.go`** - Client tests (95%+ coverage)
3. **`internal/http/retry.go`** - Retry logic with backoff
4. **`internal/http/retry_test.go`** - Retry tests
5. **`internal/http/headers.go`** - Header parsing (rate limits, etc.)
6. **`internal/http/headers_test.go`** - Header tests
7. **`internal/http/pool.go`** - Connection pooling
8. **`internal/http/pool_test.go`** - Pool tests
9. **`internal/http/doc.go`** - Package documentation

### Architecture

```go
package http

import (
	"context"
	"net/http"
	"time"
)

// Client is the shared HTTP client for all providers
type Client struct {
	httpClient *http.Client
	transport  *http.Transport
	userAgent  string
	logger     Logger
	hooks      Hooks
}

// Config for HTTP client
type Config struct {
	Timeout         time.Duration // Default 30s
	MaxRetries      int           // Default 3
	RetryDelay      time.Duration // Default 1s (exponential backoff)
	MaxIdleConns    int           // Default 100
	MaxConnsPerHost int           // Default 10
	UserAgent       string        // Default "ModelScan/0.2.0"
	Logger          Logger        // Optional logging
}

// Request represents an HTTP request
type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    []byte
	Timeout time.Duration // Override default timeout
}

// Response represents an HTTP response
type Response struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
	Latency    time.Duration
	RateLimit  *RateLimitInfo
}

// RateLimitInfo parsed from response headers
type RateLimitInfo struct {
	Limit     int   // X-RateLimit-Limit
	Remaining int   // X-RateLimit-Remaining
	Reset     int64 // X-RateLimit-Reset (Unix timestamp)
}

// Logger interface for request/response logging
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}

// Hooks for request/response interception
type Hooks struct {
	BeforeRequest  func(req *Request) error
	AfterResponse  func(resp *Response) error
	OnError        func(err error) error
}

// Core methods
func NewClient(config Config) *Client
func (c *Client) Do(ctx context.Context, req Request) (*Response, error)
func (c *Client) Close() error
```

## Implementation Requirements

### 1. Connection Pooling

```go
// pool.go
type Pool struct {
	transport *http.Transport
	config    PoolConfig
}

type PoolConfig struct {
	MaxIdleConns        int           // Default 100
	MaxIdleConnsPerHost int           // Default 10
	MaxConnsPerHost     int           // Default 10
	IdleConnTimeout     time.Duration // Default 90s
	DisableKeepAlives   bool          // Default false
}

func NewPool(config PoolConfig) *Pool
func (p *Pool) Get() *http.Client
func (p *Pool) Close() error
```

**Tests Required:**
- [ ] Pool creates connections up to limit
- [ ] Pool reuses idle connections
- [ ] Pool respects max connections per host
- [ ] Pool closes cleanly
- [ ] Thread-safe with concurrent requests

### 2. Retry Logic with Exponential Backoff

```go
// retry.go
type RetryConfig struct {
	MaxAttempts     int           // Default 3
	InitialDelay    time.Duration // Default 1s
	MaxDelay        time.Duration // Default 30s
	Multiplier      float64       // Default 2.0
	RetryableErrors []int         // Default [429, 500, 502, 503, 504]
}

func ShouldRetry(statusCode int, err error) bool
func CalculateBackoff(attempt int, config RetryConfig) time.Duration
func DoWithRetry(ctx context.Context, fn func() (*Response, error), config RetryConfig) (*Response, error)
```

**Retry Logic:**
- 429 (Rate Limit): Retry with backoff
- 500, 502, 503, 504: Retry with backoff
- 400, 401, 403, 404: Do NOT retry (client errors)
- Network errors (timeout, connection refused): Retry with backoff

**Backoff Formula:**
```
delay = min(initialDelay * multiplier^attempt, maxDelay)
```

**Tests Required:**
- [ ] Retries on 429, 500, 502, 503, 504
- [ ] Does NOT retry on 400, 401, 403, 404
- [ ] Exponential backoff timing correct
- [ ] Respects max attempts
- [ ] Respects max delay
- [ ] Context cancellation stops retries
- [ ] Jitter added to prevent thundering herd

### 3. Rate Limit Header Parsing

```go
// headers.go
func ParseRateLimitHeaders(headers http.Header) *RateLimitInfo

// Supports multiple header formats:
// - X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset
// - RateLimit-Limit, RateLimit-Remaining, RateLimit-Reset
// - X-Rate-Limit-Limit, X-Rate-Limit-Remaining, X-Rate-Limit-Reset
```

**Tests Required:**
- [ ] Parses OpenAI-style headers
- [ ] Parses Anthropic-style headers
- [ ] Parses Google-style headers
- [ ] Handles missing headers gracefully
- [ ] Handles malformed values gracefully

### 4. Timeout Management

```go
// Per-request timeout (overrides default)
req := Request{
	Method:  "POST",
	URL:     "https://api.openai.com/v1/chat/completions",
	Timeout: 60 * time.Second, // Override default 30s
}

// Context timeout
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

resp, err := client.Do(ctx, req)
// Whichever timeout is shorter wins
```

**Tests Required:**
- [ ] Request timeout honored
- [ ] Context timeout honored
- [ ] Shorter timeout wins
- [ ] Timeout error returned correctly

### 5. Context Propagation

```go
// Context cancellation
ctx, cancel := context.WithCancel(context.Background())

go func() {
	time.Sleep(5 * time.Second)
	cancel() // Cancel after 5s
}()

resp, err := client.Do(ctx, req)
// Returns context.Canceled error
```

**Tests Required:**
- [ ] Context cancellation aborts request
- [ ] Context values propagated
- [ ] Context deadline respected

### 6. Request/Response Logging

```go
// Optional logger
client := NewClient(Config{
	Logger: MyLogger{},
})

// Logs:
// - Request method, URL, headers (sanitized)
// - Response status, latency, headers
// - Errors
// - Retry attempts

// IMPORTANT: Never log full API keys (only last 4 chars)
```

**Tests Required:**
- [ ] Logs requests
- [ ] Logs responses
- [ ] Logs errors
- [ ] Sanitizes API keys
- [ ] Logs retry attempts

### 7. Hooks System

```go
client := NewClient(Config{
	Hooks: Hooks{
		BeforeRequest: func(req *Request) error {
			// Add custom headers, validate, etc.
			req.Headers["X-Custom"] = "value"
			return nil
		},
		AfterResponse: func(resp *Response) error {
			// Custom response processing
			if resp.StatusCode == 402 {
				return errors.New("payment required")
			}
			return nil
		},
		OnError: func(err error) error {
			// Custom error handling
			log.Printf("HTTP error: %v", err)
			return err
		},
	},
})
```

**Tests Required:**
- [ ] BeforeRequest hook called
- [ ] AfterResponse hook called
- [ ] OnError hook called
- [ ] Hook errors propagate correctly
- [ ] Hooks can modify requests/responses

## Test Coverage Requirements

### Unit Tests (95%+ coverage)

**client_test.go:**
- [ ] `TestNewClient_DefaultConfig`
- [ ] `TestNewClient_CustomConfig`
- [ ] `TestDo_Success`
- [ ] `TestDo_NetworkError`
- [ ] `TestDo_Timeout`
- [ ] `TestDo_ContextCancellation`
- [ ] `TestDo_InvalidURL`
- [ ] `TestDo_LargeResponse`
- [ ] `TestClose_CleansUpResources`

**retry_test.go:**
- [ ] `TestShouldRetry_RetryableCodes`
- [ ] `TestShouldRetry_NonRetryableCodes`
- [ ] `TestCalculateBackoff_Exponential`
- [ ] `TestCalculateBackoff_MaxDelay`
- [ ] `TestDoWithRetry_Success`
- [ ] `TestDoWithRetry_MaxAttempts`
- [ ] `TestDoWithRetry_ContextCancellation`
- [ ] `TestDoWithRetry_Jitter`

**headers_test.go:**
- [ ] `TestParseRateLimitHeaders_OpenAI`
- [ ] `TestParseRateLimitHeaders_Anthropic`
- [ ] `TestParseRateLimitHeaders_Google`
- [ ] `TestParseRateLimitHeaders_Missing`
- [ ] `TestParseRateLimitHeaders_Malformed`

**pool_test.go:**
- [ ] `TestPool_CreateConnections`
- [ ] `TestPool_ReuseConnections`
- [ ] `TestPool_MaxConnsPerHost`
- [ ] `TestPool_IdleTimeout`
- [ ] `TestPool_Close`
- [ ] `TestPool_ThreadSafe`

### Integration Tests

**client_integration_test.go:**
```go
// +build integration

func TestIntegration_RealHTTPRequest(t *testing.T) {
	// Make real HTTP call to httpbin.org
	// Verify response
}

func TestIntegration_RetryWithRealTimeout(t *testing.T)
func TestIntegration_ConnectionPool(t *testing.T)
```

### Benchmark Tests

**client_bench_test.go:**
```go
func BenchmarkClient_SingleRequest(b *testing.B)
func BenchmarkClient_ConcurrentRequests(b *testing.B)
func BenchmarkPool_Get(b *testing.B)
func BenchmarkRetry_CalculateBackoff(b *testing.B)
```

## Documentation

**doc.go:**
```go
/*
Package http provides a production-grade HTTP client for ModelScan providers.

Features:
- Connection pooling with configurable limits
- Automatic retry with exponential backoff
- Rate limit header parsing
- Request/response logging
- Context propagation
- Timeout management
- Zero external dependencies

Usage:

	client := http.NewClient(http.Config{
		Timeout:    30 * time.Second,
		MaxRetries: 3,
	})
	defer client.Close()

	req := http.Request{
		Method:  "POST",
		URL:     "https://api.openai.com/v1/chat/completions",
		Headers: map[string]string{
			"Authorization": "Bearer sk-...",
			"Content-Type":  "application/json",
		},
		Body: []byte(`{"model": "gpt-4", "messages": [...]}`),
	}

	resp, err := client.Do(context.Background(), req)
	if err != nil {
		// Handle error
	}

	// Use response
	fmt.Println(resp.StatusCode, string(resp.Body))
*/
package http
```

## Acceptance Criteria

### Code Quality
- [ ] Zero external dependencies (stdlib only)
- [ ] All public APIs documented
- [ ] gofmt clean
- [ ] go vet clean
- [ ] No race conditions (verified with -race)
- [ ] Proper error handling with error wrapping

### Testing
- [ ] 95%+ test coverage
- [ ] All unit tests pass
- [ ] Integration tests pass
- [ ] Benchmarks show good performance
- [ ] Thread-safe (tested with -race)

### Performance
- [ ] Connection pooling reduces latency
- [ ] Retry backoff prevents thundering herd
- [ ] Memory-efficient (no leaks)
- [ ] Handles 100+ concurrent requests

### Documentation
- [ ] Package documentation complete
- [ ] All public functions documented
- [ ] Examples provided
- [ ] README for internal/http/ directory

## How to Prove Completion

1. **Run tests:**
```bash
cd internal/http
go test -v -coverprofile=coverage.out
go tool cover -func=coverage.out | grep total
# Must show: 95.0%+ coverage
```

2. **Run race detector:**
```bash
go test -race -v
# Must pass without data races
```

3. **Run vet:**
```bash
go vet ./...
# Must pass clean
```

4. **Integration test:**
```bash
go test -tags=integration -v
# Must pass
```

5. **Benchmark:**
```bash
go test -bench=. -benchmem
# Record results
```

6. **Build:**
```bash
go build ./...
# Must succeed
```

## Lessons Learned

Worker should document in commit message:
- Challenges encountered
- Solutions found
- Performance characteristics
- Gotchas for provider implementers
- Best practices

## Timeline

**Estimated:** 4-6 hours
- Design: 1 hour
- Implementation: 2-3 hours
- Testing: 1-2 hours
- Documentation: 30 minutes

This is the foundation all 18 providers will use. Quality is critical.
