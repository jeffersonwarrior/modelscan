package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGoogleProvider_isGenerativeModel(t *testing.T) {
	provider := NewGoogleProvider("test-key")
	googleProvider := provider.(*GoogleProvider)

	tests := []struct {
		name     string
		model    googleModelInfo
		expected bool
	}{
		{
			name: "with generateContent method",
			model: googleModelInfo{
				Name:                       "models/gemini-pro",
				SupportedGenerationMethods: []string{"generateContent"},
			},
			expected: true,
		},
		{
			name: "with streamGenerateContent method",
			model: googleModelInfo{
				Name:                       "models/gemini-flash",
				SupportedGenerationMethods: []string{"streamGenerateContent"},
			},
			expected: true,
		},
		{
			name: "with both methods",
			model: googleModelInfo{
				Name:                       "models/gemini-pro",
				SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
			},
			expected: true,
		},
		{
			name: "without generative methods",
			model: googleModelInfo{
				Name:                       "models/embedding",
				SupportedGenerationMethods: []string{"embedContent"},
			},
			expected: false,
		},
		{
			name: "empty methods",
			model: googleModelInfo{
				Name:                       "models/unknown",
				SupportedGenerationMethods: []string{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := googleProvider.isGenerativeModel(tt.model)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for model %s", tt.expected, result, tt.model.Name)
			}
		})
	}
}

func TestGoogleProvider_enrichModelDetails(t *testing.T) {
	provider := NewGoogleProvider("test-key")
	googleProvider := provider.(*GoogleProvider)

	tests := []struct {
		name           string
		inputModel     Model
		checkCost      bool
		expectedInMin  float64
		expectedOutMax float64
		checkReason    bool
		expectReason   bool
		checkCategory  bool
		expectCategory string
	}{
		{
			name: "gemini-3-pro model",
			inputModel: Model{
				ID:   "gemini-3-pro",
				Name: "Gemini 3 Pro",
			},
			checkCost:      true,
			expectedInMin:  2.00,
			expectedOutMax: 12.00,
			checkReason:    true,
			expectReason:   true,
			checkCategory:  true,
			expectCategory: "reasoning",
		},
		{
			name: "gemini-3-flash model",
			inputModel: Model{
				ID:   "gemini-3-flash",
				Name: "Gemini 3 Flash",
			},
			checkCost:      true,
			expectedInMin:  0.50,
			expectedOutMax: 3.00,
			checkReason:    true,
			expectReason:   true,
			checkCategory:  true,
			expectCategory: "fast",
		},
		{
			name: "gemini-2.5-pro model",
			inputModel: Model{
				ID:   "gemini-2.5-pro",
				Name: "Gemini 2.5 Pro",
			},
			checkCost:      true,
			expectedInMin:  1.25,
			expectedOutMax: 10.00,
		},
		{
			name: "gemini-2.0-flash model",
			inputModel: Model{
				ID:   "gemini-2.0-flash-exp",
				Name: "Gemini 2.0 Flash",
			},
			checkCost: false,
		},
		{
			name: "gemini-1.5-pro model",
			inputModel: Model{
				ID:   "gemini-1.5-pro",
				Name: "Gemini 1.5 Pro",
			},
			checkCost:      true,
			expectedInMin:  1.25,
			expectedOutMax: 5.00,
		},
		{
			name: "gemini-1.5-flash model",
			inputModel: Model{
				ID:   "gemini-1.5-flash",
				Name: "Gemini 1.5 Flash",
			},
			checkCost:      true,
			expectedInMin:  0.075,
			expectedOutMax: 0.30,
		},
		{
			name: "gemini-pro model (legacy)",
			inputModel: Model{
				ID:   "gemini-pro",
				Name: "Gemini Pro",
			},
			checkCost:      true,
			expectedInMin:  1.25,
			expectedOutMax: 5.00,
		},
		{
			name: "unknown gemini model",
			inputModel: Model{
				ID:   "gemini-unknown-999",
				Name: "Unknown Gemini",
			},
			checkCost: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := googleProvider.enrichModelDetails(tt.inputModel)

			// Check common capabilities
			if !result.SupportsTools {
				t.Error("Expected SupportsTools to be true")
			}
			if !result.CanStream {
				t.Error("Expected CanStream to be true")
			}
			if !result.SupportsImages {
				t.Error("Expected SupportsImages to be true")
			}

			// Check costs if specified
			if tt.checkCost {
				if result.CostPer1MIn != tt.expectedInMin {
					t.Errorf("Expected CostPer1MIn %f, got %f", tt.expectedInMin, result.CostPer1MIn)
				}
			}

			// Check reasoning capability
			if tt.checkReason && result.CanReason != tt.expectReason {
				t.Errorf("Expected CanReason %v, got %v", tt.expectReason, result.CanReason)
			}

			// Check categories
			if tt.checkCategory {
				found := false
				for _, cat := range result.Categories {
					if cat == tt.expectCategory {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected category %s not found in %v", tt.expectCategory, result.Categories)
				}
			}
		})
	}
}

func TestGoogleProvider_TestModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify API key in query
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
				"totalTokenCount": 15
			}
		}`))
	}))
	defer server.Close()
	
	provider := &GoogleProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
	}
	
	ctx := context.Background()
	err := provider.TestModel(ctx, "gemini-1.5-flash", false)
	if err != nil {
		t.Errorf("TestModel failed: %v", err)
	}
	
	err = provider.TestModel(ctx, "gemini-1.5-flash", true)
	if err != nil {
		t.Errorf("TestModel verbose failed: %v", err)
	}
}

func TestGoogleProvider_TestModel_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "invalid model"}}`))
	}))
	defer server.Close()
	
	provider := &GoogleProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
	}
	
	ctx := context.Background()
	err := provider.TestModel(ctx, "invalid", false)
	if err == nil {
		t.Error("Expected error for invalid model")
	}
}

func TestGoogleProvider_ListModels_HTTPMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"models": [
				{
					"name": "models/gemini-1.5-flash",
					"displayName": "Gemini 1.5 Flash",
					"description": "Fast model",
					"inputTokenLimit": 1000000,
					"outputTokenLimit": 8192,
					"supportedGenerationMethods": ["generateContent"]
				},
				{
					"name": "models/gemini-1.5-pro",
					"displayName": "Gemini 1.5 Pro",
					"description": "Pro model",
					"inputTokenLimit": 2000000,
					"outputTokenLimit": 8192,
					"supportedGenerationMethods": ["generateContent"]
				}
			]
		}`))
	}))
	defer server.Close()
	
	provider := &GoogleProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
	}
	
	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	
	if len(models) < 2 {
		t.Errorf("Expected at least 2 models, got %d", len(models))
	}
}

func TestGoogleProvider_ValidateEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"models": []}`))
	}))
	defer server.Close()
	
	provider := &GoogleProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
	}
	
	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Errorf("ValidateEndpoints failed: %v", err)
	}
}


func TestGoogleProvider_EnrichModelDetails(t *testing.T) {
	provider := NewGoogleProvider("test-key")
	googleProvider := provider.(*GoogleProvider)
	
	tests := []struct {
		name           string
		modelID        string
		expectReason   bool
		expectCategory string
		minCost        float64
	}{
		{
			name:           "gemini-3-pro",
			modelID:        "gemini-3-pro-latest",
			expectReason:   true,
			expectCategory: "reasoning",
			minCost:        2.00,
		},
		{
			name:           "gemini-3-flash",
			modelID:        "gemini-3-flash",
			expectReason:   true,
			expectCategory: "fast",
			minCost:        0.50,
		},
		{
			name:           "gemini-2.5-pro",
			modelID:        "gemini-2.5-pro-latest",
			expectReason:   true,
			expectCategory: "reasoning",
			minCost:        1.25,
		},
		{
			name:           "gemini-2.5-flash",
			modelID:        "gemini-2.5-flash-latest",
			expectReason:   false,
			expectCategory: "fast",
			minCost:        0.30,
		},
		{
			name:           "gemini-2.0-flash",
			modelID:        "gemini-2.0-flash-exp",
			expectReason:   false,
			expectCategory: "balanced",
			minCost:        0.30,
		},
		{
			name:           "gemini-1.5-flash",
			modelID:        "gemini-1.5-flash-latest",
			expectReason:   false,
			expectCategory: "fast",
			minCost:        0.075,
		},
		{
			name:           "gemini-1.5-pro",
			modelID:        "gemini-1.5-pro-latest",
			expectReason:   true,
			expectCategory: "premium",
			minCost:        1.25,
		},
		{
			name:           "gemini-1.0-pro (default)",
			modelID:        "gemini-1.0-pro",
			expectReason:   false,
			expectCategory: "chat",
			minCost:        1.00,
		},
		{
			name:           "unknown model",
			modelID:        "gemini-unknown",
			expectReason:   false,
			expectCategory: "chat",
			minCost:        1.00,
		},
		{
			name:           "image generation",
			modelID:        "imagen-3.0-generate",
			expectReason:   false,
			expectCategory: "image-generation",
			minCost:        1.00,
		},
		{
			name:           "embedding",
			modelID:        "text-embedding-004",
			expectReason:   false,
			expectCategory: "embedding",
			minCost:        0.025,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := Model{
				ID:   tt.modelID,
				Name: tt.modelID,
			}
			
			enriched := googleProvider.enrichModelDetails(model)
			
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
			
			// Check pricing
			if enriched.CostPer1MIn != tt.minCost {
				t.Errorf("Expected cost %v, got %v", tt.minCost, enriched.CostPer1MIn)
			}
			
			// Check common capabilities (skip for embedding models)
			if tt.expectCategory != "embedding" {
				if !enriched.SupportsTools {
					t.Error("Expected SupportsTools to be true")
				}
				if !enriched.CanStream {
					t.Error("Expected CanStream to be true")
				}
			}
		})
	}
}

func TestGoogleProvider_ListModels_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"models": [
				{
					"name": "models/gemini-2.0-flash",
					"displayName": "Gemini 2.0 Flash",
					"description": "Latest flash model",
					"inputTokenLimit": 1000000,
					"outputTokenLimit": 8192,
					"supportedGenerationMethods": ["generateContent"]
				}
			]
		}`))
	}))
	defer server.Close()
	
	provider := &GoogleProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
	}
	
	ctx := context.Background()
	models, err := provider.ListModels(ctx, true) // verbose mode
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	
	if len(models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(models))
	}
	
	// Verify model details
	if models[0].ID != "gemini-2.0-flash" {
		t.Errorf("Expected ID gemini-2.0-flash, got %s", models[0].ID)
	}
	if models[0].Name != "Gemini 2.0 Flash" {
		t.Errorf("Expected name 'Gemini 2.0 Flash', got %s", models[0].Name)
	}
	if models[0].Description != "Latest flash model" {
		t.Errorf("Expected description, got %s", models[0].Description)
	}
	if models[0].ContextWindow != 1000000 {
		t.Errorf("Expected context window 1000000, got %d", models[0].ContextWindow)
	}
}

func TestGoogleProvider_ListModels_NonGenerativeFiltered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"models": [
				{
					"name": "models/gemini-1.5-flash",
					"displayName": "Gemini 1.5 Flash",
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
	
	provider := &GoogleProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
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
	if models[0].ID != "gemini-1.5-flash" {
		t.Errorf("Expected gemini-1.5-flash, got %s", models[0].ID)
	}
}

func TestGoogleProvider_ListModels_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"models": []}`))
	}))
	defer server.Close()
	
	provider := &GoogleProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
	}
	
	ctx := context.Background()
	models, err := provider.ListModels(ctx, true) // verbose to test that path
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	
	if len(models) != 0 {
		t.Errorf("Expected 0 models, got %d", len(models))
	}
}

func TestGoogleProvider_ListModels_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error": "forbidden"}`))
	}))
	defer server.Close()
	
	provider := &GoogleProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
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

func TestGoogleProvider_ListModels_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	}))
	defer server.Close()
	
	provider := &GoogleProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
	}
	
	ctx := context.Background()
	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}
