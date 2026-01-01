// Package proxy provides HTTP proxy functionality for forwarding requests
// to LLM providers with SSE streaming support.
package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// OpenAIProxyConfig holds configuration for the OpenAI proxy
type OpenAIProxyConfig struct {
	// Timeout for upstream requests
	Timeout time.Duration
	// DefaultMaxTokens if not specified in request
	DefaultMaxTokens int
	// OpenAIBaseURL is the upstream OpenAI API URL
	OpenAIBaseURL string
}

// DefaultOpenAIProxyConfig returns sensible defaults
func DefaultOpenAIProxyConfig() OpenAIProxyConfig {
	return OpenAIProxyConfig{
		Timeout:          5 * time.Minute,
		DefaultMaxTokens: 4096,
		OpenAIBaseURL:    "https://api.openai.com",
	}
}

// OpenAIProxy handles OpenAI Chat Completions API proxy requests
type OpenAIProxy struct {
	config          OpenAIProxyConfig
	keyProvider     KeyProvider
	remapper        ModelRemapper
	httpClient      *http.Client
	streamingClient *http.Client // Dedicated client for streaming (no timeout)
}

// NewOpenAIProxy creates a new OpenAI proxy handler
func NewOpenAIProxy(cfg OpenAIProxyConfig, keyProvider KeyProvider, remapper ModelRemapper) *OpenAIProxy {
	return &OpenAIProxy{
		config:      cfg,
		keyProvider: keyProvider,
		remapper:    remapper,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		streamingClient: &http.Client{
			Timeout: 0, // No timeout for streaming
		},
	}
}

// HandleChatCompletions handles POST /v1/chat/completions requests
// This is the main entry point for OpenAI Chat Completions API proxy
func (p *OpenAIProxy) HandleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Create context with timeout for the entire request
	ctx, cancel := context.WithTimeout(r.Context(), p.config.Timeout)
	defer cancel()

	// Parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		p.writeError(w, "failed to read request body", "invalid_request_error", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req OpenAIRequest
	if err := json.Unmarshal(body, &req); err != nil {
		p.writeError(w, fmt.Sprintf("invalid request body: %v", err), "invalid_request_error", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Model == "" {
		p.writeError(w, "model is required", "invalid_request_error", http.StatusBadRequest)
		return
	}
	if len(req.Messages) == 0 {
		p.writeError(w, "messages array is required", "invalid_request_error", http.StatusBadRequest)
		return
	}

	// Set default max_tokens if not provided
	if req.MaxTokens == nil && req.MaxCompletionTokens == nil {
		maxTokens := p.config.DefaultMaxTokens
		req.MaxTokens = &maxTokens
	}

	// Extract client ID from header (optional)
	clientID := r.Header.Get("X-Client-ID")

	// Apply model remapping if remapper is available
	targetProvider := "openai"
	if p.remapper != nil && clientID != "" {
		remapped, provider, err := p.remapper.RemapModel(ctx, req.Model, clientID)
		if err != nil {
			log.Printf("proxy: remap error for model %s: %v", req.Model, err)
			// Continue with original model on remap error
		} else if remapped != "" {
			log.Printf("proxy: remapped %s -> %s (provider: %s)", req.Model, remapped, provider)
			req.Model = remapped
			if provider != "" {
				targetProvider = provider
			}
		}
	}

	// Get API key for target provider
	apiKey, err := p.keyProvider.GetKey(ctx, targetProvider)
	if err != nil {
		p.writeError(w, fmt.Sprintf("no API key available for provider %s", targetProvider), "server_error", http.StatusServiceUnavailable)
		return
	}

	// Forward request to upstream
	if req.Stream {
		p.handleStreamingRequest(ctx, w, &req, apiKey, targetProvider)
	} else {
		p.handleNonStreamingRequest(ctx, w, &req, apiKey, targetProvider)
	}
}

// handleNonStreamingRequest handles non-streaming OpenAI requests
func (p *OpenAIProxy) handleNonStreamingRequest(ctx context.Context, w http.ResponseWriter, req *OpenAIRequest, apiKey, provider string) {
	// Build upstream request
	reqBody, err := json.Marshal(req)
	if err != nil {
		p.writeError(w, "failed to marshal request", "server_error", http.StatusInternalServerError)
		return
	}

	upstreamURL := p.getUpstreamURL(provider)
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(reqBody))
	if err != nil {
		p.writeError(w, "failed to create upstream request", "server_error", http.StatusInternalServerError)
		return
	}

	// Set headers
	p.setUpstreamHeaders(upstreamReq, apiKey, provider)

	// Execute request
	resp, err := p.httpClient.Do(upstreamReq)
	if err != nil {
		p.writeError(w, fmt.Sprintf("upstream request failed: %v", err), "server_error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Copy status code and body
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("proxy: error copying response body: %v", err)
		// Response already started, can't change status code
	}
}

// handleStreamingRequest handles SSE streaming OpenAI requests
func (p *OpenAIProxy) handleStreamingRequest(ctx context.Context, w http.ResponseWriter, req *OpenAIRequest, apiKey, provider string) {
	// Create stream writer
	sw, err := NewStreamWriter(w)
	if err != nil {
		p.writeError(w, "streaming not supported", "server_error", http.StatusInternalServerError)
		return
	}

	// Build upstream request
	reqBody, err := json.Marshal(req)
	if err != nil {
		sw.WriteError(fmt.Errorf("failed to marshal request: %w", err))
		return
	}

	upstreamURL := p.getUpstreamURL(provider)
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(reqBody))
	if err != nil {
		sw.WriteError(fmt.Errorf("failed to create upstream request: %w", err))
		return
	}

	// Set headers
	p.setUpstreamHeaders(upstreamReq, apiKey, provider)

	// Execute request with streaming client (no timeout)
	resp, err := p.streamingClient.Do(upstreamReq)
	if err != nil {
		sw.WriteError(fmt.Errorf("upstream request failed: %w", err))
		return
	}
	defer resp.Body.Close()

	// Check for non-2xx status
	if resp.StatusCode >= 400 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			sw.WriteError(fmt.Errorf("upstream error (status %d): failed to read error body: %w", resp.StatusCode, err))
			return
		}
		sw.WriteError(fmt.Errorf("upstream error (status %d): %s", resp.StatusCode, string(body)))
		return
	}

	// Stream SSE events from upstream to client
	p.streamSSEEvents(ctx, sw, resp.Body)
}

// streamSSEEvents reads SSE events from upstream and forwards to client
func (p *OpenAIProxy) streamSSEEvents(ctx context.Context, sw *StreamWriter, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	// Increase buffer size for large events (pre-allocate 64KB initial buffer)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB max

	var dataLines []string

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()

		if line == "" {
			// Empty line means end of event
			if len(dataLines) > 0 {
				data := strings.Join(dataLines, "\n")

				// Check for [DONE] marker
				if data == "[DONE]" {
					sw.Close()
					return
				}

				// Forward the event
				sw.WriteEvent([]byte(data))

				// Reset for next event
				dataLines = nil
			}
			continue
		}

		// Parse SSE line
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			if strings.HasPrefix(data, " ") {
				data = data[1:]
			}
			dataLines = append(dataLines, data)
		}
		// OpenAI doesn't use event: prefix, but ignore if present
		// Ignore other lines (comments starting with :, retry:, id:, etc.)
	}

	if err := scanner.Err(); err != nil {
		sw.WriteError(fmt.Errorf("stream read error: %w", err))
	}

	// Close stream
	sw.Close()
}

// getUpstreamURL returns the upstream URL for a provider
func (p *OpenAIProxy) getUpstreamURL(provider string) string {
	switch provider {
	case "openai":
		return p.config.OpenAIBaseURL + "/v1/chat/completions"
	case "groq":
		return "https://api.groq.com/openai/v1/chat/completions"
	case "together":
		return "https://api.together.xyz/v1/chat/completions"
	case "fireworks":
		return "https://api.fireworks.ai/inference/v1/chat/completions"
	case "deepseek":
		return "https://api.deepseek.com/v1/chat/completions"
	case "deepinfra":
		return "https://api.deepinfra.com/v1/openai/chat/completions"
	case "openrouter":
		return "https://openrouter.ai/api/v1/chat/completions"
	case "xai":
		return "https://api.x.ai/v1/chat/completions"
	case "perplexity":
		return "https://api.perplexity.ai/chat/completions"
	default:
		// Default to OpenAI
		return p.config.OpenAIBaseURL + "/v1/chat/completions"
	}
}

// setUpstreamHeaders sets the required headers for upstream requests
func (p *OpenAIProxy) setUpstreamHeaders(req *http.Request, apiKey, provider string) {
	req.Header.Set("Content-Type", "application/json")

	switch provider {
	case "openai", "groq", "together", "fireworks", "deepseek", "deepinfra", "openrouter", "xai", "perplexity":
		req.Header.Set("Authorization", "Bearer "+apiKey)
	default:
		// Default to Bearer token auth
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
}

// writeError writes an OpenAI-format error response
func (p *OpenAIProxy) writeError(w http.ResponseWriter, message, errType string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	errResp := map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
			"type":    errType,
			"code":    status,
		},
	}

	json.NewEncoder(w).Encode(errResp)
}
