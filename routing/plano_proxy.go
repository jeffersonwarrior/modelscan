package routing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PlanoProxyRouter routes requests through an external Plano proxy
type PlanoProxyRouter struct {
	config     *ProxyConfig
	httpClient *http.Client
	fallback   Router
}

// NewPlanoProxyRouter creates a new Plano proxy router
func NewPlanoProxyRouter(config *ProxyConfig) (*PlanoProxyRouter, error) {
	if config == nil {
		return nil, fmt.Errorf("proxy config is required")
	}

	if config.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	timeout := 30 * time.Second
	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}

	return &PlanoProxyRouter{
		config: config,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// SetFallback sets a fallback router
func (r *PlanoProxyRouter) SetFallback(fallback Router) {
	r.fallback = fallback
}

// Route sends the request through the Plano proxy
func (r *PlanoProxyRouter) Route(ctx context.Context, req Request) (*Response, error) {
	start := time.Now()

	// Convert to OpenAI-compatible format
	planoReq := r.convertToPlanoRequest(req)

	// Marshal request
	reqBody, err := json.Marshal(planoReq)
	if err != nil {
		if r.fallback != nil {
			return r.fallback.Route(ctx, req)
		}
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/v1/chat/completions", r.config.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		if r.fallback != nil {
			return r.fallback.Route(ctx, req)
		}
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if r.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.config.APIKey)
	}

	// Send request
	httpResp, err := r.httpClient.Do(httpReq)
	if err != nil {
		if r.fallback != nil {
			return r.fallback.Route(ctx, req)
		}
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		if r.fallback != nil {
			return r.fallback.Route(ctx, req)
		}
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if httpResp.StatusCode != http.StatusOK {
		if r.fallback != nil {
			return r.fallback.Route(ctx, req)
		}
		return nil, fmt.Errorf("plano returned status %d: %s", httpResp.StatusCode, string(respBody))
	}

	// Parse response
	var planoResp planoResponse
	if err := json.Unmarshal(respBody, &planoResp); err != nil {
		if r.fallback != nil {
			return r.fallback.Route(ctx, req)
		}
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Convert to standard response
	resp := r.convertFromPlanoResponse(planoResp)
	resp.Latency = time.Since(start)
	resp.Provider = "plano"

	return resp, nil
}

// Close closes the HTTP client
func (r *PlanoProxyRouter) Close() error {
	r.httpClient.CloseIdleConnections()
	return nil
}

// planoRequest is the OpenAI-compatible request format
type planoRequest struct {
	Model       string         `json:"model"`
	Messages    []planoMessage `json:"messages"`
	Temperature *float64       `json:"temperature,omitempty"`
	MaxTokens   *int           `json:"max_tokens,omitempty"`
	Stream      bool           `json:"stream,omitempty"`
}

type planoMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// planoResponse is the OpenAI-compatible response format
type planoResponse struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []planoChoice `json:"choices"`
	Usage   planoUsage    `json:"usage"`
}

type planoChoice struct {
	Index        int          `json:"index"`
	Message      planoMessage `json:"message"`
	FinishReason string       `json:"finish_reason"`
}

type planoUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// convertToPlanoRequest converts our request format to Plano's OpenAI-compatible format
func (r *PlanoProxyRouter) convertToPlanoRequest(req Request) planoRequest {
	planoReq := planoRequest{
		Model:    req.Model,
		Messages: make([]planoMessage, len(req.Messages)),
		Stream:   req.Stream,
	}

	// Use "none" for automatic routing in Plano
	if planoReq.Model == "" {
		planoReq.Model = "none"
	}

	for i, msg := range req.Messages {
		planoReq.Messages[i] = planoMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	if req.Temperature != 0 {
		temp := req.Temperature
		planoReq.Temperature = &temp
	}

	if req.MaxTokens > 0 {
		tokens := req.MaxTokens
		planoReq.MaxTokens = &tokens
	}

	return planoReq
}

// convertFromPlanoResponse converts Plano's response to our standard format
func (r *PlanoProxyRouter) convertFromPlanoResponse(planoResp planoResponse) *Response {
	resp := &Response{
		Model: planoResp.Model,
		Usage: Usage{
			PromptTokens:     planoResp.Usage.PromptTokens,
			CompletionTokens: planoResp.Usage.CompletionTokens,
			TotalTokens:      planoResp.Usage.TotalTokens,
		},
		Metadata: map[string]interface{}{
			"id":      planoResp.ID,
			"object":  planoResp.Object,
			"created": planoResp.Created,
		},
	}

	if len(planoResp.Choices) > 0 {
		choice := planoResp.Choices[0]
		resp.Content = choice.Message.Content
		resp.FinishReason = choice.FinishReason
	}

	return resp
}
