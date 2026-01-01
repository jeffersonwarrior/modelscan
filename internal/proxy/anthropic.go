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

// AnthropicProxyConfig holds configuration for the Anthropic proxy
type AnthropicProxyConfig struct {
	// Timeout for upstream requests
	Timeout time.Duration
	// DefaultMaxTokens if not specified in request
	DefaultMaxTokens int
	// AnthropicBaseURL is the upstream Anthropic API URL
	AnthropicBaseURL string
	// AnthropicAPIVersion is the API version header value
	AnthropicAPIVersion string
}

// DefaultAnthropicProxyConfig returns sensible defaults
func DefaultAnthropicProxyConfig() AnthropicProxyConfig {
	return AnthropicProxyConfig{
		Timeout:             5 * time.Minute,
		DefaultMaxTokens:    4096,
		AnthropicBaseURL:    "https://api.anthropic.com",
		AnthropicAPIVersion: "2023-06-01",
	}
}

// KeyProvider interface for getting API keys
type KeyProvider interface {
	GetKey(ctx context.Context, providerID string) (string, error)
}

// ModelRemapper interface for model remapping
type ModelRemapper interface {
	RemapModel(ctx context.Context, model string, clientID string) (remappedModel, targetProvider string, err error)
}

// AnthropicProxy handles Anthropic Messages API proxy requests
type AnthropicProxy struct {
	config          AnthropicProxyConfig
	keyProvider     KeyProvider
	remapper        ModelRemapper
	httpClient      *http.Client
	streamingClient *http.Client // Dedicated client for streaming (no timeout)
}

// NewAnthropicProxy creates a new Anthropic proxy handler
func NewAnthropicProxy(cfg AnthropicProxyConfig, keyProvider KeyProvider, remapper ModelRemapper) *AnthropicProxy {
	return &AnthropicProxy{
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

// HandleMessages handles POST /v1/messages requests
// This is the main entry point for Anthropic Messages API proxy
func (p *AnthropicProxy) HandleMessages(w http.ResponseWriter, r *http.Request) {
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
		p.writeError(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req AnthropicRequest
	if err := json.Unmarshal(body, &req); err != nil {
		p.writeError(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Model == "" {
		p.writeError(w, "model is required", http.StatusBadRequest)
		return
	}
	if len(req.Messages) == 0 {
		p.writeError(w, "messages array is required", http.StatusBadRequest)
		return
	}

	// Set default max_tokens if not provided
	if req.MaxTokens == 0 {
		req.MaxTokens = p.config.DefaultMaxTokens
	}

	// Extract client ID from header (optional)
	clientID := r.Header.Get("X-Client-ID")

	// Apply model remapping if remapper is available
	targetProvider := "anthropic"
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
		p.writeError(w, fmt.Sprintf("no API key available for provider %s", targetProvider), http.StatusServiceUnavailable)
		return
	}

	// Forward request to upstream
	if req.Stream {
		p.handleStreamingRequest(ctx, w, &req, apiKey, targetProvider)
	} else {
		p.handleNonStreamingRequest(ctx, w, &req, apiKey, targetProvider)
	}
}

// handleNonStreamingRequest handles non-streaming Anthropic requests
func (p *AnthropicProxy) handleNonStreamingRequest(ctx context.Context, w http.ResponseWriter, req *AnthropicRequest, apiKey, provider string) {
	// Build upstream request
	reqBody, err := json.Marshal(req)
	if err != nil {
		p.writeError(w, "failed to marshal request", http.StatusInternalServerError)
		return
	}

	upstreamURL := p.getUpstreamURL(provider)
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(reqBody))
	if err != nil {
		p.writeError(w, "failed to create upstream request", http.StatusInternalServerError)
		return
	}

	// Set headers
	p.setUpstreamHeaders(upstreamReq, apiKey, provider)

	// Execute request
	resp, err := p.httpClient.Do(upstreamReq)
	if err != nil {
		p.writeError(w, fmt.Sprintf("upstream request failed: %v", err), http.StatusBadGateway)
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

// handleStreamingRequest handles SSE streaming Anthropic requests
func (p *AnthropicProxy) handleStreamingRequest(ctx context.Context, w http.ResponseWriter, req *AnthropicRequest, apiKey, provider string) {
	// Create stream writer
	sw, err := NewStreamWriter(w)
	if err != nil {
		p.writeError(w, "streaming not supported", http.StatusInternalServerError)
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
func (p *AnthropicProxy) streamSSEEvents(ctx context.Context, sw *StreamWriter, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	// Increase buffer size for large events (pre-allocate 64KB initial buffer)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB max

	var eventType string
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
				if eventType != "" {
					sw.WriteEventWithType(eventType, []byte(data))
				} else {
					sw.WriteEvent([]byte(data))
				}

				// Reset for next event
				eventType = ""
				dataLines = nil
			}
			continue
		}

		// Parse SSE line
		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			if strings.HasPrefix(data, " ") {
				data = data[1:]
			}
			dataLines = append(dataLines, data)
		}
		// Ignore other lines (comments starting with :, retry:, id:, etc.)
	}

	if err := scanner.Err(); err != nil {
		sw.WriteError(fmt.Errorf("stream read error: %w", err))
	}

	// Close stream
	sw.Close()
}

// getUpstreamURL returns the upstream URL for a provider
func (p *AnthropicProxy) getUpstreamURL(provider string) string {
	switch provider {
	case "anthropic":
		return p.config.AnthropicBaseURL + "/v1/messages"
	default:
		// For other providers, we'd need additional configuration
		// For now, default to Anthropic
		return p.config.AnthropicBaseURL + "/v1/messages"
	}
}

// setUpstreamHeaders sets the required headers for upstream requests
func (p *AnthropicProxy) setUpstreamHeaders(req *http.Request, apiKey, provider string) {
	req.Header.Set("Content-Type", "application/json")

	switch provider {
	case "anthropic":
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", p.config.AnthropicAPIVersion)
	default:
		// Default to Anthropic-style headers
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", p.config.AnthropicAPIVersion)
	}
}

// writeError writes an Anthropic-format error response
func (p *AnthropicProxy) writeError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	errResp := map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":    "api_error",
			"message": message,
		},
	}

	json.NewEncoder(w).Encode(errResp)
}

// NoOpRemapper is a remapper that performs no remapping
type NoOpRemapper struct{}

// RemapModel returns the original model unchanged
func (r *NoOpRemapper) RemapModel(ctx context.Context, model string, clientID string) (string, string, error) {
	return model, "", nil
}
