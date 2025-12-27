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

// FALExtendedProvider implements the Provider interface for FAL.ai using internal HTTP client
type FALExtendedProvider struct {
	apiKey    string
	baseURL   string
	client    *http.Client
	endpoints []Endpoint
}

// NewFALExtendedProvider creates a new FAL Extended provider instance
func NewFALExtendedProvider(apiKey string) Provider {
	return &FALExtendedProvider{
		apiKey:  apiKey,
		baseURL: "https://fal.run",
		client: &http.Client{
			Timeout: 120 * time.Second, // Longer timeout for image/video generation
		},
	}
}

func init() {
	RegisterProvider("fal_extended", NewFALExtendedProvider)
}

// FAL API response structures
type falGenerationRequest struct {
	Prompt         string  `json:"prompt"`
	ImageSize      string  `json:"image_size,omitempty"`
	NumImages      int     `json:"num_images,omitempty"`
	NumInference   int     `json:"num_inference_steps,omitempty"`
	GuidanceScale  float64 `json:"guidance_scale,omitempty"`
	Seed           int64   `json:"seed,omitempty"`
	EnableSafety   bool    `json:"enable_safety_checker,omitempty"`
	NegativePrompt string  `json:"negative_prompt,omitempty"`
}

type falGenerationResponse struct {
	RequestID string     `json:"request_id"`
	Images    []falImage `json:"images"`
	Seed      int64      `json:"seed,omitempty"`
	HasNSFW   bool       `json:"has_nsfw_concepts,omitempty"`
	Prompt    string     `json:"prompt,omitempty"`
}

type falImage struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
}

type falVideoRequest struct {
	Prompt         string  `json:"prompt"`
	NegativePrompt string  `json:"negative_prompt,omitempty"`
	NumFrames      int     `json:"num_frames,omitempty"`
	FPS            int     `json:"fps,omitempty"`
	GuidanceScale  float64 `json:"guidance_scale,omitempty"`
	Seed           int64   `json:"seed,omitempty"`
}

type falVideoResponse struct {
	RequestID string   `json:"request_id"`
	Video     falVideo `json:"video"`
	Seed      int64    `json:"seed,omitempty"`
	Prompt    string   `json:"prompt,omitempty"`
}

type falVideo struct {
	URL         string  `json:"url"`
	ContentType string  `json:"content_type,omitempty"`
	Width       int     `json:"width,omitempty"`
	Height      int     `json:"height,omitempty"`
	Duration    float64 `json:"duration,omitempty"`
}

func (p *FALExtendedProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
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

func (p *FALExtendedProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
	url := p.baseURL + endpoint.Path

	var req *http.Request
	var err error

	if endpoint.Method == "POST" {
		// Create minimal test request for POST endpoints
		if strings.Contains(endpoint.Path, "/fal-ai/flux") || strings.Contains(endpoint.Path, "/fal-ai/stable-diffusion") {
			reqBody := falGenerationRequest{
				Prompt:    "test",
				NumImages: 1,
			}
			body, _ := json.Marshal(reqBody)
			req, err = http.NewRequestWithContext(ctx, endpoint.Method, url, bytes.NewReader(body))
			if err == nil {
				req.Header.Set("Content-Type", "application/json")
			}
		} else if strings.Contains(endpoint.Path, "/fal-ai/animatediff") {
			reqBody := falVideoRequest{
				Prompt:    "test",
				NumFrames: 16,
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

	req.Header.Set("Authorization", "Key "+p.apiKey)

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

func (p *FALExtendedProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("  Listing FAL.ai models (hardcoded catalog)...")
	}

	// FAL.ai doesn't have a models endpoint, so we provide a hardcoded catalog
	models := []Model{
		{
			ID:             "fal-ai/flux-pro",
			Name:           "FLUX Pro",
			Description:    "FLUX Pro - Highest quality text-to-image model",
			ContextWindow:  0,
			MaxTokens:      0,
			CostPer1MIn:    0,
			CostPer1MOut:   0.055, // $0.055 per image
			SupportsImages: true,
			SupportsTools:  false,
			CanStream:      false,
			CanReason:      false,
			Categories:     []string{"image-generation", "premium"},
			CreatedAt:      time.Now().Format(time.RFC3339),
			Capabilities: map[string]string{
				"resolution": "up to 2048x2048",
				"speed":      "fast",
				"quality":    "highest",
			},
		},
		{
			ID:             "fal-ai/flux-dev",
			Name:           "FLUX Dev",
			Description:    "FLUX Dev - High quality text-to-image for development",
			ContextWindow:  0,
			MaxTokens:      0,
			CostPer1MIn:    0,
			CostPer1MOut:   0.025, // $0.025 per image
			SupportsImages: true,
			SupportsTools:  false,
			CanStream:      false,
			CanReason:      false,
			Categories:     []string{"image-generation", "development"},
			CreatedAt:      time.Now().Format(time.RFC3339),
			Capabilities: map[string]string{
				"resolution": "up to 2048x2048",
				"speed":      "medium",
				"quality":    "high",
			},
		},
		{
			ID:             "fal-ai/flux-schnell",
			Name:           "FLUX Schnell",
			Description:    "FLUX Schnell - Ultra-fast text-to-image generation",
			ContextWindow:  0,
			MaxTokens:      0,
			CostPer1MIn:    0,
			CostPer1MOut:   0.003, // $0.003 per image
			SupportsImages: true,
			SupportsTools:  false,
			CanStream:      false,
			CanReason:      false,
			Categories:     []string{"image-generation", "fast", "cost-effective"},
			CreatedAt:      time.Now().Format(time.RFC3339),
			Capabilities: map[string]string{
				"resolution": "up to 1024x1024",
				"speed":      "ultra-fast",
				"quality":    "good",
			},
		},
		{
			ID:             "fal-ai/stable-diffusion-v3-medium",
			Name:           "Stable Diffusion v3 Medium",
			Description:    "SD v3 Medium - Balanced quality and speed",
			ContextWindow:  0,
			MaxTokens:      0,
			CostPer1MIn:    0,
			CostPer1MOut:   0.035,
			SupportsImages: true,
			SupportsTools:  false,
			CanStream:      false,
			CanReason:      false,
			Categories:     []string{"image-generation", "stable-diffusion"},
			CreatedAt:      time.Now().Format(time.RFC3339),
			Capabilities: map[string]string{
				"resolution": "up to 1024x1024",
				"speed":      "medium",
				"quality":    "balanced",
			},
		},
		{
			ID:             "fal-ai/animatediff",
			Name:           "AnimateDiff",
			Description:    "AnimateDiff - Text-to-video generation",
			ContextWindow:  0,
			MaxTokens:      0,
			CostPer1MIn:    0,
			CostPer1MOut:   0.15, // $0.15 per video
			SupportsImages: false,
			SupportsTools:  false,
			CanStream:      false,
			CanReason:      false,
			Categories:     []string{"video-generation"},
			CreatedAt:      time.Now().Format(time.RFC3339),
			Capabilities: map[string]string{
				"resolution": "512x512",
				"duration":   "up to 3 seconds",
				"fps":        "8-16",
			},
		},
	}

	if verbose {
		fmt.Printf("  Found %d models\n", len(models))
	}

	return models, nil
}

func (p *FALExtendedProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsChat:         false,
		SupportsFIM:          false,
		SupportsEmbeddings:   false,
		SupportsFineTuning:   false,
		SupportsAgents:       false,
		SupportsFileUpload:   true,
		SupportsStreaming:    false,
		SupportsJSONMode:     true,
		SupportsVision:       false,
		SupportsAudio:        false,
		SupportedParameters:  []string{"prompt", "image_size", "num_images", "guidance_scale", "negative_prompt", "seed"},
		SecurityFeatures:     []string{"safety_checker"},
		MaxRequestsPerMinute: 30,
		MaxTokensPerRequest:  0,
	}
}

func (p *FALExtendedProvider) GetEndpoints() []Endpoint {
	if p.endpoints != nil {
		return p.endpoints
	}

	return []Endpoint{
		{
			Path:        "/fal-ai/flux-pro",
			Method:      "POST",
			Description: "Generate images with FLUX Pro",
		},
		{
			Path:        "/fal-ai/flux-dev",
			Method:      "POST",
			Description: "Generate images with FLUX Dev",
		},
		{
			Path:        "/fal-ai/flux-schnell",
			Method:      "POST",
			Description: "Generate images with FLUX Schnell",
		},
		{
			Path:        "/fal-ai/stable-diffusion-v3-medium",
			Method:      "POST",
			Description: "Generate images with SD v3 Medium",
		},
		{
			Path:        "/fal-ai/animatediff",
			Method:      "POST",
			Description: "Generate videos with AnimateDiff",
		},
	}
}

func (p *FALExtendedProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("  Testing model: %s\n", modelID)
	}

	url := p.baseURL + "/" + modelID

	var reqBody interface{}
	if strings.Contains(modelID, "animatediff") {
		reqBody = falVideoRequest{
			Prompt:    "test generation",
			NumFrames: 16,
			FPS:       8,
		}
	} else {
		reqBody = falGenerationRequest{
			Prompt:    "test generation",
			NumImages: 1,
			ImageSize: "square_hd",
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Key "+p.apiKey)
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

	// Try to decode as image response first
	var imgResp falGenerationResponse
	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &imgResp); err == nil && len(imgResp.Images) > 0 {
		if verbose {
			fmt.Printf("    Generated %d image(s)\n", len(imgResp.Images))
			fmt.Printf("    ✓ Model is working\n")
		}
		return nil
	}

	// Try video response
	var vidResp falVideoResponse
	if err := json.Unmarshal(bodyBytes, &vidResp); err == nil && vidResp.Video.URL != "" {
		if verbose {
			fmt.Printf("    Generated video: %s\n", vidResp.Video.URL)
			fmt.Printf("    ✓ Model is working\n")
		}
		return nil
	}

	if verbose {
		fmt.Printf("    ✓ Model is working\n")
	}

	return nil
}
