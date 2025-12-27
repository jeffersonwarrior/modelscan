package routing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewPlanoProxyRouter(t *testing.T) {
	tests := []struct {
		name    string
		config  *ProxyConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "empty base URL",
			config: &ProxyConfig{
				BaseURL: "",
			},
			wantErr: true,
		},
		{
			name: "valid config",
			config: &ProxyConfig{
				BaseURL: "http://localhost:12000",
				Timeout: 30,
			},
			wantErr: false,
		},
		{
			name: "default timeout",
			config: &ProxyConfig{
				BaseURL: "http://localhost:12000",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, err := NewPlanoProxyRouter(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPlanoProxyRouter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && router == nil {
				t.Error("NewPlanoProxyRouter() returned nil router")
			}
		})
	}
}

func TestPlanoProxyRouter_Route(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("Expected path /v1/chat/completions, got %s", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Parse request
		var req planoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Send response
		resp := planoResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "gpt-4o",
			Choices: []planoChoice{
				{
					Index: 0,
					Message: planoMessage{
						Role:    "assistant",
						Content: "Test response",
					},
					FinishReason: "stop",
				},
			},
			Usage: planoUsage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create router
	router, err := NewPlanoProxyRouter(&ProxyConfig{
		BaseURL: server.URL,
		Timeout: 5,
	})
	if err != nil {
		t.Fatalf("NewPlanoProxyRouter() error = %v", err)
	}

	// Make request
	req := Request{
		Model: "gpt-4o",
		Messages: []Message{
			{Role: "user", Content: "Test"},
		},
		Temperature: 0.7,
		MaxTokens:   100,
	}

	ctx := context.Background()
	resp, err := router.Route(ctx, req)

	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	if resp == nil {
		t.Fatal("Route() returned nil response")
	}

	if resp.Model != "gpt-4o" {
		t.Errorf("Model = %v, want gpt-4o", resp.Model)
	}

	if resp.Content != "Test response" {
		t.Errorf("Content = %v, want Test response", resp.Content)
	}

	if resp.Provider != "plano" {
		t.Errorf("Provider = %v, want plano", resp.Provider)
	}

	if resp.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %v, want 15", resp.Usage.TotalTokens)
	}

	if resp.Latency == 0 {
		t.Error("Latency not set")
	}
}

func TestPlanoProxyRouter_RouteError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	// Create router
	router, err := NewPlanoProxyRouter(&ProxyConfig{
		BaseURL: server.URL,
		Timeout: 5,
	})
	if err != nil {
		t.Fatalf("NewPlanoProxyRouter() error = %v", err)
	}

	req := Request{
		Model: "gpt-4o",
		Messages: []Message{
			{Role: "user", Content: "Test"},
		},
	}

	ctx := context.Background()
	_, err = router.Route(ctx, req)

	if err == nil {
		t.Error("Route() should return error for server error")
	}
}

func TestPlanoProxyRouter_RouteWithAPIKey(t *testing.T) {
	expectedAPIKey := "test-api-key"

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		expectedAuth := "Bearer " + expectedAPIKey

		if auth != expectedAuth {
			t.Errorf("Authorization = %v, want %v", auth, expectedAuth)
		}

		resp := planoResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "gpt-4o",
			Choices: []planoChoice{
				{
					Message: planoMessage{
						Content: "Test",
					},
				},
			},
		}

		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create router with API key
	router, err := NewPlanoProxyRouter(&ProxyConfig{
		BaseURL: server.URL,
		Timeout: 5,
		APIKey:  expectedAPIKey,
	})
	if err != nil {
		t.Fatalf("NewPlanoProxyRouter() error = %v", err)
	}

	req := Request{
		Model: "gpt-4o",
		Messages: []Message{
			{Role: "user", Content: "Test"},
		},
	}

	ctx := context.Background()
	_, err = router.Route(ctx, req)

	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
}

func TestPlanoProxyRouter_ConvertToPlanoRequest(t *testing.T) {
	router, _ := NewPlanoProxyRouter(&ProxyConfig{
		BaseURL: "http://localhost:12000",
	})

	tests := []struct {
		name string
		req  Request
		want planoRequest
	}{
		{
			name: "basic request",
			req: Request{
				Model: "gpt-4o",
				Messages: []Message{
					{Role: "user", Content: "Test"},
				},
			},
			want: planoRequest{
				Model: "gpt-4o",
				Messages: []planoMessage{
					{Role: "user", Content: "Test"},
				},
			},
		},
		{
			name: "empty model uses none",
			req: Request{
				Model: "",
				Messages: []Message{
					{Role: "user", Content: "Test"},
				},
			},
			want: planoRequest{
				Model: "none",
				Messages: []planoMessage{
					{Role: "user", Content: "Test"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := router.convertToPlanoRequest(tt.req)

			if got.Model != tt.want.Model {
				t.Errorf("Model = %v, want %v", got.Model, tt.want.Model)
			}

			if len(got.Messages) != len(tt.want.Messages) {
				t.Errorf("Messages length = %v, want %v", len(got.Messages), len(tt.want.Messages))
			}
		})
	}
}

func TestPlanoProxyRouter_Close(t *testing.T) {
	router, err := NewPlanoProxyRouter(&ProxyConfig{
		BaseURL: "http://localhost:12000",
	})
	if err != nil {
		t.Fatalf("NewPlanoProxyRouter() error = %v", err)
	}

	if err := router.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}
