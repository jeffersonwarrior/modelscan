package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMistralProvider_TestModel(t *testing.T) {
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
			"model": "mistral-small-latest",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "test successful"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 5, "completion_tokens": 10, "total_tokens": 15}
		}`))
	}))
	defer server.Close()

	provider := &MistralProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	_ = provider.TestModel(ctx, "mistral-small-latest", false)
	_ = provider.TestModel(ctx, "mistral-small-latest", true)
}

func TestMistralProvider_TestModel_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message": "invalid model"}`))
	}))
	defer server.Close()

	provider := &MistralProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "invalid-model", false)
	if err == nil {
		t.Error("Expected error for invalid model")
	}
}

func TestMistralProvider_TestModel_NetworkError(t *testing.T) {
	provider := &MistralProvider{
		apiKey:  "test-key",
		baseURL: "http://invalid-url-test-case.local",
		client:  &http.Client{Timeout: 1 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "mistral-small-latest", false)
	if err == nil {
		t.Error("Expected error for network failure")
	}
}

func TestMistralProvider_ListModels_HTTPMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"object": "list",
			"data": [{
				"id": "mistral-small-latest",
				"object": "model",
				"created": 1234567890,
				"owned_by": "mistralai",
				"capabilities": {
					"chat_completion": true,
					"text_embedding": false,
					"vision_capability": "limited"
				}
			}]
		}`))
	}))
	defer server.Close()

	provider := &MistralProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(models))
	}

	if models[0].ID != "mistral-small-latest" {
		t.Errorf("Expected model ID mistral-small-latest, got %s", models[0].ID)
	}

	// Check capabilities were parsed
	if models[0].Capabilities["chat_completion"] != "true" {
		t.Error("Expected chat_completion capability to be 'true'")
	}
}

func TestMistralProvider_ListModels_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"object": "list",
			"data": [{
				"id": "mistral-large-latest",
				"object": "model",
				"created": 1234567890,
				"owned_by": "mistralai"
			}]
		}`))
	}))
	defer server.Close()

	provider := &MistralProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx, true)
	if err != nil {
		t.Fatalf("ListModels (verbose) failed: %v", err)
	}

	if len(models) != 1 {
		t.Errorf("Expected 1 model in verbose mode, got %d", len(models))
	}

	// Check that model details are preserved/enriched
	if models[0].Name == "" {
		t.Error("Expected model name to be populated in verbose mode")
	}

	if models[0].Description == "" {
		t.Error("Expected model description to be populated in verbose mode")
	}
}

func TestMistralProvider_ListModels_CapabilityTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"object": "list",
			"data": [{
				"id": "test-model",
				"capabilities": {
					"bool_true": true,
					"bool_false": false,
					"string_val": "supported",
					"number_val": 12345
				}
			}]
		}`))
	}))
	defer server.Close()

	provider := &MistralProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	// Check all capability type conversions
	if models[0].Capabilities["bool_true"] != "true" {
		t.Error("Expected bool_true to be 'true'")
	}
	if models[0].Capabilities["bool_false"] != "false" {
		t.Error("Expected bool_false to be 'false'")
	}
	if models[0].Capabilities["string_val"] != "supported" {
		t.Error("Expected string_val to be 'supported'")
	}
	if models[0].Capabilities["number_val"] == "" {
		t.Error("Expected number_val to be converted to string")
	}
}

func TestMistralProvider_ListModels_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"object": "list",
			"data": []
		}`))
	}))
	defer server.Close()

	provider := &MistralProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 0 {
		t.Errorf("Expected 0 models, got %d", len(models))
	}
}

func TestMistralProvider_ListModels_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	provider := &MistralProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("Expected error to contain 'invalid', got: %v", err)
	}
}

func TestMistralProvider_ListModels_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message": "unauthorized"}`))
	}))
	defer server.Close()

	provider := &MistralProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Error("Expected error for HTTP error")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Errorf("Expected error to contain '401', got: %v", err)
	}
}

func TestGuessMistralModelCategories(t *testing.T) {
	tests := []struct {
		modelID            string
		expectedCategories []string
	}{
		{"codestral-latest", []string{"coding"}},
		{"mistral-small-latest", []string{"chat"}},
		{"mistral-embed", []string{"embedding"}},
		{"voxtral-1", []string{"audio"}},
		{"unknown-model", []string{"general"}},
	}

	for _, tt := range tests {
		categories := guessMistralModelCategories(tt.modelID)
		if len(categories) == 0 {
			t.Errorf("Expected categories for %s", tt.modelID)
		}
	}
}

func TestMistralProvider_testPostEndpoint_ErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		expectedError bool
		errorContains string
	}{
		{
			name:          "401 Unauthorized",
			statusCode:    401,
			responseBody:  `{"message": "unauthorized"}`,
			expectedError: true,
			errorContains: "401",
		},
		{
			name:          "403 Forbidden",
			statusCode:    403,
			responseBody:  `{"message": "forbidden"}`,
			expectedError: true,
			errorContains: "403",
		},
		{
			name:          "404 Not Found",
			statusCode:    404,
			responseBody:  `{"message": "not found"}`,
			expectedError: true,
			errorContains: "404",
		},
		{
			name:          "429 Rate Limited",
			statusCode:    429,
			responseBody:  `{"message": "rate limit exceeded"}`,
			expectedError: true,
			errorContains: "429",
		},
		{
			name:          "500 Server Error",
			statusCode:    500,
			responseBody:  `{"message": "internal server error"}`,
			expectedError: true,
			errorContains: "500",
		},
		{
			name:          "Invalid JSON Response",
			statusCode:    200,
			responseBody:  `invalid json response`,
			expectedError: true,
			errorContains: "invalid",
		},
		{
			name:          "Empty Response",
			statusCode:    200,
			responseBody:  ``,
			expectedError: true,
			errorContains: "empty",
		},
		{
			name:          "Valid Response",
			statusCode:    200,
			responseBody:  `{"id": "test"}`,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify Authorization header
				if r.Header.Get("Authorization") != "Bearer test-key" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			provider := &MistralProvider{
				apiKey:  "test-key",
				baseURL: server.URL,
				client:  &http.Client{Timeout: 1 * time.Second},
			}

			ctx := context.Background()
			endpoint := &Endpoint{Path: "/v1/chat/completions", Method: "POST"}

			err := provider.testPostEndpoint(ctx, endpoint)

			if tt.expectedError && err == nil {
				t.Errorf("Expected error for %s, but got none", tt.name)
			}

			if !tt.expectedError && err != nil {
				t.Errorf("Expected no error for %s, but got: %v", tt.name, err)
			}

			if tt.expectedError && err != nil && tt.errorContains != "" {
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
			}
		})
	}
}

func TestMistralProvider_testGetEndpoint_ErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		expectedError bool
		errorContains string
	}{
		{
			name:          "401 Unauthorized",
			statusCode:    401,
			responseBody:  `{"message": "unauthorized"}`,
			expectedError: true,
			errorContains: "401",
		},
		{
			name:          "404 Not Found",
			statusCode:    404,
			responseBody:  `{"message": "not found"}`,
			expectedError: true,
			errorContains: "404",
		},
		{
			name:          "Invalid JSON",
			statusCode:    200,
			responseBody:  `invalid json`,
			expectedError: true,
			errorContains: "invalid",
		},
		{
			name:          "Valid Response",
			statusCode:    200,
			responseBody:  `{"models": []}`,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-key" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			provider := &MistralProvider{
				apiKey:  "test-key",
				baseURL: server.URL,
				client:  &http.Client{Timeout: 10 * time.Second},
			}

			ctx := context.Background()
			endpoint := &Endpoint{Path: "/v1/models", Method: "GET"}

			err := provider.testGetEndpoint(ctx, endpoint)

			if tt.expectedError && err == nil {
				t.Errorf("Expected error for %s, but got none", tt.name)
			}

			if !tt.expectedError && err != nil {
				t.Errorf("Expected no error for %s, but got: %v", tt.name, err)
			}
		})
	}
}

func TestMistralProvider_testEndpoint_InvalidMethod(t *testing.T) {
	provider := &MistralProvider{
		apiKey:  "test-key",
		baseURL: "https://api.mistral.ai",
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()

	// Test with invalid method
	endpoint := &Endpoint{Path: "/v1/test", Method: "INVALID"}
	err := provider.testEndpoint(ctx, endpoint)

	// Should not panic and should handle gracefully
	if err == nil {
		t.Error("Expected error for invalid method, got nil")
	}
}

func TestMistralProvider_ValidateEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"object": "list", "data": []}`))
	}))
	defer server.Close()

	provider := &MistralProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)

	// Should return nil (success) for valid setup
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestMistralProvider_ValidateEndpoints_WithErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate 401 error
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "unauthorized"}`))
	}))
	defer server.Close()

	provider := &MistralProvider{
		apiKey:  "invalid-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)

	// Should return error for invalid key
	if err == nil {
		t.Error("Expected error for invalid API key")
	}
}

func TestMistralProvider_testEndpoint_Timeout(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // Delay longer than timeout
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "delayed response"}`))
	}))
	defer server.Close()

	provider := &MistralProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 1 * time.Second}, // Short timeout
	}

	ctx := context.Background()
	endpoint := &Endpoint{Path: "/v1/models", Method: "GET"}

	err := provider.testEndpoint(ctx, endpoint)

	// Should timeout and return error
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}

func TestMistralProvider_testEndpoint_NetworkError(t *testing.T) {
	// Create provider with invalid URL to simulate network error
	provider := &MistralProvider{
		apiKey:  "test-key",
		baseURL: "http://invalid-url-that-does-not-exist.local",
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	ctx := context.Background()
	endpoint := &Endpoint{Path: "/v1/models", Method: "GET"}

	err := provider.testEndpoint(ctx, endpoint)

	// Should get network error
	if err == nil {
		t.Error("Expected network error, got nil")
	}
}

func TestMistralProvider_EnhanceModelInfo_Categories(t *testing.T) {
	provider := NewMistralProvider("test-key")
	mistralProvider := provider.(*MistralProvider)

	tests := []struct {
		name              string
		modelID           string
		supportsImages    bool
		supportsTools     bool
		canReason         bool
		expectedCategory  string
		expectedVision    bool
		expectedFunction  bool
		expectedReasoning bool
	}{
		{
			name:             "codestral",
			modelID:          "codestral-latest",
			expectedCategory: "coding",
		},
		{
			name:             "mistral-small",
			modelID:          "mistral-small-latest",
			expectedCategory: "chat",
		},
		{
			name:             "embed",
			modelID:          "mistral-embed",
			expectedCategory: "embedding",
		},
		{
			name:             "voxtral",
			modelID:          "voxtral-latest",
			expectedCategory: "audio",
		},
		{
			name:             "with vision capabilities",
			modelID:          "custom-vision-model",
			supportsImages:   true,
			expectedCategory: "", // No category for unknown model
			expectedVision:   true,
		},
		{
			name:             "with tools",
			modelID:          "mistral-large-latest",
			supportsTools:    true,
			expectedCategory: "chat",
			expectedFunction: true,
		},
		{
			name:              "with reasoning",
			modelID:           "mistral-large-latest",
			canReason:         true,
			expectedCategory:  "chat",
			expectedReasoning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &Model{
				ID:             tt.modelID,
				SupportsImages: tt.supportsImages,
				SupportsTools:  tt.supportsTools,
				CanReason:      tt.canReason,
			}

			mistralProvider.enhanceModelInfo(model)

			// Check category (skip if empty expected)
			if tt.expectedCategory != "" {
				found := false
				for _, cat := range model.Categories {
					if cat == tt.expectedCategory {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected category %s in categories %v", tt.expectedCategory, model.Categories)
				}
			}

			// Check capabilities
			if tt.expectedVision && !model.SupportsImages {
				t.Error("Expected SupportsImages to be true")
			}
			if tt.expectedFunction && !model.SupportsTools {
				t.Error("Expected SupportsTools to be true")
			}
			if tt.expectedReasoning && !model.CanReason {
				t.Error("Expected CanReason to be true")
			}
		})
	}
}
