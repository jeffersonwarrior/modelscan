# HTTP Client Package Documentation

**Package**: `internal/http`
**Purpose**: Production-grade HTTP client with retry logic, connection pooling, and hooks
**Stability**: Production
**Test Coverage**: 85%

---

## Overview

Production HTTP client with automatic retry on transient failures (429, 5xx), exponential backoff with jitter, connection pooling, timeout management, and extensible hook system for request/response processing.

---

## Core Types

### Client

```go
type Client struct {
    httpClient    *http.Client
    retryPolicy   RetryPolicy
    hooks         Hooks
    timeout       time.Duration
    maxRetries    int
}
```

### Configuration

```go
type Config struct {
    Timeout        time.Duration // Request timeout (default: 30s)
    MaxRetries     int          // Max retry attempts (default: 3)
    RetryPolicy    RetryPolicy  // Retry strategy
    Hooks          Hooks        // Request/response hooks
    Transport      *http.Transport // Custom transport
}
```

---

## Key Features

### 1. Automatic Retry Logic

Retries on transient errors:
- 429 (Rate Limit)
- 500-599 (Server Errors)
- Network timeouts
- Connection resets

**Strategy**: Exponential backoff with jitter

```go
backoff = min(maxBackoff, baseDelay * 2^attempt + jitter)
```

### 2. Connection Pooling

Optimized HTTP transport:

```go
transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 10,
    IdleConnTimeout:     90 * time.Second,
    DisableKeepAlives:   false,
}
```

### 3. Hook System

Extensible request/response processing:

```go
type Hooks struct {
    BeforeRequest  func(*http.Request) error
    AfterResponse  func(*Response) error
    OnError        func(error) error
}
```

### 4. Auth Header Injection

Automatic authentication header management:

```go
client.SetAuthHeader("Bearer", apiKey)
// Injects: Authorization: Bearer sk-...
```

---

## Usage

### Basic Request

```go
import "github.com/jeffersonwarrior/modelscan/internal/http"

client := http.NewClient(http.Config{
    Timeout:    30 * time.Second,
    MaxRetries: 3,
})

req, _ := http.NewRequest("POST", url, body)
resp, err := client.Do(req)
```

### With Retry Policy

```go
policy := http.RetryPolicy{
    MaxAttempts:  5,
    InitialDelay: 1 * time.Second,
    MaxDelay:     30 * time.Second,
    Multiplier:   2.0,
    Jitter:       0.1,
}

client := http.NewClient(http.Config{
    RetryPolicy: policy,
})
```

### With Hooks

```go
hooks := http.Hooks{
    BeforeRequest: func(req *http.Request) error {
        log.Printf("Request: %s %s", req.Method, req.URL)
        return nil
    },
    AfterResponse: func(resp *http.Response) error {
        log.Printf("Response: %d", resp.StatusCode)
        return nil
    },
    OnError: func(err error) error {
        log.Printf("Error: %v", err)
        return err
    },
}

client := http.NewClient(http.Config{Hooks: hooks})
```

---

## Retry Logic

### Retry Decision

```go
func shouldRetry(statusCode int, err error) bool {
    // Retry on network errors
    if err != nil {
        return isNetworkError(err)
    }

    // Retry on transient HTTP errors
    switch statusCode {
    case 429, 500, 502, 503, 504:
        return true
    default:
        return false
    }
}
```

### Backoff Calculation

```go
func calculateBackoff(attempt int, policy RetryPolicy) time.Duration {
    base := policy.InitialDelay * time.Duration(math.Pow(policy.Multiplier, float64(attempt)))
    jitter := time.Duration(rand.Float64() * policy.Jitter * float64(base))
    return min(base+jitter, policy.MaxDelay)
}
```

---

## Error Handling

### Error Types

```go
var (
    ErrTimeout       = errors.New("request timeout")
    ErrMaxRetries    = errors.New("max retries exceeded")
    ErrInvalidURL    = errors.New("invalid URL")
    ErrNetworkError  = errors.New("network error")
)
```

### Error Wrapping

```go
if err := client.Do(req); err != nil {
    if errors.Is(err, http.ErrTimeout) {
        // Handle timeout
    }
    if errors.Is(err, http.ErrMaxRetries) {
        // Handle retry exhaustion
    }
}
```

---

## Performance

### Connection Reuse

- Persistent connections with keep-alive
- Connection pooling (100 max, 10 per host)
- 90s idle timeout

### Request Metrics

Track latency and retries:

```go
resp, err := client.Do(req)
fmt.Printf("Latency: %v, Retries: %d\n", resp.Latency, resp.RetryCount)
```

---

## Testing

### Test Coverage

- Retry logic: 95%
- Hook execution: 90%
- Error handling: 85%
- Connection pooling: 80%

### Mock HTTP Server

```go
func TestClient(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(200)
        w.Write([]byte("OK"))
    }))
    defer server.Close()

    client := http.NewClient(http.Config{})
    resp, err := client.Get(server.URL)
    assert.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)
}
```

---

## Dependencies

- `net/http` - Standard HTTP client
- `context` - Cancellation support
- `time` - Timeout/retry timing
- `math/rand` - Jitter calculation

**Zero external dependencies** - Pure Go stdlib

---

**Last Updated**: December 31, 2025
**Stability**: Production
**Used By**: providers/, routing/, internal/discovery/
