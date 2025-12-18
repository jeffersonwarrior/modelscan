package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// GoogleProvider implements the Provider interface for Google Gemini
type GoogleProvider struct {
	apiKey    string
	baseURL   string
	endpoints []Endpoint
}

// NewGoogleProvider creates a new Google Gemini provider instance
func NewGoogleProvider(apiKey string) Provider {
	return &GoogleProvider{
		apiKey:  apiKey,
		baseURL: "https://generativelanguage.googleapis.com/v1beta",
	}
}

func init() {
	RegisterProvider("google", NewGoogleProvider)
}

// googleModelsResponse represents the response from the models list endpoint
type googleModelsResponse struct {
	Models        []googleModelInfo `json:"models"`
	NextPageToken string            `json:"nextPageToken,omitempty"`
}

type googleModelInfo struct {
	Name                       string   `json:"name"`
	BaseModelID                string   `json:"baseModelId,omitempty"`
	Version                    string   `json:"version,omitempty"`
	DisplayName                string   `json:"displayName"`
	Description                string   `json:"description"`
	InputTokenLimit            int      `json:"inputTokenLimit"`
	OutputTokenLimit           int      `json:"outputTokenLimit"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
	Temperature                float64  `json:"temperature,omitempty"`
	TopP                       float64  `json:"topP,omitempty"`
	TopK                       int      `json:"topK,omitempty"`
}

func (p *GoogleProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
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

func (p *GoogleProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("  Fetching available models from Google Gemini API...")
	}

	// Call the models endpoint
	url := p.baseURL + "/models?key=" + p.apiKey
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var modelsResp googleModelsResponse
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
func (p *GoogleProvider) isGenerativeModel(model googleModelInfo) bool {
	for _, method := range model.SupportedGenerationMethods {
		if method == "generateContent" || method == "streamGenerateContent" {
			return true
		}
	}
	return false
}

// enrichModelDetails adds pricing and capability information
func (p *GoogleProvider) enrichModelDetails(model Model) Model {
	// Set common capabilities
	model.SupportsTools = true
	model.CanStream = true
	model.SupportsImages = true // Most Gemini models support multimodal

	// Determine specific details based on model ID
	switch {
	// Gemini 3 Pro (latest preview)
	case strings.Contains(model.ID, "gemini-3-pro"):
		model.CostPer1MIn = 2.00
		model.CostPer1MOut = 12.00
		model.CanReason = true
		model.Categories = []string{"chat", "reasoning", "multimodal", "preview"}
		model.Capabilities = map[string]string{
			"reasoning":        "adaptive",
			"vision":           "high",
			"function_calling": "full",
			"streaming":        "supported",
			"json_mode":        "supported",
		}

	// Gemini 3 Flash (latest preview)
	case strings.Contains(model.ID, "gemini-3-flash"):
		model.CostPer1MIn = 0.50
		model.CostPer1MOut = 3.00
		model.CanReason = true
		model.Categories = []string{"chat", "fast", "multimodal", "preview"}
		model.Capabilities = map[string]string{
			"reasoning":        "advanced",
			"vision":           "high",
			"function_calling": "full",
			"streaming":        "supported",
		}

	// Gemini 2.5 Pro
	case strings.Contains(model.ID, "gemini-2.5-pro"):
		model.CostPer1MIn = 1.25
		model.CostPer1MOut = 10.00
		model.CanReason = true
		model.Categories = []string{"chat", "reasoning", "coding", "multimodal"}
		model.Capabilities = map[string]string{
			"reasoning":        "advanced",
			"vision":           "high",
			"function_calling": "full",
			"streaming":        "supported",
			"json_mode":        "supported",
		}

	// Gemini 2.5 Flash
	case strings.Contains(model.ID, "gemini-2.5-flash"):
		model.CostPer1MIn = 0.30
		model.CostPer1MOut = 2.50
		model.CanReason = false
		model.Categories = []string{"chat", "fast", "cost-effective", "multimodal"}
		model.Capabilities = map[string]string{
			"vision":           "high",
			"function_calling": "full",
			"streaming":        "supported",
		}

	// Gemini 2.5 Flash-Lite
	case strings.Contains(model.ID, "gemini-2.5-flash-lite"):
		model.CostPer1MIn = 0.10
		model.CostPer1MOut = 0.40
		model.CanReason = false
		model.Categories = []string{"chat", "fast", "ultra-efficient"}
		model.Capabilities = map[string]string{
			"function_calling": "full",
			"streaming":        "supported",
		}

	// Gemini 2.0 Flash
	case strings.Contains(model.ID, "gemini-2.0-flash"):
		model.CostPer1MIn = 0.30
		model.CostPer1MOut = 1.20
		model.CanReason = false
		model.Categories = []string{"chat", "balanced", "multimodal"}
		model.Capabilities = map[string]string{
			"vision":           "medium",
			"function_calling": "full",
			"streaming":        "supported",
		}

	// Gemini 1.5 Pro
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

	// Gemini 1.5 Flash
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

	// Image generation models
	case strings.Contains(model.ID, "image"):
		model.CostPer1MIn = 1.00
		model.CostPer1MOut = 30.00
		model.SupportsImages = false
		model.Categories = []string{"image-generation", "multimodal"}
		model.Capabilities = map[string]string{
			"image_generation": "high-fidelity",
			"image_editing":    "conversational",
		}

	// Embedding models
	case strings.Contains(model.ID, "embedding"):
		model.CostPer1MIn = 0.025
		model.CostPer1MOut = 0.00
		model.SupportsImages = false
		model.SupportsTools = false
		model.Categories = []string{"embedding"}
		model.Capabilities = map[string]string{
			"embedding": "text",
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

func (p *GoogleProvider) GetCapabilities() ProviderCapabilities {
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
		SupportedParameters:  []string{"temperature", "maxOutputTokens", "topP", "topK", "stopSequences"},
		SecurityFeatures:     []string{"safety_settings", "content_filtering", "harm_categories"},
		MaxRequestsPerMinute: 60,
		MaxTokensPerRequest:  1000000,
	}
}

func (p *GoogleProvider) GetEndpoints() []Endpoint {
	return []Endpoint{
		{
			Path:        "/v1beta/models",
			Method:      "GET",
			Description: "List available models",
		},
		{
			Path:        "/v1beta/models/gemini-2.5-flash:generateContent",
			Method:      "POST",
			Description: "Generate content (chat completion)",
		},
	}
}

func (p *GoogleProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("  Testing model: %s\n", modelID)
	}

	// Construct the generateContent endpoint
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.baseURL, modelID, p.apiKey)

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": "Say 'test successful' in 2 words"},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"maxOutputTokens": 10,
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
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

func (p *GoogleProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
	url := p.baseURL + endpoint.Path

	// Add API key as query parameter
	if !strings.Contains(url, "?") {
		url += "?key=" + p.apiKey
	}

	var req *http.Request
	var err error

	if endpoint.Method == "POST" {
		// Test generateContent endpoint
		body := `{
			"contents": [{
				"parts": [{"text": "Hi"}]
			}],
			"generationConfig": {"maxOutputTokens": 5}
		}`
		req, err = http.NewRequestWithContext(ctx, endpoint.Method, url, strings.NewReader(body))
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

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
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
