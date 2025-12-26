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

// DeepgramProvider implements the Provider interface for Deepgram STT
type DeepgramProvider struct {
	apiKey    string
	baseURL   string
	client    *http.Client
	endpoints []Endpoint
}

// NewDeepgramProvider creates a new Deepgram provider instance
func NewDeepgramProvider(apiKey string) Provider {
	return &DeepgramProvider{
		apiKey:  apiKey,
		baseURL: "https://api.deepgram.com/v1",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func init() {
	RegisterProvider("deepgram", NewDeepgramProvider)
}

// deepgramModelsResponse represents the response from /models endpoint
type deepgramModelsResponse struct {
	STT []deepgramSTTModel `json:"stt"`
}

type deepgramSTTModel struct {
	Name         string `json:"name"`
	Canonical    string `json:"canonical_name"`
	Architecture string `json:"architecture,omitempty"`
	Language     string `json:"language,omitempty"`
	Version      string `json:"version,omitempty"`
	UUID         string `json:"uuid,omitempty"`
	Batch        bool   `json:"batch"`
	Streaming    bool   `json:"streaming"`
}

// deepgramProjectsResponse represents the response from /projects endpoint
type deepgramProjectsResponse struct {
	Projects []deepgramProject `json:"projects"`
}

type deepgramProject struct {
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
}

// deepgramBalanceResponse represents the response from /projects/{projectId}/balances endpoint
type deepgramBalanceResponse struct {
	Balances []deepgramBalance `json:"balances"`
}

type deepgramBalance struct {
	BalanceID string  `json:"balance_id"`
	Amount    float64 `json:"amount"`
	Units     string  `json:"units"`
}

func (p *DeepgramProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
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

func (p *DeepgramProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("  Fetching available models from Deepgram API...")
	}

	// Call the /models endpoint to get available models
	url := p.baseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Token "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var modelsResp deepgramModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Map Deepgram models to our Model structure
	models := make([]Model, 0, len(modelsResp.STT))
	for _, m := range modelsResp.STT {
		model := Model{
			ID:          m.Canonical,
			Name:        m.Name,
			Description: fmt.Sprintf("%s - %s architecture", m.Language, m.Architecture),
			// Deepgram pricing: Nova-2 is $0.0043/min = $0.258/hour
			// Approximately $4.30 per 1M tokens (assuming ~60 tokens/min speech)
			CostPer1MIn:    4.30,
			CostPer1MOut:   0.0, // No output tokens, just transcription
			ContextWindow:  0,   // Audio-based, not token-based
			SupportsImages: false,
			SupportsTools:  false,
			CanReason:      false,
			CanStream:      m.Streaming,
			Categories:     []string{"stt", "transcription"},
		}

		if m.Batch {
			model.Categories = append(model.Categories, "batch")
		}
		if m.Streaming {
			model.Categories = append(model.Categories, "streaming")
		}
		if m.Language != "" {
			model.Categories = append(model.Categories, m.Language)
		}

		models = append(models, model)
	}

	if verbose {
		fmt.Printf("  Found %d models\n", len(models))
	}

	return models, nil
}

func (p *DeepgramProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsChat:         false,
		SupportsFIM:          false,
		SupportsEmbeddings:   false,
		SupportsFineTuning:   true, // Custom model training available
		SupportsAgents:       false,
		SupportsFileUpload:   true, // Batch transcription supports file upload
		SupportsStreaming:    true, // Live streaming transcription
		SupportsJSONMode:     false,
		SupportsVision:       false,
		SupportsAudio:        true, // Primary capability
		SupportedParameters:  []string{"model", "language", "punctuate", "diarize", "smart_format", "utterances"},
		SecurityFeatures:     []string{"on_prem_deployment", "soc2_compliant", "hipaa_compliant"},
		MaxRequestsPerMinute: 60,
		MaxTokensPerRequest:  0, // Audio-based, not token-based
	}
}

func (p *DeepgramProvider) GetEndpoints() []Endpoint {
	// Return cached endpoints if available
	if len(p.endpoints) > 0 {
		return p.endpoints
	}

	// Otherwise return fresh endpoints
	return []Endpoint{
		{
			Path:        "/models",
			Method:      "GET",
			Description: "List available STT models",
			Headers: map[string]string{
				"Authorization": "Token " + p.apiKey,
			},
		},
		{
			Path:        "/projects",
			Method:      "GET",
			Description: "List projects",
			Headers: map[string]string{
				"Authorization": "Token " + p.apiKey,
			},
		},
	}
}

func (p *DeepgramProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("  Testing model: %s\n", modelID)
	}

	// Create test request for transcription with minimal audio
	// Using a very short silent audio sample for testing
	requestBody := map[string]interface{}{
		"url": "https://static.deepgram.com/examples/Bueller-Life-moves-pretty-fast.wav",
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.baseURL + "/listen?model=" + modelID
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Token "+p.apiKey)
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

func (p *DeepgramProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
	url := p.baseURL + endpoint.Path

	req, err := http.NewRequestWithContext(ctx, endpoint.Method, url, nil)
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
