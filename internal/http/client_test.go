package http

import (
	"bytes"
	"context"
	"errors"
	"io"
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
