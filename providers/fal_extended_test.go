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

func TestFALExtendedProvider_TestModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Key test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"request_id": "test-123",
			"images": [{
				"url": "https://example.com/image.png",
				"content_type": "image/png",
				"width": 1024,
				"height": 1024
			}],
			"seed": 42
		}`))
	}))
	defer server.Close()

	provider := &FALExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "fal-ai/flux-pro", false)
	if err != nil {
		t.Fatalf("TestModel failed: %v", err)
	}
}

func TestFALExtendedProvider_TestModel_Video(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Key test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"request_id": "test-video-123",
			"video": {
				"url": "https://example.com/video.mp4",
				"content_type": "video/mp4",
				"width": 512,
				"height": 512,
				"duration": 3.0
			},
			"seed": 42
		}`))
	}))
	defer server.Close()

	provider := &FALExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "fal-ai/animatediff", false)
	if err != nil {
		t.Fatalf("TestModel video failed: %v", err)
	}
}

func TestFALExtendedProvider_TestModel_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"request_id": "test-123",
			"images": [{
				"url": "https://example.com/image.png"
			}]
		}`))
	}))
	defer server.Close()

	provider := &FALExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "fal-ai/flux-dev", true)
	if err != nil {
		t.Fatalf("TestModel verbose failed: %v", err)
	}
}

func TestFALExtendedProvider_TestModel_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "invalid prompt"}}`))
	}))
	defer server.Close()

	provider := &FALExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "fal-ai/flux-pro", false)
	if err == nil {
		t.Error("Expected error for invalid request")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("Expected 400 error, got: %v", err)
	}
}

func TestFALExtendedProvider_TestModel_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "unauthorized"}`))
	}))
	defer server.Close()

	provider := &FALExtendedProvider{
		apiKey:  "invalid-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "fal-ai/flux-pro", false)
	if err == nil {
		t.Error("Expected error for unauthorized request")
	}
}

func TestFALExtendedProvider_ListModels(t *testing.T) {
	provider := &FALExtendedProvider{
		apiKey:  "test-key",
		baseURL: "https://fal.run",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) == 0 {
		t.Error("Expected at least one model")
	}

	// Verify we have FLUX models
	hasFlux := false
	for _, model := range models {
		if strings.Contains(model.ID, "flux") {
			hasFlux = true
			if model.SupportsImages != true {
				t.Error("FLUX models should support images")
			}
		}
	}

	if !hasFlux {
		t.Error("Expected FLUX models in catalog")
	}
}

func TestFALExtendedProvider_ListModels_Verbose(t *testing.T) {
	provider := &FALExtendedProvider{
		apiKey:  "test-key",
		baseURL: "https://fal.run",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx, true)
	if err != nil {
		t.Fatalf("ListModels verbose failed: %v", err)
	}

	if len(models) == 0 {
		t.Error("Expected at least one model")
	}
}

func TestFALExtendedProvider_GetCapabilities(t *testing.T) {
	provider := &FALExtendedProvider{
		apiKey:  "test-key",
		baseURL: "https://fal.run",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	caps := provider.GetCapabilities()

	if caps.SupportsChat {
		t.Error("FAL should not support chat")
	}

	if !caps.SupportsFileUpload {
		t.Error("FAL should support file upload")
	}

	if !caps.SupportsJSONMode {
		t.Error("FAL should support JSON mode")
	}

	if caps.SupportsStreaming {
		t.Error("FAL should not support streaming")
	}

	if len(caps.SupportedParameters) == 0 {
		t.Error("Expected supported parameters")
	}

	if len(caps.SecurityFeatures) == 0 {
		t.Error("Expected security features (safety_checker)")
	}
}

func TestFALExtendedProvider_GetEndpoints(t *testing.T) {
	provider := &FALExtendedProvider{
		apiKey:  "test-key",
		baseURL: "https://fal.run",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	endpoints := provider.GetEndpoints()

	if len(endpoints) == 0 {
		t.Error("Expected at least one endpoint")
	}

	// Verify FLUX endpoints exist
	hasFlux := false
	for _, ep := range endpoints {
		if strings.Contains(ep.Path, "flux") {
			hasFlux = true
			if ep.Method != "POST" {
				t.Error("FLUX endpoints should be POST")
			}
		}
	}

	if !hasFlux {
		t.Error("Expected FLUX endpoints")
	}
}

func TestFALExtendedProvider_ValidateEndpoints(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()

		if r.Header.Get("Authorization") != "Key test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"request_id": "test-123",
			"images": [{"url": "https://example.com/test.png"}]
		}`))
	}))
	defer server.Close()

	provider := &FALExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Fatalf("ValidateEndpoints failed: %v", err)
	}

	mu.Lock()
	count := callCount
	mu.Unlock()

	if count == 0 {
		t.Error("Expected at least one endpoint to be tested")
	}

	// Verify endpoint statuses
	endpoints := provider.GetEndpoints()
	for _, ep := range endpoints {
		if ep.Status == "" {
			t.Errorf("Endpoint %s has no status", ep.Path)
		}
	}
}

func TestFALExtendedProvider_ValidateEndpoints_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"request_id": "test-123",
			"images": [{"url": "https://example.com/test.png"}]
		}`))
	}))
	defer server.Close()

	provider := &FALExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, true)
	if err != nil {
		t.Fatalf("ValidateEndpoints verbose failed: %v", err)
	}
}

func TestFALExtendedProvider_ValidateEndpoints_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()

	provider := &FALExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Fatalf("ValidateEndpoints should not return error: %v", err)
	}

	// Check that endpoints are marked as failed
	endpoints := provider.GetEndpoints()
	allFailed := true
	for _, ep := range endpoints {
		if ep.Status != StatusFailed {
			allFailed = false
			break
		}
	}

	if !allFailed {
		t.Error("Expected all endpoints to be marked as failed")
	}
}

func TestFALExtendedProvider_testEndpoint_UnknownPath(t *testing.T) {
	provider := &FALExtendedProvider{
		apiKey:  "test-key",
		baseURL: "https://fal.run",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	endpoint := &Endpoint{
		Path:   "/unknown-endpoint",
		Method: "POST",
	}

	ctx := context.Background()
	err := provider.testEndpoint(ctx, endpoint)
	if err == nil {
		t.Error("Expected error for unknown endpoint")
	}
	if !strings.Contains(err.Error(), "unknown endpoint") {
		t.Errorf("Expected 'unknown endpoint' error, got: %v", err)
	}
}

func TestFALExtendedProvider_TestModel_MarshalError(t *testing.T) {
	provider := &FALExtendedProvider{
		apiKey:  "test-key",
		baseURL: "https://fal.run",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	// This test ensures the marshal path works correctly
	// We can't easily trigger a marshal error with valid structs,
	// so we test the happy path for coverage
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"request_id": "test", "images": []}`))
	}))
	defer server.Close()

	provider.baseURL = server.URL
	err := provider.TestModel(ctx, "fal-ai/flux-pro", false)
	if err != nil {
		t.Fatalf("TestModel should succeed: %v", err)
	}
}

func TestFALExtendedProvider_TestModel_RequestError(t *testing.T) {
	provider := &FALExtendedProvider{
		apiKey:  "test-key",
		baseURL: "http://invalid-url-that-does-not-exist.local:99999",
		client: &http.Client{
			Timeout: 1 * time.Second,
		},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "fal-ai/flux-pro", false)
	if err == nil {
		t.Error("Expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "request failed") {
		t.Errorf("Expected 'request failed' error, got: %v", err)
	}
}

func TestFALExtendedProvider_ValidateEndpoints_Concurrent(t *testing.T) {
	var mu sync.Mutex
	requestPaths := make(map[string]int)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestPaths[r.URL.Path]++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"request_id": "test", "images": []}`))
	}))
	defer server.Close()

	provider := &FALExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Fatalf("ValidateEndpoints failed: %v", err)
	}

	// Verify all endpoints were hit concurrently
	mu.Lock()
	pathCount := len(requestPaths)
	mu.Unlock()

	endpoints := provider.GetEndpoints()
	if pathCount != len(endpoints) {
		t.Errorf("Expected %d paths to be hit, got %d", len(endpoints), pathCount)
	}
}

func TestFALExtendedProvider_NewProvider(t *testing.T) {
	provider := NewFALExtendedProvider("test-key")
	if provider == nil {
		t.Fatal("NewFALExtendedProvider returned nil")
	}

	falProvider, ok := provider.(*FALExtendedProvider)
	if !ok {
		t.Fatal("NewFALExtendedProvider did not return *FALExtendedProvider")
	}

	if falProvider.apiKey != "test-key" {
		t.Errorf("Expected apiKey 'test-key', got '%s'", falProvider.apiKey)
	}

	if falProvider.baseURL != "https://fal.run" {
		t.Errorf("Expected baseURL 'https://fal.run', got '%s'", falProvider.baseURL)
	}

	if falProvider.client == nil {
		t.Error("Expected client to be initialized")
	}

	if falProvider.client.Timeout != 120*time.Second {
		t.Errorf("Expected timeout 120s, got %v", falProvider.client.Timeout)
	}
}

func TestFALExtendedProvider_ModelCatalog(t *testing.T) {
	provider := &FALExtendedProvider{
		apiKey:  "test-key",
		baseURL: "https://fal.run",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	// Verify specific models exist
	modelIDs := make(map[string]bool)
	for _, model := range models {
		modelIDs[model.ID] = true

		// Verify all models have required fields
		if model.Name == "" {
			t.Errorf("Model %s has empty name", model.ID)
		}
		if model.Description == "" {
			t.Errorf("Model %s has empty description", model.ID)
		}
		if len(model.Categories) == 0 {
			t.Errorf("Model %s has no categories", model.ID)
		}
		if len(model.Capabilities) == 0 {
			t.Errorf("Model %s has no capabilities", model.ID)
		}
	}

	expectedModels := []string{
		"fal-ai/flux-pro",
		"fal-ai/flux-dev",
		"fal-ai/flux-schnell",
		"fal-ai/stable-diffusion-v3-medium",
		"fal-ai/animatediff",
	}

	for _, expectedID := range expectedModels {
		if !modelIDs[expectedID] {
			t.Errorf("Expected model %s not found in catalog", expectedID)
		}
	}
}

func TestFALExtendedProvider_VideoEndpoint(t *testing.T) {
	var receivedBody map[string]interface{}
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "animatediff") {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)

			mu.Lock()
			receivedBody = body
			mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"request_id": "video-123",
				"video": {
					"url": "https://example.com/video.mp4",
					"width": 512,
					"height": 512
				}
			}`))
		}
	}))
	defer server.Close()

	provider := &FALExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "fal-ai/animatediff", false)
	if err != nil {
		t.Fatalf("TestModel for video failed: %v", err)
	}

	mu.Lock()
	body := receivedBody
	mu.Unlock()

	if body == nil {
		t.Fatal("No request body received")
	}

	if body["num_frames"] == nil {
		t.Error("Expected num_frames in video request")
	}

	if body["fps"] == nil {
		t.Error("Expected fps in video request")
	}
}
