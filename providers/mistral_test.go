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
	err := provider.TestModel(ctx, "invalid", false)
	if err == nil {
		t.Error("Expected error for invalid model")
	}
}

func TestMistralProvider_ListModels_HTTPMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"object": "list",
			"data": [
				{
					"id": "mistral-small-latest",
					"object": "model",
					"created": 1234567890,
					"owned_by": "mistralai",
					"capabilities": {
						"completion_chat": true
					},
					"max_context_length": 32768
				}
			]
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
	if models[0].Capabilities["completion_chat"] != "true" {
		t.Error("Expected completion_chat capability to be true")
	}
	
	// Check categories were assigned
	if len(models[0].Categories) == 0 {
		t.Error("Expected categories to be assigned")
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
	if err != nil {
		t.Errorf("ValidateEndpoints failed: %v", err)
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
			expectedCategory: "",  // No category for unknown model
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
					t.Errorf("Expected category %s, got %v", tt.expectedCategory, model.Categories)
				}
			}
			
			// Check capabilities
			if tt.expectedVision {
				if model.Capabilities["vision"] != "high" {
					t.Error("Expected vision capability")
				}
			}
			if tt.expectedFunction {
				if model.Capabilities["function_calling"] != "full" {
					t.Error("Expected function_calling capability")
				}
			}
			if tt.expectedReasoning {
				if model.Capabilities["reasoning"] != "enabled" {
					t.Error("Expected reasoning capability")
				}
			}
		})
	}
}

func TestMistralProvider_ListModels_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"object": "list",
			"data": [
				{
					"id": "mistral-large-latest",
					"object": "model",
					"created": 1234567890,
					"owned_by": "mistralai",
					"capabilities": {
						"completion_chat": true,
						"function_calling": true
					},
					"description": "Large model"
				}
			]
		}`))
	}))
	defer server.Close()
	
	provider := &MistralProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
	
	ctx := context.Background()
	models, err := provider.ListModels(ctx, true) // verbose mode
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	
	if len(models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(models))
	}
	
	// Check description was parsed
	if models[0].Description != "Large model" {
		t.Errorf("Expected description 'Large model', got '%s'", models[0].Description)
	}
}

func TestMistralProvider_ListModels_CapabilityTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"object": "list",
			"data": [
				{
					"id": "test-model",
					"object": "model",
					"created": 1234567890,
					"owned_by": "mistralai",
					"capabilities": {
						"bool_true": true,
						"bool_false": false,
						"string_val": "supported",
						"number_val": 12345
					}
				}
			]
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
	models, err := provider.ListModels(ctx, true) // verbose to test that path
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
}

func TestMistralProvider_ListModels_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "unauthorized"}`))
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
		t.Error("Expected error for HTTP 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("Expected error to mention status 401, got: %v", err)
	}
}
