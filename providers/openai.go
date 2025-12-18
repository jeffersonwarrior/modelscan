package providers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements the Provider interface for OpenAI using the official SDK
type OpenAIProvider struct {
	apiKey    string
	baseURL   string
	client    *openai.Client
	endpoints []Endpoint
}

// NewOpenAIProvider creates a new OpenAI provider instance using the official SDK
func NewOpenAIProvider(apiKey string) Provider {
	client := openai.NewClient(apiKey)

	return &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1",
		client:  client,
	}
}

func init() {
	RegisterProvider("openai", NewOpenAIProvider)
}

func (p *OpenAIProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
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

func (p *OpenAIProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("  Fetching available models from OpenAI API...")
	}

	// Use the official SDK to list models
	modelsList, err := p.client.ListModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	models := make([]Model, 0, len(modelsList.Models))
	for _, apiModel := range modelsList.Models {
		// Only include models we can actually use (skip embeddings, whisper, tts, etc.)
		if !p.isUsableModel(apiModel.ID) {
			continue
		}

		model := Model{
			ID:        apiModel.ID,
			Name:      p.formatModelName(apiModel.ID),
			CreatedAt: time.Unix(apiModel.CreatedAt, 0).Format(time.RFC3339),
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
func (p *OpenAIProvider) isUsableModel(modelID string) bool {
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
func (p *OpenAIProvider) formatModelName(modelID string) string {
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
func (p *OpenAIProvider) enrichModelDetails(model Model) Model {
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

func (p *OpenAIProvider) GetCapabilities() ProviderCapabilities {
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

func (p *OpenAIProvider) GetEndpoints() []Endpoint {
	return []Endpoint{
		{
			Path:        "/v1/chat/completions",
			Method:      "POST",
			Description: "Create a chat completion",
		},
		{
			Path:        "/v1/models",
			Method:      "GET",
			Description: "List available models",
		},
		{
			Path:        "/v1/embeddings",
			Method:      "POST",
			Description: "Create embeddings",
		},
	}
}

func (p *OpenAIProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("  Testing model: %s\n", modelID)
	}

	// Use the official SDK to send a test message
	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:     modelID,
		MaxTokens: 10,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "Say 'test successful' in 2 words",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("model test failed: %w", err)
	}

	if verbose && len(resp.Choices) > 0 {
		fmt.Printf("    Response: %s\n", resp.Choices[0].Message.Content)
		fmt.Printf("    ✓ Model is working\n")
	}

	return nil
}

func (p *OpenAIProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
	switch endpoint.Path {
	case "/v1/models":
		// Test models list endpoint
		_, err := p.client.ListModels(ctx)
		return err

	case "/v1/chat/completions":
		// Test chat endpoint with minimal request
		_, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:     "gpt-3.5-turbo",
			MaxTokens: 5,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: "Hi",
				},
			},
		})
		return err

	case "/v1/embeddings":
		// Test embeddings endpoint
		_, err := p.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
			Model: openai.AdaEmbeddingV2,
			Input: []string{"test"},
		})
		return err

	default:
		return fmt.Errorf("unknown endpoint: %s", endpoint.Path)
	}
}
