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

// GoogleThinkingProvider implements the Provider interface for Google Gemini with thinking modes
type GoogleThinkingProvider struct {
	apiKey    string
	baseURL   string
	client    *http.Client
	endpoints []Endpoint
}

// NewGoogleThinkingProvider creates a new Google Gemini Thinking provider instance
func NewGoogleThinkingProvider(apiKey string) Provider {
	return &GoogleThinkingProvider{
		apiKey:  apiKey,
		baseURL: "https://generativelanguage.googleapis.com/v1beta",
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func init() {
	RegisterProvider("google_thinking", NewGoogleThinkingProvider)
}

// Google API response structures for thinking mode
type googleThinkingModelsResponse struct {
	Models        []googleThinkingModelInfo `json:"models"`
	NextPageToken string                    `json:"nextPageToken,omitempty"`
}

type googleThinkingModelInfo struct {
	Name                       string   `json:"name"`
	BaseModelID                string   `json:"baseModelId,omitempty"`
	Version                    string   `json:"version,omitempty"`
	DisplayName                string   `json:"displayName"`
	Description                string   `json:"description"`
	InputTokenLimit            int      `json:"inputTokenLimit"`
	OutputTokenLimit           int      `json:"outputTokenLimit"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
}

type googleThinkingContent struct {
	Parts []googleThinkingPart `json:"parts"`
	Role  string               `json:"role,omitempty"`
}

type googleThinkingPart struct {
	Text     string                  `json:"text,omitempty"`
	Thought  *googleThinkingThought  `json:"thought,omitempty"`
	Metadata *googleThinkingMetadata `json:"metadata,omitempty"`
}

type googleThinkingThought struct {
	ThoughtText string `json:"thought_text"`
}

type googleThinkingMetadata struct {
	ThinkingTokens int `json:"thinking_tokens,omitempty"`
}

type googleThinkingRequest struct {
	Contents         []googleThinkingContent         `json:"contents"`
	GenerationConfig *googleThinkingGenerationConfig `json:"generationConfig,omitempty"`
}

type googleThinkingGenerationConfig struct {
	MaxOutputTokens       int     `json:"maxOutputTokens,omitempty"`
	Temperature           float64 `json:"temperature,omitempty"`
	TopP                  float64 `json:"topP,omitempty"`
	TopK                  int     `json:"topK,omitempty"`
	ThoughtBeforeResponse bool    `json:"thought_before_response,omitempty"`
}

type googleThinkingResponse struct {
	Candidates     []googleThinkingCandidate `json:"candidates"`
	UsageMetadata  *googleThinkingUsage      `json:"usageMetadata,omitempty"`
	PromptFeedback *googleThinkingFeedback   `json:"promptFeedback,omitempty"`
}

type googleThinkingCandidate struct {
	Content       googleThinkingContent  `json:"content"`
	FinishReason  string                 `json:"finishReason"`
	SafetyRatings []googleThinkingSafety `json:"safetyRatings,omitempty"`
}

type googleThinkingSafety struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

type googleThinkingUsage struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
	ThinkingTokens       int `json:"thinkingTokens,omitempty"`
}

type googleThinkingFeedback struct {
	BlockReason   string                 `json:"blockReason,omitempty"`
	SafetyRatings []googleThinkingSafety `json:"safetyRatings,omitempty"`
}

func (p *GoogleThinkingProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
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

func (p *GoogleThinkingProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
	url := p.baseURL + endpoint.Path

	// Add API key as query parameter
	if !strings.Contains(url, "?") {
		url += "?key=" + p.apiKey
	}

	var req *http.Request
	var err error

	if endpoint.Method == "POST" {
		// Test generateContent endpoint with thinking mode
		reqBody := googleThinkingRequest{
			Contents: []googleThinkingContent{
				{
					Parts: []googleThinkingPart{
						{Text: "Hi"},
					},
				},
			},
			GenerationConfig: &googleThinkingGenerationConfig{
				MaxOutputTokens:       10,
				ThoughtBeforeResponse: true,
			},
		}

		bodyBytes, marshalErr := json.Marshal(reqBody)
		if marshalErr != nil {
			return fmt.Errorf("failed to marshal request: %w", marshalErr)
		}

		req, err = http.NewRequestWithContext(ctx, endpoint.Method, url, bytes.NewReader(bodyBytes))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequestWithContext(ctx, endpoint.Method, url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("endpoint returned status %d: %s", resp.StatusCode, string(body))
}

func (p *GoogleThinkingProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("  Fetching available models from Google Gemini API...")
	}

	url := p.baseURL + "/models?key=" + p.apiKey
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var modelsResp googleThinkingModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]Model, 0, len(modelsResp.Models))
	for _, apiModel := range modelsResp.Models {
		// Skip non-generative models
		if !p.isGenerativeModel(apiModel) {
			continue
		}

		// Extract model ID from name (format: "models/gemini-...")
		modelID := strings.TrimPrefix(apiModel.Name, "models/")

		model := Model{
			ID:            modelID,
			Name:          apiModel.DisplayName,
			Description:   apiModel.Description,
			ContextWindow: apiModel.InputTokenLimit,
			MaxTokens:     apiModel.OutputTokenLimit,
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

// isGenerativeModel checks if the model supports text generation
func (p *GoogleThinkingProvider) isGenerativeModel(model googleThinkingModelInfo) bool {
	for _, method := range model.SupportedGenerationMethods {
		if method == "generateContent" || method == "streamGenerateContent" {
			return true
		}
	}
	return false
}

// enrichModelDetails adds pricing and capability information with thinking mode support
func (p *GoogleThinkingProvider) enrichModelDetails(model Model) Model {
	// Set common capabilities
	model.SupportsTools = true
	model.CanStream = true
	model.SupportsImages = true

	// Determine specific details based on model ID
	switch {
	// Gemini 2.0 models support thinking modes
	case strings.Contains(model.ID, "gemini-2.0"):
		model.CostPer1MIn = 0.30
		model.CostPer1MOut = 1.20
		model.CanReason = true
		model.Categories = []string{"chat", "thinking", "multimodal"}
		model.Capabilities = map[string]string{
			"thinking":         "supported",
			"vision":           "medium",
			"function_calling": "full",
			"streaming":        "supported",
		}

	// Gemini 2.5 Pro with thinking
	case strings.Contains(model.ID, "gemini-2.5-pro"):
		model.CostPer1MIn = 1.25
		model.CostPer1MOut = 10.00
		model.CanReason = true
		model.Categories = []string{"chat", "reasoning", "thinking", "multimodal"}
		model.Capabilities = map[string]string{
			"thinking":         "advanced",
			"reasoning":        "advanced",
			"vision":           "high",
			"function_calling": "full",
			"streaming":        "supported",
			"json_mode":        "supported",
		}

	// Gemini 2.5 Flash with thinking
	case strings.Contains(model.ID, "gemini-2.5-flash"):
		model.CostPer1MIn = 0.30
		model.CostPer1MOut = 2.50
		model.CanReason = true
		model.Categories = []string{"chat", "fast", "thinking", "multimodal"}
		model.Capabilities = map[string]string{
			"thinking":         "supported",
			"vision":           "high",
			"function_calling": "full",
			"streaming":        "supported",
		}

	// Gemini 1.5 Pro (legacy, no thinking)
	case strings.Contains(model.ID, "gemini-1.5-pro") || strings.Contains(model.ID, "gemini-pro"):
		model.CostPer1MIn = 1.25
		model.CostPer1MOut = 5.00
		model.CanReason = true
		model.Categories = []string{"chat", "premium", "multimodal", "legacy"}
		model.Capabilities = map[string]string{
			"reasoning":        "good",
			"vision":           "high",
			"function_calling": "full",
			"streaming":        "supported",
		}

	// Gemini 1.5 Flash (legacy, no thinking)
	case strings.Contains(model.ID, "gemini-1.5-flash") || strings.Contains(model.ID, "gemini-flash"):
		model.CostPer1MIn = 0.075
		model.CostPer1MOut = 0.30
		model.CanReason = false
		model.Categories = []string{"chat", "fast", "legacy"}
		model.Capabilities = map[string]string{
			"vision":           "medium",
			"function_calling": "full",
			"streaming":        "supported",
		}

	default:
		// Default values for unknown models
		model.CostPer1MIn = 1.00
		model.CostPer1MOut = 3.00
		model.CanReason = false
		model.Categories = []string{"chat"}
		model.Capabilities = map[string]string{
			"function_calling": "full",
		}
	}

	return model
}

func (p *GoogleThinkingProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsChat:         true,
		SupportsFIM:          false,
		SupportsEmbeddings:   false,
		SupportsFineTuning:   true,
		SupportsAgents:       true,
		SupportsFileUpload:   true,
		SupportsStreaming:    true,
		SupportsJSONMode:     true,
		SupportsVision:       true,
		SupportsAudio:        true,
		SupportedParameters:  []string{"temperature", "maxOutputTokens", "topP", "topK", "thought_before_response"},
		SecurityFeatures:     []string{"safety_settings", "content_filtering", "harm_categories"},
		MaxRequestsPerMinute: 60,
		MaxTokensPerRequest:  2000000, // Gemini 2.0+ supports up to 2M context
	}
}

func (p *GoogleThinkingProvider) GetEndpoints() []Endpoint {
	return []Endpoint{
		{
			Path:        "/v1beta/models",
			Method:      "GET",
			Description: "List available models",
		},
		{
			Path:        "/v1beta/models/gemini-2.0-flash-exp:generateContent",
			Method:      "POST",
			Description: "Generate content with thinking mode",
		},
	}
}

func (p *GoogleThinkingProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("  Testing model: %s\n", modelID)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.baseURL, modelID, p.apiKey)

	reqBody := googleThinkingRequest{
		Contents: []googleThinkingContent{
			{
				Parts: []googleThinkingPart{
					{Text: "Say 'test successful' in 2 words"},
				},
			},
		},
		GenerationConfig: &googleThinkingGenerationConfig{
			MaxOutputTokens: 10,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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
