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

// AnthropicProvider implements the Provider interface for Anthropic Claude
type AnthropicProvider struct {
	apiKey    string
	baseURL   string
	client    *http.Client
	endpoints []Endpoint
}

// NewAnthropicProvider creates a new Anthropic provider instance
func NewAnthropicProvider(apiKey string) Provider {
	return &AnthropicProvider{
		apiKey:  apiKey,
		baseURL: "https://api.anthropic.com/v1",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func init() {
	RegisterProvider("anthropic", NewAnthropicProvider)
}

// anthropicModelsResponse represents the response from /v1/models endpoint
type anthropicModelsResponse struct {
	Data    []anthropicModelInfo `json:"data"`
	HasMore bool                 `json:"has_more"`
	FirstID string               `json:"first_id,omitempty"`
	LastID  string               `json:"last_id,omitempty"`
}

type anthropicModelInfo struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	Type        string    `json:"type"`
}

func (p *AnthropicProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
	endpoints := p.GetEndpoints()

	// Parallelize endpoint testing for better performance
	var wg sync.WaitGroup
	var mu sync.Mutex // Protect concurrent writes to endpoint status

	for i := range endpoints {
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
		}(&endpoints[i])
	}
	wg.Wait()

	p.endpoints = endpoints
	return nil
}

func (p *AnthropicProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("  Fetching available models from Anthropic API...")
	}

	// Call the /v1/models endpoint directly
	url := p.baseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var modelsResp anthropicModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Map to our Model structure with pricing and capabilities
	models := make([]Model, 0, len(modelsResp.Data))
	for _, apiModel := range modelsResp.Data {
		model := Model{
			ID:          apiModel.ID,
			Name:        apiModel.DisplayName,
			Description: fmt.Sprintf("Anthropic Claude model created at %s", apiModel.CreatedAt.Format("2006-01-02")),
			CreatedAt:   apiModel.CreatedAt.Format(time.RFC3339),
		}

		// Set pricing and capabilities based on model ID
		model = p.enrichModelDetails(model)
		models = append(models, model)
	}

	if verbose {
		fmt.Printf("  Found %d models\n", len(models))
	}

	return models, nil
}

// enrichModelDetails adds pricing, context window, and capability information
func (p *AnthropicProvider) enrichModelDetails(model Model) Model {
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
		model.Categories = []string{"chat", "reasoning", "premium"}

	case containsSubstring(model.ID, "sonnet-4"):
		model.CostPer1MIn = 3.00
		model.CostPer1MOut = 15.00
		model.ContextWindow = 200000
		model.MaxTokens = 64000
		model.Categories = []string{"chat", "reasoning", "balanced"}

	case containsSubstring(model.ID, "haiku-4"):
		model.CostPer1MIn = 1.00
		model.CostPer1MOut = 5.00
		model.ContextWindow = 200000
		model.MaxTokens = 64000
		model.Categories = []string{"chat", "fast", "cost-effective"}

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
		model.Categories = []string{"chat"}
	}

	// Add capabilities metadata
	model.Capabilities = map[string]string{
		"vision":            "high",
		"function_calling":  "full",
		"json_mode":         "supported",
		"streaming":         "supported",
		"extended_thinking": "supported",
	}

	return model
}

func (p *AnthropicProvider) GetCapabilities() ProviderCapabilities {
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
		SupportedParameters:  []string{"temperature", "max_tokens", "top_p", "top_k", "stop_sequences"},
		SecurityFeatures:     []string{"prompt_caching", "batch_api", "extended_thinking"},
		MaxRequestsPerMinute: 50,
		MaxTokensPerRequest:  200000,
	}
}

func (p *AnthropicProvider) GetEndpoints() []Endpoint {
	return []Endpoint{
		{
			Path:        "/v1/messages",
			Method:      "POST",
			Description: "Create a message (chat completion)",
			Headers: map[string]string{
				"x-api-key":         p.apiKey,
				"anthropic-version": "2023-06-01",
				"content-type":      "application/json",
			},
		},
		{
			Path:        "/v1/models",
			Method:      "GET",
			Description: "List available models",
			Headers: map[string]string{
				"x-api-key":         p.apiKey,
				"anthropic-version": "2023-06-01",
			},
		},
	}
}

func (p *AnthropicProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("  Testing model: %s\n", modelID)
	}

	// Create test request
	requestBody := map[string]interface{}{
		"model":      modelID,
		"max_tokens": 10,
		"messages": []map[string]string{
			{"role": "user", "content": "Say 'test successful' in 2 words"},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("model test failed with status %d: %s", resp.StatusCode, string(body))
	}

	if verbose {
		fmt.Printf("    ✓ Model is working\n")
	}

	return nil
}

func (p *AnthropicProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
	url := p.baseURL + endpoint.Path

	var req *http.Request
	var err error

	if endpoint.Method == "POST" {
		// Test messages endpoint with a minimal request
		body := `{
			"model": "claude-sonnet-4-5-20250929",
			"max_tokens": 10,
			"messages": [{"role": "user", "content": "Hi"}]
		}`
		req, err = http.NewRequestWithContext(ctx, endpoint.Method, url, bytes.NewBufferString(body))
	} else {
		req, err = http.NewRequestWithContext(ctx, endpoint.Method, url, nil)
	}

	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range endpoint.Headers {
		req.Header.Set(key, value)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Consider 2xx status codes as success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("endpoint returned status %d: %s", resp.StatusCode, string(body))
}
