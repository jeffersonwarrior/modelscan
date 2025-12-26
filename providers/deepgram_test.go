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

func TestNewDeepgramProvider(t *testing.T) {
	provider := NewDeepgramProvider("test-key")
	if provider == nil {
		t.Fatal("Expected provider to be created")
	}

	deepgramProvider, ok := provider.(*DeepgramProvider)
	if !ok {
		t.Fatal("Expected provider to be of type *DeepgramProvider")
	}

	if deepgramProvider.apiKey != "test-key" {
		t.Errorf("Expected apiKey 'test-key', got '%s'", deepgramProvider.apiKey)
	}

	if deepgramProvider.baseURL != "https://api.deepgram.com/v1" {
		t.Errorf("Expected baseURL 'https://api.deepgram.com/v1', got '%s'", deepgramProvider.baseURL)
	}
}

func TestNewDeepgramProvider_EmptyKey(t *testing.T) {
	provider := NewDeepgramProvider("")
	if provider == nil {
		t.Fatal("Expected provider to be created even with empty key")
	}

	deepgramProvider := provider.(*DeepgramProvider)
	if deepgramProvider.apiKey != "" {
		t.Errorf("Expected empty apiKey, got '%s'", deepgramProvider.apiKey)
	}
}

func TestDeepgramProvider_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("Expected path /models, got %s", r.URL.Path)
		}

		if r.Header.Get("Authorization") != "Token test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"stt": [
				{
					"name": "Nova-2",
					"canonical_name": "nova-2",
					"architecture": "transformer",
					"language": "en",
					"version": "2024-01-09",
					"uuid": "abc-123",
					"batch": true,
					"streaming": true
				},
				{
					"name": "Whisper Cloud",
					"canonical_name": "whisper-large",
					"architecture": "whisper",
					"language": "multilingual",
					"version": "2023-12-01",
					"batch": true,
					"streaming": false
				}
			]
		}`))
	}))
	defer server.Close()

	provider := &DeepgramProvider{
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

	// Verify first model (Nova-2)
	if models[0].ID != "nova-2" {
		t.Errorf("Expected ID 'nova-2', got '%s'", models[0].ID)
	}
	if models[0].Name != "Nova-2" {
		t.Errorf("Expected name 'Nova-2', got '%s'", models[0].Name)
	}
	if !strings.Contains(models[0].Description, "transformer") {
		t.Errorf("Expected description to contain 'transformer', got '%s'", models[0].Description)
	}
	if !models[0].CanStream {
		t.Error("Expected Nova-2 to support streaming")
	}

	// Verify categories
	foundSTT := false
	foundStreaming := false
	for _, cat := range models[0].Categories {
		if cat == "stt" {
			foundSTT = true
		}
		if cat == "streaming" {
			foundStreaming = true
		}
	}
	if !foundSTT {
		t.Error("Expected 'stt' category")
	}
	if !foundStreaming {
		t.Error("Expected 'streaming' category for Nova-2")
	}

	// Verify second model (Whisper)
	if models[1].ID != "whisper-large" {
		t.Errorf("Expected ID 'whisper-large', got '%s'", models[1].ID)
	}
	if models[1].CanStream {
		t.Error("Expected Whisper to not support streaming")
	}
}

func TestDeepgramProvider_ListModels_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"stt": []}`))
	}))
	defer server.Close()

	provider := &DeepgramProvider{
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

func TestDeepgramProvider_ListModels_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"err_code": "INVALID_AUTH", "err_msg": "Invalid credentials"}`))
	}))
	defer server.Close()

	provider := &DeepgramProvider{
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

func TestDeepgramProvider_ListModels_ContextCancelled(t *testing.T) {
	provider := &DeepgramProvider{
		apiKey:  "test-key",
		baseURL: "https://api.deepgram.com/v1",
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}

func TestDeepgramProvider_ListModels_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	provider := &DeepgramProvider{
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

func TestDeepgramProvider_GetCapabilities(t *testing.T) {
	provider := NewDeepgramProvider("test-key")
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
		t.Error("Expected SupportsFileUpload to be true for batch transcription")
	}
	if !caps.SupportsFineTuning {
		t.Error("Expected SupportsFineTuning to be true")
	}
	if caps.SupportsEmbeddings {
		t.Error("Expected SupportsEmbeddings to be false")
	}

	if caps.MaxRequestsPerMinute != 60 {
		t.Errorf("Expected MaxRequestsPerMinute 60, got %d", caps.MaxRequestsPerMinute)
	}

	// Verify supported parameters
	expectedParams := []string{"model", "language", "punctuate", "diarize", "smart_format", "utterances"}
	if len(caps.SupportedParameters) != len(expectedParams) {
		t.Errorf("Expected %d parameters, got %d", len(expectedParams), len(caps.SupportedParameters))
	}

	// Verify security features
	if len(caps.SecurityFeatures) == 0 {
		t.Error("Expected security features to be populated")
	}
}

func TestDeepgramProvider_GetEndpoints(t *testing.T) {
	provider := NewDeepgramProvider("test-key")
	endpoints := provider.GetEndpoints()

	if len(endpoints) != 2 {
		t.Errorf("Expected 2 endpoints, got %d", len(endpoints))
	}

	// Verify each endpoint
	expectedPaths := map[string]string{
		"/models":   "GET",
		"/projects": "GET",
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
		if endpoint.Headers["Authorization"] != "Token test-key" {
			t.Errorf("Expected Authorization header for %s", endpoint.Path)
		}
	}
}

func TestDeepgramProvider_TestModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/listen") {
			t.Errorf("Expected path to start with /listen, got %s", r.URL.Path)
		}

		if r.Header.Get("Authorization") != "Token test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Return mock transcription response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"metadata": {
				"transaction_key": "test",
				"request_id": "test-123",
				"duration": 1.5,
				"channels": 1
			},
			"results": {
				"channels": [
					{
						"alternatives": [
							{
								"transcript": "test audio",
								"confidence": 0.99
							}
						]
					}
				]
			}
		}`))
	}))
	defer server.Close()

	provider := &DeepgramProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "nova-2", false)
	if err != nil {
		t.Errorf("TestModel failed: %v", err)
	}
}

func TestDeepgramProvider_TestModel_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"results": {}}`))
	}))
	defer server.Close()

	provider := &DeepgramProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "test-model", true)
	if err != nil {
		t.Errorf("TestModel verbose failed: %v", err)
	}
}

func TestDeepgramProvider_TestModel_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"err_code": "INVALID_MODEL", "err_msg": "Model not found"}`))
	}))
	defer server.Close()

	provider := &DeepgramProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "invalid-model", false)
	if err == nil {
		t.Error("Expected error with invalid model")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("Expected error to mention status 400, got: %v", err)
	}
}

func TestDeepgramProvider_TestModel_ContextCancelled(t *testing.T) {
	provider := &DeepgramProvider{
		apiKey:  "test-key",
		baseURL: "https://api.deepgram.com/v1",
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := provider.TestModel(ctx, "test-model", false)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}

func TestDeepgramProvider_ValidateEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"stt": [], "projects": []}`))
	}))
	defer server.Close()

	provider := &DeepgramProvider{
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

func TestDeepgramProvider_ValidateEndpoints_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	provider := &DeepgramProvider{
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

func TestDeepgramProvider_ValidateEndpoints_SomeFailures(t *testing.T) {
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

	provider := &DeepgramProvider{
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

func TestDeepgramProvider_testEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Token test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	provider := &DeepgramProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	endpoint := &Endpoint{
		Path:   "/models",
		Method: "GET",
		Headers: map[string]string{
			"Authorization": "Token test-key",
		},
	}

	ctx := context.Background()
	err := provider.testEndpoint(ctx, endpoint)
	if err != nil {
		t.Errorf("testEndpoint failed: %v", err)
	}
}

func TestDeepgramProvider_testEndpoint_Failed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "not found"}`))
	}))
	defer server.Close()

	provider := &DeepgramProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	endpoint := &Endpoint{
		Path:   "/nonexistent",
		Method: "GET",
		Headers: map[string]string{
			"Authorization": "Token test-key",
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
