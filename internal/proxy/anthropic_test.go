package proxy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockKeyProvider implements KeyProvider for testing
type mockKeyProvider struct {
	key string
	err error
}

func (m *mockKeyProvider) GetKey(ctx context.Context, providerID string) (string, error) {
	return m.key, m.err
}

// mockRemapper implements ModelRemapper for testing
type mockRemapper struct {
	model    string
	provider string
	err      error
}

func (m *mockRemapper) RemapModel(ctx context.Context, model string, clientID string) (string, string, error) {
	if m.model == "" {
		return model, "", nil
	}
	return m.model, m.provider, m.err
}

func TestAnthropicProxy_HandleMessages_MethodNotAllowed(t *testing.T) {
	proxy := NewAnthropicProxy(DefaultAnthropicProxyConfig(), &mockKeyProvider{key: "test-key"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/messages", nil)
	w := httptest.NewRecorder()

	proxy.HandleMessages(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestAnthropicProxy_HandleMessages_InvalidJSON(t *testing.T) {
	proxy := NewAnthropicProxy(DefaultAnthropicProxyConfig(), &mockKeyProvider{key: "test-key"}, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader("invalid json"))
	w := httptest.NewRecorder()

	proxy.HandleMessages(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["type"] != "error" {
		t.Errorf("expected error response, got %v", resp)
	}
}

func TestAnthropicProxy_HandleMessages_MissingModel(t *testing.T) {
	proxy := NewAnthropicProxy(DefaultAnthropicProxyConfig(), &mockKeyProvider{key: "test-key"}, nil)

	body := `{"messages": [{"role": "user", "content": [{"type": "text", "text": "hello"}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	w := httptest.NewRecorder()

	proxy.HandleMessages(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestAnthropicProxy_HandleMessages_MissingMessages(t *testing.T) {
	proxy := NewAnthropicProxy(DefaultAnthropicProxyConfig(), &mockKeyProvider{key: "test-key"}, nil)

	body := `{"model": "claude-3-opus-20240229"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	w := httptest.NewRecorder()

	proxy.HandleMessages(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestAnthropicProxy_HandleMessages_NoAPIKey(t *testing.T) {
	proxy := NewAnthropicProxy(
		DefaultAnthropicProxyConfig(),
		&mockKeyProvider{err: io.EOF},
		nil,
	)

	body := `{"model": "claude-3-opus-20240229", "max_tokens": 1024, "messages": [{"role": "user", "content": [{"type": "text", "text": "hello"}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	w := httptest.NewRecorder()

	proxy.HandleMessages(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
}

func TestAnthropicProxy_HandleMessages_UpstreamNonStreaming(t *testing.T) {
	// Create mock upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("x-api-key") != "test-api-key" {
			t.Errorf("expected x-api-key header")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Errorf("expected anthropic-version header")
		}

		// Return mock response
		resp := AnthropicResponse{
			ID:    "msg_123",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-3-opus-20240229",
			Content: []ContentPart{
				{Type: "text", Text: "Hello! How can I help you?"},
			},
			StopReason: "end_turn",
			Usage:      &Usage{InputTokens: 10, OutputTokens: 15},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	cfg := DefaultAnthropicProxyConfig()
	cfg.AnthropicBaseURL = upstream.URL

	proxy := NewAnthropicProxy(cfg, &mockKeyProvider{key: "test-api-key"}, nil)

	body := `{"model": "claude-3-opus-20240229", "max_tokens": 1024, "messages": [{"role": "user", "content": [{"type": "text", "text": "hello"}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	w := httptest.NewRecorder()

	proxy.HandleMessages(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp AnthropicResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID != "msg_123" {
		t.Errorf("expected ID msg_123, got %s", resp.ID)
	}
	if len(resp.Content) == 0 || resp.Content[0].Text != "Hello! How can I help you?" {
		t.Errorf("unexpected content: %v", resp.Content)
	}
}

func TestAnthropicProxy_HandleMessages_UpstreamStreaming(t *testing.T) {
	// Create mock upstream server with SSE
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request to check stream flag
		var req AnthropicRequest
		json.NewDecoder(r.Body).Decode(&req)
		if !req.Stream {
			t.Errorf("expected stream: true")
		}

		// Write SSE events
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		flusher, _ := w.(http.Flusher)

		// Message start event
		w.Write([]byte("event: message_start\n"))
		w.Write([]byte(`data: {"type": "message_start", "message": {"id": "msg_123", "type": "message", "role": "assistant", "model": "claude-3-opus-20240229"}}` + "\n\n"))
		flusher.Flush()

		// Content block start
		w.Write([]byte("event: content_block_start\n"))
		w.Write([]byte(`data: {"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}}` + "\n\n"))
		flusher.Flush()

		// Content delta
		w.Write([]byte("event: content_block_delta\n"))
		w.Write([]byte(`data: {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "Hello!"}}` + "\n\n"))
		flusher.Flush()

		// Message stop
		w.Write([]byte("event: message_stop\n"))
		w.Write([]byte(`data: {"type": "message_stop"}` + "\n\n"))
		flusher.Flush()

		// Done marker
		w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer upstream.Close()

	cfg := DefaultAnthropicProxyConfig()
	cfg.AnthropicBaseURL = upstream.URL

	proxy := NewAnthropicProxy(cfg, &mockKeyProvider{key: "test-api-key"}, nil)

	body := `{"model": "claude-3-opus-20240229", "max_tokens": 1024, "stream": true, "messages": [{"role": "user", "content": [{"type": "text", "text": "hello"}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	w := httptest.NewRecorder()

	proxy.HandleMessages(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check SSE headers
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", ct)
	}

	// Verify body contains expected events
	body_str := w.Body.String()
	if !strings.Contains(body_str, "message_start") {
		t.Error("expected message_start event in response")
	}
	if !strings.Contains(body_str, "Hello!") {
		t.Error("expected 'Hello!' in response")
	}
	if !strings.Contains(body_str, "[DONE]") {
		t.Error("expected [DONE] marker in response")
	}
}

func TestAnthropicProxy_HandleMessages_WithRemapper(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req AnthropicRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Verify model was remapped
		if req.Model != "claude-3-opus-20240229" {
			t.Errorf("expected remapped model claude-3-opus-20240229, got %s", req.Model)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AnthropicResponse{
			ID:   "msg_123",
			Type: "message",
		})
	}))
	defer upstream.Close()

	cfg := DefaultAnthropicProxyConfig()
	cfg.AnthropicBaseURL = upstream.URL

	proxy := NewAnthropicProxy(
		cfg,
		&mockKeyProvider{key: "test-key"},
		&mockRemapper{model: "claude-3-opus-20240229", provider: "anthropic"},
	)

	body := `{"model": "opus", "max_tokens": 1024, "messages": [{"role": "user", "content": [{"type": "text", "text": "hello"}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("X-Client-ID", "test-client")
	w := httptest.NewRecorder()

	proxy.HandleMessages(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestAnthropicProxy_DefaultMaxTokens(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req AnthropicRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Verify default max_tokens was set
		if req.MaxTokens != 4096 {
			t.Errorf("expected default max_tokens 4096, got %d", req.MaxTokens)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AnthropicResponse{ID: "msg_123", Type: "message"})
	}))
	defer upstream.Close()

	cfg := DefaultAnthropicProxyConfig()
	cfg.AnthropicBaseURL = upstream.URL

	proxy := NewAnthropicProxy(cfg, &mockKeyProvider{key: "test-key"}, nil)

	// Request without max_tokens
	body := `{"model": "claude-3-opus-20240229", "messages": [{"role": "user", "content": [{"type": "text", "text": "hello"}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	w := httptest.NewRecorder()

	proxy.HandleMessages(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestNoOpRemapper(t *testing.T) {
	r := &NoOpRemapper{}

	model, provider, err := r.RemapModel(context.Background(), "original-model", "client-123")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if model != "original-model" {
		t.Errorf("expected original-model, got %s", model)
	}
	if provider != "" {
		t.Errorf("expected empty provider, got %s", provider)
	}
}
