package providers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewWhisperProvider(t *testing.T) {
	provider := NewWhisperProvider("test-key")
	if provider == nil {
		t.Fatal("Expected provider to be created")
	}

	whisperProvider, ok := provider.(*WhisperProvider)
	if !ok {
		t.Fatal("Expected provider to be of type *WhisperProvider")
	}

	if whisperProvider.apiKey != "test-key" {
		t.Errorf("Expected apiKey 'test-key', got '%s'", whisperProvider.apiKey)
	}

	if whisperProvider.baseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected baseURL 'https://api.openai.com/v1', got '%s'", whisperProvider.baseURL)
	}
}

func TestNewWhisperProvider_EmptyKey(t *testing.T) {
	provider := NewWhisperProvider("")
	if provider == nil {
		t.Fatal("Expected provider to be created even with empty key")
	}

	whisperProvider := provider.(*WhisperProvider)
	if whisperProvider.apiKey != "" {
		t.Errorf("Expected empty apiKey, got '%s'", whisperProvider.apiKey)
	}
}

func TestWhisperProvider_ListModels(t *testing.T) {
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
					"id": "whisper-1",
					"object": "model",
					"created": 1677649963,
					"owned_by": "openai"
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

	provider := &WhisperProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	// Should only return whisper-1, not gpt-4
	if len(models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(models))
	}

	// Verify model details
	if models[0].ID != "whisper-1" {
		t.Errorf("Expected ID 'whisper-1', got '%s'", models[0].ID)
	}
	if models[0].Name != "Whisper" {
		t.Errorf("Expected name 'Whisper', got '%s'", models[0].Name)
	}

	// Verify pricing ($0.006 per minute = $6 per 1000 minutes)
	if models[0].CostPer1MIn != 6000.0 {
		t.Errorf("Expected CostPer1MIn 6000.0, got %f", models[0].CostPer1MIn)
	}
	if models[0].CostPer1MOut != 0 {
		t.Errorf("Expected CostPer1MOut 0, got %f", models[0].CostPer1MOut)
	}

	// Verify categories
	foundAudio := false
	foundTranscription := false
	foundSTT := false
	for _, cat := range models[0].Categories {
		if cat == "audio" {
			foundAudio = true
		}
		if cat == "transcription" {
			foundTranscription = true
		}
		if cat == "stt" {
			foundSTT = true
		}
	}
	if !foundAudio {
		t.Error("Expected 'audio' category")
	}
	if !foundTranscription {
		t.Error("Expected 'transcription' category")
	}
	if !foundSTT {
		t.Error("Expected 'stt' category")
	}

	// Verify capabilities
	if models[0].Capabilities["audio_formats"] != "mp3,mp4,mpeg,mpga,m4a,wav,webm" {
		t.Error("Expected audio_formats capability")
	}
	if models[0].Capabilities["max_file_size"] != "25MB" {
		t.Error("Expected max_file_size capability")
	}
}

func TestWhisperProvider_ListModels_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": [{"id": "whisper-1", "object": "model", "created": 1677649963, "owned_by": "openai"}]}`))
	}))
	defer server.Close()

	provider := &WhisperProvider{
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

func TestWhisperProvider_ListModels_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	}))
	defer server.Close()

	provider := &WhisperProvider{
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

func TestWhisperProvider_ListModels_ContextCancelled(t *testing.T) {
	provider := &WhisperProvider{
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

func TestWhisperProvider_ListModels_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	provider := &WhisperProvider{
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

func TestWhisperProvider_GetCapabilities(t *testing.T) {
	provider := NewWhisperProvider("test-key")
	caps := provider.GetCapabilities()

	if caps.SupportsChat {
		t.Error("Expected SupportsChat to be false")
	}
	if !caps.SupportsAudio {
		t.Error("Expected SupportsAudio to be true")
	}
	if caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be false")
	}
	if !caps.SupportsFileUpload {
		t.Error("Expected SupportsFileUpload to be true for audio upload")
	}
	if !caps.SupportsJSONMode {
		t.Error("Expected SupportsJSONMode to be true")
	}
	if caps.SupportsEmbeddings {
		t.Error("Expected SupportsEmbeddings to be false")
	}

	if caps.MaxRequestsPerMinute != 50 {
		t.Errorf("Expected MaxRequestsPerMinute 50, got %d", caps.MaxRequestsPerMinute)
	}

	// Verify supported parameters
	expectedParams := []string{"file", "model", "language", "prompt", "response_format", "temperature"}
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

func TestWhisperProvider_GetEndpoints(t *testing.T) {
	provider := NewWhisperProvider("test-key")
	endpoints := provider.GetEndpoints()

	if len(endpoints) != 3 {
		t.Errorf("Expected 3 endpoints, got %d", len(endpoints))
	}

	// Verify each endpoint
	expectedPaths := map[string]string{
		"/models":               "GET",
		"/audio/transcriptions": "POST",
		"/audio/translations":   "POST",
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

func TestWhisperProvider_TestModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/transcriptions" {
			t.Errorf("Expected path /audio/transcriptions, got %s", r.URL.Path)
		}

		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Verify it's multipart form data
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Parse multipart form
		err := r.ParseMultipartForm(32 << 20)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Verify model field
		model := r.FormValue("model")
		if model != "whisper-1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Return mock transcription
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"text": "test transcription"}`))
	}))
	defer server.Close()

	provider := &WhisperProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "whisper-1", false)
	if err != nil {
		t.Errorf("TestModel failed: %v", err)
	}
}

func TestWhisperProvider_TestModel_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"text": "test"}`))
	}))
	defer server.Close()

	provider := &WhisperProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "whisper-1", true)
	if err != nil {
		t.Errorf("TestModel verbose failed: %v", err)
	}
}

func TestWhisperProvider_TestModel_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "Invalid model"}}`))
	}))
	defer server.Close()

	provider := &WhisperProvider{
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

func TestWhisperProvider_TestModel_ContextCancelled(t *testing.T) {
	provider := &WhisperProvider{
		apiKey:  "test-key",
		baseURL: "https://api.openai.com/v1",
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := provider.TestModel(ctx, "whisper-1", false)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}

func TestWhisperProvider_ValidateEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	provider := &WhisperProvider{
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

func TestWhisperProvider_ValidateEndpoints_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	provider := &WhisperProvider{
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

func TestWhisperProvider_ValidateEndpoints_SomeFailures(t *testing.T) {
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

	provider := &WhisperProvider{
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

func TestWhisperProvider_testEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	provider := &WhisperProvider{
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

func TestWhisperProvider_testEndpoint_Failed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": {"message": "not found"}}`))
	}))
	defer server.Close()

	provider := &WhisperProvider{
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

func TestCreateMinimalWAV(t *testing.T) {
	wav := createMinimalWAV()

	// Verify minimum WAV size (44 bytes header)
	if len(wav) < 44 {
		t.Errorf("Expected WAV to be at least 44 bytes, got %d", len(wav))
	}

	// Verify RIFF header
	if string(wav[0:4]) != "RIFF" {
		t.Error("Expected RIFF header")
	}

	// Verify WAVE format
	if string(wav[8:12]) != "WAVE" {
		t.Error("Expected WAVE format")
	}

	// Verify fmt subchunk
	if string(wav[12:16]) != "fmt " {
		t.Error("Expected fmt subchunk")
	}

	// Verify data subchunk
	if string(wav[36:40]) != "data" {
		t.Error("Expected data subchunk")
	}
}

func TestWhisperProvider_TestModel_MultipartFormData(t *testing.T) {
	var receivedFile []byte
	var receivedModel string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse multipart form
		err := r.ParseMultipartForm(32 << 20)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Read file
		file, _, err := r.FormFile("file")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer file.Close()

		buf := new(bytes.Buffer)
		buf.ReadFrom(file)
		receivedFile = buf.Bytes()

		// Read model
		receivedModel = r.FormValue("model")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"text": "test"}`))
	}))
	defer server.Close()

	provider := &WhisperProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "whisper-1", false)
	if err != nil {
		t.Errorf("TestModel failed: %v", err)
	}

	// Verify file was sent
	if len(receivedFile) == 0 {
		t.Error("Expected file to be sent")
	}

	// Verify it's a valid WAV
	if string(receivedFile[0:4]) != "RIFF" {
		t.Error("Expected RIFF header in sent file")
	}

	// Verify model was sent
	if receivedModel != "whisper-1" {
		t.Errorf("Expected model 'whisper-1', got '%s'", receivedModel)
	}
}

func TestAppendUint32LE(t *testing.T) {
	b := []byte{}
	b = appendUint32LE(b, 0x12345678)

	expected := []byte{0x78, 0x56, 0x34, 0x12}
	if !bytes.Equal(b, expected) {
		t.Errorf("Expected %v, got %v", expected, b)
	}
}

func TestAppendUint16LE(t *testing.T) {
	b := []byte{}
	b = appendUint16LE(b, 0x1234)

	expected := []byte{0x34, 0x12}
	if !bytes.Equal(b, expected) {
		t.Errorf("Expected %v, got %v", expected, b)
	}
}

func TestWhisperProvider_TestModel_CreateFormFileError(t *testing.T) {
	// This test verifies error handling in multipart form creation
	// We can't easily trigger CreateFormFile errors without mocking,
	// but we can test the TestModel flow with a bad server response

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	provider := &WhisperProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "whisper-1", false)
	if err == nil {
		t.Error("Expected error with server error response")
	}
}

func TestWhisperProvider_ListModels_NoWhisperModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Return only non-whisper models
		w.Write([]byte(`{"data": [{"id": "gpt-4", "object": "model", "created": 1677649963, "owned_by": "openai"}]}`))
	}))
	defer server.Close()

	provider := &WhisperProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	// Should return empty list when no whisper models found
	if len(models) != 0 {
		t.Errorf("Expected 0 models, got %d", len(models))
	}
}
