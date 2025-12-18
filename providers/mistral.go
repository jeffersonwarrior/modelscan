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

// MistralProvider implements the Provider interface for Mistral AI
type MistralProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewMistralProvider creates a new Mistral provider instance
func NewMistralProvider(apiKey string) Provider {
	return &MistralProvider{
		apiKey:  apiKey,
		baseURL: "https://api.mistral.ai/v1",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func init() {
	RegisterProvider("mistral", NewMistralProvider)
}

func (p *MistralProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
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

	return nil
}

func (p *MistralProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("  Fetching available models...")
	}

	// Call the /v1/models endpoint
	url := p.baseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResponse struct {
		Object string `json:"object"`
		Data   []struct {
			ID           string                 `json:"id"`
			Object       string                 `json:"object"`
			Created      int64                  `json:"created"`
			OwnedBy      string                 `json:"owned_by"`
			Capabilities map[string]interface{} `json:"capabilities"`
			Description  string                 `json:"description,omitempty"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to our Model format
	var models []Model
	for _, mistralModel := range apiResponse.Data {
		model := Model{
			ID:           mistralModel.ID,
			Name:         mistralModel.ID, // Mistral doesn't always provide names
			Description:  mistralModel.Description,
			CreatedAt:    time.Unix(mistralModel.Created, 0).Format(time.RFC3339),
			Capabilities: make(map[string]string),
		}

		// Add categories based on model
		model.Categories = guessMistralModelCategories(mistralModel.ID)

		// Add capabilities from the response
		for key, value := range mistralModel.Capabilities {
			if boolVal, ok := value.(bool); ok {
				if boolVal {
					model.Capabilities[key] = "true"
				} else {
					model.Capabilities[key] = "false"
				}
			} else if str, ok := value.(string); ok {
				model.Capabilities[key] = str
			} else {
				model.Capabilities[key] = fmt.Sprintf("%v", value)
			}
		}

		models = append(models, model)
	}

	if verbose {
		fmt.Printf("  Found %d models\n", len(apiResponse.Data))
	}

	return models, nil
}

func (p *MistralProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsChat:       true,
		SupportsFIM:        true,
		SupportsEmbeddings: true,
		SupportsFineTuning: true,
		SupportsAgents:     true,
		SupportsFileUpload: true,
		SupportsStreaming:  true,
		SupportsJSONMode:   true,
		SupportsVision:     true, // For certain models
		SupportsAudio:      true, // For Voxtral models
		SupportedParameters: []string{
			"model", "messages", "temperature", "top_p", "max_tokens",
			"min_tokens", "stream", "stop", "random_seed", "response_format",
			"tools", "tool_choice", "safe_prompt", "presence_penalty",
			"frequency_penalty", "n",
		},
		SecurityFeatures: []string{
			"safe_prompt",
			"content_filtering",
		},
		MaxRequestsPerMinute: 60,     // May vary by plan
		MaxTokensPerRequest:  200000, // May vary by model
	}
}

func (p *MistralProvider) GetEndpoints() []Endpoint {
	return []Endpoint{
		{
			Path:        "/models",
			Method:      "GET",
			Description: "List available models",
		},
		{
			Path:        "/chat/completions",
			Method:      "POST",
			Description: "Chat completion endpoint",
			TestParams: map[string]interface{}{
				"model": "mistral-small-latest",
				"messages": []map[string]string{
					{"role": "user", "content": "Hello"},
				},
				"max_tokens": 10,
			},
		},
		{
			Path:        "/fim/completions",
			Method:      "POST",
			Description: "Fill-in-the-middle code completion",
			TestParams: map[string]interface{}{
				"model":      "codestral-latest",
				"prompt":     "def hello():",
				"suffix":     "    print('Hello')",
				"max_tokens": 10,
			},
		},
		{
			Path:        "/agents",
			Method:      "GET",
			Description: "List agents",
		},
		{
			Path:        "/embeddings",
			Method:      "POST",
			Description: "Create embeddings",
			TestParams: map[string]interface{}{
				"model": "mistral-embed",
				"input": "Test embedding",
			},
		},
		{
			Path:        "/files",
			Method:      "GET",
			Description: "List uploaded files",
		},
		{
			Path:        "/fine_tuning/jobs",
			Method:      "GET",
			Description: "List fine-tuning jobs",
		},
	}
}

func (p *MistralProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("  Testing model: %s\n", modelID)
	}

	payload := map[string]interface{}{
		"model": modelID,
		"messages": []map[string]string{
			{"role": "user", "content": "Say 'test'"},
		},
		"max_tokens": 5,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	url := p.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to test model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("model test failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (p *MistralProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
	switch endpoint.Method {
	case "GET":
		return p.testGetEndpoint(ctx, endpoint)
	case "POST":
		return p.testPostEndpoint(ctx, endpoint)
	default:
		return fmt.Errorf("unsupported HTTP method: %s", endpoint.Method)
	}
}

func (p *MistralProvider) testGetEndpoint(ctx context.Context, endpoint *Endpoint) error {
	url := p.baseURL + endpoint.Path
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 2xx status codes are considered success
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (p *MistralProvider) testPostEndpoint(ctx context.Context, endpoint *Endpoint) error {
	url := p.baseURL + endpoint.Path

	var jsonData []byte
	var err error

	if endpoint.TestParams != nil {
		jsonData, err = json.Marshal(endpoint.TestParams)
		if err != nil {
			return fmt.Errorf("failed to encode test params: %w", err)
		}
	} else {
		// Default minimal payload
		jsonData, err = json.Marshal(map[string]string{"test": "true"})
		if err != nil {
			return fmt.Errorf("failed to encode default payload: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// For POST, we consider 2xx or 400 (bad request) as success
	// 400 might mean the endpoint exists but needs different params
	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return nil
	}

	return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}

func (p *MistralProvider) enhanceModelInfo(model *Model) {
	// Add categories based on model ID patterns
	switch {
	case containsAny(model.ID, []string{"devstral", "codestral", "magistral"}):
		model.Categories = append(model.Categories, "coding")
	case containsAny(model.ID, []string{"mistral-small", "mistral-medium", "mistral-large", "ministral"}):
		model.Categories = append(model.Categories, "chat")
	case containsAny(model.ID, []string{"embed"}):
		model.Categories = append(model.Categories, "embedding")
	case containsAny(model.ID, []string{"voxtral"}):
		model.Categories = append(model.Categories, "audio")
	}

	// Add specific capabilities
	if model.SupportsImages {
		if model.Capabilities == nil {
			model.Capabilities = make(map[string]string)
		}
		model.Capabilities["vision"] = "high"
	}

	if model.SupportsTools {
		if model.Capabilities == nil {
			model.Capabilities = make(map[string]string)
		}
		model.Capabilities["function_calling"] = "full"
	}

	if model.CanReason {
		if model.Capabilities == nil {
			model.Capabilities = make(map[string]string)
		}
		model.Capabilities["reasoning"] = "enabled"
	}
}

func guessMistralModelCategories(modelID string) []string {
	var categories []string

	if containsAny(modelID, []string{"labs-devstral", "codestral", "magistral", "mistral-code"}) {
		categories = append(categories, "coding")
	}
	if containsAny(modelID, []string{"mistral-small", "mistral-medium", "mistral-large", "ministral", "pixtral"}) {
		categories = append(categories, "chat")
	}
	if containsAny(modelID, []string{"embed"}) {
		categories = append(categories, "embedding")
	}
	if containsAny(modelID, []string{"voxtral"}) {
		categories = append(categories, "audio")
	}

	if len(categories) == 0 {
		categories = []string{"general"}
	}

	return categories
}
