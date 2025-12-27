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

// RunwayMLProvider implements the Provider interface for Runway ML Gen-2/Gen-3 video generation
type RunwayMLProvider struct {
	apiKey    string
	baseURL   string
	client    *http.Client
	endpoints []Endpoint
}

// NewRunwayMLProvider creates a new Runway ML provider instance
func NewRunwayMLProvider(apiKey string) Provider {
	return &RunwayMLProvider{
		apiKey:  apiKey,
		baseURL: "https://api.runwayml.com/v1",
		client: &http.Client{
			Timeout: 60 * time.Second, // Longer timeout for video generation
		},
	}
}

func init() {
	RegisterProvider("runwayml", NewRunwayMLProvider)
}

// runwayGeneration represents a video generation request/response
type runwayGeneration struct {
	ID            string                 `json:"id,omitempty"`
	Status        string                 `json:"status,omitempty"` // pending, processing, succeeded, failed
	Prompt        string                 `json:"prompt,omitempty"`
	Model         string                 `json:"model,omitempty"` // gen2, gen3
	Duration      int                    `json:"duration,omitempty"`
	AspectRatio   string                 `json:"aspect_ratio,omitempty"` // 16:9, 9:16, 1:1
	CreatedAt     string                 `json:"created_at,omitempty"`
	UpdatedAt     string                 `json:"updated_at,omitempty"`
	VideoURL      string                 `json:"video_url,omitempty"`
	ThumbnailURL  string                 `json:"thumbnail_url,omitempty"`
	MotionControl *runwayMotionControl   `json:"motion_control,omitempty"`
	CameraMotion  *runwayCameraMotion    `json:"camera_motion,omitempty"`
	ImageURL      string                 `json:"image_url,omitempty"` // For image-to-video
	Error         string                 `json:"error,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// runwayMotionControl defines motion parameters for generation
type runwayMotionControl struct {
	Strength   float64 `json:"strength,omitempty"`   // 0.0 to 1.0
	Smoothness float64 `json:"smoothness,omitempty"` // 0.0 to 1.0
}

// runwayCameraMotion defines camera movement parameters
type runwayCameraMotion struct {
	Type      string  `json:"type,omitempty"`      // pan, tilt, zoom, dolly, orbit
	Intensity float64 `json:"intensity,omitempty"` // 0.0 to 1.0
	Direction string  `json:"direction,omitempty"` // left, right, up, down, in, out
}

// runwayGenerationRequest is the request body for creating a generation
type runwayGenerationRequest struct {
	Prompt        string               `json:"prompt"`
	Model         string               `json:"model,omitempty"`
	Duration      int                  `json:"duration,omitempty"`
	AspectRatio   string               `json:"aspect_ratio,omitempty"`
	ImageURL      string               `json:"image_url,omitempty"`
	MotionControl *runwayMotionControl `json:"motion_control,omitempty"`
	CameraMotion  *runwayCameraMotion  `json:"camera_motion,omitempty"`
}

func (p *RunwayMLProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
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

func (p *RunwayMLProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
	url := p.baseURL + endpoint.Path

	var req *http.Request
	var err error

	// For generation endpoint, test with minimal payload to check auth
	if endpoint.Method == "POST" && endpoint.Path == "/generations" {
		reqBody := runwayGenerationRequest{
			Prompt: "test",
			Model:  "gen2",
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

	// Accept both success and client errors for testing
	if endpoint.Method == "POST" {
		if resp.StatusCode >= 200 && resp.StatusCode < 500 {
			return nil
		}
	} else {
		if resp.StatusCode >= 200 && resp.StatusCode < 500 {
			return nil
		}
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
}

func (p *RunwayMLProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("Fetching Runway ML models...")
	}

	// Runway ML currently supports Gen-2 and Gen-3 models
	models := []Model{
		{
			ID:             "gen2",
			Name:           "Gen-2",
			Description:    "Runway Gen-2 for high-quality video generation with motion control",
			CostPer1MIn:    0.0, // Charged per generation, not per token
			CostPer1MOut:   0.0,
			ContextWindow:  2000, // Character limit for prompts
			MaxTokens:      2000,
			SupportsImages: true, // Can use images for image-to-video
			SupportsTools:  false,
			CanReason:      false,
			CanStream:      false, // Video generation is async
			Categories:     []string{"video", "generation", "creative"},
			Capabilities: map[string]string{
				"duration":       "4_seconds_default",
				"max_duration":   "16_seconds",
				"resolution":     "1280x768",
				"aspect_ratios":  "16:9,9:16,1:1",
				"motion_control": "supported",
				"camera_motion":  "supported",
				"image_to_video": "supported",
				"pricing_model":  "per_generation",
			},
		},
		{
			ID:             "gen3",
			Name:           "Gen-3",
			Description:    "Runway Gen-3 for advanced video generation with enhanced motion control",
			CostPer1MIn:    0.0,
			CostPer1MOut:   0.0,
			ContextWindow:  2000,
			MaxTokens:      2000,
			SupportsImages: true,
			SupportsTools:  false,
			CanReason:      false,
			CanStream:      false,
			Categories:     []string{"video", "generation", "creative"},
			Capabilities: map[string]string{
				"duration":       "5_seconds_default",
				"max_duration":   "10_seconds",
				"resolution":     "1280x768",
				"aspect_ratios":  "16:9,9:16,1:1",
				"motion_control": "supported",
				"camera_motion":  "supported",
				"image_to_video": "supported",
				"pricing_model":  "per_generation",
			},
		},
	}

	if verbose {
		fmt.Printf("  Found %d model(s)\n", len(models))
	}

	return models, nil
}

func (p *RunwayMLProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsChat:         false,
		SupportsFIM:          false,
		SupportsEmbeddings:   false,
		SupportsFineTuning:   false,
		SupportsAgents:       false,
		SupportsFileUpload:   true,  // Image URLs for image-to-video
		SupportsStreaming:    false, // Async generation
		SupportsJSONMode:     false,
		SupportsVision:       true, // Image-to-video
		SupportsAudio:        false,
		SupportedParameters:  []string{"prompt", "model", "duration", "aspect_ratio", "image_url", "motion_control", "camera_motion"},
		SecurityFeatures:     []string{"API_key_authentication", "rate_limiting"},
		MaxRequestsPerMinute: 30, // Conservative estimate
		MaxTokensPerRequest:  2000,
	}
}

func (p *RunwayMLProvider) GetEndpoints() []Endpoint {
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

func (p *RunwayMLProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("Testing Runway ML model: %s\n", modelID)
	}

	// Create a test generation request with motion control
	request := runwayGenerationRequest{
		Prompt:      "A cinematic shot of a serene lake at sunset",
		Model:       modelID,
		Duration:    4,
		AspectRatio: "16:9",
		MotionControl: &runwayMotionControl{
			Strength:   0.5,
			Smoothness: 0.7,
		},
		CameraMotion: &runwayCameraMotion{
			Type:      "pan",
			Intensity: 0.3,
			Direction: "right",
		},
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

	var generation runwayGeneration
	if err := json.NewDecoder(resp.Body).Decode(&generation); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if verbose {
		fmt.Printf("  ✓ Generation created: %s (status: %s)\n", generation.ID, generation.Status)
	}

	return nil
}

// GetGenerationStatus retrieves the status of a video generation
func (p *RunwayMLProvider) GetGenerationStatus(ctx context.Context, generationID string) (*runwayGeneration, error) {
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

	var generation runwayGeneration
	if err := json.NewDecoder(resp.Body).Decode(&generation); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &generation, nil
}
