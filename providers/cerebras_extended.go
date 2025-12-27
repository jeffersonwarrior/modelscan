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

// CerebrasExtendedProvider implements the Provider interface for Cerebras using internal HTTP client
type CerebrasExtendedProvider struct {
	apiKey    string
	baseURL   string
	client    *http.Client
	endpoints []Endpoint
}

// NewCerebrasExtendedProvider creates a new Cerebras Extended provider instance
func NewCerebrasExtendedProvider(apiKey string) Provider {
	return &CerebrasExtendedProvider{
		apiKey:  apiKey,
		baseURL: "https://api.cerebras.ai/v1",
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func init() {
	RegisterProvider("cerebras_extended", NewCerebrasExtendedProvider)
}

// Cerebras API response structures
type cerebrasModelsResponse struct {
	Data   []cerebrasModel `json:"data"`
	Object string          `json:"object"`
}

type cerebrasModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type cerebrasChatCompletionRequest struct {
	Model       string                `json:"model"`
	Messages    []cerebrasChatMessage `json:"messages"`
	MaxTokens   int                   `json:"max_tokens,omitempty"`
	Temperature float64               `json:"temperature,omitempty"`
	TopP        float64               `json:"top_p,omitempty"`
	Stream      bool                  `json:"stream,omitempty"`
}

type cerebrasChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type cerebrasChatCompletionResponse struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []cerebrasChatChoice `json:"choices"`
	Usage   cerebrasUsage        `json:"usage"`
}

type cerebrasChatChoice struct {
	Index        int                 `json:"index"`
	Message      cerebrasChatMessage `json:"message"`
	FinishReason string              `json:"finish_reason"`
}

type cerebrasUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (p *CerebrasExtendedProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
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

func (p *CerebrasExtendedProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
	url := p.baseURL + endpoint.Path

	var req *http.Request
	var err error

	if endpoint.Method == "POST" {
		// Create minimal test request for POST endpoints
		if strings.Contains(endpoint.Path, "/chat/completions") {
			reqBody := cerebrasChatCompletionRequest{
				Model:     "llama3.1-8b",
				MaxTokens: 5,
				Messages: []cerebrasChatMessage{
					{Role: "user", Content: "Hi"},
				},
			}
			body, _ := json.Marshal(reqBody)
			req, err = http.NewRequestWithContext(ctx, endpoint.Method, url, bytes.NewReader(body))
			if err == nil {
				req.Header.Set("Content-Type", "application/json")
			}
		} else {
			return fmt.Errorf("unknown endpoint: %s", endpoint.Path)
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

func (p *CerebrasExtendedProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("  Fetching available models from Cerebras API...")
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

	var modelsResp cerebrasModelsResponse
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
func (p *CerebrasExtendedProvider) formatModelName(modelID string) string {
	// Llama 3.3 models
	if strings.Contains(modelID, "3.3") {
		return "Llama 3.3: " + modelID
	}
	// Llama 3.1 models
	if strings.Contains(modelID, "3.1") {
		return "Llama 3.1: " + modelID
	}
	return modelID
}

// enrichModelDetails adds pricing, context window, and capability information
func (p *CerebrasExtendedProvider) enrichModelDetails(model Model) Model {
	// Set common capabilities
	model.SupportsTools = true
	model.CanStream = true

	// Determine specific details based on model ID
	switch {
	// Llama 3.3 70B models (latest, ultra-fast)
	case strings.Contains(model.ID, "3.3") && strings.Contains(model.ID, "70b"):
		model.CostPer1MIn = 0.60
		model.CostPer1MOut = 0.60
		model.ContextWindow = 8192
		model.MaxTokens = 8192
		model.SupportsImages = false
		model.CanReason = true
		model.Categories = []string{"chat", "ultra-fast", "reasoning"}
		model.Description = "Ultra-fast Llama 3.3 70B - 1800 tokens/sec"

	// Llama 3.1 70B models
	case strings.Contains(model.ID, "3.1") && strings.Contains(model.ID, "70b"):
		model.CostPer1MIn = 0.60
		model.CostPer1MOut = 0.60
		model.ContextWindow = 8192
		model.MaxTokens = 8192
		model.SupportsImages = false
		model.CanReason = true
		model.Categories = []string{"chat", "ultra-fast", "reasoning"}
		model.Description = "Ultra-fast Llama 3.1 70B - 1800 tokens/sec"

	// Llama 3.1 8B models
	case strings.Contains(model.ID, "3.1") && strings.Contains(model.ID, "8b"):
		model.CostPer1MIn = 0.10
		model.CostPer1MOut = 0.10
		model.ContextWindow = 8192
		model.MaxTokens = 8192
		model.SupportsImages = false
		model.CanReason = false
		model.Categories = []string{"chat", "ultra-fast", "cost-effective"}
		model.Description = "Ultra-fast Llama 3.1 8B - 1800 tokens/sec"

	default:
		// Default values for unknown models
		model.CostPer1MIn = 0.60
		model.CostPer1MOut = 0.60
		model.ContextWindow = 8192
		model.MaxTokens = 8192
		model.SupportsImages = false
		model.CanReason = false
		model.Categories = []string{"chat"}
	}

	// Add capabilities metadata
	model.Capabilities = map[string]string{
		"streaming": "supported",
		"json_mode": "supported",
		"speed":     "ultra-fast (1800 tokens/sec)",
	}

	return model
}

func (p *CerebrasExtendedProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsChat:         true,
		SupportsFIM:          false,
		SupportsEmbeddings:   false,
		SupportsFineTuning:   false,
		SupportsAgents:       true,
		SupportsFileUpload:   false,
		SupportsStreaming:    true,
		SupportsJSONMode:     true,
		SupportsVision:       false,
		SupportsAudio:        false,
		SupportedParameters:  []string{"temperature", "max_tokens", "top_p", "stop", "frequency_penalty", "presence_penalty"},
		SecurityFeatures:     []string{},
		MaxRequestsPerMinute: 60,
		MaxTokensPerRequest:  8192,
	}
}

func (p *CerebrasExtendedProvider) GetEndpoints() []Endpoint {
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

func (p *CerebrasExtendedProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("  Testing model: %s\n", modelID)
	}

	url := p.baseURL + "/chat/completions"
	reqBody := cerebrasChatCompletionRequest{
		Model:     modelID,
		MaxTokens: 10,
		Messages: []cerebrasChatMessage{
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

	var chatResp cerebrasChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if verbose && len(chatResp.Choices) > 0 {
		fmt.Printf("    Response: %s\n", chatResp.Choices[0].Message.Content)
		fmt.Printf("    ✓ Model is working\n")
	}

	return nil
}
