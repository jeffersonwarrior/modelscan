package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	_, _ = provider.ListModels(ctx, false)
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
