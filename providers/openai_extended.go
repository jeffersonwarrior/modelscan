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

// OpenAIExtendedProvider implements the Provider interface for OpenAI using internal HTTP client
type OpenAIExtendedProvider struct {
	apiKey    string
	baseURL   string
	client    *http.Client
	endpoints []Endpoint
}

// NewOpenAIExtendedProvider creates a new OpenAI Extended provider instance
func NewOpenAIExtendedProvider(apiKey string) Provider {
	return &OpenAIExtendedProvider{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1",
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func init() {
	RegisterProvider("openai_extended", NewOpenAIExtendedProvider)
}

// OpenAI API response structures
type openaiModelsResponse struct {
	Data   []openaiModel `json:"data"`
	Object string        `json:"object"`
}

type openaiModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type openaiChatCompletionRequest struct {
	Model       string              `json:"model"`
	Messages    []openaiChatMessage `json:"messages"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
	TopP        float64             `json:"top_p,omitempty"`
	Stream      bool                `json:"stream,omitempty"`
}

type openaiChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiChatCompletionResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []openaiChatChoice `json:"choices"`
	Usage   openaiUsage        `json:"usage"`
}

type openaiChatChoice struct {
	Index        int               `json:"index"`
	Message      openaiChatMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (p *OpenAIExtendedProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
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

func (p *OpenAIExtendedProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
	url := p.baseURL + endpoint.Path

	var req *http.Request
	var err error

	if endpoint.Method == "POST" {
		// Create minimal test request for POST endpoints
		if strings.Contains(endpoint.Path, "/chat/completions") {
			reqBody := openaiChatCompletionRequest{
				Model:     "gpt-3.5-turbo",
				MaxTokens: 5,
				Messages: []openaiChatMessage{
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

func (p *OpenAIExtendedProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("  Fetching available models from OpenAI API...")
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

	var modelsResp openaiModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]Model, 0, len(modelsResp.Data))
	for _, apiModel := range modelsResp.Data {
		// Only include models we can actually use (skip embeddings, whisper, tts, etc.)
		if !p.isUsableModel(apiModel.ID) {
			continue
		}

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
		fmt.Printf("  Found %d usable models\n", len(models))
	}

	return models, nil
}

// isUsableModel filters out non-chat models
func (p *OpenAIExtendedProvider) isUsableModel(modelID string) bool {
	// Skip embedding, whisper, TTS, moderation, and DALL-E models
	skipPrefixes := []string{
		"text-embedding", "embedding",
		"whisper", "tts",
		"text-moderation",
		"dall-e", "davinci-edit", "babbage-edit",
	}

	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(modelID, prefix) {
			return false
		}
	}

	// Skip old completion models that are deprecated
	if strings.Contains(modelID, "davinci") && !strings.Contains(modelID, "gpt") {
		return false
	}
	if strings.Contains(modelID, "curie") || strings.Contains(modelID, "babbage") || strings.Contains(modelID, "ada") {
		return false
	}

	return true
}

// formatModelName creates a human-readable name from model ID
func (p *OpenAIExtendedProvider) formatModelName(modelID string) string {
	// GPT-4 models
	if strings.HasPrefix(modelID, "gpt-4o") {
		return "GPT-4 Omni: " + modelID
	}
	if strings.HasPrefix(modelID, "gpt-4-turbo") {
		return "GPT-4 Turbo: " + modelID
	}
	if strings.HasPrefix(modelID, "gpt-4") {
		return "GPT-4: " + modelID
	}

	// GPT-3.5 models
	if strings.HasPrefix(modelID, "gpt-3.5") {
		return "GPT-3.5: " + modelID
	}

	// O-series models
	if strings.HasPrefix(modelID, "o1") || strings.HasPrefix(modelID, "o3") {
		return "O-Series Reasoning: " + modelID
	}

	return modelID
}

// enrichModelDetails adds pricing, context window, and capability information
func (p *OpenAIExtendedProvider) enrichModelDetails(model Model) Model {
	// Set common capabilities
	model.SupportsTools = true
	model.CanStream = true

	// Determine specific details based on model ID
	switch {
	// GPT-4o models (latest, multimodal)
	case strings.HasPrefix(model.ID, "gpt-4o-mini"):
		model.CostPer1MIn = 0.15
		model.CostPer1MOut = 0.60
		model.ContextWindow = 128000
		model.MaxTokens = 16384
		model.SupportsImages = true
		model.CanReason = false
		model.Categories = []string{"chat", "fast", "cost-effective", "vision"}

	case strings.HasPrefix(model.ID, "gpt-4o"):
		model.CostPer1MIn = 2.50
		model.CostPer1MOut = 10.00
		model.ContextWindow = 128000
		model.MaxTokens = 16384
		model.SupportsImages = true
		model.CanReason = true
		model.Categories = []string{"chat", "multimodal", "vision", "premium"}

	// GPT-4 Turbo models
	case strings.HasPrefix(model.ID, "gpt-4-turbo"):
		model.CostPer1MIn = 10.00
		model.CostPer1MOut = 30.00
		model.ContextWindow = 128000
		model.MaxTokens = 4096
		model.SupportsImages = strings.Contains(model.ID, "vision") || strings.Contains(model.ID, "preview")
		model.CanReason = true
		model.Categories = []string{"chat", "premium", "legacy"}

	// GPT-4 base models
	case strings.HasPrefix(model.ID, "gpt-4"):
		model.CostPer1MIn = 30.00
		model.CostPer1MOut = 60.00
		model.ContextWindow = 8192
		model.MaxTokens = 4096
		model.SupportsImages = strings.Contains(model.ID, "vision")
		model.CanReason = true
		model.Categories = []string{"chat", "premium", "legacy"}

	// GPT-3.5 models
	case strings.HasPrefix(model.ID, "gpt-3.5-turbo"):
		model.CostPer1MIn = 0.50
		model.CostPer1MOut = 1.50
		model.ContextWindow = 16385
		model.MaxTokens = 4096
		model.SupportsImages = false
		model.CanReason = false
		model.Categories = []string{"chat", "cost-effective", "legacy"}

	case strings.HasPrefix(model.ID, "gpt-3.5-turbo-instruct"):
		model.CostPer1MIn = 1.50
		model.CostPer1MOut = 2.00
		model.ContextWindow = 4096
		model.MaxTokens = 4096
		model.SupportsImages = false
		model.CanReason = false
		model.Categories = []string{"completion", "legacy"}

	// O-series reasoning models
	case strings.HasPrefix(model.ID, "o1-mini"):
		model.CostPer1MIn = 3.00
		model.CostPer1MOut = 12.00
		model.ContextWindow = 128000
		model.MaxTokens = 65536
		model.SupportsImages = false
		model.CanReason = true
		model.Categories = []string{"reasoning", "problem-solving", "fast"}

	case strings.HasPrefix(model.ID, "o1"):
		model.CostPer1MIn = 15.00
		model.CostPer1MOut = 60.00
		model.ContextWindow = 128000
		model.MaxTokens = 100000
		model.SupportsImages = false
		model.CanReason = true
		model.Categories = []string{"reasoning", "problem-solving", "premium"}

	case strings.HasPrefix(model.ID, "o3"):
		model.CostPer1MIn = 20.00
		model.CostPer1MOut = 80.00
		model.ContextWindow = 128000
		model.MaxTokens = 100000
		model.SupportsImages = true
		model.CanReason = true
		model.Categories = []string{"reasoning", "multimodal", "premium"}

	default:
		// Default values for unknown models
		model.CostPer1MIn = 1.00
		model.CostPer1MOut = 2.00
		model.ContextWindow = 8192
		model.MaxTokens = 4096
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

	if model.SupportsImages {
		model.Capabilities["vision"] = "high"
	}
	if model.CanReason {
		model.Capabilities["reasoning"] = "advanced"
	}

	return model
}

func (p *OpenAIExtendedProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsChat:         true,
		SupportsFIM:          false,
		SupportsEmbeddings:   true,
		SupportsFineTuning:   true,
		SupportsAgents:       true,
		SupportsFileUpload:   true,
		SupportsStreaming:    true,
		SupportsJSONMode:     true,
		SupportsVision:       true,
		SupportsAudio:        true,
		SupportedParameters:  []string{"temperature", "max_tokens", "top_p", "frequency_penalty", "presence_penalty", "stop"},
		SecurityFeatures:     []string{"moderation_endpoint", "content_filtering"},
		MaxRequestsPerMinute: 500,
		MaxTokensPerRequest:  128000,
	}
}

func (p *OpenAIExtendedProvider) GetEndpoints() []Endpoint {
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

func (p *OpenAIExtendedProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("  Testing model: %s\n", modelID)
	}

	url := p.baseURL + "/chat/completions"
	reqBody := openaiChatCompletionRequest{
		Model:     modelID,
		MaxTokens: 10,
		Messages: []openaiChatMessage{
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

	var chatResp openaiChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if verbose && len(chatResp.Choices) > 0 {
		fmt.Printf("    Response: %s\n", chatResp.Choices[0].Message.Content)
		fmt.Printf("    ✓ Model is working\n")
	}

	return nil
}
