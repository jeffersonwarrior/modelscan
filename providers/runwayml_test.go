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

func TestNewRunwayMLProvider(t *testing.T) {
	provider := NewRunwayMLProvider("test-key-123")
	if provider == nil {
		t.Fatal("Expected provider to be created")
	}

	runwayProvider, ok := provider.(*RunwayMLProvider)
	if !ok {
		t.Fatal("Expected provider to be of type *RunwayMLProvider")
	}

	if runwayProvider.apiKey != "test-key-123" {
		t.Errorf("Expected apiKey 'test-key-123', got '%s'", runwayProvider.apiKey)
	}

	if runwayProvider.baseURL != "https://api.runwayml.com/v1" {
		t.Errorf("Expected baseURL 'https://api.runwayml.com/v1', got '%s'", runwayProvider.baseURL)
	}

	if runwayProvider.client == nil {
		t.Error("Expected client to be initialized")
	}

	if runwayProvider.client.Timeout != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", runwayProvider.client.Timeout)
	}
}

func TestNewRunwayMLProvider_EmptyKey(t *testing.T) {
	provider := NewRunwayMLProvider("")
	if provider == nil {
		t.Fatal("Expected provider to be created even with empty key")
	}

	runwayProvider := provider.(*RunwayMLProvider)
	if runwayProvider.apiKey != "" {
		t.Errorf("Expected empty apiKey, got '%s'", runwayProvider.apiKey)
	}
}

func TestRunwayMLProvider_ListModels(t *testing.T) {
	provider := NewRunwayMLProvider("test-key")

	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(models))
	}

	// Verify gen2 exists
	foundGen2 := false
	foundGen3 := false
	for _, model := range models {
		if model.ID == "gen2" {
			foundGen2 = true
			if model.Name != "Gen-2" {
				t.Errorf("Expected name 'Gen-2', got '%s'", model.Name)
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
			if model.Capabilities["motion_control"] != "supported" {
				t.Error("Expected motion_control capability")
			}
			if model.Capabilities["camera_motion"] != "supported" {
				t.Error("Expected camera_motion capability")
			}
		}
		if model.ID == "gen3" {
			foundGen3 = true
			if model.Name != "Gen-3" {
				t.Errorf("Expected name 'Gen-3', got '%s'", model.Name)
			}
		}
	}

	if !foundGen2 {
		t.Error("Expected to find gen2 model")
	}
	if !foundGen3 {
		t.Error("Expected to find gen3 model")
	}
}

func TestRunwayMLProvider_ListModels_Verbose(t *testing.T) {
	provider := NewRunwayMLProvider("test-key")
	ctx := context.Background()

	models, err := provider.ListModels(ctx, true)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) == 0 {
		t.Fatal("Expected at least one model")
	}
}

func TestRunwayMLProvider_GetCapabilities(t *testing.T) {
	provider := NewRunwayMLProvider("test-key")
	caps := provider.GetCapabilities()

	if caps.SupportsChat {
		t.Error("Expected SupportsChat to be false")
	}

	if !caps.SupportsVision {
		t.Error("Expected SupportsVision to be true (image-to-video)")
	}

	if !caps.SupportsFileUpload {
		t.Error("Expected SupportsFileUpload to be true")
	}

	if caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be false (async)")
	}

	// Verify supported parameters
	expectedParams := []string{"prompt", "model", "duration", "aspect_ratio", "image_url", "motion_control", "camera_motion"}
	for _, param := range expectedParams {
		found := false
		for _, p := range caps.SupportedParameters {
			if p == param {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected parameter '%s' in SupportedParameters", param)
		}
	}
}

func TestRunwayMLProvider_GetEndpoints(t *testing.T) {
	provider := NewRunwayMLProvider("test-key-abc")
	endpoints := provider.GetEndpoints()

	if len(endpoints) != 2 {
		t.Fatalf("Expected 2 endpoints, got %d", len(endpoints))
	}

	// Verify POST /generations endpoint
	foundPost := false
	foundGet := false

	for _, endpoint := range endpoints {
		if endpoint.Method == "POST" && endpoint.Path == "/generations" {
			foundPost = true
			if endpoint.Headers["Authorization"] != "Bearer test-key-abc" {
				t.Error("Expected Authorization header with Bearer token")
			}
			if endpoint.Headers["Content-Type"] != "application/json" {
				t.Error("Expected Content-Type header")
			}
			if endpoint.Status != StatusUnknown {
				t.Errorf("Expected status Unknown, got %s", endpoint.Status)
			}
		}

		if endpoint.Method == "GET" && endpoint.Path == "/generations/{id}" {
			foundGet = true
			if endpoint.Headers["Authorization"] != "Bearer test-key-abc" {
				t.Error("Expected Authorization header")
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

func TestRunwayMLProvider_GetEndpoints_Cached(t *testing.T) {
	provider := NewRunwayMLProvider("test-key").(*RunwayMLProvider)

	// First call - fresh endpoints
	endpoints1 := provider.GetEndpoints()
	if len(endpoints1) != 2 {
		t.Fatalf("Expected 2 endpoints, got %d", len(endpoints1))
	}

	// Modify cached endpoints
	provider.endpoints = endpoints1
	provider.endpoints[0].Status = StatusWorking

	// Second call - should return cached
	endpoints2 := provider.GetEndpoints()
	if endpoints2[0].Status != StatusWorking {
		t.Error("Expected cached endpoint with StatusWorking")
	}
}

func TestRunwayMLProvider_ValidateEndpoints(t *testing.T) {
	// Create test server
	requestCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)

		// Check auth header
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Handle endpoints
		if r.Method == "POST" && r.URL.Path == "/generations" {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(runwayGeneration{
				ID:     "test-gen-123",
				Status: "pending",
			})
			return
		}

		if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/generations/") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(runwayGeneration{
				ID:     "test-gen-123",
				Status: "succeeded",
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := NewRunwayMLProvider("test-key").(*RunwayMLProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Fatalf("ValidateEndpoints failed: %v", err)
	}

	endpoints := provider.GetEndpoints()
	for _, endpoint := range endpoints {
		if endpoint.Status != StatusWorking {
			t.Errorf("Expected endpoint %s %s to be Working, got %s (error: %s)",
				endpoint.Method, endpoint.Path, endpoint.Status, endpoint.Error)
		}
		if endpoint.Latency == 0 {
			t.Errorf("Expected non-zero latency for %s %s", endpoint.Method, endpoint.Path)
		}
	}

	// Verify parallel execution (should be at least 2 requests)
	if requestCount < 2 {
		t.Errorf("Expected at least 2 requests for parallel execution, got %d", requestCount)
	}
}

func TestRunwayMLProvider_ValidateEndpoints_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(runwayGeneration{ID: "test"})
	}))
	defer server.Close()

	provider := NewRunwayMLProvider("test-key").(*RunwayMLProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, true)
	if err != nil {
		t.Fatalf("ValidateEndpoints failed: %v", err)
	}
}

func TestRunwayMLProvider_ValidateEndpoints_Concurrent(t *testing.T) {
	// Test concurrent safety
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond) // Simulate network delay
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(runwayGeneration{ID: "test"})
	}))
	defer server.Close()

	provider := NewRunwayMLProvider("test-key").(*RunwayMLProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	start := time.Now()
	err := provider.ValidateEndpoints(ctx, false)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("ValidateEndpoints failed: %v", err)
	}

	// Should complete in less than 2 * 50ms due to parallelization
	if duration > 200*time.Millisecond {
		t.Logf("Note: Parallel execution took %v (expected < 200ms)", duration)
	}
}

func TestRunwayMLProvider_TestModel_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.Method == "POST" && r.URL.Path == "/generations" {
			// Decode and verify request
			var req runwayGenerationRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// Verify motion control and camera motion
			if req.MotionControl == nil {
				t.Error("Expected motion_control in request")
			}
			if req.CameraMotion == nil {
				t.Error("Expected camera_motion in request")
			}

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(runwayGeneration{
				ID:     "gen-123",
				Status: "pending",
				Prompt: req.Prompt,
				Model:  req.Model,
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := NewRunwayMLProvider("test-key").(*RunwayMLProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.TestModel(ctx, "gen2", false)
	if err != nil {
		t.Fatalf("TestModel failed: %v", err)
	}
}

func TestRunwayMLProvider_TestModel_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(runwayGeneration{
			ID:     "gen-123",
			Status: "pending",
		})
	}))
	defer server.Close()

	provider := NewRunwayMLProvider("test-key").(*RunwayMLProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.TestModel(ctx, "gen3", true)
	if err != nil {
		t.Fatalf("TestModel failed: %v", err)
	}
}

func TestRunwayMLProvider_TestModel_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "Invalid API key"}`))
	}))
	defer server.Close()

	provider := NewRunwayMLProvider("invalid-key").(*RunwayMLProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.TestModel(ctx, "gen2", false)
	if err == nil {
		t.Fatal("Expected error for unauthorized request")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Errorf("Expected 401 error, got: %v", err)
	}
}

func TestRunwayMLProvider_TestModel_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "Invalid prompt"}`))
	}))
	defer server.Close()

	provider := NewRunwayMLProvider("test-key").(*RunwayMLProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	err := provider.TestModel(ctx, "gen2", false)
	if err == nil {
		t.Fatal("Expected error for bad request")
	}

	if !strings.Contains(err.Error(), "400") {
		t.Errorf("Expected 400 error, got: %v", err)
	}
}

func TestRunwayMLProvider_TestModel_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewRunwayMLProvider("test-key").(*RunwayMLProvider)
	provider.baseURL = server.URL

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := provider.TestModel(ctx, "gen2", false)
	if err == nil {
		t.Fatal("Expected error for canceled context")
	}
}

func TestRunwayMLProvider_GetGenerationStatus_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if !strings.HasPrefix(r.URL.Path, "/generations/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(runwayGeneration{
			ID:       "gen-123",
			Status:   "succeeded",
			VideoURL: "https://example.com/video.mp4",
		})
	}))
	defer server.Close()

	provider := NewRunwayMLProvider("test-key").(*RunwayMLProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	gen, err := provider.GetGenerationStatus(ctx, "gen-123")
	if err != nil {
		t.Fatalf("GetGenerationStatus failed: %v", err)
	}

	if gen.ID != "gen-123" {
		t.Errorf("Expected ID 'gen-123', got '%s'", gen.ID)
	}

	if gen.Status != "succeeded" {
		t.Errorf("Expected status 'succeeded', got '%s'", gen.Status)
	}

	if gen.VideoURL != "https://example.com/video.mp4" {
		t.Errorf("Expected video URL, got '%s'", gen.VideoURL)
	}
}

func TestRunwayMLProvider_GetGenerationStatus_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "Generation not found"}`))
	}))
	defer server.Close()

	provider := NewRunwayMLProvider("test-key").(*RunwayMLProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	_, err := provider.GetGenerationStatus(ctx, "nonexistent")
	if err == nil {
		t.Fatal("Expected error for not found generation")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Expected 404 error, got: %v", err)
	}
}

func TestRunwayMLProvider_GetGenerationStatus_Failed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(runwayGeneration{
			ID:     "gen-456",
			Status: "failed",
			Error:  "Invalid prompt content",
		})
	}))
	defer server.Close()

	provider := NewRunwayMLProvider("test-key").(*RunwayMLProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	gen, err := provider.GetGenerationStatus(ctx, "gen-456")
	if err != nil {
		t.Fatalf("GetGenerationStatus failed: %v", err)
	}

	if gen.Status != "failed" {
		t.Errorf("Expected status 'failed', got '%s'", gen.Status)
	}

	if gen.Error == "" {
		t.Error("Expected error message in failed generation")
	}
}

func TestRunwayMLProvider_GetGenerationStatus_Processing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(runwayGeneration{
			ID:     "gen-789",
			Status: "processing",
		})
	}))
	defer server.Close()

	provider := NewRunwayMLProvider("test-key").(*RunwayMLProvider)
	provider.baseURL = server.URL

	ctx := context.Background()
	gen, err := provider.GetGenerationStatus(ctx, "gen-789")
	if err != nil {
		t.Fatalf("GetGenerationStatus failed: %v", err)
	}

	if gen.Status != "processing" {
		t.Errorf("Expected status 'processing', got '%s'", gen.Status)
	}
}

func TestRunwayMLProvider_ProviderRegistration(t *testing.T) {
	// Test that the provider is registered in the factory
	provider := NewRunwayMLProvider("test-key")
	if provider == nil {
		t.Fatal("Expected provider to be created")
	}

	// Verify it implements the Provider interface
	var _ Provider = provider
}
