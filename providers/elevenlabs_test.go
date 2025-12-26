package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewElevenLabsProvider(t *testing.T) {
	provider := NewElevenLabsProvider("test-key")
	if provider == nil {
		t.Fatal("Expected provider to be created")
	}

	elevenlabsProvider, ok := provider.(*ElevenLabsProvider)
	if !ok {
		t.Fatal("Expected provider to be of type *ElevenLabsProvider")
	}

	if elevenlabsProvider.apiKey != "test-key" {
		t.Errorf("Expected apiKey 'test-key', got '%s'", elevenlabsProvider.apiKey)
	}

	if elevenlabsProvider.baseURL != "https://api.elevenlabs.io/v1" {
		t.Errorf("Expected baseURL 'https://api.elevenlabs.io/v1', got '%s'", elevenlabsProvider.baseURL)
	}
}

func TestNewElevenLabsProvider_EmptyKey(t *testing.T) {
	provider := NewElevenLabsProvider("")
	if provider == nil {
		t.Fatal("Expected provider to be created even with empty key")
	}

	elevenlabsProvider := provider.(*ElevenLabsProvider)
	if elevenlabsProvider.apiKey != "" {
		t.Errorf("Expected empty apiKey, got '%s'", elevenlabsProvider.apiKey)
	}
}

func TestElevenLabsProvider_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/voices" {
			t.Errorf("Expected path /voices, got %s", r.URL.Path)
		}

		if r.Header.Get("xi-api-key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"voices": [
				{
					"voice_id": "21m00Tcm4TlvDq8ikWAM",
					"name": "Rachel",
					"category": "premade",
					"description": "Calm and professional"
				},
				{
					"voice_id": "AZnzlk1XvdvUeBnXmlld",
					"name": "Domi",
					"category": "premade",
					"description": "Strong and confident"
				}
			]
		}`))
	}))
	defer server.Close()

	provider := &ElevenLabsProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}

	// Verify first voice
	if models[0].ID != "21m00Tcm4TlvDq8ikWAM" {
		t.Errorf("Expected ID '21m00Tcm4TlvDq8ikWAM', got '%s'", models[0].ID)
	}
	if models[0].Name != "Rachel" {
		t.Errorf("Expected name 'Rachel', got '%s'", models[0].Name)
	}
	if models[0].Description != "Calm and professional" {
		t.Errorf("Expected description 'Calm and professional', got '%s'", models[0].Description)
	}

	// Verify pricing
	if models[0].CostPer1MIn != 180.0 {
		t.Errorf("Expected CostPer1MIn 180.0, got %f", models[0].CostPer1MIn)
	}

	// Verify categories
	foundTTS := false
	foundPremade := false
	for _, cat := range models[0].Categories {
		if cat == "tts" {
			foundTTS = true
		}
		if cat == "premade" {
			foundPremade = true
		}
	}
	if !foundTTS {
		t.Error("Expected 'tts' category")
	}
	if !foundPremade {
		t.Error("Expected 'premade' category")
	}
}

func TestElevenLabsProvider_ListModels_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"voices": []}`))
	}))
	defer server.Close()

	provider := &ElevenLabsProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	_, err := provider.ListModels(ctx, true)
	if err != nil {
		t.Fatalf("ListModels verbose failed: %v", err)
	}
}

func TestElevenLabsProvider_ListModels_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"detail": {"status": "invalid_api_key"}}`))
	}))
	defer server.Close()

	provider := &ElevenLabsProvider{
		apiKey:  "invalid-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Error("Expected error with invalid API key")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("Expected error to mention status 401, got: %v", err)
	}
}

func TestElevenLabsProvider_ListModels_ContextCancelled(t *testing.T) {
	provider := &ElevenLabsProvider{
		apiKey:  "test-key",
		baseURL: "https://api.elevenlabs.io/v1",
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}

func TestElevenLabsProvider_ListModels_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	provider := &ElevenLabsProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Error("Expected error with invalid JSON")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("Expected decode error, got: %v", err)
	}
}

func TestElevenLabsProvider_GetCapabilities(t *testing.T) {
	provider := NewElevenLabsProvider("test-key")
	caps := provider.GetCapabilities()

	if caps.SupportsChat {
		t.Error("Expected SupportsChat to be false")
	}
	if !caps.SupportsAudio {
		t.Error("Expected SupportsAudio to be true")
	}
	if !caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be true")
	}
	if !caps.SupportsFileUpload {
		t.Error("Expected SupportsFileUpload to be true for voice cloning")
	}
	if caps.SupportsEmbeddings {
		t.Error("Expected SupportsEmbeddings to be false")
	}

	if caps.MaxRequestsPerMinute != 20 {
		t.Errorf("Expected MaxRequestsPerMinute 20, got %d", caps.MaxRequestsPerMinute)
	}
	if caps.MaxTokensPerRequest != 5000 {
		t.Errorf("Expected MaxTokensPerRequest 5000, got %d", caps.MaxTokensPerRequest)
	}

	// Verify supported parameters
	expectedParams := []string{"voice_id", "model_id", "voice_settings", "stability", "similarity_boost"}
	if len(caps.SupportedParameters) != len(expectedParams) {
		t.Errorf("Expected %d parameters, got %d", len(expectedParams), len(caps.SupportedParameters))
	}
}

func TestElevenLabsProvider_GetEndpoints(t *testing.T) {
	provider := NewElevenLabsProvider("test-key")
	endpoints := provider.GetEndpoints()

	if len(endpoints) != 4 {
		t.Errorf("Expected 4 endpoints, got %d", len(endpoints))
	}

	// Verify each endpoint
	expectedPaths := map[string]string{
		"/voices":            "GET",
		"/models":            "GET",
		"/user/subscription": "GET",
		"/history":           "GET",
	}

	for _, endpoint := range endpoints {
		expectedMethod, exists := expectedPaths[endpoint.Path]
		if !exists {
			t.Errorf("Unexpected endpoint path: %s", endpoint.Path)
			continue
		}
		if endpoint.Method != expectedMethod {
			t.Errorf("Expected method %s for %s, got %s", expectedMethod, endpoint.Path, endpoint.Method)
		}
		if endpoint.Headers["xi-api-key"] != "test-key" {
			t.Errorf("Expected xi-api-key header for %s", endpoint.Path)
		}
	}
}

func TestElevenLabsProvider_TestModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/text-to-speech/") {
			t.Errorf("Expected path to start with /text-to-speech/, got %s", r.URL.Path)
		}

		if r.Header.Get("xi-api-key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Return mock audio data (just a placeholder)
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("mock audio data"))
	}))
	defer server.Close()

	provider := &ElevenLabsProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "21m00Tcm4TlvDq8ikWAM", false)
	if err != nil {
		t.Errorf("TestModel failed: %v", err)
	}
}

func TestElevenLabsProvider_TestModel_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("audio"))
	}))
	defer server.Close()

	provider := &ElevenLabsProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "test-voice", true)
	if err != nil {
		t.Errorf("TestModel verbose failed: %v", err)
	}
}

func TestElevenLabsProvider_TestModel_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"detail": {"status": "voice_not_found"}}`))
	}))
	defer server.Close()

	provider := &ElevenLabsProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "invalid-voice", false)
	if err == nil {
		t.Error("Expected error with invalid voice")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("Expected error to mention status 400, got: %v", err)
	}
}

func TestElevenLabsProvider_TestModel_ContextCancelled(t *testing.T) {
	provider := &ElevenLabsProvider{
		apiKey:  "test-key",
		baseURL: "https://api.elevenlabs.io/v1",
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := provider.TestModel(ctx, "test-voice", false)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}

func TestElevenLabsProvider_ValidateEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"voices": []}`))
	}))
	defer server.Close()

	provider := &ElevenLabsProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Errorf("ValidateEndpoints failed: %v", err)
	}

	// Verify endpoints were updated with status
	endpoints := provider.GetEndpoints()
	for _, endpoint := range endpoints {
		if endpoint.Status != StatusWorking {
			t.Errorf("Expected endpoint %s to be working, got status %s", endpoint.Path, endpoint.Status)
		}
	}
}

func TestElevenLabsProvider_ValidateEndpoints_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	provider := &ElevenLabsProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, true)
	if err != nil {
		t.Errorf("ValidateEndpoints verbose failed: %v", err)
	}
}

func TestElevenLabsProvider_ValidateEndpoints_SomeFailures(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		// Make every other endpoint fail
		if count%2 == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "server error"}`))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		}
	}))
	defer server.Close()

	provider := &ElevenLabsProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	// ValidateEndpoints doesn't return an error, it sets endpoint statuses
	if err != nil {
		t.Errorf("ValidateEndpoints should not return error: %v", err)
	}

	// Verify some endpoints failed
	failedCount := 0
	workingCount := 0
	for _, endpoint := range provider.endpoints {
		if endpoint.Status == StatusFailed {
			failedCount++
			if endpoint.Error == "" {
				t.Error("Expected error message for failed endpoint")
			}
		} else if endpoint.Status == StatusWorking {
			workingCount++
		}
	}

	if failedCount == 0 {
		t.Error("Expected some endpoints to fail")
	}
	if workingCount == 0 {
		t.Error("Expected some endpoints to work")
	}
}

func TestElevenLabsProvider_testEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("xi-api-key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	provider := &ElevenLabsProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	endpoint := &Endpoint{
		Path:   "/voices",
		Method: "GET",
		Headers: map[string]string{
			"xi-api-key": "test-key",
		},
	}

	ctx := context.Background()
	err := provider.testEndpoint(ctx, endpoint)
	if err != nil {
		t.Errorf("testEndpoint failed: %v", err)
	}
}

func TestElevenLabsProvider_testEndpoint_Failed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "not found"}`))
	}))
	defer server.Close()

	provider := &ElevenLabsProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	endpoint := &Endpoint{
		Path:   "/nonexistent",
		Method: "GET",
		Headers: map[string]string{
			"xi-api-key": "test-key",
		},
	}

	ctx := context.Background()
	err := provider.testEndpoint(ctx, endpoint)
	if err == nil {
		t.Error("Expected error for failed endpoint")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Expected error to mention status 404, got: %v", err)
	}
}
