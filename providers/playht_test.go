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

func TestNewPlayHTProvider(t *testing.T) {
	provider := NewPlayHTProvider("user123:test-key")
	if provider == nil {
		t.Fatal("Expected provider to be created")
	}

	playhtProvider, ok := provider.(*PlayHTProvider)
	if !ok {
		t.Fatal("Expected provider to be of type *PlayHTProvider")
	}

	if playhtProvider.userID != "user123" {
		t.Errorf("Expected userID 'user123', got '%s'", playhtProvider.userID)
	}

	if playhtProvider.apiKey != "test-key" {
		t.Errorf("Expected apiKey 'test-key', got '%s'", playhtProvider.apiKey)
	}

	if playhtProvider.baseURL != "https://api.play.ht/api/v2" {
		t.Errorf("Expected baseURL 'https://api.play.ht/api/v2', got '%s'", playhtProvider.baseURL)
	}
}

func TestNewPlayHTProvider_NoColon(t *testing.T) {
	provider := NewPlayHTProvider("single-key")
	if provider == nil {
		t.Fatal("Expected provider to be created even without colon")
	}

	playhtProvider := provider.(*PlayHTProvider)
	if playhtProvider.userID != "" {
		t.Errorf("Expected empty userID, got '%s'", playhtProvider.userID)
	}
	if playhtProvider.apiKey != "single-key" {
		t.Errorf("Expected apiKey 'single-key', got '%s'", playhtProvider.apiKey)
	}
}

func TestNewPlayHTProvider_EmptyKey(t *testing.T) {
	provider := NewPlayHTProvider("")
	if provider == nil {
		t.Fatal("Expected provider to be created even with empty key")
	}

	playhtProvider := provider.(*PlayHTProvider)
	if playhtProvider.apiKey != "" {
		t.Errorf("Expected empty apiKey, got '%s'", playhtProvider.apiKey)
	}
}

func TestPlayHTProvider_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/voices" {
			t.Errorf("Expected path /voices, got %s", r.URL.Path)
		}

		if r.Header.Get("X-USER-ID") != "user123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.Header.Get("AUTHORIZATION") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{
				"id": "s3://voice-cloning-zero-shot/d9ff78ba-d016-47f6-b0ef-dd630f59414e/female-cs/manifest.json",
				"name": "Adriana",
				"language": "Czech",
				"gender": "female",
				"accent": "czech",
				"style": "narrative",
				"age": "young"
			},
			{
				"id": "s3://voice-cloning-zero-shot/775ae416-49bb-4fb6-bd45-740f205d20a1/male-en-us/manifest.json",
				"name": "Angelo",
				"language": "English (US)",
				"gender": "male",
				"accent": "american",
				"style": "conversational",
				"age": "middle-aged"
			}
		]`))
	}))
	defer server.Close()

	provider := &PlayHTProvider{
		userID:  "user123",
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
	if !strings.Contains(models[0].ID, "female-cs") {
		t.Errorf("Expected ID to contain 'female-cs', got '%s'", models[0].ID)
	}
	if models[0].Name != "Adriana" {
		t.Errorf("Expected name 'Adriana', got '%s'", models[0].Name)
	}
	if !strings.Contains(models[0].Description, "Czech") {
		t.Errorf("Expected description to contain 'Czech', got '%s'", models[0].Description)
	}

	// Verify pricing
	if models[0].CostPer1MIn != 40.0 {
		t.Errorf("Expected CostPer1MIn 40.0, got %f", models[0].CostPer1MIn)
	}

	// Verify categories
	foundTTS := false
	foundVoice := false
	for _, cat := range models[0].Categories {
		if cat == "tts" {
			foundTTS = true
		}
		if cat == "voice" {
			foundVoice = true
		}
	}
	if !foundTTS {
		t.Error("Expected 'tts' category")
	}
	if !foundVoice {
		t.Error("Expected 'voice' category")
	}

	// Verify capabilities
	if models[0].Capabilities["language"] != "Czech" {
		t.Errorf("Expected language 'Czech', got '%s'", models[0].Capabilities["language"])
	}
	if models[0].Capabilities["gender"] != "female" {
		t.Errorf("Expected gender 'female', got '%s'", models[0].Capabilities["gender"])
	}
}

func TestPlayHTProvider_ListModels_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	provider := &PlayHTProvider{
		userID:  "user123",
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

func TestPlayHTProvider_ListModels_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "Invalid credentials"}`))
	}))
	defer server.Close()

	provider := &PlayHTProvider{
		userID:  "invalid",
		apiKey:  "invalid-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Error("Expected error with invalid credentials")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("Expected error to mention status 401, got: %v", err)
	}
}

func TestPlayHTProvider_ListModels_ContextCancelled(t *testing.T) {
	provider := &PlayHTProvider{
		userID:  "user123",
		apiKey:  "test-key",
		baseURL: "https://api.play.ht/api/v2",
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}

func TestPlayHTProvider_ListModels_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	provider := &PlayHTProvider{
		userID:  "user123",
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

func TestPlayHTProvider_GetCapabilities(t *testing.T) {
	provider := NewPlayHTProvider("user123:test-key")
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

	// Verify supported parameters
	expectedParams := []string{"text", "voice", "quality", "output_format", "speed", "sample_rate", "voice_engine"}
	if len(caps.SupportedParameters) != len(expectedParams) {
		t.Errorf("Expected %d parameters, got %d", len(expectedParams), len(caps.SupportedParameters))
	}

	// Check max tokens
	if caps.MaxTokensPerRequest != 5000 {
		t.Errorf("Expected MaxTokensPerRequest 5000, got %d", caps.MaxTokensPerRequest)
	}
}

func TestPlayHTProvider_GetEndpoints(t *testing.T) {
	provider := NewPlayHTProvider("user123:test-key")
	endpoints := provider.GetEndpoints()

	if len(endpoints) != 2 {
		t.Errorf("Expected 2 endpoints, got %d", len(endpoints))
	}

	// Verify voices endpoint
	foundVoices := false
	foundTTS := false
	for _, ep := range endpoints {
		if ep.Path == "/voices" && ep.Method == "GET" {
			foundVoices = true
			if ep.Headers["X-USER-ID"] != "user123" {
				t.Errorf("Expected X-USER-ID header 'user123', got '%s'", ep.Headers["X-USER-ID"])
			}
			if ep.Headers["AUTHORIZATION"] != "test-key" {
				t.Errorf("Expected AUTHORIZATION header 'test-key', got '%s'", ep.Headers["AUTHORIZATION"])
			}
		}
		if ep.Path == "/tts" && ep.Method == "POST" {
			foundTTS = true
			if ep.Headers["Content-Type"] != "application/json" {
				t.Errorf("Expected Content-Type header 'application/json', got '%s'", ep.Headers["Content-Type"])
			}
		}
	}

	if !foundVoices {
		t.Error("Expected to find /voices endpoint")
	}
	if !foundTTS {
		t.Error("Expected to find /tts endpoint")
	}
}

func TestPlayHTProvider_ValidateEndpoints(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)

		if r.Header.Get("X-USER-ID") != "user123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/voices" {
			w.Write([]byte(`[]`))
		}
	}))
	defer server.Close()

	provider := &PlayHTProvider{
		userID:  "user123",
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Fatalf("ValidateEndpoints failed: %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 endpoint calls, got %d", callCount)
	}

	// Verify endpoints were updated
	endpoints := provider.GetEndpoints()
	for _, ep := range endpoints {
		if ep.Status != StatusWorking {
			t.Errorf("Expected endpoint %s to be working, got status %s", ep.Path, ep.Status)
		}
		if ep.Latency == 0 {
			t.Errorf("Expected endpoint %s to have latency recorded", ep.Path)
		}
	}
}

func TestPlayHTProvider_ValidateEndpoints_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/voices" {
			w.Write([]byte(`[]`))
		}
	}))
	defer server.Close()

	provider := &PlayHTProvider{
		userID:  "user123",
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

func TestPlayHTProvider_ValidateEndpoints_Failed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Server error"}`))
	}))
	defer server.Close()

	provider := &PlayHTProvider{
		userID:  "user123",
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Fatalf("ValidateEndpoints should not return error: %v", err)
	}

	// Check that endpoints are marked as failed
	endpoints := provider.GetEndpoints()
	failedCount := 0
	for _, ep := range endpoints {
		if ep.Status == StatusFailed {
			failedCount++
			if ep.Error == "" {
				t.Errorf("Expected endpoint %s to have error message", ep.Path)
			}
		}
	}

	if failedCount == 0 {
		t.Error("Expected at least one endpoint to be marked as failed")
	}
}

func TestPlayHTProvider_ValidateEndpoints_ContextCancelled(t *testing.T) {
	provider := &PlayHTProvider{
		userID:  "user123",
		apiKey:  "test-key",
		baseURL: "https://api.play.ht/api/v2",
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Fatalf("ValidateEndpoints should not return error even with cancelled context: %v", err)
	}

	// Endpoints should be marked as failed
	endpoints := provider.GetEndpoints()
	for _, ep := range endpoints {
		if ep.Status != StatusFailed {
			t.Errorf("Expected endpoint %s to fail with cancelled context", ep.Path)
		}
	}
}

func TestPlayHTProvider_TestModel(t *testing.T) {
	var receivedRequest playHTTTSRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tts" {
			t.Errorf("Expected path /tts, got %s", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		if r.Header.Get("X-USER-ID") != "user123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.Header.Get("AUTHORIZATION") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Decode request to verify structure
		if err := json.NewDecoder(r.Body).Decode(&receivedRequest); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`binary audio data`))
	}))
	defer server.Close()

	provider := &PlayHTProvider{
		userID:  "user123",
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "test-voice-id", false)
	if err != nil {
		t.Fatalf("TestModel failed: %v", err)
	}

	// Verify request structure
	if receivedRequest.Text != "Test" {
		t.Errorf("Expected text 'Test', got '%s'", receivedRequest.Text)
	}
	if receivedRequest.Voice != "test-voice-id" {
		t.Errorf("Expected voice 'test-voice-id', got '%s'", receivedRequest.Voice)
	}
	if receivedRequest.Quality != "medium" {
		t.Errorf("Expected quality 'medium', got '%s'", receivedRequest.Quality)
	}
	if receivedRequest.OutputFormat != "mp3" {
		t.Errorf("Expected output_format 'mp3', got '%s'", receivedRequest.OutputFormat)
	}
}

func TestPlayHTProvider_TestModel_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`binary audio data`))
	}))
	defer server.Close()

	provider := &PlayHTProvider{
		userID:  "user123",
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "test-voice-id", true)
	if err != nil {
		t.Fatalf("TestModel verbose failed: %v", err)
	}
}

func TestPlayHTProvider_TestModel_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "Invalid voice ID"}`))
	}))
	defer server.Close()

	provider := &PlayHTProvider{
		userID:  "user123",
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "invalid-voice", false)
	if err == nil {
		t.Error("Expected error with invalid voice ID")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("Expected error to mention status 400, got: %v", err)
	}
}

func TestPlayHTProvider_TestModel_ContextCancelled(t *testing.T) {
	provider := &PlayHTProvider{
		userID:  "user123",
		apiKey:  "test-key",
		baseURL: "https://api.play.ht/api/v2",
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := provider.TestModel(ctx, "test-voice-id", false)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}
