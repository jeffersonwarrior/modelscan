package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNewAnthropicExtendedProvider(t *testing.T) {
	apiKey := "test-api-key"
	provider := NewAnthropicExtendedProvider(apiKey)

	if provider == nil {
		t.Fatal("Expected provider to be created, got nil")
	}

	anthProvider, ok := provider.(*AnthropicExtendedProvider)
	if !ok {
		t.Fatal("Expected provider to be of type *AnthropicExtendedProvider")
	}

	if anthProvider.apiKey != apiKey {
		t.Errorf("Expected API key %s, got %s", apiKey, anthProvider.apiKey)
	}

	if anthProvider.baseURL != "https://api.anthropic.com/v1" {
		t.Errorf("Expected base URL https://api.anthropic.com/v1, got %s", anthProvider.baseURL)
	}

	if anthProvider.client == nil {
		t.Error("Expected HTTP client to be initialized")
	}

	if anthProvider.client.Timeout != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", anthProvider.client.Timeout)
	}
}

func TestAnthropicExtendedProviderRegistration(t *testing.T) {
	factory, exists := GetProviderFactory("anthropic_extended")
	if !exists {
		t.Fatal("Expected anthropic_extended provider to be registered")
	}

	provider := factory("test-key")
	if provider == nil {
		t.Fatal("Expected factory to create provider")
	}

	_, ok := provider.(*AnthropicExtendedProvider)
	if !ok {
		t.Error("Expected provider to be of type *AnthropicExtendedProvider")
	}
}

func TestAnthropicExtendedProvider_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("Expected path /models, got %s", r.URL.Path)
		}

		if r.Method != "GET" {
			t.Errorf("Expected GET method, got %s", r.Method)
		}

		apiKey := r.Header.Get("x-api-key")
		if apiKey != "test-key" {
			t.Errorf("Expected x-api-key header test-key, got %s", apiKey)
		}

		version := r.Header.Get("anthropic-version")
		if version != "2023-06-01" {
			t.Errorf("Expected anthropic-version 2023-06-01, got %s", version)
		}

		response := anthropicExtendedModelsResponse{
			Data: []anthropicExtendedModelInfo{
				{
					ID:          "claude-sonnet-4-5-20250929",
					DisplayName: "Claude Sonnet 4.5",
					CreatedAt:   time.Now(),
					Type:        "model",
				},
				{
					ID:          "claude-opus-4-0-20250514",
					DisplayName: "Claude Opus 4.0",
					CreatedAt:   time.Now(),
					Type:        "model",
				},
			},
			HasMore: false,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := &AnthropicExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	models, err := provider.ListModels(context.Background(), false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(models))
	}

	// Verify first model
	if models[0].ID != "claude-sonnet-4-5-20250929" {
		t.Errorf("Expected model ID claude-sonnet-4-5-20250929, got %s", models[0].ID)
	}

	if models[0].Name != "Claude Sonnet 4.5" {
		t.Errorf("Expected model name Claude Sonnet 4.5, got %s", models[0].Name)
	}

	// Verify extended thinking capabilities
	if models[0].Capabilities["extended_thinking"] != "supported" {
		t.Error("Expected extended_thinking capability to be supported")
	}

	if models[0].Capabilities["thinking_budget"] != "configurable" {
		t.Error("Expected thinking_budget capability to be configurable")
	}

	// Verify sonnet-4 pricing
	if models[0].CostPer1MIn != 3.00 {
		t.Errorf("Expected cost per 1M input 3.00, got %f", models[0].CostPer1MIn)
	}

	if models[0].CostPer1MOut != 15.00 {
		t.Errorf("Expected cost per 1M output 15.00, got %f", models[0].CostPer1MOut)
	}

	// Verify opus-4 pricing
	if models[1].CostPer1MIn != 5.00 {
		t.Errorf("Expected opus cost per 1M input 5.00, got %f", models[1].CostPer1MIn)
	}

	if models[1].CostPer1MOut != 25.00 {
		t.Errorf("Expected opus cost per 1M output 25.00, got %f", models[1].CostPer1MOut)
	}
}

func TestAnthropicExtendedProvider_ListModelsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	}))
	defer server.Close()

	provider := &AnthropicExtendedProvider{
		apiKey:  "invalid-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	_, err := provider.ListModels(context.Background(), false)
	if err == nil {
		t.Fatal("Expected error for invalid API key")
	}
}

func TestAnthropicExtendedProvider_ValidateEndpoints(t *testing.T) {
	requestCount := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		if r.URL.Path == "/messages" {
			response := anthropicExtendedResponse{
				ID:   "msg_123",
				Type: "message",
				Role: "assistant",
				Content: []anthropicExtendedContent{
					{Type: "text", Text: "Hi there!"},
				},
				Model:      "claude-sonnet-4-5-20250929",
				StopReason: "end_turn",
				Usage: anthropicExtendedUsage{
					InputTokens:  10,
					OutputTokens: 5,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if r.URL.Path == "/models" {
			response := anthropicExtendedModelsResponse{
				Data: []anthropicExtendedModelInfo{
					{
						ID:          "claude-sonnet-4-5-20250929",
						DisplayName: "Claude Sonnet 4.5",
						CreatedAt:   time.Now(),
						Type:        "model",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	provider := &AnthropicExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	err := provider.ValidateEndpoints(context.Background(), false)
	if err != nil {
		t.Fatalf("ValidateEndpoints failed: %v", err)
	}

	endpoints := provider.GetEndpoints()
	if len(endpoints) != 2 {
		t.Fatalf("Expected 2 endpoints, got %d", len(endpoints))
	}

	// Verify all endpoints were tested
	for _, endpoint := range endpoints {
		if endpoint.Status != StatusWorking {
			t.Errorf("Expected endpoint %s to be working, got status %s", endpoint.Path, endpoint.Status)
		}
		if endpoint.Latency == 0 {
			t.Errorf("Expected endpoint %s to have non-zero latency", endpoint.Path)
		}
	}

	mu.Lock()
	if requestCount != 2 {
		t.Errorf("Expected 2 requests (concurrent), got %d", requestCount)
	}
	mu.Unlock()
}

func TestAnthropicExtendedProvider_ValidateEndpointsFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	provider := &AnthropicExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	err := provider.ValidateEndpoints(context.Background(), false)
	if err != nil {
		t.Fatalf("ValidateEndpoints should not return error: %v", err)
	}

	endpoints := provider.GetEndpoints()
	failedCount := 0
	for _, endpoint := range endpoints {
		if endpoint.Status == StatusFailed {
			failedCount++
			if endpoint.Error == "" {
				t.Error("Expected error message for failed endpoint")
			}
		}
	}

	if failedCount == 0 {
		t.Error("Expected at least one endpoint to fail")
	}
}

func TestAnthropicExtendedProvider_TestModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			t.Errorf("Expected path /messages, got %s", r.URL.Path)
		}

		var req anthropicExtendedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.Model != "claude-sonnet-4-5-20250929" {
			t.Errorf("Expected model claude-sonnet-4-5-20250929, got %s", req.Model)
		}

		if len(req.Messages) == 0 {
			t.Error("Expected messages in request")
		}

		response := anthropicExtendedResponse{
			ID:   "msg_test",
			Type: "message",
			Role: "assistant",
			Content: []anthropicExtendedContent{
				{Type: "text", Text: "Test successful"},
			},
			Model:      req.Model,
			StopReason: "end_turn",
			Usage: anthropicExtendedUsage{
				InputTokens:  10,
				OutputTokens: 5,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := &AnthropicExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	err := provider.TestModel(context.Background(), "claude-sonnet-4-5-20250929", false)
	if err != nil {
		t.Fatalf("TestModel failed: %v", err)
	}
}

func TestAnthropicExtendedProvider_TestModelWithThinking(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req anthropicExtendedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		response := anthropicExtendedResponse{
			ID:   "msg_thinking",
			Type: "message",
			Role: "assistant",
			Content: []anthropicExtendedContent{
				{Type: "text", Text: "Thoughtful response"},
			},
			Model:      req.Model,
			StopReason: "end_turn",
			Usage: anthropicExtendedUsage{
				InputTokens:    20,
				OutputTokens:   15,
				ThinkingTokens: 100,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := &AnthropicExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	err := provider.TestModel(context.Background(), "claude-sonnet-4-5-20250929", true)
	if err != nil {
		t.Fatalf("TestModel with thinking failed: %v", err)
	}
}

func TestAnthropicExtendedProvider_TestModelError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "Invalid request"}}`))
	}))
	defer server.Close()

	provider := &AnthropicExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	err := provider.TestModel(context.Background(), "invalid-model", false)
	if err == nil {
		t.Fatal("Expected error for invalid model")
	}
}

func TestAnthropicExtendedProvider_GetCapabilities(t *testing.T) {
	provider := NewAnthropicExtendedProvider("test-key")
	caps := provider.GetCapabilities()

	if !caps.SupportsChat {
		t.Error("Expected SupportsChat to be true")
	}

	if !caps.SupportsVision {
		t.Error("Expected SupportsVision to be true")
	}

	if !caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be true")
	}

	if !caps.SupportsAgents {
		t.Error("Expected SupportsAgents to be true")
	}

	// Verify thinking_budget parameter support
	hasThinkingBudget := false
	for _, param := range caps.SupportedParameters {
		if param == "thinking_budget" {
			hasThinkingBudget = true
			break
		}
	}
	if !hasThinkingBudget {
		t.Error("Expected thinking_budget in SupportedParameters")
	}

	// Verify extended thinking security feature
	hasExtendedThinking := false
	for _, feature := range caps.SecurityFeatures {
		if feature == "extended_thinking" {
			hasExtendedThinking = true
			break
		}
	}
	if !hasExtendedThinking {
		t.Error("Expected extended_thinking in SecurityFeatures")
	}

	if caps.MaxTokensPerRequest != 200000 {
		t.Errorf("Expected MaxTokensPerRequest 200000, got %d", caps.MaxTokensPerRequest)
	}
}

func TestAnthropicExtendedProvider_GetEndpoints(t *testing.T) {
	provider := NewAnthropicExtendedProvider("test-key")
	endpoints := provider.GetEndpoints()

	if len(endpoints) != 2 {
		t.Fatalf("Expected 2 endpoints, got %d", len(endpoints))
	}

	// Verify messages endpoint
	messagesFound := false
	modelsFound := false

	for _, endpoint := range endpoints {
		if endpoint.Path == "/messages" && endpoint.Method == "POST" {
			messagesFound = true
			if endpoint.Description == "" {
				t.Error("Expected messages endpoint to have description")
			}
		}
		if endpoint.Path == "/models" && endpoint.Method == "GET" {
			modelsFound = true
		}
	}

	if !messagesFound {
		t.Error("Expected to find /messages POST endpoint")
	}

	if !modelsFound {
		t.Error("Expected to find /models GET endpoint")
	}
}

func TestAnthropicExtendedProvider_EnrichModelDetails(t *testing.T) {
	provider := &AnthropicExtendedProvider{}

	tests := []struct {
		modelID          string
		expectedCostIn   float64
		expectedCostOut  float64
		expectedContext  int
		expectedMaxToken int
		expectedCategory string
	}{
		{"claude-sonnet-4-5-20250929", 3.00, 15.00, 200000, 64000, "extended-thinking"},
		{"claude-opus-4-0-20250514", 5.00, 25.00, 200000, 64000, "extended-thinking"},
		{"claude-haiku-4-0-20250514", 1.00, 5.00, 200000, 64000, "extended-thinking"},
		{"claude-sonnet-3.5-20241022", 3.00, 15.00, 200000, 8192, "legacy"},
		{"claude-haiku-3.5-20241022", 0.80, 4.00, 200000, 4096, "legacy"},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			model := Model{ID: tt.modelID}
			enriched := provider.enrichModelDetails(model)

			if enriched.CostPer1MIn != tt.expectedCostIn {
				t.Errorf("Expected cost per 1M input %f, got %f", tt.expectedCostIn, enriched.CostPer1MIn)
			}

			if enriched.CostPer1MOut != tt.expectedCostOut {
				t.Errorf("Expected cost per 1M output %f, got %f", tt.expectedCostOut, enriched.CostPer1MOut)
			}

			if enriched.ContextWindow != tt.expectedContext {
				t.Errorf("Expected context window %d, got %d", tt.expectedContext, enriched.ContextWindow)
			}

			if enriched.MaxTokens != tt.expectedMaxToken {
				t.Errorf("Expected max tokens %d, got %d", tt.expectedMaxToken, enriched.MaxTokens)
			}

			categoryFound := false
			for _, cat := range enriched.Categories {
				if cat == tt.expectedCategory {
					categoryFound = true
					break
				}
			}
			if !categoryFound {
				t.Errorf("Expected category %s not found in %v", tt.expectedCategory, enriched.Categories)
			}

			// All models should support extended thinking
			if enriched.Capabilities["extended_thinking"] != "supported" {
				t.Error("Expected extended_thinking capability to be supported")
			}

			if enriched.Capabilities["thinking_budget"] != "configurable" {
				t.Error("Expected thinking_budget capability to be configurable")
			}

			// All Claude models have these capabilities
			if !enriched.SupportsImages {
				t.Error("Expected SupportsImages to be true")
			}

			if !enriched.SupportsTools {
				t.Error("Expected SupportsTools to be true")
			}

			if !enriched.CanReason {
				t.Error("Expected CanReason to be true")
			}
		})
	}
}

func TestAnthropicExtendedProvider_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := &AnthropicExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := provider.TestModel(ctx, "claude-sonnet-4-5-20250929", false)
	if err == nil {
		t.Fatal("Expected error from cancelled context")
	}
}

func TestAnthropicExtendedProvider_ConcurrentRequests(t *testing.T) {
	var requestTimes []time.Time
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestTimes = append(requestTimes, time.Now())
		mu.Unlock()

		time.Sleep(10 * time.Millisecond)

		if r.URL.Path == "/messages" {
			response := anthropicExtendedResponse{
				ID:   "msg_concurrent",
				Type: "message",
				Role: "assistant",
				Content: []anthropicExtendedContent{
					{Type: "text", Text: "Response"},
				},
				Model:      "claude-sonnet-4-5-20250929",
				StopReason: "end_turn",
				Usage: anthropicExtendedUsage{
					InputTokens:  10,
					OutputTokens: 5,
				},
			}
			json.NewEncoder(w).Encode(response)
		} else {
			response := anthropicExtendedModelsResponse{
				Data: []anthropicExtendedModelInfo{
					{ID: "test-model", DisplayName: "Test", CreatedAt: time.Now(), Type: "model"},
				},
			}
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	provider := &AnthropicExtendedProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	err := provider.ValidateEndpoints(context.Background(), false)
	if err != nil {
		t.Fatalf("ValidateEndpoints failed: %v", err)
	}

	mu.Lock()
	reqCount := len(requestTimes)
	mu.Unlock()

	if reqCount != 2 {
		t.Fatalf("Expected 2 concurrent requests, got %d", reqCount)
	}

	// Verify requests were concurrent (started within 20ms of each other)
	mu.Lock()
	timeDiff := requestTimes[1].Sub(requestTimes[0])
	mu.Unlock()

	if timeDiff > 20*time.Millisecond {
		t.Errorf("Requests not concurrent, time difference: %v", timeDiff)
	}
}
