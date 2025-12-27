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

// MidjourneyProvider implements the Provider interface for Midjourney V6
type MidjourneyProvider struct {
	apiKey    string
	baseURL   string
	client    *http.Client
	endpoints []Endpoint
}

// NewMidjourneyProvider creates a new Midjourney provider instance
func NewMidjourneyProvider(apiKey string) Provider {
	return &MidjourneyProvider{
		apiKey:  apiKey,
		baseURL: "https://api.midjourney.com/v1",
		client: &http.Client{
			Timeout: 90 * time.Second, // Longer timeout for image generation
		},
	}
}

func init() {
	RegisterProvider("midjourney", NewMidjourneyProvider)
}

// midjourneyImagineRequest represents a text-to-image generation request
type midjourneyImagineRequest struct {
	Prompt      string `json:"prompt"`
	Model       string `json:"model,omitempty"`        // v6, v6.1
	AspectRatio string `json:"aspect_ratio,omitempty"` // 1:1, 16:9, 9:16, 4:3, 3:2
	Quality     string `json:"quality,omitempty"`      // low, medium, high
	Stylize     int    `json:"stylize,omitempty"`      // 0-1000
	Chaos       int    `json:"chaos,omitempty"`        // 0-100
	Weird       int    `json:"weird,omitempty"`        // 0-3000
	Tile        bool   `json:"tile,omitempty"`         // Seamless tiling
}

// midjourneyImagineResponse represents the response from /imagine
type midjourneyImagineResponse struct {
	ID        string `json:"id"`
	Status    string `json:"status"` // pending, processing, completed, failed
	Prompt    string `json:"prompt"`
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// midjourneyStatusResponse represents the response from /status
type midjourneyStatusResponse struct {
	ID          string   `json:"id"`
	Status      string   `json:"status"` // pending, processing, completed, failed
	Prompt      string   `json:"prompt"`
	Model       string   `json:"model"`
	ImageURLs   []string `json:"image_urls,omitempty"`
	Progress    int      `json:"progress,omitempty"` // 0-100
	Error       string   `json:"error,omitempty"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at,omitempty"`
	CompletedAt string   `json:"completed_at,omitempty"`
}

// midjourneyErrorResponse represents error responses
type midjourneyErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}

func (p *MidjourneyProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
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

func (p *MidjourneyProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
	url := p.baseURL + endpoint.Path

	var req *http.Request
	var err error

	// For /imagine endpoint, test with minimal payload
	if endpoint.Method == "POST" && endpoint.Path == "/imagine" {
		reqBody := midjourneyImagineRequest{
			Prompt: "test image",
			Model:  "v6",
		}
		bodyBytes, _ := json.Marshal(reqBody)
		req, err = http.NewRequestWithContext(ctx, endpoint.Method, url, bytes.NewReader(bodyBytes))
	} else if endpoint.Method == "GET" && endpoint.Path == "/status/{id}" {
		// For status endpoint, use a test ID
		testURL := p.baseURL + "/status/test-id"
		req, err = http.NewRequestWithContext(ctx, endpoint.Method, testURL, nil)
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

	// Accept 2xx-4xx status codes for testing (includes auth errors which are expected)
	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
}

func (p *MidjourneyProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("Fetching Midjourney models...")
	}

	// Midjourney models are well-known, return static list
	models := []Model{
		{
			ID:             "v6",
			Name:           "Midjourney V6",
			Description:    "Latest Midjourney model with improved photorealism and prompt adherence",
			CostPer1MIn:    0, // Midjourney uses subscription pricing
			CostPer1MOut:   0,
			ContextWindow:  0, // N/A for image generation
			MaxTokens:      0,
			SupportsImages: true,
			SupportsTools:  false,
			CanReason:      false,
			CanStream:      false,
			Categories:     []string{"image-generation", "text-to-image"},
			Capabilities: map[string]string{
				"max_prompt_length": "350",
				"aspect_ratios":     "1:1,16:9,9:16,4:3,3:2",
				"stylize_range":     "0-1000",
				"chaos_range":       "0-100",
				"weird_range":       "0-3000",
			},
		},
		{
			ID:             "v6.1",
			Name:           "Midjourney V6.1",
			Description:    "Enhanced V6 with improved coherence and detail",
			CostPer1MIn:    0,
			CostPer1MOut:   0,
			ContextWindow:  0,
			MaxTokens:      0,
			SupportsImages: true,
			SupportsTools:  false,
			CanReason:      false,
			CanStream:      false,
			Categories:     []string{"image-generation", "text-to-image"},
			Capabilities: map[string]string{
				"max_prompt_length": "350",
				"aspect_ratios":     "1:1,16:9,9:16,4:3,3:2",
				"stylize_range":     "0-1000",
				"chaos_range":       "0-100",
				"weird_range":       "0-3000",
			},
		},
	}

	if verbose {
		fmt.Printf("Found %d Midjourney models\n", len(models))
	}

	return models, nil
}

func (p *MidjourneyProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsChat:         false,
		SupportsFIM:          false,
		SupportsEmbeddings:   false,
		SupportsFineTuning:   false,
		SupportsAgents:       false,
		SupportsFileUpload:   false, // V6 supports image prompts but not in basic API
		SupportsStreaming:    false,
		SupportsJSONMode:     false,
		SupportsVision:       false, // This is for input, Midjourney generates images
		SupportsAudio:        false,
		SupportedParameters:  []string{"prompt", "model", "aspect_ratio", "quality", "stylize", "chaos", "weird", "tile"},
		SecurityFeatures:     []string{"api_key_auth"},
		MaxRequestsPerMinute: 60,
		MaxTokensPerRequest:  350, // Max prompt length
	}
}

func (p *MidjourneyProvider) GetEndpoints() []Endpoint {
	// Return cached endpoints if available
	if len(p.endpoints) > 0 {
		return p.endpoints
	}

	// Otherwise return fresh endpoints
	headers := map[string]string{
		"Authorization": "Bearer " + p.apiKey,
		"Content-Type":  "application/json",
	}

	return []Endpoint{
		{
			Path:        "/imagine",
			Method:      "POST",
			Description: "Generate image from text prompt",
			Headers:     headers,
			Status:      StatusUnknown,
		},
		{
			Path:        "/status/{id}",
			Method:      "GET",
			Description: "Check generation status",
			Headers:     headers,
			Status:      StatusUnknown,
		},
	}
}

func (p *MidjourneyProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("Testing Midjourney model: %s\n", modelID)
	}

	// Validate model ID
	validModels := map[string]bool{
		"v6":   true,
		"v6.1": true,
	}

	if !validModels[modelID] {
		return fmt.Errorf("invalid model ID: %s (valid: v6, v6.1)", modelID)
	}

	// Create a minimal test request
	reqBody := midjourneyImagineRequest{
		Prompt: "test prompt for validation",
		Model:  modelID,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseURL + "/imagine"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var imgResp midjourneyImagineResponse
		if err := json.Unmarshal(body, &imgResp); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		if verbose {
			fmt.Printf("  ✓ Model %s test successful (generation ID: %s)\n", modelID, imgResp.ID)
		}
		return nil
	}

	// Parse error response
	var errResp midjourneyErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return fmt.Errorf("HTTP %d: %s - %s", resp.StatusCode, errResp.Error, errResp.Message)
}

// Imagine creates a new image generation request
func (p *MidjourneyProvider) Imagine(ctx context.Context, req midjourneyImagineRequest) (*midjourneyImagineResponse, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseURL + "/imagine"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var imgResp midjourneyImagineResponse
		if err := json.Unmarshal(body, &imgResp); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
		return &imgResp, nil
	}

	var errResp midjourneyErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil, fmt.Errorf("HTTP %d: %s - %s", resp.StatusCode, errResp.Error, errResp.Message)
}

// GetStatus checks the status of a generation
func (p *MidjourneyProvider) GetStatus(ctx context.Context, generationID string) (*midjourneyStatusResponse, error) {
	url := p.baseURL + "/status/" + generationID
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var statusResp midjourneyStatusResponse
		if err := json.Unmarshal(body, &statusResp); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
		return &statusResp, nil
	}

	var errResp midjourneyErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil, fmt.Errorf("HTTP %d: %s - %s", resp.StatusCode, errResp.Error, errResp.Message)
}
