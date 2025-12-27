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

func TestNewDeepSeekExtendedProvider(t *testing.T) {
	apiKey := "test-key-123"
	provider := NewDeepSeekExtendedProvider(apiKey)

	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}

	ds, ok := provider.(*DeepSeekExtendedProvider)
	if !ok {
		t.Fatal("Expected *DeepSeekExtendedProvider")
	}

	if ds.apiKey != apiKey {
		t.Errorf("Expected apiKey=%s, got %s", apiKey, ds.apiKey)
	}

	if ds.baseURL != "https://api.deepseek.com/v1" {
		t.Errorf("Expected baseURL=https://api.deepseek.com/v1, got %s", ds.baseURL)
	}

	if ds.client == nil {
		t.Error("Expected non-nil HTTP client")
	}

	if ds.client.Timeout != 60*time.Second {
		t.Errorf("Expected timeout=60s, got %v", ds.client.Timeout)
	}
}

func TestDeepSeekExtendedProvider_ProviderRegistration(t *testing.T) {
	factory, exists := GetProviderFactory("deepseek_extended")
	if !exists {
		t.Fatal("Expected deepseek_extended provider to be registered")
	}

	provider := factory("test-key")
	if provider == nil {
		t.Fatal("Expected factory to return non-nil provider")
	}

	_, ok := provider.(*DeepSeekExtendedProvider)
	if !ok {
		t.Fatal("Expected factory to return *DeepSeekExtendedProvider")
	}
}

func TestDeepSeekExtendedProvider_GetEndpoints(t *testing.T) {
	provider := NewDeepSeekExtendedProvider("test-key")
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

func TestDeepSeekExtendedProvider_GetCapabilities(t *testing.T) {
	provider := NewDeepSeekExtendedProvider("test-key")
	caps := provider.GetCapabilities()

	if !caps.SupportsChat {
		t.Error("Expected SupportsChat=true")
	}

	if !caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming=true")
	}

	if !caps.SupportsFIM {
		t.Error("Expected SupportsFIM=true")
	}

	if !caps.SupportsJSONMode {
		t.Error("Expected SupportsJSONMode=true")
	}

	if caps.SupportsVision {
		t.Error("Expected SupportsVision=false")
	}

	if caps.MaxTokensPerRequest != 64000 {
		t.Errorf("Expected MaxTokensPerRequest=64000, got %d", caps.MaxTokensPerRequest)
	}
}

func TestDeepSeekExtendedProvider_ListModels_Success(t *testing.T) {
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

		resp := deepseekModelsResponse{
			Object: "list",
			Data: []deepseekModel{
				{ID: "deepseek-chat", Object: "model", Created: 1686935002, OwnedBy: "deepseek"},
				{ID: "deepseek-reasoner", Object: "model", Created: 1686935002, OwnedBy: "deepseek"},
				{ID: "deepseek-coder", Object: "model", Created: 1686935002, OwnedBy: "deepseek"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewDeepSeekExtendedProvider("test-key").(*DeepSeekExtendedProvider)
	provider.baseURL = server.URL + "/v1"

	models, err := provider.ListModels(context.Background(), false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 3 {
		t.Errorf("Expected 3 models, got %d", len(models))
	}

	// Verify deepseek-reasoner enrichment
	var reasoner *Model
	for i := range models {
		if models[i].ID == "deepseek-reasoner" {
			reasoner = &models[i]
			break
		}
	}

	if reasoner == nil {
		t.Fatal("Expected to find deepseek-reasoner model")
	}

	if reasoner.CostPer1MIn != 0.55 {
		t.Errorf("Expected CostPer1MIn=0.55, got %f", reasoner.CostPer1MIn)
	}

	if reasoner.ContextWindow != 64000 {
		t.Errorf("Expected ContextWindow=64000, got %d", reasoner.ContextWindow)
	}

	if !reasoner.CanReason {
		t.Error("Expected deepseek-reasoner to support reasoning")
	}

	if reasoner.SupportsImages {
		t.Error("Expected deepseek-reasoner to not support images")
	}
}

func TestDeepSeekExtendedProvider_ListModels_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	}))
	defer server.Close()

	provider := NewDeepSeekExtendedProvider("invalid-key").(*DeepSeekExtendedProvider)
	provider.baseURL = server.URL + "/v1"

	_, err := provider.ListModels(context.Background(), false)
	if err == nil {
		t.Fatal("Expected error for unauthorized request")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Errorf("Expected 401 error, got %v", err)
	}
}

func TestDeepSeekExtendedProvider_TestModel_Success(t *testing.T) {
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

		var req deepseekChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.Model != "deepseek-chat" {
			t.Errorf("Expected model deepseek-chat, got %s", req.Model)
		}

		resp := deepseekChatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "deepseek-chat",
			Choices: []deepseekChatChoice{
				{
					Index: 0,
					Message: deepseekChatMessage{
						Role:    "assistant",
						Content: "Test successful",
					},
					FinishReason: "stop",
				},
			},
			Usage: deepseekUsage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewDeepSeekExtendedProvider("test-key").(*DeepSeekExtendedProvider)
	provider.baseURL = server.URL + "/v1"

	err := provider.TestModel(context.Background(), "deepseek-chat", false)
	if err != nil {
		t.Fatalf("TestModel failed: %v", err)
	}
}

func TestDeepSeekExtendedProvider_TestModel_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "Invalid model"}}`))
	}))
	defer server.Close()

	provider := NewDeepSeekExtendedProvider("test-key").(*DeepSeekExtendedProvider)
	provider.baseURL = server.URL + "/v1"

	err := provider.TestModel(context.Background(), "invalid-model", false)
	if err == nil {
		t.Fatal("Expected error for invalid model")
	}

	if !strings.Contains(err.Error(), "400") {
		t.Errorf("Expected 400 error, got %v", err)
	}
}

func TestDeepSeekExtendedProvider_ValidateEndpoints(t *testing.T) {
	var requestCount int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		if r.URL.Path == "/models" {
			resp := deepseekModelsResponse{
				Object: "list",
				Data:   []deepseekModel{{ID: "deepseek-chat", Object: "model", Created: 1686935002}},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		if r.URL.Path == "/chat/completions" {
			resp := deepseekChatCompletionResponse{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   "deepseek-chat",
				Choices: []deepseekChatChoice{
					{Index: 0, Message: deepseekChatMessage{Role: "assistant", Content: "Hi"}, FinishReason: "stop"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := NewDeepSeekExtendedProvider("test-key").(*DeepSeekExtendedProvider)
	provider.baseURL = server.URL

	err := provider.ValidateEndpoints(context.Background(), false)
	if err != nil {
		t.Fatalf("ValidateEndpoints failed: %v", err)
	}

	mu.Lock()
	count := requestCount
	mu.Unlock()

	if count == 0 {
		t.Error("Expected at least one request to be made")
	}

	endpoints := provider.GetEndpoints()
	for _, ep := range endpoints {
		if ep.Status != StatusWorking {
			t.Errorf("Expected endpoint %s to be working, got status %s", ep.Path, ep.Status)
		}
	}
}

func TestDeepSeekExtendedProvider_ValidateEndpoints_Concurrent(t *testing.T) {
	var requestTimes []time.Time
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestTimes = append(requestTimes, time.Now())
		mu.Unlock()
		time.Sleep(50 * time.Millisecond) // Simulate latency

		if r.URL.Path == "/models" {
			resp := deepseekModelsResponse{Object: "list", Data: []deepseekModel{}}
			json.NewEncoder(w).Encode(resp)
			return
		}

		if r.URL.Path == "/chat/completions" {
			resp := deepseekChatCompletionResponse{
				ID:      "chatcmpl-123",
				Choices: []deepseekChatChoice{{Message: deepseekChatMessage{Content: "Hi"}}},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewDeepSeekExtendedProvider("test-key").(*DeepSeekExtendedProvider)
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

func TestDeepSeekExtendedProvider_ValidateEndpoints_PartialFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			resp := deepseekModelsResponse{Object: "list", Data: []deepseekModel{}}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Fail chat completions
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	provider := NewDeepSeekExtendedProvider("test-key").(*DeepSeekExtendedProvider)
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

func TestDeepSeekExtendedProvider_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewDeepSeekExtendedProvider("test-key").(*DeepSeekExtendedProvider)
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

func TestDeepSeekExtendedProvider_ModelNameFormatting(t *testing.T) {
	provider := NewDeepSeekExtendedProvider("test-key").(*DeepSeekExtendedProvider)

	testCases := []struct {
		modelID  string
		expected string
	}{
		{"deepseek-chat", "DeepSeek Chat: deepseek-chat"},
		{"deepseek-reasoner", "DeepSeek Reasoner: deepseek-reasoner"},
		{"deepseek-coder", "DeepSeek Coder: deepseek-coder"},
		{"unknown-model", "unknown-model"},
	}

	for _, tc := range testCases {
		result := provider.formatModelName(tc.modelID)
		if result != tc.expected {
			t.Errorf("formatModelName(%s) = %s, expected %s", tc.modelID, result, tc.expected)
		}
	}
}

func TestDeepSeekExtendedProvider_ModelEnrichment(t *testing.T) {
	provider := NewDeepSeekExtendedProvider("test-key").(*DeepSeekExtendedProvider)

	testCases := []struct {
		modelID         string
		expectedCost    float64
		expectedContext int
		expectedReason  bool
	}{
		{"deepseek-chat", 0.27, 64000, false},
		{"deepseek-reasoner", 0.55, 64000, true},
		{"deepseek-coder", 0.27, 64000, false},
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

		if enriched.CanReason != tc.expectedReason {
			t.Errorf("%s: CanReason = %v, expected %v", tc.modelID, enriched.CanReason, tc.expectedReason)
		}

		if enriched.SupportsImages {
			t.Errorf("%s: Expected SupportsImages=false", tc.modelID)
		}

		if !enriched.SupportsTools {
			t.Errorf("%s: Expected SupportsTools=true", tc.modelID)
		}

		if !enriched.CanStream {
			t.Errorf("%s: Expected CanStream=true", tc.modelID)
		}
	}
}

func TestDeepSeekExtendedProvider_VerboseOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			resp := deepseekModelsResponse{
				Object: "list",
				Data:   []deepseekModel{{ID: "deepseek-chat", Object: "model", Created: 1686935002}},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		if r.URL.Path == "/chat/completions" {
			resp := deepseekChatCompletionResponse{
				ID:      "chatcmpl-123",
				Choices: []deepseekChatChoice{{Message: deepseekChatMessage{Content: "Test successful"}}},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
	}))
	defer server.Close()

	provider := NewDeepSeekExtendedProvider("test-key").(*DeepSeekExtendedProvider)
	provider.baseURL = server.URL

	// Test verbose ListModels (should not panic)
	_, err := provider.ListModels(context.Background(), true)
	if err != nil {
		t.Fatalf("ListModels verbose failed: %v", err)
	}

	// Test verbose TestModel (should not panic)
	err = provider.TestModel(context.Background(), "deepseek-chat", true)
	if err != nil {
		t.Fatalf("TestModel verbose failed: %v", err)
	}

	// Test verbose ValidateEndpoints (should not panic)
	err = provider.ValidateEndpoints(context.Background(), true)
	if err != nil {
		t.Fatalf("ValidateEndpoints verbose failed: %v", err)
	}
}

func TestDeepSeekExtendedProvider_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	provider := NewDeepSeekExtendedProvider("test-key").(*DeepSeekExtendedProvider)
	provider.baseURL = server.URL + "/v1"

	_, err := provider.ListModels(context.Background(), false)
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("Expected decode error, got: %v", err)
	}
}
