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

func TestNewOpenAIExtendedProvider(t *testing.T) {
	apiKey := "test-key-123"
	provider := NewOpenAIExtendedProvider(apiKey)

	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}

	oai, ok := provider.(*OpenAIExtendedProvider)
	if !ok {
		t.Fatal("Expected *OpenAIExtendedProvider")
	}

	if oai.apiKey != apiKey {
		t.Errorf("Expected apiKey=%s, got %s", apiKey, oai.apiKey)
	}

	if oai.baseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected baseURL=https://api.openai.com/v1, got %s", oai.baseURL)
	}

	if oai.client == nil {
		t.Error("Expected non-nil HTTP client")
	}

	if oai.client.Timeout != 60*time.Second {
		t.Errorf("Expected timeout=60s, got %v", oai.client.Timeout)
	}
}

func TestOpenAIExtendedProvider_GetEndpoints(t *testing.T) {
	provider := NewOpenAIExtendedProvider("test-key")
	endpoints := provider.GetEndpoints()

	if len(endpoints) == 0 {
		t.Fatal("Expected at least one endpoint")
	}

	// Verify chat completions endpoint
	found := false
	for _, ep := range endpoints {
		if ep.Path == "/chat/completions" && ep.Method == "POST" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected /chat/completions endpoint")
	}

	// Verify models endpoint
	found = false
	for _, ep := range endpoints {
		if ep.Path == "/models" && ep.Method == "GET" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected /models endpoint")
	}
}

func TestOpenAIExtendedProvider_GetCapabilities(t *testing.T) {
	provider := NewOpenAIExtendedProvider("test-key")
	caps := provider.GetCapabilities()

	if !caps.SupportsChat {
		t.Error("Expected SupportsChat=true")
	}

	if !caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming=true")
	}

	if !caps.SupportsVision {
		t.Error("Expected SupportsVision=true")
	}

	if !caps.SupportsJSONMode {
		t.Error("Expected SupportsJSONMode=true")
	}

	if caps.MaxTokensPerRequest != 128000 {
		t.Errorf("Expected MaxTokensPerRequest=128000, got %d", caps.MaxTokensPerRequest)
	}
}

func TestOpenAIExtendedProvider_ListModels_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("Expected path /v1/models, got %s", r.URL.Path)
		}

		if r.Method != "GET" {
			t.Errorf("Expected method GET, got %s", r.Method)
		}

		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("Expected Bearer token, got %s", auth)
		}

		resp := openaiModelsResponse{
			Object: "list",
			Data: []openaiModel{
				{ID: "gpt-4o", Object: "model", Created: 1686935002, OwnedBy: "openai"},
				{ID: "gpt-4-turbo", Object: "model", Created: 1686935002, OwnedBy: "openai"},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1686935002, OwnedBy: "openai"},
				{ID: "text-embedding-ada-002", Object: "model", Created: 1686935002, OwnedBy: "openai"}, // Should be filtered
				{ID: "whisper-1", Object: "model", Created: 1686935002, OwnedBy: "openai"},              // Should be filtered
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOpenAIExtendedProvider("test-key").(*OpenAIExtendedProvider)
	provider.baseURL = server.URL + "/v1"

	models, err := provider.ListModels(context.Background(), false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 3 {
		t.Errorf("Expected 3 models (filtered), got %d", len(models))
	}

	// Verify GPT-4o enrichment
	var gpt4o *Model
	for i := range models {
		if models[i].ID == "gpt-4o" {
			gpt4o = &models[i]
			break
		}
	}

	if gpt4o == nil {
		t.Fatal("Expected to find gpt-4o model")
	}

	if gpt4o.CostPer1MIn != 2.50 {
		t.Errorf("Expected CostPer1MIn=2.50, got %f", gpt4o.CostPer1MIn)
	}

	if gpt4o.ContextWindow != 128000 {
		t.Errorf("Expected ContextWindow=128000, got %d", gpt4o.ContextWindow)
	}

	if !gpt4o.SupportsImages {
		t.Error("Expected gpt-4o to support images")
	}

	if !gpt4o.CanReason {
		t.Error("Expected gpt-4o to support reasoning")
	}
}

func TestOpenAIExtendedProvider_ListModels_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	}))
	defer server.Close()

	provider := NewOpenAIExtendedProvider("invalid-key").(*OpenAIExtendedProvider)
	provider.baseURL = server.URL + "/v1"

	_, err := provider.ListModels(context.Background(), false)
	if err == nil {
		t.Fatal("Expected error for unauthorized request")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Errorf("Expected 401 error, got %v", err)
	}
}

func TestOpenAIExtendedProvider_TestModel_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("Expected path /v1/chat/completions, got %s", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("Expected method POST, got %s", r.Method)
		}

		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("Expected Bearer token, got %s", auth)
		}

		var req openaiChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.Model != "gpt-4o" {
			t.Errorf("Expected model gpt-4o, got %s", req.Model)
		}

		resp := openaiChatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4o",
			Choices: []openaiChatChoice{
				{
					Index: 0,
					Message: openaiChatMessage{
						Role:    "assistant",
						Content: "Test successful",
					},
					FinishReason: "stop",
				},
			},
			Usage: openaiUsage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOpenAIExtendedProvider("test-key").(*OpenAIExtendedProvider)
	provider.baseURL = server.URL + "/v1"

	err := provider.TestModel(context.Background(), "gpt-4o", false)
	if err != nil {
		t.Fatalf("TestModel failed: %v", err)
	}
}

func TestOpenAIExtendedProvider_TestModel_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "Invalid model"}}`))
	}))
	defer server.Close()

	provider := NewOpenAIExtendedProvider("test-key").(*OpenAIExtendedProvider)
	provider.baseURL = server.URL + "/v1"

	err := provider.TestModel(context.Background(), "invalid-model", false)
	if err == nil {
		t.Fatal("Expected error for invalid model")
	}

	if !strings.Contains(err.Error(), "400") {
		t.Errorf("Expected 400 error, got %v", err)
	}
}

func TestOpenAIExtendedProvider_ValidateEndpoints(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		if r.URL.Path == "/models" {
			resp := openaiModelsResponse{
				Object: "list",
				Data:   []openaiModel{{ID: "gpt-4o", Object: "model", Created: 1686935002}},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		if r.URL.Path == "/chat/completions" {
			resp := openaiChatCompletionResponse{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   "gpt-3.5-turbo",
				Choices: []openaiChatChoice{
					{Index: 0, Message: openaiChatMessage{Role: "assistant", Content: "Hi"}, FinishReason: "stop"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := NewOpenAIExtendedProvider("test-key").(*OpenAIExtendedProvider)
	provider.baseURL = server.URL

	err := provider.ValidateEndpoints(context.Background(), false)
	if err != nil {
		t.Fatalf("ValidateEndpoints failed: %v", err)
	}

	if requestCount == 0 {
		t.Error("Expected at least one request to be made")
	}

	endpoints := provider.GetEndpoints()
	for _, ep := range endpoints {
		if ep.Status != StatusWorking {
			t.Errorf("Expected endpoint %s to be working, got status %s", ep.Path, ep.Status)
		}
	}
}

func TestOpenAIExtendedProvider_ValidateEndpoints_Concurrent(t *testing.T) {
	var requestTimes []time.Time
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestTimes = append(requestTimes, time.Now())
		mu.Unlock()
		time.Sleep(50 * time.Millisecond) // Simulate latency

		if r.URL.Path == "/models" {
			resp := openaiModelsResponse{Object: "list", Data: []openaiModel{}}
			json.NewEncoder(w).Encode(resp)
			return
		}

		if r.URL.Path == "/chat/completions" {
			resp := openaiChatCompletionResponse{
				ID:      "chatcmpl-123",
				Choices: []openaiChatChoice{{Message: openaiChatMessage{Content: "Hi"}}},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewOpenAIExtendedProvider("test-key").(*OpenAIExtendedProvider)
	provider.baseURL = server.URL

	start := time.Now()
	err := provider.ValidateEndpoints(context.Background(), false)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("ValidateEndpoints failed: %v", err)
	}

	// With 2 endpoints and 50ms sleep per request, sequential would take 100ms+
	// Concurrent should take ~50ms (plus overhead)
	if elapsed > 90*time.Millisecond {
		t.Logf("Warning: ValidateEndpoints took %v, may not be concurrent", elapsed)
	}

	if len(requestTimes) < 2 {
		t.Fatal("Expected at least 2 requests")
	}

	// Check that requests started close together (within 10ms)
	timeDiff := requestTimes[1].Sub(requestTimes[0])
	if timeDiff > 10*time.Millisecond {
		t.Logf("Note: Request time difference is %v, requests may be sequential", timeDiff)
	}
}

func TestOpenAIExtendedProvider_ValidateEndpoints_PartialFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			resp := openaiModelsResponse{Object: "list", Data: []openaiModel{}}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Fail chat completions
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	provider := NewOpenAIExtendedProvider("test-key").(*OpenAIExtendedProvider)
	provider.baseURL = server.URL

	err := provider.ValidateEndpoints(context.Background(), false)
	if err != nil {
		t.Fatalf("ValidateEndpoints failed: %v", err)
	}

	endpoints := provider.GetEndpoints()
	workingCount := 0
	failedCount := 0

	for _, ep := range endpoints {
		if ep.Status == StatusWorking {
			workingCount++
		} else if ep.Status == StatusFailed {
			failedCount++
		}
	}

	if workingCount == 0 {
		t.Error("Expected at least one working endpoint")
	}

	if failedCount == 0 {
		t.Error("Expected at least one failed endpoint")
	}
}

func TestOpenAIExtendedProvider_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewOpenAIExtendedProvider("test-key").(*OpenAIExtendedProvider)
	provider.baseURL = server.URL + "/v1"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Fatal("Expected context cancellation error")
	}

	if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "canceled") && !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected context cancellation error, got: %v", err)
	}
}

func TestOpenAIExtendedProvider_ModelFiltering(t *testing.T) {
	provider := NewOpenAIExtendedProvider("test-key").(*OpenAIExtendedProvider)

	testCases := []struct {
		modelID  string
		expected bool
	}{
		{"gpt-4o", true},
		{"gpt-4-turbo", true},
		{"gpt-3.5-turbo", true},
		{"text-embedding-ada-002", false},
		{"text-embedding-3-small", false},
		{"whisper-1", false},
		{"tts-1", false},
		{"dall-e-3", false},
		{"text-moderation-latest", false},
		{"davinci-002", false},
		{"babbage-002", false},
		{"o1-preview", true},
		{"o1-mini", true},
	}

	for _, tc := range testCases {
		result := provider.isUsableModel(tc.modelID)
		if result != tc.expected {
			t.Errorf("isUsableModel(%s) = %v, expected %v", tc.modelID, result, tc.expected)
		}
	}
}

func TestOpenAIExtendedProvider_ModelNameFormatting(t *testing.T) {
	provider := NewOpenAIExtendedProvider("test-key").(*OpenAIExtendedProvider)

	testCases := []struct {
		modelID  string
		expected string
	}{
		{"gpt-4o", "GPT-4 Omni: gpt-4o"},
		{"gpt-4o-mini", "GPT-4 Omni: gpt-4o-mini"},
		{"gpt-4-turbo", "GPT-4 Turbo: gpt-4-turbo"},
		{"gpt-4", "GPT-4: gpt-4"},
		{"gpt-3.5-turbo", "GPT-3.5: gpt-3.5-turbo"},
		{"o1-preview", "O-Series Reasoning: o1-preview"},
		{"o3-mini", "O-Series Reasoning: o3-mini"},
	}

	for _, tc := range testCases {
		result := provider.formatModelName(tc.modelID)
		if result != tc.expected {
			t.Errorf("formatModelName(%s) = %s, expected %s", tc.modelID, result, tc.expected)
		}
	}
}

func TestOpenAIExtendedProvider_ModelEnrichment(t *testing.T) {
	provider := NewOpenAIExtendedProvider("test-key").(*OpenAIExtendedProvider)

	testCases := []struct {
		modelID         string
		expectedCost    float64
		expectedContext int
		expectedVision  bool
		expectedReason  bool
	}{
		{"gpt-4o", 2.50, 128000, true, true},
		{"gpt-4o-mini", 0.15, 128000, true, false},
		{"gpt-4-turbo", 10.00, 128000, false, true},
		{"gpt-3.5-turbo", 0.50, 16385, false, false},
		{"o1-preview", 15.00, 128000, false, true},
		{"o1-mini", 3.00, 128000, false, true},
	}

	for _, tc := range testCases {
		model := Model{ID: tc.modelID}
		enriched := provider.enrichModelDetails(model)

		if enriched.CostPer1MIn != tc.expectedCost {
			t.Errorf("%s: CostPer1MIn = %f, expected %f", tc.modelID, enriched.CostPer1MIn, tc.expectedCost)
		}

		if enriched.ContextWindow != tc.expectedContext {
			t.Errorf("%s: ContextWindow = %d, expected %d", tc.modelID, enriched.ContextWindow, tc.expectedContext)
		}

		if enriched.SupportsImages != tc.expectedVision {
			t.Errorf("%s: SupportsImages = %v, expected %v", tc.modelID, enriched.SupportsImages, tc.expectedVision)
		}

		if enriched.CanReason != tc.expectedReason {
			t.Errorf("%s: CanReason = %v, expected %v", tc.modelID, enriched.CanReason, tc.expectedReason)
		}

		if !enriched.SupportsTools {
			t.Errorf("%s: Expected SupportsTools=true", tc.modelID)
		}

		if !enriched.CanStream {
			t.Errorf("%s: Expected CanStream=true", tc.modelID)
		}
	}
}

func TestOpenAIExtendedProvider_VerboseOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			resp := openaiModelsResponse{
				Object: "list",
				Data:   []openaiModel{{ID: "gpt-4o", Object: "model", Created: 1686935002}},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		if r.URL.Path == "/chat/completions" {
			resp := openaiChatCompletionResponse{
				ID:      "chatcmpl-123",
				Choices: []openaiChatChoice{{Message: openaiChatMessage{Content: "Test successful"}}},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
	}))
	defer server.Close()

	provider := NewOpenAIExtendedProvider("test-key").(*OpenAIExtendedProvider)
	provider.baseURL = server.URL

	// Test verbose ListModels (should not panic)
	_, err := provider.ListModels(context.Background(), true)
	if err != nil {
		t.Fatalf("ListModels verbose failed: %v", err)
	}

	// Test verbose TestModel (should not panic)
	err = provider.TestModel(context.Background(), "gpt-4o", true)
	if err != nil {
		t.Fatalf("TestModel verbose failed: %v", err)
	}

	// Test verbose ValidateEndpoints (should not panic)
	err = provider.ValidateEndpoints(context.Background(), true)
	if err != nil {
		t.Fatalf("ValidateEndpoints verbose failed: %v", err)
	}
}
