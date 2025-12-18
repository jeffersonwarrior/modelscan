package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIProvider_enrichModelDetails(t *testing.T) {
	provider := NewOpenAIProvider("test-key")
	openaiProvider := provider.(*OpenAIProvider)

	tests := []struct {
		name              string
		inputModel        Model
		expectedInCost    float64
		expectedOutCost   float64
		expectedContext   int
		expectedMaxTokens int
		expectImages      bool
		expectReason      bool
		expectedCategory  string
	}{
		{
			name: "gpt-4o-mini model",
			inputModel: Model{
				ID:   "gpt-4o-mini",
				Name: "GPT-4o Mini",
			},
			expectedInCost:    0.15,
			expectedOutCost:   0.60,
			expectedContext:   128000,
			expectedMaxTokens: 16384,
			expectImages:      true,
			expectReason:      false,
			expectedCategory:  "fast",
		},
		{
			name: "gpt-4o model",
			inputModel: Model{
				ID:   "gpt-4o",
				Name: "GPT-4o",
			},
			expectedInCost:    2.50,
			expectedOutCost:   10.00,
			expectedContext:   128000,
			expectedMaxTokens: 16384,
			expectImages:      true,
			expectReason:      true,
			expectedCategory:  "multimodal",
		},
		{
			name: "gpt-4-turbo model",
			inputModel: Model{
				ID:   "gpt-4-turbo",
				Name: "GPT-4 Turbo",
			},
			expectedInCost:    10.00,
			expectedOutCost:   30.00,
			expectedContext:   128000,
			expectedMaxTokens: 4096,
			expectImages:      false,
			expectReason:      true,
			expectedCategory:  "premium",
		},
		{
			name: "gpt-4-turbo with vision",
			inputModel: Model{
				ID:   "gpt-4-turbo-vision",
				Name: "GPT-4 Turbo Vision",
			},
			expectedInCost:    10.00,
			expectedOutCost:   30.00,
			expectedContext:   128000,
			expectedMaxTokens: 4096,
			expectImages:      true,
			expectReason:      true,
			expectedCategory:  "premium",
		},
		{
			name: "gpt-4 base model",
			inputModel: Model{
				ID:   "gpt-4",
				Name: "GPT-4",
			},
			expectedInCost:    30.00,
			expectedOutCost:   60.00,
			expectedContext:   8192,
			expectedMaxTokens: 4096,
			expectImages:      false,
			expectReason:      true,
			expectedCategory:  "premium",
		},
		{
			name: "gpt-4-vision model",
			inputModel: Model{
				ID:   "gpt-4-vision-preview",
				Name: "GPT-4 Vision",
			},
			expectedInCost:    30.00,
			expectedOutCost:   60.00,
			expectedContext:   8192,
			expectedMaxTokens: 4096,
			expectImages:      true,
			expectReason:      true,
			expectedCategory:  "premium",
		},
		{
			name: "gpt-3.5-turbo model",
			inputModel: Model{
				ID:   "gpt-3.5-turbo",
				Name: "GPT-3.5 Turbo",
			},
			expectedInCost:    0.50,
			expectedOutCost:   1.50,
			expectedContext:   16385,
			expectedMaxTokens: 4096,
			expectImages:      false,
			expectReason:      false,
			expectedCategory:  "cost-effective",
		},
		{
			name: "gpt-3.5-turbo-instruct model",
			inputModel: Model{
				ID:   "gpt-3.5-turbo-instruct",
				Name: "GPT-3.5 Turbo Instruct",
			},
			expectedInCost:    0.50,
			expectedOutCost:   1.50,
			expectedContext:   16385,
			expectedMaxTokens: 4096,
			expectImages:      false,
			expectReason:      false,
			expectedCategory:  "cost-effective",
		},
		{
			name: "o1-mini reasoning model",
			inputModel: Model{
				ID:   "o1-mini",
				Name: "O1 Mini",
			},
			expectedInCost:    3.00,
			expectedOutCost:   12.00,
			expectedContext:   128000,
			expectedMaxTokens: 65536,
			expectImages:      false,
			expectReason:      true,
			expectedCategory:  "reasoning",
		},
		{
			name: "o1 reasoning model",
			inputModel: Model{
				ID:   "o1-preview",
				Name: "O1",
			},
			expectedInCost:    15.00,
			expectedOutCost:   60.00,
			expectedContext:   128000,
			expectedMaxTokens: 100000,
			expectImages:      false,
			expectReason:      true,
			expectedCategory:  "reasoning",
		},
		{
			name: "o3 reasoning model",
			inputModel: Model{
				ID:   "o3-preview",
				Name: "O3",
			},
			expectedInCost:    20.00,
			expectedOutCost:   80.00,
			expectedContext:   128000,
			expectedMaxTokens: 100000,
			expectImages:      true,
			expectReason:      true,
			expectedCategory:  "multimodal",
		},
		{
			name: "unknown model",
			inputModel: Model{
				ID:   "unknown-model-999",
				Name: "Unknown Model",
			},
			expectedInCost:    1.00,
			expectedOutCost:   2.00,
			expectedContext:   8192,
			expectedMaxTokens: 4096,
			expectImages:      false,
			expectReason:      false,
			expectedCategory:  "chat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := openaiProvider.enrichModelDetails(tt.inputModel)

			// Check common capabilities
			if !result.SupportsTools {
				t.Error("Expected SupportsTools to be true")
			}
			if !result.CanStream {
				t.Error("Expected CanStream to be true")
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

			// Check image support
			if result.SupportsImages != tt.expectImages {
				t.Errorf("Expected SupportsImages %v, got %v", tt.expectImages, result.SupportsImages)
			}

			// Check reasoning capability
			if result.CanReason != tt.expectReason {
				t.Errorf("Expected CanReason %v, got %v", tt.expectReason, result.CanReason)
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
				if result.Capabilities["function_calling"] != "full" {
					t.Error("Expected function_calling capability to be 'full'")
				}
				if result.Capabilities["streaming"] != "supported" {
					t.Error("Expected streaming capability to be 'supported'")
				}
				if tt.expectImages && result.Capabilities["vision"] != "high" {
					t.Error("Expected vision capability for image-supporting models")
				}
				if tt.expectReason && result.Capabilities["reasoning"] != "advanced" {
					t.Error("Expected reasoning capability for reasoning models")
				}
			}
		})
	}
}

func _TestOpenAIProvider_TestModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		
		// Return chat completion response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "chatcmpl-test",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "gpt-4o-mini",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "test successful"},
				"finish_reason": "stop"
			}]
		}`))
	}))
	defer server.Close()
	
	provider := &OpenAIProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		
	}
	
	ctx := context.Background()
	err := provider.TestModel(ctx, "gpt-4o-mini", false)
	if err != nil {
		t.Errorf("TestModel failed: %v", err)
	}
	
	// Test with verbose
	err = provider.TestModel(ctx, "gpt-4o-mini", true)
	if err != nil {
		t.Errorf("TestModel verbose failed: %v", err)
	}
}

func _TestOpenAIProvider_TestModel_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "invalid model"}}`))
	}))
	defer server.Close()
	
	provider := &OpenAIProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		
	}
	
	ctx := context.Background()
	err := provider.TestModel(ctx, "invalid-model", false)
	if err == nil {
		t.Error("Expected error for invalid model")
	}
}

func _TestOpenAIProvider_ListModels_HTTPMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify endpoint
		if r.URL.Path != "/models" {
			t.Errorf("Expected /models, got %s", r.URL.Path)
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"object": "list",
			"data": [
				{"id": "gpt-4o-mini", "object": "model", "created": 1234567890, "owned_by": "openai"},
				{"id": "gpt-4o", "object": "model", "created": 1234567890, "owned_by": "openai"}
			]
		}`))
	}))
	defer server.Close()
	
	provider := &OpenAIProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		
	}
	
	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	
	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}
}
