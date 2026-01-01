package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jeffersonwarrior/modelscan/internal/admin"
	"github.com/jeffersonwarrior/modelscan/internal/proxy"
)

// ====== Mock implementations for proxy tests ======

// mockKeyProvider implements proxy.KeyProvider for testing
type mockKeyProvider struct {
	keys map[string]string
}

func newMockKeyProvider() *mockKeyProvider {
	return &mockKeyProvider{
		keys: map[string]string{
			"openai":    "sk-test-openai-key",
			"anthropic": "sk-test-anthropic-key",
		},
	}
}

func (m *mockKeyProvider) GetKey(ctx context.Context, providerID string) (string, error) {
	if key, ok := m.keys[providerID]; ok {
		return key, nil
	}
	return "", context.DeadlineExceeded
}

// mockModelRemapper implements proxy.ModelRemapper for testing
type mockModelRemapper struct {
	remaps map[string]struct {
		model    string
		provider string
	}
}

func newMockModelRemapper() *mockModelRemapper {
	return &mockModelRemapper{
		remaps: map[string]struct {
			model    string
			provider string
		}{
			"gpt-4-alias": {model: "gpt-4-turbo", provider: "openai"},
		},
	}
}

func (m *mockModelRemapper) RemapModel(ctx context.Context, model string, clientID string) (string, string, error) {
	if remap, ok := m.remaps[model]; ok {
		return remap.model, remap.provider, nil
	}
	return model, "", nil
}

// ====== OpenAI Proxy Tests ======

func TestOpenAIProxy_HandleChatCompletions_MethodNotAllowed(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := proxy.DefaultOpenAIProxyConfig()
	p := proxy.NewOpenAIProxy(cfg, newMockKeyProvider(), newMockModelRemapper())

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	w := httptest.NewRecorder()

	p.HandleChatCompletions(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestOpenAIProxy_HandleChatCompletions_InvalidJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := proxy.DefaultOpenAIProxyConfig()
	p := proxy.NewOpenAIProxy(cfg, newMockKeyProvider(), newMockModelRemapper())

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("invalid json"))
	w := httptest.NewRecorder()

	p.HandleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var errResp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if _, ok := errResp["error"]; !ok {
		t.Error("expected error field in response")
	}
}

func TestOpenAIProxy_HandleChatCompletions_MissingModel(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := proxy.DefaultOpenAIProxyConfig()
	p := proxy.NewOpenAIProxy(cfg, newMockKeyProvider(), newMockModelRemapper())

	body := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(jsonBody))
	w := httptest.NewRecorder()

	p.HandleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestOpenAIProxy_HandleChatCompletions_MissingMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := proxy.DefaultOpenAIProxyConfig()
	p := proxy.NewOpenAIProxy(cfg, newMockKeyProvider(), newMockModelRemapper())

	body := map[string]interface{}{
		"model": "gpt-4",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(jsonBody))
	w := httptest.NewRecorder()

	p.HandleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestOpenAIProxy_HandleChatCompletions_NoAPIKey(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := proxy.DefaultOpenAIProxyConfig()
	// Use empty key provider
	emptyKeyProvider := &mockKeyProvider{keys: map[string]string{}}
	p := proxy.NewOpenAIProxy(cfg, emptyKeyProvider, newMockModelRemapper())

	body := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(jsonBody))
	w := httptest.NewRecorder()

	p.HandleChatCompletions(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}
}

// ====== Anthropic Proxy Tests ======

func TestAnthropicProxy_HandleMessages_MethodNotAllowed(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := proxy.DefaultAnthropicProxyConfig()
	p := proxy.NewAnthropicProxy(cfg, newMockKeyProvider(), newMockModelRemapper())

	req := httptest.NewRequest(http.MethodGet, "/v1/messages", nil)
	w := httptest.NewRecorder()

	p.HandleMessages(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestAnthropicProxy_HandleMessages_InvalidJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := proxy.DefaultAnthropicProxyConfig()
	p := proxy.NewAnthropicProxy(cfg, newMockKeyProvider(), newMockModelRemapper())

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader("invalid json"))
	w := httptest.NewRecorder()

	p.HandleMessages(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestAnthropicProxy_HandleMessages_MissingModel(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := proxy.DefaultAnthropicProxyConfig()
	p := proxy.NewAnthropicProxy(cfg, newMockKeyProvider(), newMockModelRemapper())

	body := map[string]interface{}{
		"messages": []map[string]interface{}{
			{"role": "user", "content": []map[string]string{{"type": "text", "text": "Hello"}}},
		},
		"max_tokens": 100,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(jsonBody))
	w := httptest.NewRecorder()

	p.HandleMessages(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestAnthropicProxy_HandleMessages_MissingMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := proxy.DefaultAnthropicProxyConfig()
	p := proxy.NewAnthropicProxy(cfg, newMockKeyProvider(), newMockModelRemapper())

	body := map[string]interface{}{
		"model":      "claude-3-opus-20240229",
		"max_tokens": 100,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(jsonBody))
	w := httptest.NewRecorder()

	p.HandleMessages(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestAnthropicProxy_HandleMessages_NoAPIKey(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := proxy.DefaultAnthropicProxyConfig()
	emptyKeyProvider := &mockKeyProvider{keys: map[string]string{}}
	p := proxy.NewAnthropicProxy(cfg, emptyKeyProvider, newMockModelRemapper())

	body := map[string]interface{}{
		"model": "claude-3-opus-20240229",
		"messages": []map[string]interface{}{
			{"role": "user", "content": []map[string]string{{"type": "text", "text": "Hello"}}},
		},
		"max_tokens": 100,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(jsonBody))
	w := httptest.NewRecorder()

	p.HandleMessages(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}
}

// ====== Stream Writer Tests ======

func TestStreamWriter_Creation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	w := httptest.NewRecorder()
	sw, err := proxy.NewStreamWriter(w)

	if err != nil {
		t.Fatalf("failed to create stream writer: %v", err)
	}

	if sw == nil {
		t.Error("expected non-nil stream writer")
	}

	// Check headers
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", w.Header().Get("Content-Type"))
	}

	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("expected Cache-Control no-cache, got %s", w.Header().Get("Cache-Control"))
	}

	if w.Header().Get("Connection") != "keep-alive" {
		t.Errorf("expected Connection keep-alive, got %s", w.Header().Get("Connection"))
	}
}

func TestStreamWriter_WriteEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	w := httptest.NewRecorder()
	sw, err := proxy.NewStreamWriter(w)
	if err != nil {
		t.Fatalf("failed to create stream writer: %v", err)
	}

	// Write an event
	data := []byte(`{"test": "data"}`)
	sw.WriteEvent(data)

	body := w.Body.String()
	if !strings.Contains(body, "data:") {
		t.Error("expected SSE data prefix in output")
	}
	if !strings.Contains(body, `{"test": "data"}`) {
		t.Error("expected event data in output")
	}
}

func TestStreamWriter_WriteEventWithType(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	w := httptest.NewRecorder()
	sw, err := proxy.NewStreamWriter(w)
	if err != nil {
		t.Fatalf("failed to create stream writer: %v", err)
	}

	// Write an event with type
	sw.WriteEventWithType("content_block_delta", []byte(`{"delta": {"text": "Hello"}}`))

	body := w.Body.String()
	if !strings.Contains(body, "event: content_block_delta") {
		t.Error("expected event type in output")
	}
	if !strings.Contains(body, "data:") {
		t.Error("expected data prefix in output")
	}
}

func TestStreamWriter_Close(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	w := httptest.NewRecorder()
	sw, err := proxy.NewStreamWriter(w)
	if err != nil {
		t.Fatalf("failed to create stream writer: %v", err)
	}

	sw.Close()

	body := w.Body.String()
	if !strings.Contains(body, "data: [DONE]") {
		t.Error("expected [DONE] marker in output")
	}
}

// ====== Request/Response Translation Tests ======

func TestTranslation_ToOpenAI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	anthropicReq := &proxy.AnthropicRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		Messages: []proxy.AnthropicMessage{
			{
				Role: "user",
				Content: []proxy.ContentPart{
					{Type: "text", Text: "Hello, Claude!"},
				},
			},
		},
		System: "You are a helpful assistant.",
	}

	openaiReq, err := proxy.ToOpenAI(anthropicReq)
	if err != nil {
		t.Fatalf("failed to translate to OpenAI: %v", err)
	}

	if openaiReq.Model != "claude-3-opus-20240229" {
		t.Errorf("expected model claude-3-opus-20240229, got %s", openaiReq.Model)
	}

	if *openaiReq.MaxTokens != 1024 {
		t.Errorf("expected max_tokens 1024, got %d", *openaiReq.MaxTokens)
	}

	// Should have system message + user message
	if len(openaiReq.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(openaiReq.Messages))
	}

	// First message should be system
	if openaiReq.Messages[0].Role != "system" {
		t.Errorf("expected first message role to be system, got %s", openaiReq.Messages[0].Role)
	}
}

func TestTranslation_ToAnthropic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	maxTokens := 1024
	openaiReq := &proxy.OpenAIRequest{
		Model:     "gpt-4",
		MaxTokens: &maxTokens,
		Messages: []proxy.OpenAIMessage{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello!"},
		},
	}

	anthropicReq, err := proxy.ToAnthropic(openaiReq)
	if err != nil {
		t.Fatalf("failed to translate to Anthropic: %v", err)
	}

	if anthropicReq.Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", anthropicReq.Model)
	}

	if anthropicReq.MaxTokens != 1024 {
		t.Errorf("expected max_tokens 1024, got %d", anthropicReq.MaxTokens)
	}

	if anthropicReq.System != "You are helpful." {
		t.Errorf("expected system message 'You are helpful.', got %s", anthropicReq.System)
	}

	// Should have only user message (system moved to System field)
	if len(anthropicReq.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(anthropicReq.Messages))
	}
}

func TestTranslation_NilRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	_, err := proxy.ToOpenAI(nil)
	if err == nil {
		t.Error("expected error for nil request")
	}

	_, err = proxy.ToAnthropic(nil)
	if err == nil {
		t.Error("expected error for nil request")
	}
}

// ====== Admin API Integration Tests ======

// mockAdminDB implements admin.Database for testing
type mockAdminDB struct{}

func (m *mockAdminDB) CreateProvider(p *admin.Provider) error {
	return nil
}

func (m *mockAdminDB) GetProvider(id string) (*admin.Provider, error) {
	return &admin.Provider{ID: id, Name: "Test Provider"}, nil
}

func (m *mockAdminDB) ListProviders() ([]*admin.Provider, error) {
	return []*admin.Provider{
		{ID: "openai", Name: "OpenAI", Status: "online"},
		{ID: "anthropic", Name: "Anthropic", Status: "online"},
	}, nil
}

func (m *mockAdminDB) CreateAPIKey(providerID, apiKey string) (*admin.APIKey, error) {
	return &admin.APIKey{ID: 1, ProviderID: providerID, Active: true}, nil
}

func (m *mockAdminDB) GetAPIKey(id int) (*admin.APIKey, error) {
	if id == 1 {
		prefix := "sk-test..."
		return &admin.APIKey{ID: 1, ProviderID: "openai", KeyPrefix: &prefix, Active: true}, nil
	}
	return nil, nil
}

func (m *mockAdminDB) DeleteAPIKey(id int) error {
	return nil
}

func (m *mockAdminDB) ListActiveAPIKeys(providerID string) ([]*admin.APIKey, error) {
	return []*admin.APIKey{{ID: 1, ProviderID: providerID, Active: true}}, nil
}

func (m *mockAdminDB) GetUsageStats(modelID string, since time.Time) (map[string]interface{}, error) {
	return map[string]interface{}{"total_requests": 100}, nil
}

func (m *mockAdminDB) GetKeyStats(keyID int, since time.Time) (*admin.KeyStats, error) {
	return &admin.KeyStats{
		RequestsToday:    50,
		TokensToday:      1000,
		RateLimitPercent: 25.0,
		DegradationCount: 0,
	}, nil
}

type mockAdminDiscovery struct{}

func (m *mockAdminDiscovery) Discover(providerID string, apiKey string) (*admin.DiscoveryResult, error) {
	return &admin.DiscoveryResult{ProviderID: providerID, Success: true}, nil
}

type mockAdminGenerator struct{}

func (m *mockAdminGenerator) Generate(req admin.GenerateRequest) (*admin.GenerateResult, error) {
	return &admin.GenerateResult{Success: true}, nil
}

func (m *mockAdminGenerator) List() ([]string, error) {
	return []string{"sdk1", "sdk2"}, nil
}

func (m *mockAdminGenerator) Delete(providerID string) error {
	return nil
}

type mockAdminKeyManager struct{}

func (m *mockAdminKeyManager) GetKey(providerID string) (*admin.APIKey, error) {
	return &admin.APIKey{ID: 1, ProviderID: providerID}, nil
}

func (m *mockAdminKeyManager) ListKeys(providerID string) ([]*admin.APIKey, error) {
	return []*admin.APIKey{{ID: 1, ProviderID: providerID}}, nil
}

func (m *mockAdminKeyManager) CountKeys() (int, error) {
	return 1, nil
}

func (m *mockAdminKeyManager) RegisterActualKey(keyHash, actualKey string) {}

func (m *mockAdminKeyManager) TestKey(keyID int) (*admin.KeyTestResult, error) {
	return &admin.KeyTestResult{Valid: true}, nil
}

func TestAdminAPI_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	api := admin.NewAPI(
		admin.Config{Host: "127.0.0.1", Port: 8080},
		&mockAdminDB{},
		&mockAdminDiscovery{},
		&mockAdminGenerator{},
		&mockAdminKeyManager{},
	)

	tests := []struct {
		name       string
		method     string
		path       string
		body       interface{}
		wantStatus int
	}{
		{
			name:       "health check",
			method:     http.MethodGet,
			path:       "/health",
			wantStatus: http.StatusOK,
		},
		{
			name:       "list providers",
			method:     http.MethodGet,
			path:       "/api/providers",
			wantStatus: http.StatusOK,
		},
		{
			name:       "list keys",
			method:     http.MethodGet,
			path:       "/api/keys?provider=openai",
			wantStatus: http.StatusOK,
		},
		{
			name:       "list sdks",
			method:     http.MethodGet,
			path:       "/api/sdks",
			wantStatus: http.StatusOK,
		},
		{
			name:       "get key",
			method:     http.MethodGet,
			path:       "/api/keys/1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "get key not found",
			method:     http.MethodGet,
			path:       "/api/keys/999",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "delete key",
			method:     http.MethodDelete,
			path:       "/api/keys/1",
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "test key",
			method:     http.MethodPost,
			path:       "/api/keys/1/test",
			wantStatus: http.StatusOK,
		},
		{
			name:   "add key",
			method: http.MethodPost,
			path:   "/api/keys/add",
			body: map[string]string{
				"provider_id": "openai",
				"api_key":     "sk-test",
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "discover provider",
			method: http.MethodPost,
			path:   "/api/discover",
			body: map[string]string{
				"identifier": "openai",
				"api_key":    "sk-test",
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var body io.Reader
			if tc.body != nil {
				jsonBody, _ := json.Marshal(tc.body)
				body = bytes.NewReader(jsonBody)
			}

			req := httptest.NewRequest(tc.method, tc.path, body)
			if tc.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()

			api.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, w.Code)
			}
		})
	}
}

// ====== Proxy with Mock Upstream Tests ======

func TestOpenAIProxy_WithMockUpstream_NonStreaming(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create mock upstream server
	mockResponse := `{
		"id": "chatcmpl-test123",
		"object": "chat.completion",
		"created": 1234567890,
		"model": "gpt-4",
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "Hello! How can I help you?"},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 15, "total_tokens": 25}
	}`

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test-openai-key" {
			t.Errorf("unexpected Authorization header: %s", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer upstream.Close()

	// Create proxy with mock upstream URL
	cfg := proxy.OpenAIProxyConfig{
		Timeout:          5 * time.Second,
		DefaultMaxTokens: 4096,
		OpenAIBaseURL:    upstream.URL,
	}
	p := proxy.NewOpenAIProxy(cfg, newMockKeyProvider(), nil)

	// Create request
	body := map[string]interface{}{
		"model":  "gpt-4",
		"stream": false,
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(jsonBody))
	w := httptest.NewRecorder()

	p.HandleChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["id"] != "chatcmpl-test123" {
		t.Errorf("expected id chatcmpl-test123, got %v", resp["id"])
	}
}

func TestOpenAIProxy_WithMockUpstream_Streaming(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create mock SSE streaming response
	sseEvents := []string{
		`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		``,
		`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
		``,
		`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		``,
		`data: [DONE]`,
		``,
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("expected ResponseWriter to implement Flusher")
			return
		}

		for _, event := range sseEvents {
			w.Write([]byte(event + "\n"))
			flusher.Flush()
		}
	}))
	defer upstream.Close()

	cfg := proxy.OpenAIProxyConfig{
		Timeout:          5 * time.Second,
		DefaultMaxTokens: 4096,
		OpenAIBaseURL:    upstream.URL,
	}
	p := proxy.NewOpenAIProxy(cfg, newMockKeyProvider(), nil)

	body := map[string]interface{}{
		"model":  "gpt-4",
		"stream": true,
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(jsonBody))
	w := httptest.NewRecorder()

	p.HandleChatCompletions(w, req)

	// For streaming, we check the response contains SSE markers
	respBody := w.Body.String()
	if !strings.Contains(respBody, "data:") {
		t.Error("expected SSE data prefix in streaming response")
	}
}

func TestAnthropicProxy_WithMockUpstream_NonStreaming(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockResponse := `{
		"id": "msg_test123",
		"type": "message",
		"role": "assistant",
		"content": [{"type": "text", "text": "Hello! How can I help you?"}],
		"model": "claude-3-opus-20240229",
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 10, "output_tokens": 15}
	}`

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("x-api-key") != "sk-test-anthropic-key" {
			t.Errorf("unexpected x-api-key header: %s", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("expected anthropic-version header")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer upstream.Close()

	cfg := proxy.AnthropicProxyConfig{
		Timeout:             5 * time.Second,
		DefaultMaxTokens:    4096,
		AnthropicBaseURL:    upstream.URL,
		AnthropicAPIVersion: "2023-06-01",
	}
	p := proxy.NewAnthropicProxy(cfg, newMockKeyProvider(), nil)

	body := map[string]interface{}{
		"model":      "claude-3-opus-20240229",
		"max_tokens": 1024,
		"messages": []map[string]interface{}{
			{"role": "user", "content": []map[string]string{{"type": "text", "text": "Hello"}}},
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(jsonBody))
	w := httptest.NewRecorder()

	p.HandleMessages(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["id"] != "msg_test123" {
		t.Errorf("expected id msg_test123, got %v", resp["id"])
	}
}

// ====== Default Config Tests ======

func TestDefaultOpenAIProxyConfig(t *testing.T) {
	cfg := proxy.DefaultOpenAIProxyConfig()

	if cfg.Timeout != 5*time.Minute {
		t.Errorf("expected timeout 5m, got %v", cfg.Timeout)
	}

	if cfg.DefaultMaxTokens != 4096 {
		t.Errorf("expected default max tokens 4096, got %d", cfg.DefaultMaxTokens)
	}

	if cfg.OpenAIBaseURL != "https://api.openai.com" {
		t.Errorf("expected OpenAI base URL, got %s", cfg.OpenAIBaseURL)
	}
}

func TestDefaultAnthropicProxyConfig(t *testing.T) {
	cfg := proxy.DefaultAnthropicProxyConfig()

	if cfg.Timeout != 5*time.Minute {
		t.Errorf("expected timeout 5m, got %v", cfg.Timeout)
	}

	if cfg.DefaultMaxTokens != 4096 {
		t.Errorf("expected default max tokens 4096, got %d", cfg.DefaultMaxTokens)
	}

	if cfg.AnthropicBaseURL != "https://api.anthropic.com" {
		t.Errorf("expected Anthropic base URL, got %s", cfg.AnthropicBaseURL)
	}

	if cfg.AnthropicAPIVersion != "2023-06-01" {
		t.Errorf("expected API version 2023-06-01, got %s", cfg.AnthropicAPIVersion)
	}
}

// ====== NoOpRemapper Tests ======

func TestNoOpRemapper(t *testing.T) {
	remapper := &proxy.NoOpRemapper{}

	model, provider, err := remapper.RemapModel(context.Background(), "gpt-4", "client123")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", model)
	}

	if provider != "" {
		t.Errorf("expected empty provider, got %s", provider)
	}
}
