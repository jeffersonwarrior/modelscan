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

func TestNewVoyageAIProvider(t *testing.T) {
	apiKey := "test-key-123"
	provider := NewVoyageAIProvider(apiKey)

	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}

	va, ok := provider.(*VoyageAIProvider)
	if !ok {
		t.Fatal("Expected *VoyageAIProvider")
	}

	if va.apiKey != apiKey {
		t.Errorf("Expected apiKey=%s, got %s", apiKey, va.apiKey)
	}

	if va.baseURL != "https://api.voyageai.com/v1" {
		t.Errorf("Expected baseURL=https://api.voyageai.com/v1, got %s", va.baseURL)
	}

	if va.client == nil {
		t.Error("Expected non-nil HTTP client")
	}

	if va.client.Timeout != 60*time.Second {
		t.Errorf("Expected timeout=60s, got %v", va.client.Timeout)
	}
}

func TestVoyageAIProvider_ProviderRegistration(t *testing.T) {
	factory, exists := GetProviderFactory("voyageai")
	if !exists {
		t.Fatal("Expected voyageai provider to be registered")
	}

	provider := factory("test-key")
	if provider == nil {
		t.Fatal("Expected factory to return non-nil provider")
	}

	_, ok := provider.(*VoyageAIProvider)
	if !ok {
		t.Fatal("Expected factory to return *VoyageAIProvider")
	}
}

func TestVoyageAIProvider_GetEndpoints(t *testing.T) {
	provider := NewVoyageAIProvider("test-key")
	endpoints := provider.GetEndpoints()

	if len(endpoints) == 0 {
		t.Fatal("Expected at least one endpoint")
	}

	// Verify embeddings endpoint
	found := false
	for _, ep := range endpoints {
		if ep.Path == "/embeddings" && ep.Method == "POST" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected /embeddings endpoint")
	}
}

func TestVoyageAIProvider_GetCapabilities(t *testing.T) {
	provider := NewVoyageAIProvider("test-key")
	caps := provider.GetCapabilities()

	if caps.SupportsChat {
		t.Error("Expected SupportsChat=false for embeddings provider")
	}

	if !caps.SupportsEmbeddings {
		t.Error("Expected SupportsEmbeddings=true")
	}

	if caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming=false for embeddings")
	}

	if caps.MaxTokensPerRequest != 16000 {
		t.Errorf("Expected MaxTokensPerRequest=16000, got %d", caps.MaxTokensPerRequest)
	}
}

func TestVoyageAIProvider_ListModels(t *testing.T) {
	provider := NewVoyageAIProvider("test-key")
	ctx := context.Background()

	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) < 3 {
		t.Errorf("Expected at least 3 models, got %d", len(models))
	}

	// Verify voyage-2 model
	foundVoyage2 := false
	for _, m := range models {
		if m.ID == "voyage-2" {
			foundVoyage2 = true
			if m.ContextWindow != 16000 {
				t.Errorf("voyage-2: expected context window 16000, got %d", m.ContextWindow)
			}
			if !contains(m.Categories, "embeddings") {
				t.Error("voyage-2: expected 'embeddings' in categories")
			}
		}
	}
	if !foundVoyage2 {
		t.Error("Expected to find voyage-2 model")
	}

	// Verify voyage-code-2 model
	foundCode := false
	for _, m := range models {
		if m.ID == "voyage-code-2" {
			foundCode = true
			if !contains(m.Categories, "code") {
				t.Error("voyage-code-2: expected 'code' in categories")
			}
		}
	}
	if !foundCode {
		t.Error("Expected to find voyage-code-2 model")
	}

	// Verify voyage-large-2 model
	foundLarge := false
	for _, m := range models {
		if m.ID == "voyage-large-2" {
			foundLarge = true
			if !contains(m.Categories, "high-performance") {
				t.Error("voyage-large-2: expected 'high-performance' in categories")
			}
		}
	}
	if !foundLarge {
		t.Error("Expected to find voyage-large-2 model")
	}
}

func TestVoyageAIProvider_ListModels_Verbose(t *testing.T) {
	provider := NewVoyageAIProvider("test-key")
	ctx := context.Background()

	// Test verbose mode doesn't cause errors
	models, err := provider.ListModels(ctx, true)
	if err != nil {
		t.Fatalf("ListModels verbose failed: %v", err)
	}

	if len(models) == 0 {
		t.Error("Expected models in verbose mode")
	}
}

func TestVoyageAIProvider_TestModel_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("Expected /v1/embeddings, got %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Error("Expected Authorization header with Bearer token")
		}

		// Parse request body
		var req voyageEmbeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		if req.Model == "" {
			t.Error("Expected model in request")
		}

		// Send response
		resp := voyageEmbeddingResponse{
			Object: "list",
			Data: []voyageEmbedding{
				{
					Object:    "embedding",
					Embedding: make([]float64, 1024),
					Index:     0,
				},
			},
			Model: req.Model,
			Usage: voyageEmbeddingUsage{
				TotalTokens: 5,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewVoyageAIProvider("test-key")
	va := provider.(*VoyageAIProvider)
	va.baseURL = server.URL + "/v1"

	ctx := context.Background()
	err := provider.TestModel(ctx, "voyage-2", false)
	if err != nil {
		t.Fatalf("TestModel failed: %v", err)
	}
}

func TestVoyageAIProvider_TestModel_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := voyageEmbeddingResponse{
			Object: "list",
			Data: []voyageEmbedding{
				{
					Object:    "embedding",
					Embedding: make([]float64, 1536),
					Index:     0,
				},
			},
			Model: "voyage-code-2",
			Usage: voyageEmbeddingUsage{
				TotalTokens: 8,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewVoyageAIProvider("test-key")
	va := provider.(*VoyageAIProvider)
	va.baseURL = server.URL + "/v1"

	ctx := context.Background()
	err := provider.TestModel(ctx, "voyage-code-2", true)
	if err != nil {
		t.Fatalf("TestModel verbose failed: %v", err)
	}
}

func TestVoyageAIProvider_TestModel_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "Invalid API key"}`))
	}))
	defer server.Close()

	provider := NewVoyageAIProvider("invalid-key")
	va := provider.(*VoyageAIProvider)
	va.baseURL = server.URL + "/v1"

	ctx := context.Background()
	err := provider.TestModel(ctx, "voyage-2", false)
	if err == nil {
		t.Fatal("Expected error for unauthorized request")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Errorf("Expected error to mention 401, got: %v", err)
	}
}

func TestVoyageAIProvider_TestModel_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	provider := NewVoyageAIProvider("test-key")
	va := provider.(*VoyageAIProvider)
	va.baseURL = server.URL + "/v1"

	ctx := context.Background()
	err := provider.TestModel(ctx, "voyage-2", false)
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("Expected decode error, got: %v", err)
	}
}

func TestVoyageAIProvider_ValidateEndpoints_Success(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		resp := voyageEmbeddingResponse{
			Object: "list",
			Data: []voyageEmbedding{
				{
					Object:    "embedding",
					Embedding: make([]float64, 1024),
					Index:     0,
				},
			},
			Model: "voyage-2",
			Usage: voyageEmbeddingUsage{
				TotalTokens: 3,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewVoyageAIProvider("test-key")
	va := provider.(*VoyageAIProvider)
	va.baseURL = server.URL + "/v1"

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Fatalf("ValidateEndpoints failed: %v", err)
	}

	if callCount == 0 {
		t.Error("Expected at least one API call")
	}

	endpoints := provider.GetEndpoints()
	for _, ep := range endpoints {
		if ep.Status != StatusWorking {
			t.Errorf("Expected endpoint %s to be working, got status: %s", ep.Path, ep.Status)
		}
	}
}

func TestVoyageAIProvider_ValidateEndpoints_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := voyageEmbeddingResponse{
			Object: "list",
			Data: []voyageEmbedding{
				{
					Object:    "embedding",
					Embedding: make([]float64, 1024),
					Index:     0,
				},
			},
			Model: "voyage-2",
			Usage: voyageEmbeddingUsage{
				TotalTokens: 3,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewVoyageAIProvider("test-key")
	va := provider.(*VoyageAIProvider)
	va.baseURL = server.URL + "/v1"

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, true)
	if err != nil {
		t.Fatalf("ValidateEndpoints verbose failed: %v", err)
	}
}

func TestVoyageAIProvider_ValidateEndpoints_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	provider := NewVoyageAIProvider("test-key")
	va := provider.(*VoyageAIProvider)
	va.baseURL = server.URL + "/v1"

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Fatalf("ValidateEndpoints should not return error, got: %v", err)
	}

	endpoints := provider.GetEndpoints()
	for _, ep := range endpoints {
		if ep.Status != StatusFailed {
			t.Errorf("Expected endpoint %s to fail, got status: %s", ep.Path, ep.Status)
		}
		if ep.Error == "" {
			t.Error("Expected error message for failed endpoint")
		}
	}
}

func TestVoyageAIProvider_ValidateEndpoints_Concurrent(t *testing.T) {
	requestTimes := make(map[string]time.Time)
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestTimes[r.URL.Path] = time.Now()
		mu.Unlock()

		// Simulate some processing time
		time.Sleep(10 * time.Millisecond)

		resp := voyageEmbeddingResponse{
			Object: "list",
			Data: []voyageEmbedding{
				{
					Object:    "embedding",
					Embedding: make([]float64, 1024),
					Index:     0,
				},
			},
			Model: "voyage-2",
			Usage: voyageEmbeddingUsage{
				TotalTokens: 3,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewVoyageAIProvider("test-key")
	va := provider.(*VoyageAIProvider)
	va.baseURL = server.URL + "/v1"

	ctx := context.Background()
	start := time.Now()
	err := provider.ValidateEndpoints(ctx, false)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("ValidateEndpoints failed: %v", err)
	}

	// With concurrent execution, should be much faster than sequential
	// 1 endpoint * 10ms = 10ms, add buffer for processing
	if elapsed > 100*time.Millisecond {
		t.Errorf("Expected concurrent execution to complete quickly, took %v", elapsed)
	}
}

func TestVoyageAIProvider_ValidateEndpoints_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewVoyageAIProvider("test-key")
	va := provider.(*VoyageAIProvider)
	va.baseURL = server.URL + "/v1"

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Fatalf("ValidateEndpoints should not return error, got: %v", err)
	}

	endpoints := provider.GetEndpoints()
	for _, ep := range endpoints {
		if ep.Status != StatusFailed {
			t.Errorf("Expected endpoint %s to fail due to timeout, got status: %s", ep.Path, ep.Status)
		}
	}
}

func TestVoyageAIProvider_EmbeddingRequest_StringInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req voyageEmbeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		// Verify string input
		if str, ok := req.Input.(string); !ok {
			t.Errorf("Expected string input, got %T", req.Input)
		} else if str != "test embedding" {
			t.Errorf("Expected 'test embedding', got '%s'", str)
		}

		resp := voyageEmbeddingResponse{
			Object: "list",
			Data: []voyageEmbedding{
				{
					Object:    "embedding",
					Embedding: make([]float64, 1024),
					Index:     0,
				},
			},
			Model: "voyage-2",
			Usage: voyageEmbeddingUsage{
				TotalTokens: 5,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewVoyageAIProvider("test-key")
	va := provider.(*VoyageAIProvider)
	va.baseURL = server.URL + "/v1"

	ctx := context.Background()
	err := provider.TestModel(ctx, "voyage-2", false)
	if err != nil {
		t.Fatalf("TestModel failed: %v", err)
	}
}

// Helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
