//go:build integration
// +build integration

package http

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const httpbinURL = "https://httpbin.org"

func TestIntegrationHTTPGet(t *testing.T) {
	client := NewClient(Config{
		BaseURL: httpbinURL,
		Timeout: 10 * time.Second,
	})

	req, _ := http.NewRequest("GET", httpbinURL+"/get", nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"url"`) {
		t.Error("Response should contain URL field")
	}
}

func TestIntegrationHTTPPost(t *testing.T) {
	client := NewClient(Config{
		BaseURL: httpbinURL,
		Timeout: 10 * time.Second,
	})

	bodyContent := `{"test":"data"}`
	req, _ := http.NewRequest("POST", httpbinURL+"/post", strings.NewReader(bodyContent))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	responseBody := string(body)

	if !strings.Contains(responseBody, `"test"`) || !strings.Contains(responseBody, `"data"`) {
		t.Error("Response should echo the posted data")
	}
}

func TestIntegrationHTTPDelay(t *testing.T) {
	client := NewClient(Config{
		BaseURL: httpbinURL,
		Timeout: 5 * time.Second,
	})

	// Request a 1 second delay
	req, _ := http.NewRequest("GET", httpbinURL+"/delay/1", nil)

	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Should take at least 1 second
	if elapsed < 1*time.Second {
		t.Errorf("elapsed = %v, want at least 1s", elapsed)
	}
}

func TestIntegrationHTTPStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", 200},
		{"201 Created", 201},
		{"400 Bad Request", 400},
		{"404 Not Found", 404},
		{"500 Internal Server Error", 500},
	}

	client := NewClient(Config{
		BaseURL: httpbinURL,
		Timeout: 10 * time.Second,
		Retry: RetryConfig{
			MaxAttempts: 1, // No retries for this test
		},
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", httpbinURL+"/status/"+string(rune(tt.statusCode)), nil)
			resp, err := client.Do(req)

			if err != nil {
				t.Fatalf("Do() error = %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.statusCode {
				t.Errorf("StatusCode = %d, want %d", resp.StatusCode, tt.statusCode)
			}
		})
	}
}

func TestIntegrationHTTPHeaders(t *testing.T) {
	client := NewClient(Config{
		BaseURL: httpbinURL,
		APIKey:  "test-api-key-12345",
		Timeout: 10 * time.Second,
	})

	req, _ := http.NewRequest("GET", httpbinURL+"/headers", nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	responseBody := string(body)

	// httpbin.org echoes headers back
	if !strings.Contains(responseBody, "Bearer test-api-key-12345") {
		t.Error("Authorization header was not set correctly")
	}
}

func TestIntegrationHTTPTimeout(t *testing.T) {
	client := NewClient(Config{
		BaseURL: httpbinURL,
		Timeout: 500 * time.Millisecond, // Short timeout
	})

	// Request a 2 second delay (will exceed timeout)
	ctx := context.Background()
	req, _ := http.NewRequestWithContext(ctx, "GET", httpbinURL+"/delay/2", nil)

	_, err := client.Do(req)

	if err == nil {
		t.Fatal("Do() should return error when timeout is exceeded")
	}

	// Should be a timeout error
	if !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "timeout") {
		t.Errorf("error = %v, want timeout/deadline error", err)
	}
}

func TestIntegrationHTTPRetryOn503(t *testing.T) {
	client := NewClient(Config{
		BaseURL: httpbinURL,
		Timeout: 10 * time.Second,
		Retry: RetryConfig{
			MaxAttempts:   3,
			BaseDelay:     100 * time.Millisecond,
			JitterPercent: 0.0,
		},
	})

	// httpbin returns 503 once, then 200
	req, _ := http.NewRequest("GET", httpbinURL+"/status/503", nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	// Note: httpbin always returns 503, so we expect that
	// This test verifies retries are attempted, not that they succeed
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Logf("StatusCode = %d (expected 503, got different)", resp.StatusCode)
	}

	// Verify that retries were attempted
	if resp.Attempt < 2 {
		t.Errorf("Attempt = %d, want at least 2 (retries should have occurred)", resp.Attempt)
	}
}

func TestIntegrationHTTPUserAgent(t *testing.T) {
	client := NewClient(Config{
		BaseURL: httpbinURL,
		Timeout: 10 * time.Second,
	})

	req, _ := http.NewRequest("GET", httpbinURL+"/user-agent", nil)
	req.Header.Set("User-Agent", "ModelScan/1.0")

	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	responseBody := string(body)

	if !strings.Contains(responseBody, "ModelScan/1.0") {
		t.Error("User-Agent header was not set correctly")
	}
}

func TestIntegrationHTTPGzip(t *testing.T) {
	client := NewClient(Config{
		BaseURL: httpbinURL,
		Timeout: 10 * time.Second,
	})

	req, _ := http.NewRequest("GET", httpbinURL+"/gzip", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Go's http.Client automatically decompresses gzip responses
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"gzipped"`) {
		t.Error("Response should indicate gzip compression was used")
	}
}
