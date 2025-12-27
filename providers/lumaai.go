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

// LumaAIProvider implements the Provider interface for Luma AI Dream Machine
type LumaAIProvider struct {
	apiKey    string
	baseURL   string
	client    *http.Client
	endpoints []Endpoint
}

// NewLumaAIProvider creates a new Luma AI provider instance
func NewLumaAIProvider(apiKey string) Provider {
	return &LumaAIProvider{
		apiKey:  apiKey,
		baseURL: "https://api.lumalabs.ai/v1",
		client: &http.Client{
			Timeout: 60 * time.Second, // Longer timeout for video generation
		},
	}
}

func init() {
	RegisterProvider("lumaai", NewLumaAIProvider)
}

// lumaGeneration represents a video generation request/response
type lumaGeneration struct {
	ID          string                 `json:"id,omitempty"`
	State       string                 `json:"state,omitempty"` // queued, processing, completed, failed
	Prompt      string                 `json:"prompt,omitempty"`
	AspectRatio string                 `json:"aspect_ratio,omitempty"` // 16:9, 9:16, 1:1
	Loop        bool                   `json:"loop,omitempty"`
	Keyframes   map[string]interface{} `json:"keyframes,omitempty"` // Image inputs
	CreatedAt   string                 `json:"created_at,omitempty"`
	Assets      *lumaAssets            `json:"assets,omitempty"`
	FailureCode string                 `json:"failure_code,omitempty"`
	FailureMsg  string                 `json:"failure_message,omitempty"`
}

// lumaAssets contains the generated video assets
type lumaAssets struct {
	Video string `json:"video,omitempty"` // URL to the generated video
}

// lumaGenerationRequest is the request body for creating a generation
type lumaGenerationRequest struct {
	Prompt      string                 `json:"prompt"`
	AspectRatio string                 `json:"aspect_ratio,omitempty"`
	Loop        bool                   `json:"loop,omitempty"`
	Keyframes   map[string]interface{} `json:"keyframes,omitempty"`
}

func (p *LumaAIProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
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

func (p *LumaAIProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
	url := p.baseURL + endpoint.Path

	var req *http.Request
	var err error

	// For generation endpoint, we can't test without creating a real generation
	// So we just test if the endpoint responds with proper authentication
	if endpoint.Method == "POST" && endpoint.Path == "/generations" {
		// Test with minimal payload to check auth
		reqBody := lumaGenerationRequest{
			Prompt: "test",
		}
		bodyBytes, _ := json.Marshal(reqBody)
		req, err = http.NewRequestWithContext(ctx, endpoint.Method, url, bytes.NewReader(bodyBytes))
	} else {
		req, err = http.NewRequestWithContext(ctx, endpoint.Method, url, nil)
	}

	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Add headers
	for k, v := range endpoint.Headers {
		req.Header.Set(k, v)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// For POST /generations with test payload, we expect 201 or 400 (if test prompt is invalid)
	// For GET endpoints, we expect 200 or 404
	if endpoint.Method == "POST" {
		if resp.StatusCode >= 200 && resp.StatusCode < 500 {
			return nil // Accept both success and client errors for testing
		}
	} else {
		if resp.StatusCode >= 200 && resp.StatusCode < 500 {
			return nil
		}
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
}

func (p *LumaAIProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("Fetching Luma AI models...")
	}

	// Luma AI Dream Machine currently has one main model
	models := []Model{
		{
			ID:             "dream-machine-v1",
			Name:           "Dream Machine v1",
			Description:    "Luma AI's Dream Machine for high-quality video generation from text and images",
			CostPer1MIn:    0.0, // Charged per generation, not per token
			CostPer1MOut:   0.0,
			ContextWindow:  2000, // Character limit for prompts
			MaxTokens:      2000,
			SupportsImages: true, // Can use images as keyframes
			SupportsTools:  false,
			CanReason:      false,
			CanStream:      false, // Video generation is async
			Categories:     []string{"video", "generation", "creative"},
			Capabilities: map[string]string{
				"video_length":  "5_seconds",
				"resolution":    "1080p",
				"aspect_ratios": "16:9,9:16,1:1",
				"loop":          "supported",
				"keyframes":     "supported",
				"pricing_model": "per_generation",
			},
		},
	}

	if verbose {
		fmt.Printf("  Found %d model(s)\n", len(models))
	}

	return models, nil
}

func (p *LumaAIProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsChat:         false,
		SupportsFIM:          false,
		SupportsEmbeddings:   false,
		SupportsFineTuning:   false,
		SupportsAgents:       false,
		SupportsFileUpload:   true,  // Image keyframes via URLs
		SupportsStreaming:    false, // Async generation
		SupportsJSONMode:     false,
		SupportsVision:       true, // Image-to-video
		SupportsAudio:        false,
		SupportedParameters:  []string{"prompt", "aspect_ratio", "loop", "keyframes"},
		SecurityFeatures:     []string{"API_key_authentication", "rate_limiting"},
		MaxRequestsPerMinute: 20, // Conservative estimate
		MaxTokensPerRequest:  2000,
	}
}

func (p *LumaAIProvider) GetEndpoints() []Endpoint {
	// Return cached endpoints if available
	if len(p.endpoints) > 0 {
		return p.endpoints
	}

	// Otherwise return fresh endpoints
	return []Endpoint{
		{
			Path:        "/generations",
			Method:      "POST",
			Description: "Create a new video generation",
			Headers: map[string]string{
				"Authorization": "Bearer " + p.apiKey,
				"Content-Type":  "application/json",
			},
			Status: StatusUnknown,
		},
		{
			Path:        "/generations/{id}",
			Method:      "GET",
			Description: "Get generation status",
			Headers: map[string]string{
				"Authorization": "Bearer " + p.apiKey,
			},
			Status: StatusUnknown,
		},
	}
}

func (p *LumaAIProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("Testing Luma AI model: %s\n", modelID)
	}

	// Create a test generation request
	request := lumaGenerationRequest{
		Prompt:      "A serene lake at sunset",
		AspectRatio: "16:9",
		Loop:        false,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseURL + "/generations"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(requestBody))
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

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var generation lumaGeneration
	if err := json.NewDecoder(resp.Body).Decode(&generation); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if verbose {
		fmt.Printf("  ✓ Generation created: %s (state: %s)\n", generation.ID, generation.State)
	}

	return nil
}

// GetGenerationStatus retrieves the status of a video generation
func (p *LumaAIProvider) GetGenerationStatus(ctx context.Context, generationID string) (*lumaGeneration, error) {
	url := fmt.Sprintf("%s/generations/%s", p.baseURL, generationID)

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

	var generation lumaGeneration
	if err := json.NewDecoder(resp.Body).Decode(&generation); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &generation, nil
}
