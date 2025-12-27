package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// AnthropicExtendedProvider implements the Provider interface for Anthropic Claude with extended thinking
type AnthropicExtendedProvider struct {
	apiKey    string
	baseURL   string
	client    *http.Client
	endpoints []Endpoint
}

// NewAnthropicExtendedProvider creates a new Anthropic Extended provider instance
func NewAnthropicExtendedProvider(apiKey string) Provider {
	return &AnthropicExtendedProvider{
		apiKey:  apiKey,
		baseURL: "https://api.anthropic.com/v1",
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func init() {
	RegisterProvider("anthropic_extended", NewAnthropicExtendedProvider)
}

// Anthropic API response structures
type anthropicExtendedModelsResponse struct {
	Data    []anthropicExtendedModelInfo `json:"data"`
	HasMore bool                         `json:"has_more"`
	FirstID string                       `json:"first_id,omitempty"`
	LastID  string                       `json:"last_id,omitempty"`
}

type anthropicExtendedModelInfo struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	Type        string    `json:"type"`
}

type anthropicExtendedMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicExtendedRequest struct {
	Model          string                     `json:"model"`
	Messages       []anthropicExtendedMessage `json:"messages"`
	MaxTokens      int                        `json:"max_tokens"`
	Temperature    float64                    `json:"temperature,omitempty"`
	TopP           float64                    `json:"top_p,omitempty"`
	TopK           int                        `json:"top_k,omitempty"`
	ThinkingBudget int                        `json:"thinking_budget,omitempty"`
}

type anthropicExtendedResponse struct {
	ID           string                     `json:"id"`
	Type         string                     `json:"type"`
	Role         string                     `json:"role"`
	Content      []anthropicExtendedContent `json:"content"`
	Model        string                     `json:"model"`
	StopReason   string                     `json:"stop_reason"`
	StopSequence string                     `json:"stop_sequence,omitempty"`
	Usage        anthropicExtendedUsage     `json:"usage"`
}

type anthropicExtendedContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicExtendedUsage struct {
	InputTokens    int `json:"input_tokens"`
	OutputTokens   int `json:"output_tokens"`
	ThinkingTokens int `json:"thinking_tokens,omitempty"`
}

func (p *AnthropicExtendedProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
	// Initialize endpoints if not already set
	if p.endpoints == nil {
		p.endpoints = p.GetEndpoints()
	}

	// Parallelize endpoint testing for better performance
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := range p.endpoints {
		wg.Add(1)
		go func(endpoint *Endpoint) {
			defer wg.Done()

			if verbose {
				mu.Lock()
				fmt.Printf("  Testing endpoint: %s %s\n", endpoint.Method, endpoint.Path)
				mu.Unlock()
			}

			start := time.Now()
			err := p.testEndpoint(ctx, endpoint)
			latency := time.Since(start)

			mu.Lock()
			endpoint.Latency = latency
			if err != nil {
				endpoint.Status = StatusFailed
				endpoint.Error = err.Error()
				if verbose {
					fmt.Printf("    ✗ Failed: %v\n", err)
				}
			} else {
				endpoint.Status = StatusWorking
				if verbose {
					fmt.Printf("    ✓ Working (%v)\n", latency)
				}
			}
			mu.Unlock()
		}(&p.endpoints[i])
	}
	wg.Wait()

	return nil
}

func (p *AnthropicExtendedProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
	url := p.baseURL + endpoint.Path

	var req *http.Request
	var err error

	if endpoint.Method == "POST" {
		// Test messages endpoint with a minimal request
		reqBody := anthropicExtendedRequest{
			Model:     "claude-sonnet-4-5-20250929",
			MaxTokens: 10,
			Messages: []anthropicExtendedMessage{
				{Role: "user", Content: "Hi"},
			},
		}
		body, _ := json.Marshal(reqBody)
		req, err = http.NewRequestWithContext(ctx, endpoint.Method, url, bytes.NewReader(body))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
		}
	} else {
		req, err = http.NewRequestWithContext(ctx, endpoint.Method, url, nil)
	}

	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return nil
}

func (p *AnthropicExtendedProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("  Fetching available models from Anthropic API...")
	}

	url := p.baseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var modelsResp anthropicExtendedModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]Model, 0, len(modelsResp.Data))
	for _, apiModel := range modelsResp.Data {
		model := Model{
			ID:          apiModel.ID,
			Name:        apiModel.DisplayName,
			Description: fmt.Sprintf("Anthropic Claude model created at %s", apiModel.CreatedAt.Format("2006-01-02")),
			CreatedAt:   apiModel.CreatedAt.Format(time.RFC3339),
		}

		// Enrich with pricing and capabilities
		model = p.enrichModelDetails(model)
		models = append(models, model)
	}

	if verbose {
		fmt.Printf("  Found %d models\n", len(models))
	}

	return models, nil
}

// enrichModelDetails adds pricing, context window, and capability information
func (p *AnthropicExtendedProvider) enrichModelDetails(model Model) Model {
	// Set common capabilities for all Claude models
	model.SupportsImages = true
	model.SupportsTools = true
	model.CanStream = true
	model.CanReason = true

	// Determine specific details based on model ID
	switch {
	case containsSubstring(model.ID, "opus-4"):
		model.CostPer1MIn = 5.00
		model.CostPer1MOut = 25.00
		model.ContextWindow = 200000
		model.MaxTokens = 64000
		model.Categories = []string{"chat", "reasoning", "premium", "extended-thinking"}

	case containsSubstring(model.ID, "sonnet-4"):
		model.CostPer1MIn = 3.00
		model.CostPer1MOut = 15.00
		model.ContextWindow = 200000
		model.MaxTokens = 64000
		model.Categories = []string{"chat", "reasoning", "balanced", "extended-thinking"}

	case containsSubstring(model.ID, "haiku-4"):
		model.CostPer1MIn = 1.00
		model.CostPer1MOut = 5.00
		model.ContextWindow = 200000
		model.MaxTokens = 64000
		model.Categories = []string{"chat", "fast", "cost-effective", "extended-thinking"}

	case containsSubstring(model.ID, "opus-3.5"):
		model.CostPer1MIn = 15.00
		model.CostPer1MOut = 75.00
		model.ContextWindow = 200000
		model.MaxTokens = 4096
		model.Categories = []string{"chat", "premium", "legacy"}

	case containsSubstring(model.ID, "sonnet-3.5"):
		model.CostPer1MIn = 3.00
		model.CostPer1MOut = 15.00
		model.ContextWindow = 200000
		model.MaxTokens = 8192
		model.Categories = []string{"chat", "balanced", "legacy"}

	case containsSubstring(model.ID, "haiku-3.5"):
		model.CostPer1MIn = 0.80
		model.CostPer1MOut = 4.00
		model.ContextWindow = 200000
		model.MaxTokens = 4096
		model.Categories = []string{"chat", "fast", "legacy"}

	default:
		// Default values for unknown models
		model.CostPer1MIn = 3.00
		model.CostPer1MOut = 15.00
		model.ContextWindow = 200000
		model.MaxTokens = 4096
		model.Categories = []string{"chat", "extended-thinking"}
	}

	// Add capabilities metadata with extended thinking support
	model.Capabilities = map[string]string{
		"vision":            "high",
		"function_calling":  "full",
		"json_mode":         "supported",
		"streaming":         "supported",
		"extended_thinking": "supported",
		"thinking_budget":   "configurable",
	}

	return model
}

func (p *AnthropicExtendedProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsChat:         true,
		SupportsFIM:          false,
		SupportsEmbeddings:   false,
		SupportsFineTuning:   false,
		SupportsAgents:       true,
		SupportsFileUpload:   true,
		SupportsStreaming:    true,
		SupportsJSONMode:     true,
		SupportsVision:       true,
		SupportsAudio:        false,
		SupportedParameters:  []string{"temperature", "max_tokens", "top_p", "top_k", "stop_sequences", "thinking_budget"},
		SecurityFeatures:     []string{"prompt_caching", "batch_api", "extended_thinking", "thinking_budget"},
		MaxRequestsPerMinute: 50,
		MaxTokensPerRequest:  200000,
	}
}

func (p *AnthropicExtendedProvider) GetEndpoints() []Endpoint {
	if p.endpoints != nil {
		return p.endpoints
	}

	return []Endpoint{
		{
			Path:        "/messages",
			Method:      "POST",
			Description: "Create a message with optional extended thinking",
		},
		{
			Path:        "/models",
			Method:      "GET",
			Description: "List available models",
		},
	}
}

func (p *AnthropicExtendedProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("  Testing model: %s\n", modelID)
	}

	url := p.baseURL + "/messages"
	reqBody := anthropicExtendedRequest{
		Model:     modelID,
		MaxTokens: 10,
		Messages: []anthropicExtendedMessage{
			{Role: "user", Content: "Say 'test successful' in 2 words"},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var chatResp anthropicExtendedResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if verbose && len(chatResp.Content) > 0 {
		fmt.Printf("    Response: %s\n", chatResp.Content[0].Text)
		if chatResp.Usage.ThinkingTokens > 0 {
			fmt.Printf("    Thinking tokens: %d\n", chatResp.Usage.ThinkingTokens)
		}
		fmt.Printf("    ✓ Model is working\n")
	}

	return nil
}
