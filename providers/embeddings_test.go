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

func TestNewEmbeddingsProvider(t *testing.T) {
	provider := NewEmbeddingsProvider("test-key")
	if provider == nil {
		t.Fatal("Expected provider instance, got nil")
	}

	ep, ok := provider.(*EmbeddingsProvider)
	if !ok {
		t.Fatal("Expected *EmbeddingsProvider type")
	}

	if ep.apiKey != "test-key" {
		t.Errorf("Expected API key 'test-key', got %s", ep.apiKey)
	}

	if ep.baseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected base URL 'https://api.openai.com/v1', got %s", ep.baseURL)
	}

	if ep.client == nil {
		t.Error("Expected HTTP client to be initialized")
	}
}

func TestEmbeddingsProvider_GetCapabilities(t *testing.T) {
	provider := NewEmbeddingsProvider("test-key").(*EmbeddingsProvider)
	caps := provider.GetCapabilities()

	if caps.SupportsEmbeddings != true {
		t.Error("Expected SupportsEmbeddings to be true")
	}

	if caps.SupportsChat {
		t.Error("Expected SupportsChat to be false")
	}

	if caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be false")
	}

	if caps.MaxTokensPerRequest != 8191 {
		t.Errorf("Expected MaxTokensPerRequest 8191, got %d", caps.MaxTokensPerRequest)
	}

	if len(caps.SupportedParameters) == 0 {
		t.Error("Expected supported parameters to be populated")
	}
}

func TestEmbeddingsProvider_GetEndpoints(t *testing.T) {
	provider := NewEmbeddingsProvider("test-key").(*EmbeddingsProvider)
	endpoints := provider.GetEndpoints()

	if len(endpoints) != 2 {
		t.Fatalf("Expected 2 endpoints, got %d", len(endpoints))
	}

	// Check models endpoint
	modelsEndpoint := endpoints[0]
	if modelsEndpoint.Path != "/models" {
		t.Errorf("Expected path '/models', got %s", modelsEndpoint.Path)
	}
	if modelsEndpoint.Method != "GET" {
		t.Errorf("Expected method 'GET', got %s", modelsEndpoint.Method)
	}

	// Check embeddings endpoint
	embEndpoint := endpoints[1]
	if embEndpoint.Path != "/embeddings" {
		t.Errorf("Expected path '/embeddings', got %s", embEndpoint.Path)
	}
	if embEndpoint.Method != "POST" {
		t.Errorf("Expected method 'POST', got %s", embEndpoint.Method)
	}
}

func TestEmbeddingsProvider_ValidateEndpoints(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewEmbeddingsProvider("test-key").(*EmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Fatalf("ValidateEndpoints failed: %v", err)
	}

	if provider.endpoints == nil {
		t.Error("Expected endpoints to be set after validation")
	}
}

func TestEmbeddingsProvider_ValidateEndpoints_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewEmbeddingsProvider("test-key").(*EmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, true)
	if err != nil {
		t.Fatalf("ValidateEndpoints failed: %v", err)
	}
}

func TestEmbeddingsProvider_ValidateEndpoints_Failed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	provider := NewEmbeddingsProvider("test-key").(*EmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Fatalf("ValidateEndpoints should not return error: %v", err)
	}

	// Check that endpoints are marked as failed
	allFailed := true
	for _, ep := range provider.endpoints {
		if ep.Status != StatusFailed {
			allFailed = false
			break
		}
	}
	if !allFailed {
		t.Error("Expected all endpoints to be marked as failed")
	}
}

func TestEmbeddingsProvider_ValidateEndpoints_Concurrent(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		time.Sleep(10 * time.Millisecond) // Simulate latency
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewEmbeddingsProvider("test-key").(*EmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	start := time.Now()
	err := provider.ValidateEndpoints(ctx, false)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("ValidateEndpoints failed: %v", err)
	}

	// Should complete in less than 100ms if concurrent (vs 20ms+ if sequential)
	if elapsed > 100*time.Millisecond {
		t.Errorf("Validation took too long (%v), may not be concurrent", elapsed)
	}

	mu.Lock()
	count := callCount
	mu.Unlock()

	if count != 2 {
		t.Errorf("Expected 2 endpoint calls, got %d", count)
	}
}

func TestEmbeddingsProvider_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("Expected path '/models', got %s", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("Expected 'Bearer test-key', got %s", auth)
		}

		resp := embeddingsModelResponse{
			Data: []embeddingsModel{
				{ID: "text-embedding-3-small", Object: "model", Created: 1234567890, OwnedBy: "openai"},
				{ID: "text-embedding-3-large", Object: "model", Created: 1234567890, OwnedBy: "openai"},
				{ID: "text-embedding-ada-002", Object: "model", Created: 1234567890, OwnedBy: "openai"},
				{ID: "gpt-4", Object: "model", Created: 1234567890, OwnedBy: "openai"}, // Should be filtered out
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewEmbeddingsProvider("test-key").(*EmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 3 {
		t.Fatalf("Expected 3 models, got %d", len(models))
	}

	// Verify model IDs
	expectedIDs := map[string]bool{
		"text-embedding-3-small": true,
		"text-embedding-3-large": true,
		"text-embedding-ada-002": true,
	}

	for _, m := range models {
		if !expectedIDs[m.ID] {
			t.Errorf("Unexpected model ID: %s", m.ID)
		}

		// Verify categories
		if len(m.Categories) == 0 {
			t.Errorf("Model %s has no categories", m.ID)
		}

		// Verify capabilities
		if len(m.Capabilities) == 0 {
			t.Errorf("Model %s has no capabilities", m.ID)
		}
	}
}

func TestEmbeddingsProvider_ListModels_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := embeddingsModelResponse{
			Data: []embeddingsModel{
				{ID: "text-embedding-3-small", Object: "model", Created: 1234567890, OwnedBy: "openai"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewEmbeddingsProvider("test-key").(*EmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	models, err := provider.ListModels(ctx, true)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 1 {
		t.Fatalf("Expected 1 model, got %d", len(models))
	}
}

func TestEmbeddingsProvider_ListModels_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	provider := NewEmbeddingsProvider("test-key").(*EmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Fatal("Expected error for HTTP 401, got nil")
	}

	if !strings.Contains(err.Error(), "HTTP 401") {
		t.Errorf("Expected HTTP error, got: %v", err)
	}
}

func TestEmbeddingsProvider_TestModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Errorf("Expected path '/embeddings', got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		var req embeddingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.Model != "text-embedding-3-small" {
			t.Errorf("Expected model 'text-embedding-3-small', got %s", req.Model)
		}

		if len(req.Input) != 1 || req.Input[0] != "test" {
			t.Errorf("Expected input ['test'], got %v", req.Input)
		}

		if req.Dimensions != 1536 {
			t.Errorf("Expected dimensions 1536, got %d", req.Dimensions)
		}

		resp := embeddingsResponse{
			Object: "list",
			Model:  req.Model,
			Data: []embeddingObject{
				{
					Object:    "embedding",
					Embedding: make([]float64, 1536),
					Index:     0,
				},
			},
			Usage: embeddingsUsage{
				PromptTokens: 1,
				TotalTokens:  1,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewEmbeddingsProvider("test-key").(*EmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.TestModel(ctx, "text-embedding-3-small", false)
	if err != nil {
		t.Fatalf("TestModel failed: %v", err)
	}
}

func TestEmbeddingsProvider_TestModel_Large(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req embeddingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.Dimensions != 3072 {
			t.Errorf("Expected dimensions 3072 for large model, got %d", req.Dimensions)
		}

		resp := embeddingsResponse{
			Object: "list",
			Model:  req.Model,
			Data: []embeddingObject{
				{
					Object:    "embedding",
					Embedding: make([]float64, 3072),
					Index:     0,
				},
			},
			Usage: embeddingsUsage{
				PromptTokens: 1,
				TotalTokens:  1,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewEmbeddingsProvider("test-key").(*EmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.TestModel(ctx, "text-embedding-3-large", false)
	if err != nil {
		t.Fatalf("TestModel failed: %v", err)
	}
}

func TestEmbeddingsProvider_TestModel_Ada(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req embeddingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		// Ada model should not have dimensions parameter
		if req.Dimensions != 0 {
			t.Errorf("Expected no dimensions parameter for ada-002, got %d", req.Dimensions)
		}

		resp := embeddingsResponse{
			Object: "list",
			Model:  req.Model,
			Data: []embeddingObject{
				{
					Object:    "embedding",
					Embedding: make([]float64, 1536),
					Index:     0,
				},
			},
			Usage: embeddingsUsage{
				PromptTokens: 1,
				TotalTokens:  1,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewEmbeddingsProvider("test-key").(*EmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.TestModel(ctx, "text-embedding-ada-002", false)
	if err != nil {
		t.Fatalf("TestModel failed: %v", err)
	}
}

func TestEmbeddingsProvider_TestModel_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := embeddingsResponse{
			Object: "list",
			Model:  "text-embedding-3-small",
			Data: []embeddingObject{
				{
					Object:    "embedding",
					Embedding: make([]float64, 1536),
					Index:     0,
				},
			},
			Usage: embeddingsUsage{
				PromptTokens: 1,
				TotalTokens:  1,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewEmbeddingsProvider("test-key").(*EmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.TestModel(ctx, "text-embedding-3-small", true)
	if err != nil {
		t.Fatalf("TestModel failed: %v", err)
	}
}

func TestEmbeddingsProvider_TestModel_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	provider := NewEmbeddingsProvider("test-key").(*EmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.TestModel(ctx, "text-embedding-3-small", false)
	if err == nil {
		t.Fatal("Expected error for HTTP 400, got nil")
	}

	if !strings.Contains(err.Error(), "HTTP 400") {
		t.Errorf("Expected HTTP error, got: %v", err)
	}
}

func TestEmbeddingsProvider_TestModel_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := embeddingsResponse{
			Object: "list",
			Model:  "text-embedding-3-small",
			Data:   []embeddingObject{}, // Empty data
			Usage: embeddingsUsage{
				PromptTokens: 1,
				TotalTokens:  1,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewEmbeddingsProvider("test-key").(*EmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.TestModel(ctx, "text-embedding-3-small", false)
	if err == nil {
		t.Fatal("Expected error for empty embeddings, got nil")
	}

	if !strings.Contains(err.Error(), "no embeddings") {
		t.Errorf("Expected 'no embeddings' error, got: %v", err)
	}
}

func TestEmbeddingHelperFunctions(t *testing.T) {
	tests := []struct {
		modelID    string
		expectName string
		expectDims string
		expectCost float64
		expectUse  string
	}{
		{
			modelID:    "text-embedding-3-small",
			expectName: "Text Embedding 3 Small",
			expectDims: "1536 (configurable: 512-1536)",
			expectCost: 20.0,
			expectUse:  "semantic search, clustering, recommendations (cost-effective)",
		},
		{
			modelID:    "text-embedding-3-large",
			expectName: "Text Embedding 3 Large",
			expectDims: "3072 (configurable: 256-3072)",
			expectCost: 130.0,
			expectUse:  "semantic search, clustering, recommendations (highest quality)",
		},
		{
			modelID:    "text-embedding-ada-002",
			expectName: "Text Embedding Ada 002",
			expectDims: "1536",
			expectCost: 100.0,
			expectUse:  "semantic search, clustering, recommendations (legacy)",
		},
		{
			modelID:    "unknown-model",
			expectName: "unknown-model",
			expectDims: "unknown",
			expectCost: 0.0,
			expectUse:  "general purpose",
		},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			name := getEmbeddingModelName(tt.modelID)
			if name != tt.expectName {
				t.Errorf("Expected name %s, got %s", tt.expectName, name)
			}

			dims := getEmbeddingDimensions(tt.modelID)
			if dims != tt.expectDims {
				t.Errorf("Expected dimensions %s, got %s", tt.expectDims, dims)
			}

			cost := getEmbeddingModelCost(tt.modelID)
			if cost != tt.expectCost {
				t.Errorf("Expected cost %f, got %f", tt.expectCost, cost)
			}

			useCase := getEmbeddingUseCase(tt.modelID)
			if useCase != tt.expectUse {
				t.Errorf("Expected use case %s, got %s", tt.expectUse, useCase)
			}
		})
	}
}

func TestEmbeddingsProvider_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewEmbeddingsProvider("test-key").(*EmbeddingsProvider)
	provider.baseURL = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Fatalf("ValidateEndpoints should not return error: %v", err)
	}

	// Check that at least some endpoints failed due to timeout
	hasTimeout := false
	for _, ep := range provider.endpoints {
		if ep.Status == StatusFailed && strings.Contains(ep.Error, "context") {
			hasTimeout = true
			break
		}
	}
	if !hasTimeout {
		t.Error("Expected at least one endpoint to fail with context timeout")
	}
}

func TestEmbeddingsProvider_ModelDescriptions(t *testing.T) {
	tests := []struct {
		modelID string
		wantLen int
	}{
		{"text-embedding-3-small", 10},
		{"text-embedding-3-large", 10},
		{"text-embedding-ada-002", 10},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			desc := getEmbeddingModelDescription(tt.modelID)
			if len(desc) < tt.wantLen {
				t.Errorf("Description too short: %s", desc)
			}
		})
	}
}
