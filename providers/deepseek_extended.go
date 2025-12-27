package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DeepSeekExtendedProvider implements the Provider interface for DeepSeek using internal HTTP client
type DeepSeekExtendedProvider struct {
	apiKey    string
	baseURL   string
	client    *http.Client
	endpoints []Endpoint
}

// NewDeepSeekExtendedProvider creates a new DeepSeek Extended provider instance
func NewDeepSeekExtendedProvider(apiKey string) Provider {
	return &DeepSeekExtendedProvider{
		apiKey:  apiKey,
		baseURL: "https://api.deepseek.com/v1",
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func init() {
	RegisterProvider("deepseek_extended", NewDeepSeekExtendedProvider)
}

// DeepSeek API response structures
type deepseekModelsResponse struct {
	Data   []deepseekModel `json:"data"`
	Object string          `json:"object"`
}

type deepseekModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type deepseekChatCompletionRequest struct {
	Model       string                `json:"model"`
	Messages    []deepseekChatMessage `json:"messages"`
	MaxTokens   int                   `json:"max_tokens,omitempty"`
	Temperature float64               `json:"temperature,omitempty"`
	TopP        float64               `json:"top_p,omitempty"`
	Stream      bool                  `json:"stream,omitempty"`
}

type deepseekChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type deepseekChatCompletionResponse struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []deepseekChatChoice `json:"choices"`
	Usage   deepseekUsage        `json:"usage"`
}

type deepseekChatChoice struct {
	Index        int                 `json:"index"`
	Message      deepseekChatMessage `json:"message"`
	FinishReason string              `json:"finish_reason"`
}

type deepseekUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (p *DeepSeekExtendedProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
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

func (p *DeepSeekExtendedProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
	url := p.baseURL + endpoint.Path

	var req *http.Request
	var err error

	if endpoint.Method == "POST" {
		// Create minimal test request for POST endpoints
		if strings.Contains(endpoint.Path, "/chat/completions") {
			reqBody := deepseekChatCompletionRequest{
				Model:     "deepseek-chat",
				MaxTokens: 5,
				Messages: []deepseekChatMessage{
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
	} else {
		req, err = http.NewRequestWithContext(ctx, endpoint.Method, url, nil)
	}

	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)

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

func (p *DeepSeekExtendedProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("  Fetching available models from DeepSeek API...")
	}

	url := p.baseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var modelsResp deepseekModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]Model, 0, len(modelsResp.Data))
	for _, apiModel := range modelsResp.Data {
		model := Model{
			ID:        apiModel.ID,
			Name:      p.formatModelName(apiModel.ID),
			CreatedAt: time.Unix(apiModel.Created, 0).Format(time.RFC3339),
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

// formatModelName creates a human-readable name from model ID
func (p *DeepSeekExtendedProvider) formatModelName(modelID string) string {
	switch {
	case strings.HasPrefix(modelID, "deepseek-chat"):
		return "DeepSeek Chat: " + modelID
	case strings.HasPrefix(modelID, "deepseek-reasoner"):
		return "DeepSeek Reasoner: " + modelID
	case strings.HasPrefix(modelID, "deepseek-coder"):
		return "DeepSeek Coder: " + modelID
	default:
		return modelID
	}
}

// enrichModelDetails adds pricing, context window, and capability information
func (p *DeepSeekExtendedProvider) enrichModelDetails(model Model) Model {
	// Set common capabilities
	model.SupportsTools = true
	model.CanStream = true

	// Determine specific details based on model ID
	switch {
	case strings.HasPrefix(model.ID, "deepseek-chat"):
		model.CostPer1MIn = 0.27
		model.CostPer1MOut = 1.10
		model.ContextWindow = 64000
		model.MaxTokens = 8192
		model.SupportsImages = false
		model.CanReason = false
		model.Categories = []string{"chat", "cost-effective"}

	case strings.HasPrefix(model.ID, "deepseek-reasoner"):
		model.CostPer1MIn = 0.55
		model.CostPer1MOut = 2.19
		model.ContextWindow = 64000
		model.MaxTokens = 8192
		model.SupportsImages = false
		model.CanReason = true
		model.Categories = []string{"reasoning", "problem-solving"}

	case strings.HasPrefix(model.ID, "deepseek-coder"):
		model.CostPer1MIn = 0.27
		model.CostPer1MOut = 1.10
		model.ContextWindow = 64000
		model.MaxTokens = 8192
		model.SupportsImages = false
		model.CanReason = false
		model.Categories = []string{"coding", "cost-effective"}

	default:
		// Default values for unknown models
		model.CostPer1MIn = 0.27
		model.CostPer1MOut = 1.10
		model.ContextWindow = 64000
		model.MaxTokens = 8192
		model.SupportsImages = false
		model.CanReason = false
		model.Categories = []string{"chat"}
	}

	// Add capabilities metadata
	model.Capabilities = map[string]string{
		"function_calling": "full",
		"json_mode":        "supported",
		"streaming":        "supported",
	}

	if model.CanReason {
		model.Capabilities["reasoning"] = "advanced"
	}

	return model
}

func (p *DeepSeekExtendedProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsChat:         true,
		SupportsFIM:          true,
		SupportsEmbeddings:   false,
		SupportsFineTuning:   false,
		SupportsAgents:       true,
		SupportsFileUpload:   false,
		SupportsStreaming:    true,
		SupportsJSONMode:     true,
		SupportsVision:       false,
		SupportsAudio:        false,
		SupportedParameters:  []string{"temperature", "max_tokens", "top_p", "frequency_penalty", "presence_penalty", "stop"},
		SecurityFeatures:     []string{},
		MaxRequestsPerMinute: 100,
		MaxTokensPerRequest:  64000,
	}
}

func (p *DeepSeekExtendedProvider) GetEndpoints() []Endpoint {
	if p.endpoints != nil {
		return p.endpoints
	}

	return []Endpoint{
		{
			Path:        "/chat/completions",
			Method:      "POST",
			Description: "Create a chat completion",
		},
		{
			Path:        "/models",
			Method:      "GET",
			Description: "List available models",
		},
	}
}

func (p *DeepSeekExtendedProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("  Testing model: %s\n", modelID)
	}

	url := p.baseURL + "/chat/completions"
	reqBody := deepseekChatCompletionRequest{
		Model:     modelID,
		MaxTokens: 10,
		Messages: []deepseekChatMessage{
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

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
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

	var chatResp deepseekChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if verbose && len(chatResp.Choices) > 0 {
		fmt.Printf("    Response: %s\n", chatResp.Choices[0].Message.Content)
		fmt.Printf("    ✓ Model is working\n")
	}

	return nil
}
