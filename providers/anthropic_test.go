package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAnthropicProvider_enrichModelDetails(t *testing.T) {
	provider := NewAnthropicProvider("test-key")
	anthropicProvider := provider.(*AnthropicProvider)

	tests := []struct {
		name              string
		inputModel        Model
		expectedInCost    float64
		expectedOutCost   float64
		expectedContext   int
		expectedMaxTokens int
		expectedCategory  string
	}{
		{
			name: "opus-4 model",
			inputModel: Model{
				ID:   "claude-opus-4-20250514",
				Name: "Claude Opus 4",
			},
			expectedInCost:    5.00,
			expectedOutCost:   25.00,
			expectedContext:   200000,
			expectedMaxTokens: 64000,
			expectedCategory:  "premium",
		},
		{
			name: "sonnet-4 model",
			inputModel: Model{
				ID:   "claude-sonnet-4-20250514",
				Name: "Claude Sonnet 4",
			},
			expectedInCost:    3.00,
			expectedOutCost:   15.00,
			expectedContext:   200000,
			expectedMaxTokens: 64000,
			expectedCategory:  "balanced",
		},
		{
			name: "haiku-4 model",
			inputModel: Model{
				ID:   "claude-haiku-4-20250514",
				Name: "Claude Haiku 4",
			},
			expectedInCost:    1.00,
			expectedOutCost:   5.00,
			expectedContext:   200000,
			expectedMaxTokens: 64000,
			expectedCategory:  "cost-effective",
		},
		{
			name: "opus-3.5 model",
			inputModel: Model{
				ID:   "claude-opus-3.5-20240229",
				Name: "Claude Opus 3.5",
			},
			expectedInCost:    15.00,
			expectedOutCost:   75.00,
			expectedContext:   200000,
			expectedMaxTokens: 4096,
			expectedCategory:  "legacy",
		},
		{
			name: "sonnet-3.5 model",
			inputModel: Model{
				ID:   "claude-sonnet-3.5-20240620",
				Name: "Claude Sonnet 3.5",
			},
			expectedInCost:    3.00,
			expectedOutCost:   15.00,
			expectedContext:   200000,
			expectedMaxTokens: 8192,
			expectedCategory:  "legacy",
		},
		{
			name: "haiku-3.5 model",
			inputModel: Model{
				ID:   "claude-haiku-3.5-20240307",
				Name: "Claude Haiku 3.5",
			},
			expectedInCost:    0.80,
			expectedOutCost:   4.00,
			expectedContext:   200000,
			expectedMaxTokens: 4096,
			expectedCategory:  "legacy",
		},
		{
			name: "unknown claude model",
			inputModel: Model{
				ID:   "claude-unknown-999",
				Name: "Unknown Claude",
			},
			expectedInCost:    3.00,
			expectedOutCost:   15.00,
			expectedContext:   200000,
			expectedMaxTokens: 4096,
			expectedCategory:  "chat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := anthropicProvider.enrichModelDetails(tt.inputModel)

			// Check common capabilities
			if !result.SupportsImages {
				t.Error("Expected SupportsImages to be true")
			}
			if !result.SupportsTools {
				t.Error("Expected SupportsTools to be true")
			}
			if !result.CanStream {
				t.Error("Expected CanStream to be true")
			}
			if !result.CanReason {
				t.Error("Expected CanReason to be true")
			}

			// Check costs
			if result.CostPer1MIn != tt.expectedInCost {
				t.Errorf("Expected CostPer1MIn %f, got %f", tt.expectedInCost, result.CostPer1MIn)
			}
			if result.CostPer1MOut != tt.expectedOutCost {
				t.Errorf("Expected CostPer1MOut %f, got %f", tt.expectedOutCost, result.CostPer1MOut)
			}

			// Check context window
			if result.ContextWindow != tt.expectedContext {
				t.Errorf("Expected ContextWindow %d, got %d", tt.expectedContext, result.ContextWindow)
			}

			// Check max tokens
			if result.MaxTokens != tt.expectedMaxTokens {
				t.Errorf("Expected MaxTokens %d, got %d", tt.expectedMaxTokens, result.MaxTokens)
			}

			// Check categories
			found := false
			for _, cat := range result.Categories {
				if cat == tt.expectedCategory {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected category %s not found in %v", tt.expectedCategory, result.Categories)
			}

			// Check capabilities metadata
			if result.Capabilities == nil {
				t.Error("Expected Capabilities map to be set")
			} else {
				if result.Capabilities["vision"] != "high" {
					t.Error("Expected vision capability to be 'high'")
				}
				if result.Capabilities["function_calling"] != "full" {
					t.Error("Expected function_calling capability to be 'full'")
				}
			}
		})
	}
}

func TestAnthropicProvider_ListModels(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("x-api-key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Return mock models response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": [
				{
					"id": "claude-3-opus-20240229",
					"created": 1709251200,
					"type": "model"
				},
				{
					"id": "claude-3-sonnet-20240229",
					"created": 1709251200,
					"type": "model"
				}
			]
		}`))
	}))
	defer server.Close()

	// Create provider with mock server URL
	provider := &AnthropicProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}

	// Verify model details are enriched
	for _, model := range models {
		if model.ID == "claude-3-opus-20240229" {
			if model.CostPer1MIn == 0 {
				t.Error("Expected enriched cost for opus model")
			}
		}
	}
}

func TestAnthropicProvider_ListModels_Error(t *testing.T) {
	// Test with invalid API key
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "invalid_api_key"}}`))
	}))
	defer server.Close()

	provider := &AnthropicProvider{
		apiKey:  "invalid-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Error("Expected error with invalid API key")
	}
}

func TestAnthropicProvider_ListModels_ContextCancelled(t *testing.T) {
	// Create a provider with a long timeout
	provider := &AnthropicProvider{
		apiKey:  "test-key",
		baseURL: "https://api.anthropic.com/v1",
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}

func TestAnthropicProvider_TestModel(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Header.Get("x-api-key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Return successful response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "msg_test",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "test successful"}],
			"model": "claude-3-opus-20240229"
		}`))
	}))
	defer server.Close()

	provider := &AnthropicProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "claude-3-opus-20240229", false)
	if err != nil {
		t.Errorf("TestModel failed: %v", err)
	}

	// Test with verbose output
	err = provider.TestModel(ctx, "claude-3-opus-20240229", true)
	if err != nil {
		t.Errorf("TestModel with verbose failed: %v", err)
	}
}

func TestAnthropicProvider_TestModel_Error(t *testing.T) {
	// Test with error response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "invalid_model"}}`))
	}))
	defer server.Close()

	provider := &AnthropicProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "invalid-model", false)
	if err == nil {
		t.Error("Expected error with invalid model")
	}
}

func TestAnthropicProvider_TestModel_ContextCancelled(t *testing.T) {
	provider := &AnthropicProvider{
		apiKey:  "test-key",
		baseURL: "https://api.anthropic.com/v1",
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := provider.TestModel(ctx, "claude-3-opus-20240229", false)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}

func TestAnthropicProvider_ValidateEndpoints_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "msg_test",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "test"}]
		}`))
	}))
	defer server.Close()
	
	provider := &AnthropicProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
	
	ctx := context.Background()
	// Test verbose mode
	err := provider.ValidateEndpoints(ctx, true)
	if err != nil {
		t.Errorf("ValidateEndpoints verbose failed: %v", err)
	}
}
