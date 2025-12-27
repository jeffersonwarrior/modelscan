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

func TestCerebrasExtendedProvider_TestModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "chat-test",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "llama3.3-70b",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "test successful"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 5, "completion_tokens": 10, "total_tokens": 15}
		}`))
	}))
	defer server.Close()

	provider := &CerebrasExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "llama3.3-70b", false)
	if err != nil {
		t.Fatalf("TestModel failed: %v", err)
	}
}

func TestCerebrasExtendedProvider_TestModel_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [{
				"message": {"content": "test successful"}
			}]
		}`))
	}))
	defer server.Close()

	provider := &CerebrasExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "llama3.3-70b", true)
	if err != nil {
		t.Fatalf("TestModel verbose failed: %v", err)
	}
}

func TestCerebrasExtendedProvider_TestModel_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "invalid model"}}`))
	}))
	defer server.Close()

	provider := &CerebrasExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "invalid-model", false)
	if err == nil {
		t.Error("Expected error for invalid model")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("Expected 400 error, got: %v", err)
	}
}

func TestCerebrasExtendedProvider_TestModel_NetworkError(t *testing.T) {
	provider := &CerebrasExtendedProvider{
		apiKey:  "test-key",
		baseURL: "http://invalid-url-test-case.local",
		client: &http.Client{
			Timeout: 1 * time.Second,
		},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "llama3.3-70b", false)
	if err == nil {
		t.Error("Expected error for network failure")
	}
}

func TestCerebrasExtendedProvider_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"object": "list",
			"data": [
				{
					"id": "llama3.3-70b",
					"object": "model",
					"created": 1234567890,
					"owned_by": "cerebras"
				},
				{
					"id": "llama3.1-8b",
					"object": "model",
					"created": 1234567890,
					"owned_by": "cerebras"
				}
			]
		}`))
	}))
	defer server.Close()

	provider := &CerebrasExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}

	if models[0].ID != "llama3.3-70b" {
		t.Errorf("Expected model ID llama3.3-70b, got %s", models[0].ID)
	}

	if models[0].CostPer1MIn != 0.60 {
		t.Errorf("Expected cost per 1M in 0.60, got %f", models[0].CostPer1MIn)
	}

	if models[0].Capabilities["speed"] != "ultra-fast (1800 tokens/sec)" {
		t.Error("Expected ultra-fast speed capability")
	}
}

func TestCerebrasExtendedProvider_ListModels_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"object": "list",
			"data": [{
				"id": "llama3.3-70b",
				"object": "model",
				"created": 1234567890,
				"owned_by": "cerebras"
			}]
		}`))
	}))
	defer server.Close()

	provider := &CerebrasExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx, true)
	if err != nil {
		t.Fatalf("ListModels verbose failed: %v", err)
	}

	if len(models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(models))
	}
}

func TestCerebrasExtendedProvider_ListModels_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	provider := &CerebrasExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Error("Expected error for server error")
	}
}

func TestCerebrasExtendedProvider_ListModels_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	provider := &CerebrasExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestCerebrasExtendedProvider_GetCapabilities(t *testing.T) {
	provider := NewCerebrasExtendedProvider("test-key")
	caps := provider.GetCapabilities()

	if !caps.SupportsChat {
		t.Error("Expected chat support")
	}

	if !caps.SupportsStreaming {
		t.Error("Expected streaming support")
	}

	if !caps.SupportsJSONMode {
		t.Error("Expected JSON mode support")
	}

	if caps.SupportsVision {
		t.Error("Did not expect vision support")
	}

	if caps.SupportsEmbeddings {
		t.Error("Did not expect embeddings support")
	}

	if caps.MaxRequestsPerMinute != 60 {
		t.Errorf("Expected 60 RPM, got %d", caps.MaxRequestsPerMinute)
	}

	expectedParams := []string{"temperature", "max_tokens", "top_p", "stop", "frequency_penalty", "presence_penalty"}
	if len(caps.SupportedParameters) != len(expectedParams) {
		t.Errorf("Expected %d parameters, got %d", len(expectedParams), len(caps.SupportedParameters))
	}
}

func TestCerebrasExtendedProvider_GetEndpoints(t *testing.T) {
	provider := NewCerebrasExtendedProvider("test-key")
	endpoints := provider.GetEndpoints()

	if len(endpoints) != 2 {
		t.Errorf("Expected 2 endpoints, got %d", len(endpoints))
	}

	foundChat := false
	foundModels := false
	for _, ep := range endpoints {
		if ep.Path == "/chat/completions" && ep.Method == "POST" {
			foundChat = true
		}
		if ep.Path == "/models" && ep.Method == "GET" {
			foundModels = true
		}
	}

	if !foundChat {
		t.Error("Missing chat completions endpoint")
	}
	if !foundModels {
		t.Error("Missing models endpoint")
	}
}

func TestCerebrasExtendedProvider_ValidateEndpoints(t *testing.T) {
	chatCalled := false
	modelsCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/models") {
			modelsCalled = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"object": "list", "data": []}`))
			return
		}

		if strings.Contains(r.URL.Path, "/chat/completions") {
			chatCalled = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices": [{"message": {"content": "hi"}}]}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := &CerebrasExtendedProvider{
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

	if !chatCalled {
		t.Error("Chat endpoint was not called")
	}
	if !modelsCalled {
		t.Error("Models endpoint was not called")
	}

	endpoints := provider.GetEndpoints()
	for _, ep := range endpoints {
		if ep.Status != StatusWorking {
			t.Errorf("Endpoint %s should be working", ep.Path)
		}
	}
}

func TestCerebrasExtendedProvider_ValidateEndpoints_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if strings.Contains(r.URL.Path, "/models") {
			w.Write([]byte(`{"object": "list", "data": []}`))
		} else {
			w.Write([]byte(`{"choices": [{"message": {"content": "hi"}}]}`))
		}
	}))
	defer server.Close()

	provider := &CerebrasExtendedProvider{
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

func TestCerebrasExtendedProvider_ValidateEndpoints_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "unauthorized"}`))
	}))
	defer server.Close()

	provider := &CerebrasExtendedProvider{
		apiKey:  "invalid-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	_ = provider.ValidateEndpoints(ctx, false)

	endpoints := provider.GetEndpoints()
	allFailed := true
	for _, ep := range endpoints {
		if ep.Status != StatusFailed {
			allFailed = false
		}
	}
	if !allFailed {
		t.Error("Expected all endpoints to be marked as failed")
	}
}

func TestCerebrasExtendedProvider_FormatModelName(t *testing.T) {
	provider := &CerebrasExtendedProvider{}

	tests := []struct {
		input    string
		expected string
	}{
		{"llama3.3-70b", "Llama 3.3: llama3.3-70b"},
		{"llama-3.3-70b", "Llama 3.3: llama-3.3-70b"},
		{"llama3.1-8b", "Llama 3.1: llama3.1-8b"},
		{"llama-3.1-70b", "Llama 3.1: llama-3.1-70b"},
		{"unknown-model", "unknown-model"},
	}

	for _, tt := range tests {
		result := provider.formatModelName(tt.input)
		if result != tt.expected {
			t.Errorf("formatModelName(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestCerebrasExtendedProvider_EnrichModelDetails(t *testing.T) {
	provider := &CerebrasExtendedProvider{}

	tests := []struct {
		modelID             string
		expectedCost        float64
		expectedContext     int
		expectedCanReason   bool
		expectedDescription string
	}{
		{
			"llama3.3-70b",
			0.60,
			8192,
			true,
			"Ultra-fast Llama 3.3 70B - 1800 tokens/sec",
		},
		{
			"llama-3.3-70b",
			0.60,
			8192,
			true,
			"Ultra-fast Llama 3.3 70B - 1800 tokens/sec",
		},
		{
			"llama3.1-70b",
			0.60,
			8192,
			true,
			"Ultra-fast Llama 3.1 70B - 1800 tokens/sec",
		},
		{
			"llama3.1-8b",
			0.10,
			8192,
			false,
			"Ultra-fast Llama 3.1 8B - 1800 tokens/sec",
		},
		{
			"unknown-model",
			0.60,
			8192,
			false,
			"",
		},
	}

	for _, tt := range tests {
		model := Model{ID: tt.modelID}
		enriched := provider.enrichModelDetails(model)

		if enriched.CostPer1MIn != tt.expectedCost {
			t.Errorf("%s: expected cost %f, got %f", tt.modelID, tt.expectedCost, enriched.CostPer1MIn)
		}

		if enriched.ContextWindow != tt.expectedContext {
			t.Errorf("%s: expected context %d, got %d", tt.modelID, tt.expectedContext, enriched.ContextWindow)
		}

		if enriched.CanReason != tt.expectedCanReason {
			t.Errorf("%s: expected CanReason %v, got %v", tt.modelID, tt.expectedCanReason, enriched.CanReason)
		}

		if tt.expectedDescription != "" && enriched.Description != tt.expectedDescription {
			t.Errorf("%s: expected description %s, got %s", tt.modelID, tt.expectedDescription, enriched.Description)
		}

		if !enriched.SupportsTools {
			t.Errorf("%s: expected SupportsTools to be true", tt.modelID)
		}

		if !enriched.CanStream {
			t.Errorf("%s: expected CanStream to be true", tt.modelID)
		}

		if enriched.Capabilities["speed"] != "ultra-fast (1800 tokens/sec)" {
			t.Errorf("%s: expected ultra-fast speed capability", tt.modelID)
		}
	}
}

func TestCerebrasExtendedProvider_TestEndpoint_ChatCompletions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)

		if reqBody["model"] != "llama3.1-8b" {
			t.Error("Expected model llama3.1-8b in request")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices": [{"message": {"content": "hi"}}]}`))
	}))
	defer server.Close()

	provider := &CerebrasExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	endpoint := &Endpoint{
		Path:   "/chat/completions",
		Method: "POST",
	}

	ctx := context.Background()
	err := provider.testEndpoint(ctx, endpoint)
	if err != nil {
		t.Fatalf("testEndpoint failed: %v", err)
	}
}

func TestCerebrasExtendedProvider_TestEndpoint_Models(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"object": "list", "data": []}`))
	}))
	defer server.Close()

	provider := &CerebrasExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	endpoint := &Endpoint{
		Path:   "/models",
		Method: "GET",
	}

	ctx := context.Background()
	err := provider.testEndpoint(ctx, endpoint)
	if err != nil {
		t.Fatalf("testEndpoint failed: %v", err)
	}
}

func TestCerebrasExtendedProvider_TestEndpoint_UnknownEndpoint(t *testing.T) {
	provider := &CerebrasExtendedProvider{
		apiKey:  "test-key",
		baseURL: "http://localhost",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	endpoint := &Endpoint{
		Path:   "/unknown",
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

func TestCerebrasExtendedProvider_Registration(t *testing.T) {
	factory, exists := GetProviderFactory("cerebras_extended")
	if !exists {
		t.Fatal("Cerebras Extended provider not registered")
	}

	provider := factory("test-key")
	if provider == nil {
		t.Fatal("Factory returned nil provider")
	}

	_, ok := provider.(*CerebrasExtendedProvider)
	if !ok {
		t.Error("Factory did not return CerebrasExtendedProvider instance")
	}
}

func TestCerebrasExtendedProvider_NewCerebrasExtendedProvider(t *testing.T) {
	provider := NewCerebrasExtendedProvider("test-api-key")
	cerebrasProvider, ok := provider.(*CerebrasExtendedProvider)
	if !ok {
		t.Fatal("NewCerebrasExtendedProvider did not return *CerebrasExtendedProvider")
	}

	if cerebrasProvider.apiKey != "test-api-key" {
		t.Errorf("Expected API key 'test-api-key', got '%s'", cerebrasProvider.apiKey)
	}

	if cerebrasProvider.baseURL != "https://api.cerebras.ai/v1" {
		t.Errorf("Expected base URL 'https://api.cerebras.ai/v1', got '%s'", cerebrasProvider.baseURL)
	}

	if cerebrasProvider.client == nil {
		t.Error("Client should not be nil")
	}
}

func TestCerebrasExtendedProvider_ValidateEndpoints_Concurrent(t *testing.T) {
	var mu sync.Mutex
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if strings.Contains(r.URL.Path, "/models") {
			w.Write([]byte(`{"object": "list", "data": []}`))
		} else {
			w.Write([]byte(`{"choices": [{"message": {"content": "hi"}}]}`))
		}
	}))
	defer server.Close()

	provider := &CerebrasExtendedProvider{
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
	finalCount := callCount
	mu.Unlock()

	if finalCount != 2 {
		t.Errorf("Expected 2 concurrent calls, got %d", finalCount)
	}
}

func TestCerebrasExtendedProvider_TestModel_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json}`))
	}))
	defer server.Close()

	provider := &CerebrasExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "llama3.3-70b", false)
	if err == nil {
		t.Error("Expected error for invalid JSON response")
	}
}

func TestCerebrasExtendedProvider_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := &CerebrasExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := provider.TestModel(ctx, "llama3.3-70b", false)
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}
