package http

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	cfg := Config{
		BaseURL: "https://api.example.com",
		APIKey:  "sk-test-key",
		Timeout: 10 * time.Second,
	}

	client := NewClient(cfg)

	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	if client.baseURL != cfg.BaseURL {
		t.Errorf("baseURL = %q, want %q", client.baseURL, cfg.BaseURL)
	}

	if client.apiKey != cfg.APIKey {
		t.Errorf("apiKey = %q, want %q", client.apiKey, cfg.APIKey)
	}
}

func TestNewClientDefaults(t *testing.T) {
	cfg := Config{
		BaseURL: "https://api.example.com",
		APIKey:  "sk-test-key",
		// Leave other fields zero-valued
	}

	client := NewClient(cfg)

	// Check that defaults were applied
	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", client.httpClient.Timeout)
	}

	transport := client.httpClient.Transport.(*http.Transport)

	if transport.MaxIdleConns != 100 {
		t.Errorf("MaxIdleConns = %d, want 100", transport.MaxIdleConns)
	}

	if transport.MaxIdleConnsPerHost != 10 {
		t.Errorf("MaxIdleConnsPerHost = %d, want 10", transport.MaxIdleConnsPerHost)
	}
}

func TestClientDoSuccess(t *testing.T) {
	// Mock server that returns 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer sk-test-key" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer sk-test-key")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test-key",
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"status":"ok"}` {
		t.Errorf("Body = %q, want %q", string(body), `{"status":"ok"}`)
	}
}

func TestClientDoWithRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// Return 503 for first two attempts
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		// Success on third attempt
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test-key",
		Retry: RetryConfig{
			MaxAttempts:   3,
			BaseDelay:     10 * time.Millisecond,
			MaxDelay:      100 * time.Millisecond,
			Multiplier:    2.0,
			JitterPercent: 0.0, // No jitter for predictable tests
		},
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Check that retries actually delayed
	// First delay: 10ms, second delay: 20ms = ~30ms total
	if elapsed < 20*time.Millisecond {
		t.Errorf("elapsed = %v, want at least 20ms", elapsed)
	}
}

func TestClientDoNoRetryOn4xx(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusUnauthorized) // 401
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test-key",
		Retry: RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   10 * time.Millisecond,
		},
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	// Should NOT retry on 401
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (no retry on 4xx)", attempts)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestClientDoContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test-key",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/test", nil)
	_, err := client.Do(req)

	if err == nil {
		t.Fatal("Do() should return error when context is canceled")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want context.DeadlineExceeded", err)
	}
}

func TestClientDoRateLimitParsing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ratelimit-Limit-Requests", "1000")
		w.Header().Set("X-Ratelimit-Remaining-Requests", "999")
		w.Header().Set("X-Ratelimit-Reset-Requests", "1s")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test-key",
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.RateLimit == nil {
		t.Fatal("RateLimit should not be nil")
	}

	if resp.RateLimit.LimitRequests != 1000 {
		t.Errorf("LimitRequests = %d, want 1000", resp.RateLimit.LimitRequests)
	}

	if resp.RateLimit.RemainingRequests != 999 {
		t.Errorf("RemainingRequests = %d, want 999", resp.RateLimit.RemainingRequests)
	}
}

func TestClientDoBeforeRequestHook(t *testing.T) {
	hookCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that hook added the custom header
		if r.Header.Get("X-Custom") != "test-value" {
			t.Error("BeforeRequestHook did not add custom header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test-key",
		BeforeRequest: func(req *http.Request) error {
			hookCalled = true
			req.Header.Set("X-Custom", "test-value")
			return nil
		},
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	_, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}

	if !hookCalled {
		t.Error("BeforeRequestHook was not called")
	}
}

func TestClientDoBeforeRequestHookError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Server should not be called when BeforeRequestHook returns error")
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test-key",
		BeforeRequest: func(req *http.Request) error {
			return errors.New("hook error")
		},
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	_, err := client.Do(req)

	if err == nil {
		t.Fatal("Do() should return error when BeforeRequestHook fails")
	}

	if err.Error() != "hook error" {
		t.Errorf("error = %v, want %q", err, "hook error")
	}
}

func TestClientDoAfterResponseHook(t *testing.T) {
	hookCalled := false
	var capturedStatus int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test-key",
		AfterResponse: func(req *http.Request, resp *http.Response) error {
			hookCalled = true
			capturedStatus = resp.StatusCode
			return nil
		},
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	if !hookCalled {
		t.Error("AfterResponseHook was not called")
	}

	if capturedStatus != http.StatusCreated {
		t.Errorf("capturedStatus = %d, want %d", capturedStatus, http.StatusCreated)
	}
}

func TestClientDoOnRetryHook(t *testing.T) {
	attempts := 0
	var retryAttempts []int
	var retryDelays []time.Duration

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test-key",
		Retry: RetryConfig{
			MaxAttempts:   3,
			BaseDelay:     10 * time.Millisecond,
			JitterPercent: 0.0,
		},
		OnRetry: func(req *http.Request, attempt int, delay time.Duration) error {
			retryAttempts = append(retryAttempts, attempt)
			retryDelays = append(retryDelays, delay)
			return nil
		},
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	_, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}

	if len(retryAttempts) != 2 {
		t.Errorf("OnRetryHook called %d times, want 2", len(retryAttempts))
	}

	// First retry should be attempt 1, second retry should be attempt 2
	if retryAttempts[0] != 1 || retryAttempts[1] != 2 {
		t.Errorf("retryAttempts = %v, want [1, 2]", retryAttempts)
	}
}

func TestClientDoConcurrentRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test-key",
	})

	// Make 100 concurrent requests
	var wg sync.WaitGroup
	errChan := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req, _ := http.NewRequest("GET", server.URL+"/test", nil)
			resp, err := client.Do(req)
			if err != nil {
				errChan <- err
				return
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				errChan <- errors.New("unexpected status code")
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		t.Errorf("concurrent request failed: %v", err)
	}
}

func TestClientDoMaxRetries(t *testing.T) {
	attempts := atomic.Int32{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable) // Always fail
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test-key",
		Retry: RetryConfig{
			MaxAttempts:   3,
			BaseDelay:     1 * time.Millisecond,
			JitterPercent: 0.0,
		},
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	// Should try 3 times total (initial + 2 retries)
	if attempts.Load() != 3 {
		t.Errorf("attempts = %d, want 3", attempts.Load())
	}

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

func TestClientDoWithBody(t *testing.T) {
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test-key",
	})

	bodyContent := `{"test":"data"}`
	req, _ := http.NewRequest("POST", server.URL+"/test", strings.NewReader(bodyContent))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	if string(receivedBody) != bodyContent {
		t.Errorf("received body = %q, want %q", string(receivedBody), bodyContent)
	}
}

func TestClientDoBodyPreservedOnRetry(t *testing.T) {
	attempts := 0
	var bodies []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))
		attempts++

		if attempts < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test-key",
		Retry: RetryConfig{
			MaxAttempts:   2,
			BaseDelay:     1 * time.Millisecond,
			JitterPercent: 0.0,
		},
	})

	bodyContent := `{"retry":"test"}`
	req, _ := http.NewRequest("POST", server.URL+"/test", bytes.NewReader([]byte(bodyContent)))

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	if len(bodies) != 2 {
		t.Fatalf("received %d bodies, want 2", len(bodies))
	}

	// Both attempts should receive the same body
	if bodies[0] != bodyContent || bodies[1] != bodyContent {
		t.Errorf("bodies = %v, want both to be %q", bodies, bodyContent)
	}
}

func TestClientDoWithLogger(t *testing.T) {
	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ratelimit-Limit-Requests", "1000")
		w.Header().Set("X-Ratelimit-Remaining-Requests", "999")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test-key-12345",
		Logger:  logger,
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	logOutput := logBuf.String()

	// Check request log
	if !strings.Contains(logOutput, "[HTTP] Request") {
		t.Error("Log should contain request log")
	}

	// Check that API key is sanitized (last 7 chars: y-12345)
	if !strings.Contains(logOutput, "sk-***y-12345") {
		t.Errorf("Log should contain sanitized API key, got: %s", logOutput)
	}

	// Check full key is NOT in log
	if strings.Contains(logOutput, "sk-test-key-12345") {
		t.Error("Log should NOT contain full API key")
	}

	// Check response log
	if !strings.Contains(logOutput, "[HTTP] Response") {
		t.Error("Log should contain response log")
	}

	// Check rate limit info in log
	if !strings.Contains(logOutput, "requests=999/1000") {
		t.Error("Log should contain rate limit info")
	}
}

func TestClientDoLoggerWithRetry(t *testing.T) {
	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test",
		Logger:  logger,
		Retry: RetryConfig{
			MaxAttempts:   2,
			BaseDelay:     1 * time.Millisecond,
			JitterPercent: 0.0,
		},
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	_, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}

	logOutput := logBuf.String()

	// Should log both attempts
	if !strings.Contains(logOutput, "attempt 1") {
		t.Error("Log should contain first attempt")
	}

	if !strings.Contains(logOutput, "attempt 2") {
		t.Error("Log should contain second attempt")
	}
}

func TestClientDoOnErrorHook(t *testing.T) {
	errorCalled := false
	var capturedErr error

	// Create a server that closes immediately to trigger a network error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't write anything, force connection close
	}))
	server.Close() // Close immediately to cause connection error

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test",
		Retry: RetryConfig{
			MaxAttempts: 1, // No retries
		},
		OnError: func(req *http.Request, err error) error {
			errorCalled = true
			capturedErr = err
			return nil
		},
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	_, err := client.Do(req)

	if err == nil {
		t.Fatal("Do() should return error for closed server")
	}

	if !errorCalled {
		t.Error("OnError hook was not called")
	}

	if capturedErr == nil {
		t.Error("OnError hook should have received an error")
	}
}

func TestClientDoOnRetryHookError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test",
		Retry: RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   1 * time.Millisecond,
		},
		OnRetry: func(req *http.Request, attempt int, delay time.Duration) error {
			// Abort retry after first attempt
			return errors.New("retry aborted")
		},
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	_, err := client.Do(req)

	if err == nil {
		t.Fatal("Do() should return error when OnRetry hook returns error")
	}

	if err.Error() != "retry aborted" {
		t.Errorf("error = %v, want 'retry aborted'", err)
	}

	// Should only make 1 attempt (no retry due to hook error)
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (retry was aborted)", attempts)
	}
}

// TestClientDoAllRetriesExhaustedWithResponse tests the path where all retries
// return retryable status codes (no network error), and we return the last response.
// This covers lines 180-191 in client.go
func TestClientDoAllRetriesExhaustedWithResponse(t *testing.T) {
	attempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		// Always return 500 (retryable) with response body
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test-key",
		Retry: RetryConfig{
			MaxAttempts:   3,
			BaseDelay:     1 * time.Millisecond,
			JitterPercent: 0.0,
		},
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	resp, err := client.Do(req)

	// Should succeed (no error) but return the error status code
	if err != nil {
		t.Fatalf("Do() error = %v, expected no error (returns response)", err)
	}
	defer resp.Body.Close()

	// Should have made MaxAttempts
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}

	// Should return the last response with 500 status
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	// Should have attempt count
	if resp.Attempt != 2 { // 0-indexed, so 2 = third attempt
		t.Errorf("Attempt = %d, want 2", resp.Attempt)
	}

	// Verify response body is intact
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "server error") {
		t.Errorf("Body = %q, want to contain 'server error'", string(body))
	}
}

// TestClientDoAfterResponseHookOnSuccess tests AfterResponse hook on successful response.
// This covers line 140-143 in client.go
func TestClientDoAfterResponseHookOnSuccess(t *testing.T) {
	hookCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test-key",
		AfterResponse: func(req *http.Request, resp *http.Response) error {
			hookCalled = true
			if resp.StatusCode != http.StatusOK {
				t.Errorf("AfterResponse hook: StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			return nil
		},
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	_, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}

	if !hookCalled {
		t.Error("AfterResponse hook was not called")
	}
}

// TestClientDoRetryDiscardsResponseBody tests that response body is discarded
// before retrying. This covers lines 152-154 in client.go
func TestClientDoRetryDiscardsResponseBody(t *testing.T) {
	attempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			// First attempt: return large body with 503
			w.WriteHeader(http.StatusServiceUnavailable)
			// Write large body to ensure it gets discarded
			w.Write(bytes.Repeat([]byte("x"), 10000))
			return
		}
		// Second attempt: success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test-key",
		Retry: RetryConfig{
			MaxAttempts:   2,
			BaseDelay:     1 * time.Millisecond,
			JitterPercent: 0.0,
		},
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	// Should succeed on second attempt
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// TestClientDoRequestBodyReadError tests the error path when reading request body fails.
// This covers line 71-74 in client.go
func TestClientDoRequestBodyReadError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test-key",
	})

	// Create a body that will error on Read
	errorBody := &errorReader{}
	req, _ := http.NewRequest("POST", server.URL+"/test", errorBody)

	_, err := client.Do(req)

	if err == nil {
		t.Fatal("Do() should return error when body read fails")
	}

	if !strings.Contains(err.Error(), "failed to read request body") {
		t.Errorf("error = %v, want 'failed to read request body'", err)
	}
}

// errorReader always returns an error on Read
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("simulated read error")
}

func (e *errorReader) Close() error {
	return nil
}

// TestClientDoAuthorizationHeaderPreserved tests that existing Authorization header is preserved.
// This covers line 86-88 in client.go (the branch where header already exists)
func TestClientDoAuthorizationHeaderPreserved(t *testing.T) {
	customAuth := "Bearer custom-token"
	receivedAuth := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test-key", // Client has API key
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	req.Header.Set("Authorization", customAuth) // But request already has auth

	_, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}

	// Should preserve the request's auth, not use client's API key
	if receivedAuth != customAuth {
		t.Errorf("Authorization = %q, want %q (should preserve existing)", receivedAuth, customAuth)
	}
}

// TestClientDoWithoutAPIKey tests client behavior when no API key is configured.
// This covers the branch where c.apiKey == "" at line 86
func TestClientDoWithoutAPIKey(t *testing.T) {
	receivedAuth := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		// No APIKey set
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	_, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}

	// Should not add Authorization header
	if receivedAuth != "" {
		t.Errorf("Authorization = %q, want empty (no API key)", receivedAuth)
	}
}

// TestClientDoAfterResponseHookError tests that AfterResponse hook errors are handled.
// Note: Current implementation ignores the error, but we test the hook is called.
func TestClientDoAfterResponseHookError(t *testing.T) {
	hookCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		AfterResponse: func(req *http.Request, resp *http.Response) error {
			hookCalled = true
			return errors.New("hook error")
		},
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	resp, err := client.Do(req)

	// Should succeed despite hook error (current implementation ignores it)
	if err != nil {
		t.Fatalf("Do() error = %v, want nil", err)
	}

	if resp == nil {
		t.Fatal("Do() resp = nil, want non-nil")
	}

	if !hookCalled {
		t.Error("AfterResponse hook was not called")
	}
}

// TestClientDoNetworkErrorExhaustsRetries tests the path where all retries fail with network errors.
// This covers the "all retries exhausted" error return path (lines 180-183 in client.go)
func TestClientDoNetworkErrorExhaustsRetries(t *testing.T) {
	// Server that closes connection immediately
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Force connection close before response
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("webserver doesn't support hijacking")
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Fatal(err)
		}
		conn.Close() // Close connection to simulate network error
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		Retry: RetryConfig{
			MaxAttempts:   2,
			BaseDelay:     1 * time.Millisecond,
			JitterPercent: 0.0,
		},
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	_, err := client.Do(req)

	// Should return error after exhausting retries
	if err == nil {
		t.Fatal("Do() should return error after all retries fail")
	}

	// Should be a network error
	if !strings.Contains(err.Error(), "EOF") && !strings.Contains(err.Error(), "connection") {
		t.Logf("Got error: %v (expected network error)", err)
	}
}

// TestClientDoLoggerWithRetryAndFailure tests logger being called during retries that fail.
func TestClientDoLoggerWithRetryAndFailure(t *testing.T) {
	attempts := 0
	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable) // Always fail
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "sk-test12345",
		Retry: RetryConfig{
			MaxAttempts:   2,
			BaseDelay:     1 * time.Millisecond,
			JitterPercent: 0.0,
		},
		Logger: logger,
	})

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	// Should have logged both attempts
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "attempt 1") {
		t.Errorf("Log should contain 'attempt 1', got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "attempt 2") {
		t.Errorf("Log should contain 'attempt 2', got: %s", logOutput)
	}

	// Should have sanitized API key in logs
	if strings.Contains(logOutput, "sk-test12345") {
		t.Error("Log contains full API key, should be sanitized")
	}
	if !strings.Contains(logOutput, "sk-***st12345") {
		t.Errorf("Log should contain sanitized API key (sk-***st12345), got: %s", logOutput)
	}
}
