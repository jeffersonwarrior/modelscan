package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewGoogleThinkingProvider(t *testing.T) {
	provider := NewGoogleThinkingProvider("test-key")
	if provider == nil {
		t.Fatal("Expected provider instance, got nil")
	}

	gp, ok := provider.(*GoogleThinkingProvider)
	if !ok {
		t.Fatal("Expected GoogleThinkingProvider type")
	}

	if gp.apiKey != "test-key" {
		t.Errorf("Expected apiKey 'test-key', got '%s'", gp.apiKey)
	}

	if gp.baseURL != "https://generativelanguage.googleapis.com/v1beta" {
		t.Errorf("Expected baseURL 'https://generativelanguage.googleapis.com/v1beta', got '%s'", gp.baseURL)
	}

	if gp.client == nil {
		t.Error("Expected HTTP client to be initialized")
	}
}

func TestGoogleThinkingProviderRegistration(t *testing.T) {
	factory, ok := GetProviderFactory("google_thinking")
	if !ok {
		t.Fatal("google_thinking provider not registered")
	}
	if factory == nil {
		t.Fatal("Factory is nil")
	}

	provider := factory("test-key")
	if provider == nil {
		t.Fatal("Factory returned nil provider")
	}

	_, ok = provider.(*GoogleThinkingProvider)
	if !ok {
		t.Error("Factory did not return GoogleThinkingProvider")
	}
}

func TestGoogleThinkingProvider_GetEndpoints(t *testing.T) {
	provider := NewGoogleThinkingProvider("test-key")
	endpoints := provider.GetEndpoints()

	if len(endpoints) != 2 {
		t.Fatalf("Expected 2 endpoints, got %d", len(endpoints))
	}

	// Check models list endpoint
	if endpoints[0].Path != "/v1beta/models" {
		t.Errorf("Expected path '/v1beta/models', got '%s'", endpoints[0].Path)
	}
	if endpoints[0].Method != "GET" {
		t.Errorf("Expected method 'GET', got '%s'", endpoints[0].Method)
	}

	// Check generateContent endpoint
	if endpoints[1].Method != "POST" {
		t.Errorf("Expected method 'POST', got '%s'", endpoints[1].Method)
	}
	if !strings.Contains(endpoints[1].Path, "generateContent") {
		t.Errorf("Expected path to contain 'generateContent', got '%s'", endpoints[1].Path)
	}
}

func TestGoogleThinkingProvider_GetCapabilities(t *testing.T) {
	provider := NewGoogleThinkingProvider("test-key")
	caps := provider.GetCapabilities()

	if !caps.SupportsChat {
		t.Error("Expected SupportsChat to be true")
	}
	if !caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be true")
	}
	if !caps.SupportsVision {
		t.Error("Expected SupportsVision to be true")
	}
	if !caps.SupportsJSONMode {
		t.Error("Expected SupportsJSONMode to be true")
	}
	if caps.MaxTokensPerRequest != 2000000 {
		t.Errorf("Expected MaxTokensPerRequest 2000000, got %d", caps.MaxTokensPerRequest)
	}

	// Check for thought_before_response parameter
	found := false
	for _, param := range caps.SupportedParameters {
		if param == "thought_before_response" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'thought_before_response' in SupportedParameters")
	}
}

func TestGoogleThinkingProvider_ValidateEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if r.Method == "GET" {
			w.Write([]byte(`{"models": []}`))
		} else {
			w.Write([]byte(`{
				"candidates": [{
					"content": {"parts": [{"text": "Hi"}]},
					"finishReason": "STOP"
				}]
			}`))
		}
	}))
	defer server.Close()

	provider := &GoogleThinkingProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{},
	}

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Errorf("ValidateEndpoints failed: %v", err)
	}

	// Verify endpoints were updated with status
	endpoints := provider.endpoints
	if len(endpoints) == 0 {
		t.Fatal("No endpoints after validation")
	}

	for _, endpoint := range endpoints {
		if endpoint.Status != StatusWorking {
			t.Errorf("Expected endpoint status %s, got %s: %s", StatusWorking, endpoint.Status, endpoint.Error)
		}
	}
}

func TestGoogleThinkingProvider_ValidateEndpoints_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"models": []}`))
	}))
	defer server.Close()

	provider := &GoogleThinkingProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{},
	}

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, true)
	if err != nil {
		t.Errorf("ValidateEndpoints verbose failed: %v", err)
	}
}

func TestGoogleThinkingProvider_ValidateEndpoints_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "unauthorized"}`))
	}))
	defer server.Close()

	provider := &GoogleThinkingProvider{
		apiKey:  "invalid-key",
		baseURL: server.URL,
		client:  &http.Client{},
	}

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Errorf("ValidateEndpoints should not return error: %v", err)
	}

	// Check that endpoints are marked as failed
	for _, endpoint := range provider.endpoints {
		if endpoint.Status != StatusFailed {
			t.Errorf("Expected endpoint status %s, got %s", StatusFailed, endpoint.Status)
		}
	}
}

func TestGoogleThinkingProvider_ValidateEndpoints_Concurrent(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"models": []}`))
	}))
	defer server.Close()

	provider := &GoogleThinkingProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{},
	}

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Errorf("ValidateEndpoints failed: %v", err)
	}

	mu.Lock()
	if callCount < 2 {
		t.Errorf("Expected at least 2 concurrent calls, got %d", callCount)
	}
	mu.Unlock()
}

func TestGoogleThinkingProvider_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"models": [
				{
					"name": "models/gemini-2.0-flash-exp",
					"displayName": "Gemini 2.0 Flash",
					"description": "Flash model with thinking mode",
					"inputTokenLimit": 1000000,
					"outputTokenLimit": 8192,
					"supportedGenerationMethods": ["generateContent"]
				},
				{
					"name": "models/gemini-2.5-pro",
					"displayName": "Gemini 2.5 Pro",
					"description": "Pro model with advanced thinking",
					"inputTokenLimit": 2000000,
					"outputTokenLimit": 8192,
					"supportedGenerationMethods": ["generateContent", "streamGenerateContent"]
				}
			]
		}`))
	}))
	defer server.Close()

	provider := &GoogleThinkingProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{},
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(models))
	}

	// Verify first model
	if models[0].ID != "gemini-2.0-flash-exp" {
		t.Errorf("Expected ID 'gemini-2.0-flash-exp', got '%s'", models[0].ID)
	}
	if !models[0].CanReason {
		t.Error("Expected Gemini 2.0 to support reasoning")
	}

	// Verify thinking capability
	hasThinking := false
	for _, cat := range models[0].Categories {
		if cat == "thinking" {
			hasThinking = true
			break
		}
	}
	if !hasThinking {
		t.Error("Expected 'thinking' category for Gemini 2.0")
	}
}

func TestGoogleThinkingProvider_ListModels_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"models": [{
				"name": "models/gemini-2.5-flash",
				"displayName": "Gemini 2.5 Flash",
				"description": "Fast model",
				"inputTokenLimit": 1000000,
				"outputTokenLimit": 8192,
				"supportedGenerationMethods": ["generateContent"]
			}]
		}`))
	}))
	defer server.Close()

	provider := &GoogleThinkingProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{},
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx, true)
	if err != nil {
		t.Fatalf("ListModels verbose failed: %v", err)
	}

	if len(models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(models))
	}
}

func TestGoogleThinkingProvider_ListModels_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error": "forbidden"}`))
	}))
	defer server.Close()

	provider := &GoogleThinkingProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{},
	}

	ctx := context.Background()
	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Error("Expected error for HTTP 403")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("Expected error to mention status 403, got: %v", err)
	}
}

func TestGoogleThinkingProvider_ListModels_NonGenerativeFiltered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"models": [
				{
					"name": "models/gemini-2.0-flash",
					"displayName": "Gemini 2.0 Flash",
					"description": "Generative model",
					"inputTokenLimit": 1000000,
					"outputTokenLimit": 8192,
					"supportedGenerationMethods": ["generateContent"]
				},
				{
					"name": "models/embedding-001",
					"displayName": "Embedding Model",
					"description": "Embedding model",
					"inputTokenLimit": 1000,
					"outputTokenLimit": 0,
					"supportedGenerationMethods": ["embedContent"]
				}
			]
		}`))
	}))
	defer server.Close()

	provider := &GoogleThinkingProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{},
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	// Should only get the generative model
	if len(models) != 1 {
		t.Errorf("Expected 1 generative model, got %d", len(models))
	}
	if models[0].ID != "gemini-2.0-flash" {
		t.Errorf("Expected gemini-2.0-flash, got %s", models[0].ID)
	}
}

func TestGoogleThinkingProvider_TestModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"candidates": [{
				"content": {
					"parts": [{"text": "test successful"}],
					"role": "model"
				},
				"finishReason": "STOP"
			}],
			"usageMetadata": {
				"promptTokenCount": 5,
				"candidatesTokenCount": 10,
				"totalTokenCount": 15,
				"thinkingTokens": 50
			}
		}`))
	}))
	defer server.Close()

	provider := &GoogleThinkingProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "gemini-2.0-flash-exp", false)
	if err != nil {
		t.Errorf("TestModel failed: %v", err)
	}
}

func TestGoogleThinkingProvider_TestModel_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"candidates": [{
				"content": {"parts": [{"text": "ok"}]},
				"finishReason": "STOP"
			}]
		}`))
	}))
	defer server.Close()

	provider := &GoogleThinkingProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "gemini-2.5-pro", true)
	if err != nil {
		t.Errorf("TestModel verbose failed: %v", err)
	}
}

func TestGoogleThinkingProvider_TestModel_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "invalid model"}}`))
	}))
	defer server.Close()

	provider := &GoogleThinkingProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "invalid-model", false)
	if err == nil {
		t.Error("Expected error for invalid model")
	}
}

func TestGoogleThinkingProvider_isGenerativeModel(t *testing.T) {
	provider := NewGoogleThinkingProvider("test-key")
	gp := provider.(*GoogleThinkingProvider)

	tests := []struct {
		name     string
		model    googleThinkingModelInfo
		expected bool
	}{
		{
			name: "with generateContent method",
			model: googleThinkingModelInfo{
				Name:                       "models/gemini-2.0-flash",
				SupportedGenerationMethods: []string{"generateContent"},
			},
			expected: true,
		},
		{
			name: "with streamGenerateContent method",
			model: googleThinkingModelInfo{
				Name:                       "models/gemini-2.5-pro",
				SupportedGenerationMethods: []string{"streamGenerateContent"},
			},
			expected: true,
		},
		{
			name: "with both methods",
			model: googleThinkingModelInfo{
				Name:                       "models/gemini-2.5-flash",
				SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
			},
			expected: true,
		},
		{
			name: "without generative methods",
			model: googleThinkingModelInfo{
				Name:                       "models/embedding",
				SupportedGenerationMethods: []string{"embedContent"},
			},
			expected: false,
		},
		{
			name: "empty methods",
			model: googleThinkingModelInfo{
				Name:                       "models/unknown",
				SupportedGenerationMethods: []string{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gp.isGenerativeModel(tt.model)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for model %s", tt.expected, result, tt.model.Name)
			}
		})
	}
}

func TestGoogleThinkingProvider_enrichModelDetails(t *testing.T) {
	provider := NewGoogleThinkingProvider("test-key")
	gp := provider.(*GoogleThinkingProvider)

	tests := []struct {
		name           string
		modelID        string
		expectReason   bool
		expectCategory string
		expectThinking bool
		minCost        float64
	}{
		{
			name:           "gemini-2.0-flash",
			modelID:        "gemini-2.0-flash-exp",
			expectReason:   true,
			expectCategory: "thinking",
			expectThinking: true,
			minCost:        0.30,
		},
		{
			name:           "gemini-2.5-pro",
			modelID:        "gemini-2.5-pro-latest",
			expectReason:   true,
			expectCategory: "thinking",
			expectThinking: true,
			minCost:        1.25,
		},
		{
			name:           "gemini-2.5-flash",
			modelID:        "gemini-2.5-flash",
			expectReason:   true,
			expectCategory: "thinking",
			expectThinking: true,
			minCost:        0.30,
		},
		{
			name:           "gemini-1.5-pro (no thinking)",
			modelID:        "gemini-1.5-pro-latest",
			expectReason:   true,
			expectCategory: "premium",
			expectThinking: false,
			minCost:        1.25,
		},
		{
			name:           "gemini-1.5-flash (no thinking)",
			modelID:        "gemini-1.5-flash",
			expectReason:   false,
			expectCategory: "fast",
			expectThinking: false,
			minCost:        0.075,
		},
		{
			name:           "unknown model",
			modelID:        "gemini-unknown",
			expectReason:   false,
			expectCategory: "chat",
			expectThinking: false,
			minCost:        1.00,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := Model{
				ID:   tt.modelID,
				Name: tt.modelID,
			}

			enriched := gp.enrichModelDetails(model)

			// Check reasoning
			if enriched.CanReason != tt.expectReason {
				t.Errorf("Expected CanReason=%v, got %v", tt.expectReason, enriched.CanReason)
			}

			// Check category
			found := false
			for _, cat := range enriched.Categories {
				if cat == tt.expectCategory {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected category %s, got %v", tt.expectCategory, enriched.Categories)
			}

			// Check thinking capability
			if tt.expectThinking {
				if enriched.Capabilities["thinking"] == "" {
					t.Error("Expected 'thinking' capability for Gemini 2.x models")
				}
			}

			// Check pricing
			if enriched.CostPer1MIn != tt.minCost {
				t.Errorf("Expected cost %v, got %v", tt.minCost, enriched.CostPer1MIn)
			}

			// Check common capabilities
			if !enriched.SupportsTools {
				t.Error("Expected SupportsTools to be true")
			}
			if !enriched.CanStream {
				t.Error("Expected CanStream to be true")
			}
			if !enriched.SupportsImages {
				t.Error("Expected SupportsImages to be true")
			}
		})
	}
}

func TestGoogleThinkingProvider_TestModelWithThinking(t *testing.T) {
	receivedThoughtParam := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request body to check for thought_before_response
		var reqBody googleThinkingRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err == nil {
			if reqBody.GenerationConfig != nil && reqBody.GenerationConfig.ThoughtBeforeResponse {
				receivedThoughtParam = true
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"candidates": [{
				"content": {
					"parts": [
						{"thought": {"thought_text": "Let me think about this..."}},
						{"text": "test successful"}
					]
				},
				"finishReason": "STOP"
			}],
			"usageMetadata": {
				"promptTokenCount": 5,
				"candidatesTokenCount": 10,
				"totalTokenCount": 15,
				"thinkingTokens": 100
			}
		}`))
	}))
	defer server.Close()

	provider := &GoogleThinkingProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{},
	}

	// Test endpoint validation which includes thinking mode
	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Errorf("ValidateEndpoints with thinking failed: %v", err)
	}

	if !receivedThoughtParam {
		t.Error("Expected thought_before_response parameter to be sent in validation request")
	}
}

func TestGoogleThinkingProvider_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		select {
		case <-r.Context().Done():
			return
		case <-time.After(100 * time.Millisecond):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"models": []}`))
		}
	}))
	defer server.Close()

	provider := &GoogleThinkingProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := provider.TestModel(ctx, "gemini-2.0-flash", false)
	if err == nil {
		t.Error("Expected error due to context cancellation")
	}
}
