package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewLumaAIProvider(t *testing.T) {
	provider := NewLumaAIProvider("test-key-123")
	if provider == nil {
		t.Fatal("Expected provider to be created")
	}

	lumaProvider, ok := provider.(*LumaAIProvider)
	if !ok {
		t.Fatal("Expected provider to be of type *LumaAIProvider")
	}

	if lumaProvider.apiKey != "test-key-123" {
		t.Errorf("Expected apiKey 'test-key-123', got '%s'", lumaProvider.apiKey)
	}

	if lumaProvider.baseURL != "https://api.lumalabs.ai/v1" {
		t.Errorf("Expected baseURL 'https://api.lumalabs.ai/v1', got '%s'", lumaProvider.baseURL)
	}

	if lumaProvider.client == nil {
		t.Error("Expected client to be initialized")
	}

	if lumaProvider.client.Timeout != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", lumaProvider.client.Timeout)
	}
}

func TestNewLumaAIProvider_EmptyKey(t *testing.T) {
	provider := NewLumaAIProvider("")
	if provider == nil {
		t.Fatal("Expected provider to be created even with empty key")
	}

	lumaProvider := provider.(*LumaAIProvider)
	if lumaProvider.apiKey != "" {
		t.Errorf("Expected empty apiKey, got '%s'", lumaProvider.apiKey)
	}
}

func TestLumaAIProvider_ListModels(t *testing.T) {
	provider := NewLumaAIProvider("test-key")

	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) == 0 {
		t.Fatal("Expected at least one model")
	}

	// Verify dream-machine-v1 exists
	found := false
	for _, model := range models {
		if model.ID == "dream-machine-v1" {
			found = true
			if model.Name != "Dream Machine v1" {
				t.Errorf("Expected name 'Dream Machine v1', got '%s'", model.Name)
			}
			if !model.SupportsImages {
				t.Error("Expected model to support images")
			}
			if model.CanStream {
				t.Error("Expected model to not support streaming (async only)")
			}

			// Verify categories
			foundVideo := false
			for _, cat := range model.Categories {
				if cat == "video" {
					foundVideo = true
					break
				}
			}
			if !foundVideo {
				t.Error("Expected 'video' category")
			}

			// Verify capabilities
			if model.Capabilities["aspect_ratios"] == "" {
				t.Error("Expected aspect_ratios capability")
			}
			break
		}
	}

	if !found {
		t.Error("Expected to find dream-machine-v1 model")
	}
}

func TestLumaAIProvider_ListModels_Verbose(t *testing.T) {
	provider := NewLumaAIProvider("test-key")

	ctx := context.Background()
	models, err := provider.ListModels(ctx, true)
	if err != nil {
		t.Fatalf("ListModels verbose failed: %v", err)
	}

	if len(models) == 0 {
		t.Fatal("Expected at least one model")
	}
}

func TestLumaAIProvider_GetCapabilities(t *testing.T) {
	provider := NewLumaAIProvider("test-key")

	caps := provider.GetCapabilities()

	if caps.SupportsChat {
		t.Error("Expected SupportsChat to be false")
	}

	if !caps.SupportsFileUpload {
		t.Error("Expected SupportsFileUpload to be true (image keyframes)")
	}

	if !caps.SupportsVision {
		t.Error("Expected SupportsVision to be true (image-to-video)")
	}

	if caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be false (async generation)")
	}

	if caps.SupportsAudio {
		t.Error("Expected SupportsAudio to be false")
	}

	// Verify supported parameters
	foundPrompt := false
	foundAspectRatio := false
	for _, param := range caps.SupportedParameters {
		if param == "prompt" {
			foundPrompt = true
		}
		if param == "aspect_ratio" {
			foundAspectRatio = true
		}
	}
	if !foundPrompt {
		t.Error("Expected 'prompt' in supported parameters")
	}
	if !foundAspectRatio {
		t.Error("Expected 'aspect_ratio' in supported parameters")
	}

	if caps.MaxTokensPerRequest != 2000 {
		t.Errorf("Expected MaxTokensPerRequest 2000, got %d", caps.MaxTokensPerRequest)
	}
}

func TestLumaAIProvider_GetEndpoints(t *testing.T) {
	provider := NewLumaAIProvider("test-key-123")

	endpoints := provider.GetEndpoints()
	if len(endpoints) == 0 {
		t.Fatal("Expected at least one endpoint")
	}

	// Verify POST /generations endpoint
	foundPost := false
	foundGet := false
	for _, ep := range endpoints {
		if ep.Method == "POST" && ep.Path == "/generations" {
			foundPost = true
			if ep.Headers["Authorization"] != "Bearer test-key-123" {
				t.Errorf("Expected Authorization header with Bearer token")
			}
			if ep.Headers["Content-Type"] != "application/json" {
				t.Error("Expected Content-Type: application/json")
			}
		}
		if ep.Method == "GET" && strings.Contains(ep.Path, "/generations/") {
			foundGet = true
			if ep.Headers["Authorization"] != "Bearer test-key-123" {
				t.Errorf("Expected Authorization header with Bearer token")
			}
		}
	}

	if !foundPost {
		t.Error("Expected POST /generations endpoint")
	}
	if !foundGet {
		t.Error("Expected GET /generations/{id} endpoint")
	}
}

func TestLumaAIProvider_TestModel_Success(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)

		if r.URL.Path != "/generations" {
			t.Errorf("Expected path /generations, got %s", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "missing bearer token"}`))
			return
		}

		// Decode and validate request body
		var req lumaGenerationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": "invalid JSON"}`))
			return
		}

		if req.Prompt == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": "prompt required"}`))
			return
		}

		// Return successful generation response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{
			"id": "gen_123456",
			"state": "queued",
			"prompt": "A serene lake at sunset",
			"created_at": "2025-12-27T00:00:00Z"
		}`))
	}))
	defer server.Close()

	provider := &LumaAIProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "dream-machine-v1", false)
	if err != nil {
		t.Fatalf("TestModel failed: %v", err)
	}

	if atomic.LoadInt32(&requestCount) != 1 {
		t.Errorf("Expected 1 request, got %d", requestCount)
	}
}

func TestLumaAIProvider_TestModel_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{
			"id": "gen_verbose",
			"state": "processing",
			"prompt": "test"
		}`))
	}))
	defer server.Close()

	provider := &LumaAIProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "dream-machine-v1", true)
	if err != nil {
		t.Fatalf("TestModel verbose failed: %v", err)
	}
}

func TestLumaAIProvider_TestModel_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid API key"}`))
	}))
	defer server.Close()

	provider := &LumaAIProvider{
		apiKey:  "invalid-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "dream-machine-v1", false)
	if err == nil {
		t.Fatal("Expected error for unauthorized request")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Errorf("Expected 401 error, got: %v", err)
	}
}

func TestLumaAIProvider_TestModel_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "invalid prompt"}`))
	}))
	defer server.Close()

	provider := &LumaAIProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "dream-machine-v1", false)
	if err == nil {
		t.Fatal("Expected error for bad request")
	}

	if !strings.Contains(err.Error(), "400") {
		t.Errorf("Expected 400 error, got: %v", err)
	}
}

func TestLumaAIProvider_TestModel_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": "gen_cancel"}`))
	}))
	defer server.Close()

	provider := &LumaAIProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := provider.TestModel(ctx, "dream-machine-v1", false)
	if err == nil {
		t.Fatal("Expected error for cancelled context")
	}
}

func TestLumaAIProvider_ValidateEndpoints(t *testing.T) {
	var postCount, getCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			atomic.AddInt32(&postCount, 1)
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id": "gen_test", "state": "queued"}`))
		} else if r.Method == "GET" {
			atomic.AddInt32(&getCount, 1)
			// GET requests might return 404 for non-existent IDs
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "not found"}`))
		}
	}))
	defer server.Close()

	provider := &LumaAIProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Fatalf("ValidateEndpoints failed: %v", err)
	}

	// Verify both endpoints were tested
	if atomic.LoadInt32(&postCount) == 0 {
		t.Error("Expected POST endpoint to be tested")
	}
	if atomic.LoadInt32(&getCount) == 0 {
		t.Error("Expected GET endpoint to be tested")
	}

	// Check endpoint status was updated
	endpoints := provider.GetEndpoints()
	for _, ep := range endpoints {
		if ep.Status == StatusUnknown {
			t.Errorf("Endpoint %s %s still has unknown status", ep.Method, ep.Path)
		}
		if ep.Latency == 0 {
			t.Errorf("Endpoint %s %s has zero latency", ep.Method, ep.Path)
		}
	}
}

func TestLumaAIProvider_ValidateEndpoints_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": "test"}`))
	}))
	defer server.Close()

	provider := &LumaAIProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, true)
	if err != nil {
		t.Fatalf("ValidateEndpoints verbose failed: %v", err)
	}
}

func TestLumaAIProvider_ValidateEndpoints_Concurrent(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		// Simulate some latency to ensure concurrency
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	provider := &LumaAIProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	start := time.Now()
	err := provider.ValidateEndpoints(ctx, false)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("ValidateEndpoints failed: %v", err)
	}

	// With 2 endpoints and 50ms each, serial would take ~100ms
	// Concurrent should be closer to 50ms
	if elapsed > 90*time.Millisecond {
		t.Logf("Warning: validation took %v, may not be running concurrently", elapsed)
	}

	if atomic.LoadInt32(&requestCount) < 2 {
		t.Errorf("Expected at least 2 requests, got %d", requestCount)
	}
}

func TestLumaAIProvider_GetGenerationStatus_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET method, got %s", r.Method)
		}

		if !strings.Contains(r.URL.Path, "/generations/") {
			t.Errorf("Expected path to contain /generations/, got %s", r.URL.Path)
		}

		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "gen_123456",
			"state": "completed",
			"prompt": "A serene lake at sunset",
			"created_at": "2025-12-27T00:00:00Z",
			"assets": {
				"video": "https://cdn.lumalabs.ai/videos/gen_123456.mp4"
			}
		}`))
	}))
	defer server.Close()

	provider := &LumaAIProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	generation, err := provider.GetGenerationStatus(ctx, "gen_123456")
	if err != nil {
		t.Fatalf("GetGenerationStatus failed: %v", err)
	}

	if generation.ID != "gen_123456" {
		t.Errorf("Expected ID 'gen_123456', got '%s'", generation.ID)
	}

	if generation.State != "completed" {
		t.Errorf("Expected state 'completed', got '%s'", generation.State)
	}

	if generation.Assets == nil {
		t.Fatal("Expected assets to be present")
	}

	if generation.Assets.Video == "" {
		t.Error("Expected video URL to be present")
	}
}

func TestLumaAIProvider_GetGenerationStatus_Failed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "gen_failed",
			"state": "failed",
			"failure_code": "invalid_prompt",
			"failure_message": "The prompt violates content policy"
		}`))
	}))
	defer server.Close()

	provider := &LumaAIProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	generation, err := provider.GetGenerationStatus(ctx, "gen_failed")
	if err != nil {
		t.Fatalf("GetGenerationStatus failed: %v", err)
	}

	if generation.State != "failed" {
		t.Errorf("Expected state 'failed', got '%s'", generation.State)
	}

	if generation.FailureCode != "invalid_prompt" {
		t.Errorf("Expected failure_code 'invalid_prompt', got '%s'", generation.FailureCode)
	}
}

func TestLumaAIProvider_GetGenerationStatus_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "generation not found"}`))
	}))
	defer server.Close()

	provider := &LumaAIProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	_, err := provider.GetGenerationStatus(ctx, "nonexistent")
	if err == nil {
		t.Fatal("Expected error for not found generation")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Expected 404 error, got: %v", err)
	}
}

func TestLumaAIProvider_ProviderRegistration(t *testing.T) {
	factory, exists := GetProviderFactory("lumaai")
	if !exists {
		t.Fatal("Expected lumaai provider to be registered")
	}

	provider := factory("test-key")
	if provider == nil {
		t.Fatal("Expected factory to create provider")
	}

	_, ok := provider.(*LumaAIProvider)
	if !ok {
		t.Fatal("Expected provider to be of type *LumaAIProvider")
	}
}
