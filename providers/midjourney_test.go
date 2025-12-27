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

func TestNewMidjourneyProvider(t *testing.T) {
	provider := NewMidjourneyProvider("test-api-key")
	if provider == nil {
		t.Fatal("Expected provider to be created, got nil")
	}

	mjProvider, ok := provider.(*MidjourneyProvider)
	if !ok {
		t.Fatal("Expected MidjourneyProvider type")
	}

	if mjProvider.apiKey != "test-api-key" {
		t.Errorf("Expected apiKey 'test-api-key', got '%s'", mjProvider.apiKey)
	}

	if mjProvider.baseURL != "https://api.midjourney.com/v1" {
		t.Errorf("Expected baseURL 'https://api.midjourney.com/v1', got '%s'", mjProvider.baseURL)
	}

	if mjProvider.client == nil {
		t.Error("Expected client to be initialized")
	}

	if mjProvider.client.Timeout != 90*time.Second {
		t.Errorf("Expected timeout 90s, got %v", mjProvider.client.Timeout)
	}
}

func TestMidjourneyProvider_ProviderRegistration(t *testing.T) {
	factory, exists := GetProviderFactory("midjourney")
	if !exists {
		t.Fatal("Expected midjourney provider to be registered")
	}

	provider := factory("test-key")
	if provider == nil {
		t.Fatal("Expected factory to create provider")
	}

	_, ok := provider.(*MidjourneyProvider)
	if !ok {
		t.Fatal("Expected MidjourneyProvider type from factory")
	}
}

func TestMidjourneyProvider_GetCapabilities(t *testing.T) {
	provider := NewMidjourneyProvider("test-key")
	caps := provider.GetCapabilities()

	if caps.SupportsChat {
		t.Error("Expected SupportsChat to be false")
	}

	if caps.SupportsEmbeddings {
		t.Error("Expected SupportsEmbeddings to be false")
	}

	if caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be false")
	}

	if caps.MaxRequestsPerMinute != 60 {
		t.Errorf("Expected MaxRequestsPerMinute 60, got %d", caps.MaxRequestsPerMinute)
	}

	if caps.MaxTokensPerRequest != 350 {
		t.Errorf("Expected MaxTokensPerRequest 350, got %d", caps.MaxTokensPerRequest)
	}

	expectedParams := []string{"prompt", "model", "aspect_ratio", "quality", "stylize", "chaos", "weird", "tile"}
	if len(caps.SupportedParameters) != len(expectedParams) {
		t.Errorf("Expected %d supported parameters, got %d", len(expectedParams), len(caps.SupportedParameters))
	}
}

func TestMidjourneyProvider_GetEndpoints(t *testing.T) {
	provider := NewMidjourneyProvider("test-key")
	endpoints := provider.GetEndpoints()

	if len(endpoints) != 2 {
		t.Fatalf("Expected 2 endpoints, got %d", len(endpoints))
	}

	// Check /imagine endpoint
	imagineFound := false
	statusFound := false

	for _, ep := range endpoints {
		if ep.Path == "/imagine" && ep.Method == "POST" {
			imagineFound = true
			if ep.Headers["Authorization"] != "Bearer test-key" {
				t.Errorf("Expected Authorization header 'Bearer test-key', got '%s'", ep.Headers["Authorization"])
			}
		}
		if ep.Path == "/status/{id}" && ep.Method == "GET" {
			statusFound = true
		}
	}

	if !imagineFound {
		t.Error("Expected /imagine endpoint not found")
	}
	if !statusFound {
		t.Error("Expected /status/{id} endpoint not found")
	}
}

func TestMidjourneyProvider_ListModels(t *testing.T) {
	provider := NewMidjourneyProvider("test-key")
	ctx := context.Background()

	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(models))
	}

	// Check v6 model
	v6Found := false
	v61Found := false

	for _, model := range models {
		if model.ID == "v6" {
			v6Found = true
			if model.Name != "Midjourney V6" {
				t.Errorf("Expected name 'Midjourney V6', got '%s'", model.Name)
			}
			if !model.SupportsImages {
				t.Error("Expected v6 to support images")
			}
		}
		if model.ID == "v6.1" {
			v61Found = true
			if !model.SupportsImages {
				t.Error("Expected v6.1 to support images")
			}
		}
	}

	if !v6Found {
		t.Error("Expected v6 model not found")
	}
	if !v61Found {
		t.Error("Expected v6.1 model not found")
	}
}

func TestMidjourneyProvider_ListModelsVerbose(t *testing.T) {
	provider := NewMidjourneyProvider("test-key")
	ctx := context.Background()

	// Test verbose mode (just ensure it doesn't error)
	models, err := provider.ListModels(ctx, true)
	if err != nil {
		t.Fatalf("Expected no error in verbose mode, got %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("Expected 2 models in verbose mode, got %d", len(models))
	}
}

func TestMidjourneyProvider_ValidateEndpoints(t *testing.T) {
	// Create a test server
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)

		// Check authorization
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(midjourneyErrorResponse{
				Error:   "unauthorized",
				Message: "Invalid API key",
			})
			return
		}

		if r.URL.Path == "/imagine" && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(midjourneyImagineResponse{
				ID:        "test-123",
				Status:    "pending",
				Prompt:    "test",
				Model:     "v6",
				CreatedAt: time.Now().Format(time.RFC3339),
			})
			return
		}

		if strings.HasPrefix(r.URL.Path, "/status/") && r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(midjourneyStatusResponse{
				ID:        "test-123",
				Status:    "completed",
				Prompt:    "test",
				Model:     "v6",
				ImageURLs: []string{"https://example.com/image.png"},
				Progress:  100,
				CreatedAt: time.Now().Format(time.RFC3339),
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := NewMidjourneyProvider("test-key")
	mjProvider := provider.(*MidjourneyProvider)
	mjProvider.baseURL = server.URL

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check that endpoints were tested
	endpoints := provider.GetEndpoints()
	workingCount := 0
	for _, ep := range endpoints {
		if ep.Status == StatusWorking {
			workingCount++
		}
	}

	if workingCount != 2 {
		t.Errorf("Expected 2 working endpoints, got %d", workingCount)
	}

	// Verify requests were made in parallel
	if atomic.LoadInt32(&requestCount) != 2 {
		t.Errorf("Expected 2 requests, got %d", atomic.LoadInt32(&requestCount))
	}
}

func TestMidjourneyProvider_ValidateEndpointsVerbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	provider := NewMidjourneyProvider("test-key")
	mjProvider := provider.(*MidjourneyProvider)
	mjProvider.baseURL = server.URL

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, true)
	if err != nil {
		t.Fatalf("Expected no error in verbose mode, got %v", err)
	}
}

func TestMidjourneyProvider_ValidateEndpointsFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
	}))
	defer server.Close()

	provider := NewMidjourneyProvider("test-key")
	mjProvider := provider.(*MidjourneyProvider)
	mjProvider.baseURL = server.URL

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Fatalf("ValidateEndpoints should not return error, got %v", err)
	}

	// Check that endpoints failed
	endpoints := provider.GetEndpoints()
	failedCount := 0
	for _, ep := range endpoints {
		if ep.Status == StatusFailed {
			failedCount++
		}
	}

	if failedCount != 2 {
		t.Errorf("Expected 2 failed endpoints, got %d", failedCount)
	}
}

func TestMidjourneyProvider_TestModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/imagine" {
			t.Errorf("Expected path /imagine, got %s", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("Expected method POST, got %s", r.Method)
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(midjourneyImagineResponse{
			ID:        "test-456",
			Status:    "pending",
			Prompt:    "test prompt for validation",
			Model:     "v6",
			CreatedAt: time.Now().Format(time.RFC3339),
		})
	}))
	defer server.Close()

	provider := NewMidjourneyProvider("test-key")
	mjProvider := provider.(*MidjourneyProvider)
	mjProvider.baseURL = server.URL

	ctx := context.Background()
	err := provider.TestModel(ctx, "v6", false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestMidjourneyProvider_TestModelVerbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(midjourneyImagineResponse{
			ID:        "test-789",
			Status:    "pending",
			Prompt:    "test",
			Model:     "v6.1",
			CreatedAt: time.Now().Format(time.RFC3339),
		})
	}))
	defer server.Close()

	provider := NewMidjourneyProvider("test-key")
	mjProvider := provider.(*MidjourneyProvider)
	mjProvider.baseURL = server.URL

	ctx := context.Background()
	err := provider.TestModel(ctx, "v6.1", true)
	if err != nil {
		t.Fatalf("Expected no error in verbose mode, got %v", err)
	}
}

func TestMidjourneyProvider_TestModelInvalid(t *testing.T) {
	provider := NewMidjourneyProvider("test-key")
	ctx := context.Background()

	err := provider.TestModel(ctx, "invalid-model", false)
	if err == nil {
		t.Fatal("Expected error for invalid model, got nil")
	}

	if !strings.Contains(err.Error(), "invalid model ID") {
		t.Errorf("Expected 'invalid model ID' error, got %v", err)
	}
}

func TestMidjourneyProvider_TestModelError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(midjourneyErrorResponse{
			Error:   "bad_request",
			Message: "Invalid prompt",
			Code:    "INVALID_PROMPT",
		})
	}))
	defer server.Close()

	provider := NewMidjourneyProvider("test-key")
	mjProvider := provider.(*MidjourneyProvider)
	mjProvider.baseURL = server.URL

	ctx := context.Background()
	err := provider.TestModel(ctx, "v6", false)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "bad_request") {
		t.Errorf("Expected 'bad_request' in error, got %v", err)
	}
}

func TestMidjourneyProvider_Imagine(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var req midjourneyImagineRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if req.Prompt == "" {
			t.Error("Expected non-empty prompt")
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(midjourneyImagineResponse{
			ID:        "gen-123",
			Status:    "pending",
			Prompt:    req.Prompt,
			Model:     req.Model,
			CreatedAt: time.Now().Format(time.RFC3339),
		})
	}))
	defer server.Close()

	provider := NewMidjourneyProvider("test-key")
	mjProvider := provider.(*MidjourneyProvider)
	mjProvider.baseURL = server.URL

	ctx := context.Background()
	resp, err := mjProvider.Imagine(ctx, midjourneyImagineRequest{
		Prompt:      "A beautiful sunset",
		Model:       "v6",
		AspectRatio: "16:9",
		Quality:     "high",
		Stylize:     500,
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.ID != "gen-123" {
		t.Errorf("Expected ID 'gen-123', got '%s'", resp.ID)
	}

	if resp.Status != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", resp.Status)
	}

	if resp.Prompt != "A beautiful sunset" {
		t.Errorf("Expected prompt 'A beautiful sunset', got '%s'", resp.Prompt)
	}
}

func TestMidjourneyProvider_ImagineError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(midjourneyErrorResponse{
			Error:   "invalid_request",
			Message: "Prompt is too long",
			Code:    "PROMPT_TOO_LONG",
		})
	}))
	defer server.Close()

	provider := NewMidjourneyProvider("test-key")
	mjProvider := provider.(*MidjourneyProvider)
	mjProvider.baseURL = server.URL

	ctx := context.Background()
	_, err := mjProvider.Imagine(ctx, midjourneyImagineRequest{
		Prompt: "test",
		Model:  "v6",
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "invalid_request") {
		t.Errorf("Expected 'invalid_request' in error, got %v", err)
	}

	if !strings.Contains(err.Error(), "Prompt is too long") {
		t.Errorf("Expected 'Prompt is too long' in error, got %v", err)
	}
}

func TestMidjourneyProvider_GetStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}

		if !strings.HasPrefix(r.URL.Path, "/status/") {
			t.Errorf("Expected path to start with /status/, got %s", r.URL.Path)
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(midjourneyStatusResponse{
			ID:          "gen-123",
			Status:      "completed",
			Prompt:      "A beautiful sunset",
			Model:       "v6",
			ImageURLs:   []string{"https://example.com/image1.png", "https://example.com/image2.png"},
			Progress:    100,
			CreatedAt:   time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
			UpdatedAt:   time.Now().Add(-1 * time.Minute).Format(time.RFC3339),
			CompletedAt: time.Now().Format(time.RFC3339),
		})
	}))
	defer server.Close()

	provider := NewMidjourneyProvider("test-key")
	mjProvider := provider.(*MidjourneyProvider)
	mjProvider.baseURL = server.URL

	ctx := context.Background()
	resp, err := mjProvider.GetStatus(ctx, "gen-123")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.ID != "gen-123" {
		t.Errorf("Expected ID 'gen-123', got '%s'", resp.ID)
	}

	if resp.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", resp.Status)
	}

	if len(resp.ImageURLs) != 2 {
		t.Errorf("Expected 2 image URLs, got %d", len(resp.ImageURLs))
	}

	if resp.Progress != 100 {
		t.Errorf("Expected progress 100, got %d", resp.Progress)
	}
}

func TestMidjourneyProvider_GetStatusProcessing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(midjourneyStatusResponse{
			ID:       "gen-456",
			Status:   "processing",
			Prompt:   "test",
			Model:    "v6",
			Progress: 45,
		})
	}))
	defer server.Close()

	provider := NewMidjourneyProvider("test-key")
	mjProvider := provider.(*MidjourneyProvider)
	mjProvider.baseURL = server.URL

	ctx := context.Background()
	resp, err := mjProvider.GetStatus(ctx, "gen-456")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.Status != "processing" {
		t.Errorf("Expected status 'processing', got '%s'", resp.Status)
	}

	if resp.Progress != 45 {
		t.Errorf("Expected progress 45, got %d", resp.Progress)
	}
}

func TestMidjourneyProvider_GetStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(midjourneyErrorResponse{
			Error:   "not_found",
			Message: "Generation not found",
			Code:    "GENERATION_NOT_FOUND",
		})
	}))
	defer server.Close()

	provider := NewMidjourneyProvider("test-key")
	mjProvider := provider.(*MidjourneyProvider)
	mjProvider.baseURL = server.URL

	ctx := context.Background()
	_, err := mjProvider.GetStatus(ctx, "invalid-id")

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "not_found") {
		t.Errorf("Expected 'not_found' in error, got %v", err)
	}
}

func TestMidjourneyProvider_ContextCancellation(t *testing.T) {
	// Test that context cancellation is respected
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewMidjourneyProvider("test-key")
	mjProvider := provider.(*MidjourneyProvider)
	mjProvider.baseURL = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := mjProvider.Imagine(ctx, midjourneyImagineRequest{
		Prompt: "test",
		Model:  "v6",
	})

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "deadline") {
		t.Errorf("Expected context/deadline error, got %v", err)
	}
}

func TestMidjourneyProvider_AllModelsValid(t *testing.T) {
	// Ensure all models returned by ListModels are valid in TestModel
	provider := NewMidjourneyProvider("test-key")
	mjProvider := provider.(*MidjourneyProvider)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(midjourneyImagineResponse{
			ID:     "test",
			Status: "pending",
		})
	}))
	defer server.Close()

	mjProvider.baseURL = server.URL

	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels error: %v", err)
	}

	for _, model := range models {
		err := provider.TestModel(ctx, model.ID, false)
		if err != nil {
			t.Errorf("Model %s failed TestModel: %v", model.ID, err)
		}
	}
}
