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

func TestNewCohereEmbeddingsProvider(t *testing.T) {
	provider := NewCohereEmbeddingsProvider("test-key")
	if provider == nil {
		t.Fatal("Expected provider, got nil")
	}

	cp, ok := provider.(*CohereEmbeddingsProvider)
	if !ok {
		t.Fatal("Expected *CohereEmbeddingsProvider type")
	}

	if cp.apiKey != "test-key" {
		t.Errorf("Expected apiKey 'test-key', got '%s'", cp.apiKey)
	}

	if cp.baseURL != "https://api.cohere.ai/v1" {
		t.Errorf("Expected baseURL 'https://api.cohere.ai/v1', got '%s'", cp.baseURL)
	}

	if cp.client == nil {
		t.Error("Expected client to be initialized")
	}
}

func TestCohereEmbeddingsProvider_GetCapabilities(t *testing.T) {
	provider := NewCohereEmbeddingsProvider("test-key").(*CohereEmbeddingsProvider)
	caps := provider.GetCapabilities()

	if caps.SupportsChat {
		t.Error("Expected SupportsChat to be false")
	}
	if !caps.SupportsEmbeddings {
		t.Error("Expected SupportsEmbeddings to be true")
	}
	if caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be false")
	}
	if caps.MaxRequestsPerMinute != 1000 {
		t.Errorf("Expected MaxRequestsPerMinute 1000, got %d", caps.MaxRequestsPerMinute)
	}
	if caps.MaxTokensPerRequest != 96 {
		t.Errorf("Expected MaxTokensPerRequest 96, got %d", caps.MaxTokensPerRequest)
	}

	expectedParams := []string{"input_type", "truncate", "embedding_types"}
	if len(caps.SupportedParameters) != len(expectedParams) {
		t.Errorf("Expected %d supported parameters, got %d", len(expectedParams), len(caps.SupportedParameters))
	}
}

func TestCohereEmbeddingsProvider_GetEndpoints(t *testing.T) {
	provider := NewCohereEmbeddingsProvider("test-key").(*CohereEmbeddingsProvider)
	endpoints := provider.GetEndpoints()

	if len(endpoints) != 2 {
		t.Fatalf("Expected 2 endpoints, got %d", len(endpoints))
	}

	// Check embed endpoint
	if endpoints[0].Path != "/embed" {
		t.Errorf("Expected first endpoint path '/embed', got '%s'", endpoints[0].Path)
	}
	if endpoints[0].Method != "POST" {
		t.Errorf("Expected first endpoint method 'POST', got '%s'", endpoints[0].Method)
	}

	// Check models endpoint
	if endpoints[1].Path != "/models" {
		t.Errorf("Expected second endpoint path '/models', got '%s'", endpoints[1].Path)
	}
	if endpoints[1].Method != "GET" {
		t.Errorf("Expected second endpoint method 'GET', got '%s'", endpoints[1].Method)
	}

	// Test caching
	endpoints2 := provider.GetEndpoints()
	if len(endpoints2) != 2 {
		t.Errorf("Expected cached endpoints to return 2 endpoints")
	}
}

func TestCohereEmbeddingsProvider_ValidateEndpoints(t *testing.T) {
	embedCount := 0
	modelsCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch {
		case r.URL.Path == "/embed" && r.Method == "POST":
			embedCount++
			resp := cohereEmbedResponse{
				ID:         "test-id",
				Embeddings: [][]float64{{0.1, 0.2, 0.3}},
				Texts:      []string{"test"},
			}
			json.NewEncoder(w).Encode(resp)

		case r.URL.Path == "/models" && r.Method == "GET":
			modelsCount++
			resp := cohereModelsResponse{
				Models: []cohereModel{
					{Name: "embed-english-v3.0", Endpoints: []string{"embed"}},
				},
			}
			json.NewEncoder(w).Encode(resp)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	provider := NewCohereEmbeddingsProvider("test-key").(*CohereEmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify both endpoints were tested
	if embedCount == 0 {
		t.Error("Embed endpoint was not tested")
	}
	if modelsCount == 0 {
		t.Error("Models endpoint was not tested")
	}

	// Check endpoint statuses
	endpoints := provider.GetEndpoints()
	for _, ep := range endpoints {
		if ep.Status != StatusWorking {
			t.Errorf("Expected endpoint %s to have status StatusWorking, got %s", ep.Path, ep.Status)
		}
		if ep.Latency == 0 {
			t.Errorf("Expected endpoint %s to have non-zero latency", ep.Path)
		}
	}
}

func TestCohereEmbeddingsProvider_ValidateEndpoints_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := cohereEmbedResponse{
			ID:         "test-id",
			Embeddings: [][]float64{{0.1, 0.2}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewCohereEmbeddingsProvider("test-key").(*CohereEmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, true)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestCohereEmbeddingsProvider_ValidateEndpoints_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	provider := NewCohereEmbeddingsProvider("test-key").(*CohereEmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, true)
	if err != nil {
		t.Errorf("ValidateEndpoints should not return error, got: %v", err)
	}

	// Check that endpoints are marked as failed
	endpoints := provider.GetEndpoints()
	for _, ep := range endpoints {
		if ep.Status != StatusFailed {
			t.Errorf("Expected endpoint %s to have status StatusFailed, got %s", ep.Path, ep.Status)
		}
		if ep.Error == "" {
			t.Errorf("Expected endpoint %s to have error message", ep.Path)
		}
	}
}

func TestCohereEmbeddingsProvider_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		resp := cohereModelsResponse{
			Models: []cohereModel{
				{Name: "embed-english-v3.0", Endpoints: []string{"embed"}},
				{Name: "embed-multilingual-v3.0", Endpoints: []string{"embed"}},
				{Name: "command", Endpoints: []string{"generate"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewCohereEmbeddingsProvider("test-key").(*CohereEmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should only return embedding models (2 out of 3)
	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}

	// Verify model details
	for _, model := range models {
		if !strings.HasPrefix(model.ID, "embed-") {
			t.Errorf("Expected embedding model, got: %s", model.ID)
		}
		if model.CostPer1MIn == 0 {
			t.Errorf("Expected model %s to have pricing info", model.ID)
		}
		if len(model.Categories) == 0 {
			t.Errorf("Expected model %s to have categories", model.ID)
		}
		if len(model.Capabilities) == 0 {
			t.Errorf("Expected model %s to have capabilities", model.ID)
		}
	}
}

func TestCohereEmbeddingsProvider_ListModels_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := cohereModelsResponse{
			Models: []cohereModel{
				{Name: "embed-english-v3.0", Endpoints: []string{"embed"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewCohereEmbeddingsProvider("test-key").(*CohereEmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	models, err := provider.ListModels(ctx, true)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(models))
	}
}

func TestCohereEmbeddingsProvider_ListModels_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	provider := NewCohereEmbeddingsProvider("test-key").(*CohereEmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Error("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("Expected HTTP 500 error, got: %v", err)
	}
}

func TestCohereEmbeddingsProvider_TestModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embed" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var req cohereEmbedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		resp := cohereEmbedResponse{
			ID:         "test-id",
			Embeddings: [][]float64{{0.1, 0.2, 0.3, 0.4}},
			Texts:      req.Texts,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewCohereEmbeddingsProvider("test-key").(*CohereEmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.TestModel(ctx, "embed-english-v3.0", false)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestCohereEmbeddingsProvider_TestModel_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := cohereEmbedResponse{
			ID:         "test-id",
			Embeddings: [][]float64{{0.1, 0.2, 0.3}},
			Texts:      []string{"Hello, world!"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewCohereEmbeddingsProvider("test-key").(*CohereEmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.TestModel(ctx, "embed-english-v3.0", true)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestCohereEmbeddingsProvider_TestModel_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid model"))
	}))
	defer server.Close()

	provider := NewCohereEmbeddingsProvider("test-key").(*CohereEmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.TestModel(ctx, "invalid-model", false)
	if err == nil {
		t.Error("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 400") {
		t.Errorf("Expected HTTP 400 error, got: %v", err)
	}
}

func TestCohereEmbeddingsProvider_FormatModelName(t *testing.T) {
	provider := NewCohereEmbeddingsProvider("test-key").(*CohereEmbeddingsProvider)

	tests := []struct {
		input    string
		expected string
	}{
		{"embed-english-v3.0", "Cohere Embed English V3: embed-english-v3.0"},
		{"embed-multilingual-v3.0", "Cohere Embed Multilingual V3: embed-multilingual-v3.0"},
		{"embed-english-light-v3.0", "Cohere Embed English Light V3: embed-english-light-v3.0"},
		{"embed-multilingual-light-v3.0", "Cohere Embed Multilingual Light V3: embed-multilingual-light-v3.0"},
		{"unknown-model", "Cohere Embedding: unknown-model"},
	}

	for _, tt := range tests {
		result := provider.formatModelName(tt.input)
		if result != tt.expected {
			t.Errorf("formatModelName(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestCohereEmbeddingsProvider_EnrichModelDetails(t *testing.T) {
	provider := NewCohereEmbeddingsProvider("test-key").(*CohereEmbeddingsProvider)

	tests := []struct {
		modelID          string
		expectedCost     float64
		expectedContext  int
		expectedCategory string
	}{
		{"embed-english-v3.0", 0.10, 512, "embeddings"},
		{"embed-multilingual-v3.0", 0.10, 512, "multilingual"},
		{"embed-english-light-v3.0", 0.10, 512, "light"},
		{"embed-multilingual-light-v3.0", 0.10, 512, "light"},
		{"unknown-model", 0.10, 512, "embeddings"},
	}

	for _, tt := range tests {
		model := Model{ID: tt.modelID}
		enriched := provider.enrichModelDetails(model)

		if enriched.CostPer1MIn != tt.expectedCost {
			t.Errorf("Model %s: expected cost %f, got %f", tt.modelID, tt.expectedCost, enriched.CostPer1MIn)
		}
		if enriched.CostPer1MOut != 0.0 {
			t.Errorf("Model %s: expected output cost 0.0, got %f", tt.modelID, enriched.CostPer1MOut)
		}
		if enriched.ContextWindow != tt.expectedContext {
			t.Errorf("Model %s: expected context %d, got %d", tt.modelID, tt.expectedContext, enriched.ContextWindow)
		}
		if len(enriched.Categories) == 0 {
			t.Errorf("Model %s: expected categories to be populated", tt.modelID)
		}
		if len(enriched.Capabilities) == 0 {
			t.Errorf("Model %s: expected capabilities to be populated", tt.modelID)
		}
		if enriched.SupportsTools {
			t.Errorf("Model %s: embeddings should not support tools", tt.modelID)
		}
		if enriched.CanStream {
			t.Errorf("Model %s: embeddings should not support streaming", tt.modelID)
		}
	}
}

func TestCohereEmbeddingsProvider_TestEndpoint_InvalidURL(t *testing.T) {
	provider := NewCohereEmbeddingsProvider("test-key").(*CohereEmbeddingsProvider)
	provider.baseURL = "://invalid-url"

	ctx := context.Background()
	endpoint := &Endpoint{Path: "/embed", Method: "POST"}
	err := provider.testEndpoint(ctx, endpoint)
	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}
}

func TestCohereEmbeddingsProvider_TestEndpoint_RequestFailed(t *testing.T) {
	provider := NewCohereEmbeddingsProvider("test-key").(*CohereEmbeddingsProvider)
	provider.baseURL = "http://localhost:1"
	provider.client = &http.Client{Timeout: 1 * time.Millisecond}

	ctx := context.Background()
	endpoint := &Endpoint{Path: "/embed", Method: "POST"}
	err := provider.testEndpoint(ctx, endpoint)
	if err == nil {
		t.Error("Expected error for failed request, got nil")
	}
}

func TestCohereEmbeddingsProvider_ListModels_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	provider := NewCohereEmbeddingsProvider("test-key").(*CohereEmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Error("Expected decode error, got nil")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("Expected decode error, got: %v", err)
	}
}

func TestCohereEmbeddingsProvider_TestModel_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	provider := NewCohereEmbeddingsProvider("test-key").(*CohereEmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.TestModel(ctx, "embed-english-v3.0", false)
	if err == nil {
		t.Error("Expected decode error, got nil")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("Expected decode error, got: %v", err)
	}
}

func TestCohereEmbeddingsProvider_ConcurrentValidation(t *testing.T) {
	requestCount := 0
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		if strings.Contains(r.URL.Path, "embed") {
			resp := cohereEmbedResponse{Embeddings: [][]float64{{0.1}}}
			json.NewEncoder(w).Encode(resp)
		} else {
			resp := cohereModelsResponse{Models: []cohereModel{}}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	provider := NewCohereEmbeddingsProvider("test-key").(*CohereEmbeddingsProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	start := time.Now()
	err := provider.ValidateEndpoints(ctx, false)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// With concurrent execution, should complete faster than sequential
	if elapsed > 50*time.Millisecond {
		t.Errorf("Concurrent validation took too long: %v", elapsed)
	}

	mu.Lock()
	count := requestCount
	mu.Unlock()
	if count != 2 {
		t.Errorf("Expected 2 requests, got %d", count)
	}
}

func TestCohereEmbeddingsProvider_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		resp := cohereEmbedResponse{Embeddings: [][]float64{{0.1}}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewCohereEmbeddingsProvider("test-key").(*CohereEmbeddingsProvider)
	provider.baseURL = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := provider.TestModel(ctx, "embed-english-v3.0", false)
	if err == nil {
		t.Error("Expected context cancellation error, got nil")
	}
}
