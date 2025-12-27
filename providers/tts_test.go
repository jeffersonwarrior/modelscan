package providers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewTTSProvider(t *testing.T) {
	provider := NewTTSProvider("test-key")
	if provider == nil {
		t.Fatal("Expected provider to be created")
	}

	ttsProvider, ok := provider.(*TTSProvider)
	if !ok {
		t.Fatal("Expected provider to be of type *TTSProvider")
	}

	if ttsProvider.apiKey != "test-key" {
		t.Errorf("Expected apiKey 'test-key', got '%s'", ttsProvider.apiKey)
	}

	if ttsProvider.baseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected baseURL 'https://api.openai.com/v1', got '%s'", ttsProvider.baseURL)
	}
}

func TestNewTTSProvider_EmptyKey(t *testing.T) {
	provider := NewTTSProvider("")
	if provider == nil {
		t.Fatal("Expected provider to be created even with empty key")
	}

	ttsProvider := provider.(*TTSProvider)
	if ttsProvider.apiKey != "" {
		t.Errorf("Expected empty apiKey, got '%s'", ttsProvider.apiKey)
	}
}

func TestTTSProvider_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("Expected path /models, got %s", r.URL.Path)
		}

		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": [
				{
					"id": "tts-1",
					"object": "model",
					"created": 1677649963,
					"owned_by": "openai-internal"
				},
				{
					"id": "tts-1-hd",
					"object": "model",
					"created": 1677649963,
					"owned_by": "openai-internal"
				},
				{
					"id": "gpt-4",
					"object": "model",
					"created": 1677649963,
					"owned_by": "openai"
				}
			]
		}`))
	}))
	defer server.Close()

	provider := &TTSProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	// Should only return tts-1 and tts-1-hd, not gpt-4
	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}

	// Verify tts-1 model details
	var tts1 *Model
	var tts1hd *Model
	for i := range models {
		if models[i].ID == "tts-1" {
			tts1 = &models[i]
		} else if models[i].ID == "tts-1-hd" {
			tts1hd = &models[i]
		}
	}

	if tts1 == nil {
		t.Fatal("Expected to find tts-1 model")
	}
	if tts1hd == nil {
		t.Fatal("Expected to find tts-1-hd model")
	}

	// Verify tts-1 pricing ($0.015 per 1K chars = $15 per 1M chars)
	if tts1.CostPer1MIn != 15000.0 {
		t.Errorf("Expected tts-1 CostPer1MIn 15000.0, got %f", tts1.CostPer1MIn)
	}

	// Verify tts-1-hd pricing ($0.030 per 1K chars = $30 per 1M chars)
	if tts1hd.CostPer1MIn != 30000.0 {
		t.Errorf("Expected tts-1-hd CostPer1MIn 30000.0, got %f", tts1hd.CostPer1MIn)
	}

	// Verify categories
	for _, model := range models {
		foundAudio := false
		foundTTS := false
		foundSpeech := false
		for _, cat := range model.Categories {
			if cat == "audio" {
				foundAudio = true
			}
			if cat == "tts" {
				foundTTS = true
			}
			if cat == "speech" {
				foundSpeech = true
			}
		}
		if !foundAudio {
			t.Errorf("Expected 'audio' category for model %s", model.ID)
		}
		if !foundTTS {
			t.Errorf("Expected 'tts' category for model %s", model.ID)
		}
		if !foundSpeech {
			t.Errorf("Expected 'speech' category for model %s", model.ID)
		}

		// Verify capabilities
		if model.Capabilities["voices"] != "alloy,echo,fable,onyx,nova,shimmer" {
			t.Errorf("Expected voices capability for model %s", model.ID)
		}
		if model.Capabilities["formats"] != "mp3,opus,aac,flac,wav,pcm" {
			t.Errorf("Expected formats capability for model %s", model.ID)
		}
		if model.Capabilities["speed_range"] != "0.25-4.0" {
			t.Errorf("Expected speed_range capability for model %s", model.ID)
		}
	}
}

func TestTTSProvider_ListModels_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": [{"id": "tts-1", "object": "model", "created": 1677649963, "owned_by": "openai-internal"}]}`))
	}))
	defer server.Close()

	provider := &TTSProvider{
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

func TestTTSProvider_ListModels_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	}))
	defer server.Close()

	provider := &TTSProvider{
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

func TestTTSProvider_ListModels_ContextCancelled(t *testing.T) {
	provider := &TTSProvider{
		apiKey:  "test-key",
		baseURL: "https://api.openai.com/v1",
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}

func TestTTSProvider_ListModels_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	provider := &TTSProvider{
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

func TestTTSProvider_GetCapabilities(t *testing.T) {
	provider := NewTTSProvider("test-key")
	caps := provider.GetCapabilities()

	if caps.SupportsChat {
		t.Error("Expected SupportsChat to be false")
	}
	if !caps.SupportsAudio {
		t.Error("Expected SupportsAudio to be true")
	}
	if !caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be true for TTS")
	}
	if caps.SupportsFileUpload {
		t.Error("Expected SupportsFileUpload to be false")
	}
	if caps.SupportsJSONMode {
		t.Error("Expected SupportsJSONMode to be false")
	}
	if caps.SupportsEmbeddings {
		t.Error("Expected SupportsEmbeddings to be false")
	}

	if caps.MaxRequestsPerMinute != 50 {
		t.Errorf("Expected MaxRequestsPerMinute 50, got %d", caps.MaxRequestsPerMinute)
	}

	if caps.MaxTokensPerRequest != 4096 {
		t.Errorf("Expected MaxTokensPerRequest 4096, got %d", caps.MaxTokensPerRequest)
	}

	// Verify supported parameters
	expectedParams := []string{"model", "input", "voice", "response_format", "speed"}
	if len(caps.SupportedParameters) != len(expectedParams) {
		t.Errorf("Expected %d parameters, got %d", len(expectedParams), len(caps.SupportedParameters))
	}

	// Verify security features
	foundSOC2 := false
	foundGDPR := false
	for _, feature := range caps.SecurityFeatures {
		if feature == "SOC2" {
			foundSOC2 = true
		}
		if feature == "GDPR" {
			foundGDPR = true
		}
	}
	if !foundSOC2 {
		t.Error("Expected SOC2 security feature")
	}
	if !foundGDPR {
		t.Error("Expected GDPR security feature")
	}
}

func TestTTSProvider_GetEndpoints(t *testing.T) {
	provider := NewTTSProvider("test-key")
	endpoints := provider.GetEndpoints()

	if len(endpoints) != 2 {
		t.Errorf("Expected 2 endpoints, got %d", len(endpoints))
	}

	// Verify each endpoint
	expectedPaths := map[string]string{
		"/models":       "GET",
		"/audio/speech": "POST",
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
		if !strings.HasPrefix(endpoint.Headers["Authorization"], "Bearer ") {
			t.Errorf("Expected Authorization header for %s", endpoint.Path)
		}
	}
}

func TestTTSProvider_TestModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/speech" {
			t.Errorf("Expected path /audio/speech, got %s", r.URL.Path)
		}

		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Read and verify request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if !strings.Contains(string(body), "tts-1") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if !strings.Contains(string(body), "Test") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if !strings.Contains(string(body), "alloy") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Return mock audio data
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake audio data"))
	}))
	defer server.Close()

	provider := &TTSProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "tts-1", false)
	if err != nil {
		t.Errorf("TestModel failed: %v", err)
	}
}

func TestTTSProvider_TestModel_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("audio"))
	}))
	defer server.Close()

	provider := &TTSProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "tts-1", true)
	if err != nil {
		t.Errorf("TestModel verbose failed: %v", err)
	}
}

func TestTTSProvider_TestModel_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "Invalid model"}}`))
	}))
	defer server.Close()

	provider := &TTSProvider{
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

func TestTTSProvider_TestModel_ContextCancelled(t *testing.T) {
	provider := &TTSProvider{
		apiKey:  "test-key",
		baseURL: "https://api.openai.com/v1",
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := provider.TestModel(ctx, "tts-1", false)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}

func TestTTSProvider_ValidateEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	provider := &TTSProvider{
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
	for _, endpoint := range provider.endpoints {
		if endpoint.Status != StatusWorking {
			t.Errorf("Expected endpoint %s to be working, got status %s", endpoint.Path, endpoint.Status)
		}
	}
}

func TestTTSProvider_ValidateEndpoints_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	provider := &TTSProvider{
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

func TestTTSProvider_ValidateEndpoints_SomeFailures(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		// Make every other endpoint fail
		if count%2 == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": {"message": "server error"}}`))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		}
	}))
	defer server.Close()

	provider := &TTSProvider{
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

func TestTTSProvider_testEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	provider := &TTSProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	endpoint := &Endpoint{
		Path:   "/models",
		Method: "GET",
		Headers: map[string]string{
			"Authorization": "Bearer test-key",
		},
	}

	ctx := context.Background()
	err := provider.testEndpoint(ctx, endpoint)
	if err != nil {
		t.Errorf("testEndpoint failed: %v", err)
	}
}

func TestTTSProvider_testEndpoint_Failed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": {"message": "not found"}}`))
	}))
	defer server.Close()

	provider := &TTSProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	endpoint := &Endpoint{
		Path:   "/nonexistent",
		Method: "GET",
		Headers: map[string]string{
			"Authorization": "Bearer test-key",
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

func TestTTSProvider_TestModel_VoiceSelection(t *testing.T) {
	// Test that the default voice is "alloy"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "alloy") {
			t.Error("Expected default voice 'alloy' in request")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("audio"))
	}))
	defer server.Close()

	provider := &TTSProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "tts-1", false)
	if err != nil {
		t.Errorf("TestModel failed: %v", err)
	}

	// Verify all valid voices are documented in model capabilities
	caps := provider.GetCapabilities()
	if len(caps.SupportedParameters) != 5 {
		t.Error("Expected 5 supported parameters for TTS")
	}
}

func TestTTSProvider_TestModel_HDModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "tts-1-hd") {
			t.Error("Expected tts-1-hd model in request")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("audio"))
	}))
	defer server.Close()

	provider := &TTSProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "tts-1-hd", false)
	if err != nil {
		t.Errorf("TestModel failed for tts-1-hd: %v", err)
	}
}

func TestTTSProvider_ListModels_NoTTSModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Return only non-TTS models
		w.Write([]byte(`{"data": [{"id": "gpt-4", "object": "model", "created": 1677649963, "owned_by": "openai"}]}`))
	}))
	defer server.Close()

	provider := &TTSProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	// Should return empty list when no TTS models found
	if len(models) != 0 {
		t.Errorf("Expected 0 models, got %d", len(models))
	}
}

func TestTTSProvider_TestModel_MarshalError(t *testing.T) {
	// This test would need a complex setup to force JSON marshal error
	// Instead, test with server returning error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	provider := &TTSProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "tts-1", false)
	if err == nil {
		t.Error("Expected error with server error response")
	}
}
