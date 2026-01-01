package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIProxy_HandleChatCompletions_MethodNotAllowed(t *testing.T) {
	proxy := NewOpenAIProxy(DefaultOpenAIProxyConfig(), &mockKeyProvider{key: "test-key"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	w := httptest.NewRecorder()

	proxy.HandleChatCompletions(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestOpenAIProxy_HandleChatCompletions_InvalidJSON(t *testing.T) {
	proxy := NewOpenAIProxy(DefaultOpenAIProxyConfig(), &mockKeyProvider{key: "test-key"}, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("invalid json"))
	w := httptest.NewRecorder()

	proxy.HandleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if _, ok := resp["error"]; !ok {
		t.Errorf("expected error response, got %v", resp)
	}
}

func TestOpenAIProxy_HandleChatCompletions_MissingModel(t *testing.T) {
	proxy := NewOpenAIProxy(DefaultOpenAIProxyConfig(), &mockKeyProvider{key: "test-key"}, nil)

	body := `{"messages": [{"role": "user", "content": "hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()

	proxy.HandleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestOpenAIProxy_HandleChatCompletions_MissingMessages(t *testing.T) {
	proxy := NewOpenAIProxy(DefaultOpenAIProxyConfig(), &mockKeyProvider{key: "test-key"}, nil)

	body := `{"model": "gpt-4"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()

	proxy.HandleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestOpenAIProxy_HandleChatCompletions_NoAPIKey(t *testing.T) {
	proxy := NewOpenAIProxy(
		DefaultOpenAIProxyConfig(),
		&mockKeyProvider{err: io.EOF},
		nil,
	)

	body := `{"model": "gpt-4", "messages": [{"role": "user", "content": "hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()

	proxy.HandleChatCompletions(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
}

func TestOpenAIProxy_HandleChatCompletions_UpstreamNonStreaming(t *testing.T) {
	// Create mock upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("expected Authorization Bearer header")
		}

		// Return mock response
		resp := OpenAIResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: 1677652288,
			Model:   "gpt-4",
			Choices: []OpenAIChoice{
				{
					Index: 0,
					Message: OpenAIMessage{
						Role:    "assistant",
						Content: "Hello! How can I help you?",
					},
					FinishReason: "stop",
				},
			},
			Usage: &OpenAIUsage{
				PromptTokens:     10,
				CompletionTokens: 15,
				TotalTokens:      25,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	cfg := DefaultOpenAIProxyConfig()
	cfg.OpenAIBaseURL = upstream.URL

	proxy := NewOpenAIProxy(cfg, &mockKeyProvider{key: "test-api-key"}, nil)

	body := `{"model": "gpt-4", "messages": [{"role": "user", "content": "hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()

	proxy.HandleChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp OpenAIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID != "chatcmpl-123" {
		t.Errorf("expected ID chatcmpl-123, got %s", resp.ID)
	}
	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content != "Hello! How can I help you?" {
		t.Errorf("unexpected content: %v", resp.Choices)
	}
}

func TestOpenAIProxy_HandleChatCompletions_UpstreamStreaming(t *testing.T) {
	// Create mock upstream server with SSE
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request to check stream flag
		var req OpenAIRequest
		json.NewDecoder(r.Body).Decode(&req)
		if !req.Stream {
			t.Errorf("expected stream: true")
		}

		// Write SSE events
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		flusher, _ := w.(http.Flusher)

		// First chunk with role
		w.Write([]byte(`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}` + "\n\n"))
		flusher.Flush()

		// Content chunks
		w.Write([]byte(`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}` + "\n\n"))
		flusher.Flush()

		w.Write([]byte(`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}` + "\n\n"))
		flusher.Flush()

		// Final chunk
		w.Write([]byte(`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n"))
		flusher.Flush()

		// Done marker
		w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer upstream.Close()

	cfg := DefaultOpenAIProxyConfig()
	cfg.OpenAIBaseURL = upstream.URL

	proxy := NewOpenAIProxy(cfg, &mockKeyProvider{key: "test-api-key"}, nil)

	body := `{"model": "gpt-4", "stream": true, "messages": [{"role": "user", "content": "hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()

	proxy.HandleChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check SSE headers
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", ct)
	}

	// Verify body contains expected events
	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, "chatcmpl-123") {
		t.Error("expected chatcmpl-123 in response")
	}
	if !strings.Contains(bodyStr, "Hello") {
		t.Error("expected 'Hello' in response")
	}
	if !strings.Contains(bodyStr, "[DONE]") {
		t.Error("expected [DONE] marker in response")
	}
}

func TestOpenAIProxy_HandleChatCompletions_WithRemapper(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req OpenAIRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Verify model was remapped
		if req.Model != "gpt-4o" {
			t.Errorf("expected remapped model gpt-4o, got %s", req.Model)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(OpenAIResponse{
			ID:     "chatcmpl-123",
			Object: "chat.completion",
			Choices: []OpenAIChoice{
				{Index: 0, Message: OpenAIMessage{Role: "assistant", Content: "ok"}},
			},
		})
	}))
	defer upstream.Close()

	cfg := DefaultOpenAIProxyConfig()
	cfg.OpenAIBaseURL = upstream.URL

	proxy := NewOpenAIProxy(
		cfg,
		&mockKeyProvider{key: "test-key"},
		&mockRemapper{model: "gpt-4o", provider: "openai"},
	)

	body := `{"model": "gpt4", "messages": [{"role": "user", "content": "hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("X-Client-ID", "test-client")
	w := httptest.NewRecorder()

	proxy.HandleChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestOpenAIProxy_DefaultMaxTokens(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req OpenAIRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Verify default max_tokens was set
		if req.MaxTokens == nil || *req.MaxTokens != 4096 {
			t.Errorf("expected default max_tokens 4096, got %v", req.MaxTokens)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(OpenAIResponse{
			ID:     "chatcmpl-123",
			Object: "chat.completion",
			Choices: []OpenAIChoice{
				{Index: 0, Message: OpenAIMessage{Role: "assistant", Content: "ok"}},
			},
		})
	}))
	defer upstream.Close()

	cfg := DefaultOpenAIProxyConfig()
	cfg.OpenAIBaseURL = upstream.URL

	proxy := NewOpenAIProxy(cfg, &mockKeyProvider{key: "test-key"}, nil)

	// Request without max_tokens
	body := `{"model": "gpt-4", "messages": [{"role": "user", "content": "hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()

	proxy.HandleChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestOpenAIProxy_GetUpstreamURL(t *testing.T) {
	cfg := DefaultOpenAIProxyConfig()
	proxy := NewOpenAIProxy(cfg, &mockKeyProvider{key: "test"}, nil)

	tests := []struct {
		provider string
		want     string
	}{
		{"openai", "https://api.openai.com/v1/chat/completions"},
		{"groq", "https://api.groq.com/openai/v1/chat/completions"},
		{"together", "https://api.together.xyz/v1/chat/completions"},
		{"fireworks", "https://api.fireworks.ai/inference/v1/chat/completions"},
		{"deepseek", "https://api.deepseek.com/v1/chat/completions"},
		{"deepinfra", "https://api.deepinfra.com/v1/openai/chat/completions"},
		{"openrouter", "https://openrouter.ai/api/v1/chat/completions"},
		{"xai", "https://api.x.ai/v1/chat/completions"},
		{"perplexity", "https://api.perplexity.ai/chat/completions"},
		{"unknown", "https://api.openai.com/v1/chat/completions"}, // defaults to OpenAI
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := proxy.getUpstreamURL(tt.provider)
			if got != tt.want {
				t.Errorf("getUpstreamURL(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestOpenAIProxy_UpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("upstream error"))
	}))
	defer upstream.Close()

	cfg := DefaultOpenAIProxyConfig()
	cfg.OpenAIBaseURL = upstream.URL

	proxy := NewOpenAIProxy(cfg, &mockKeyProvider{key: "test-key"}, nil)

	body := `{"model": "gpt-4", "messages": [{"role": "user", "content": "hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()

	proxy.HandleChatCompletions(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected status %d, got %d", http.StatusBadGateway, w.Code)
	}
}

func TestOpenAIProxy_StreamingUpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	}))
	defer upstream.Close()

	cfg := DefaultOpenAIProxyConfig()
	cfg.OpenAIBaseURL = upstream.URL

	proxy := NewOpenAIProxy(cfg, &mockKeyProvider{key: "test-key"}, nil)

	body := `{"model": "gpt-4", "stream": true, "messages": [{"role": "user", "content": "hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()

	proxy.HandleChatCompletions(w, req)

	// Should return SSE with error event
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d for streaming, got %d", http.StatusOK, w.Code)
	}

	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, "error") {
		t.Error("expected error event in streaming response")
	}
}

func TestOpenAIProxyConfig_Defaults(t *testing.T) {
	cfg := DefaultOpenAIProxyConfig()

	if cfg.Timeout == 0 {
		t.Error("expected non-zero timeout")
	}
	if cfg.DefaultMaxTokens == 0 {
		t.Error("expected non-zero default max tokens")
	}
	if cfg.OpenAIBaseURL == "" {
		t.Error("expected non-empty OpenAI base URL")
	}
}

func TestOpenAIProxy_WithMaxCompletionTokens(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req OpenAIRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Should respect existing max_completion_tokens
		if req.MaxCompletionTokens == nil || *req.MaxCompletionTokens != 2048 {
			t.Errorf("expected max_completion_tokens 2048, got %v", req.MaxCompletionTokens)
		}

		// max_tokens should remain nil
		if req.MaxTokens != nil {
			t.Errorf("expected max_tokens nil, got %v", req.MaxTokens)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(OpenAIResponse{
			ID:     "chatcmpl-123",
			Object: "chat.completion",
			Choices: []OpenAIChoice{
				{Index: 0, Message: OpenAIMessage{Role: "assistant", Content: "ok"}},
			},
		})
	}))
	defer upstream.Close()

	cfg := DefaultOpenAIProxyConfig()
	cfg.OpenAIBaseURL = upstream.URL

	proxy := NewOpenAIProxy(cfg, &mockKeyProvider{key: "test-key"}, nil)

	// Request with max_completion_tokens
	body := `{"model": "gpt-4", "max_completion_tokens": 2048, "messages": [{"role": "user", "content": "hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()

	proxy.HandleChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}
