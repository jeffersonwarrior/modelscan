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

// ElevenLabsProvider implements the Provider interface for ElevenLabs TTS
type ElevenLabsProvider struct {
	apiKey    string
	baseURL   string
	client    *http.Client
	endpoints []Endpoint
}

// NewElevenLabsProvider creates a new ElevenLabs provider instance
func NewElevenLabsProvider(apiKey string) Provider {
	return &ElevenLabsProvider{
		apiKey:  apiKey,
		baseURL: "https://api.elevenlabs.io/v1",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func init() {
	RegisterProvider("elevenlabs", NewElevenLabsProvider)
}

// elevenLabsVoicesResponse represents the response from /voices endpoint
type elevenLabsVoicesResponse struct {
	Voices []elevenLabsVoice `json:"voices"`
}

type elevenLabsVoice struct {
	VoiceID     string            `json:"voice_id"`
	Name        string            `json:"name"`
	Category    string            `json:"category,omitempty"`
	Description string            `json:"description,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// elevenLabsModelsResponse represents the response from /models endpoint
type elevenLabsModelsResponse struct {
	Models []elevenLabsModel `json:"models"`
}

type elevenLabsModel struct {
	ModelID     string `json:"model_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// elevenLabsUserResponse represents the response from /user/subscription endpoint
type elevenLabsUserResponse struct {
	CharacterCount              int    `json:"character_count"`
	CharacterLimit              int    `json:"character_limit"`
	Tier                        string `json:"tier,omitempty"`
	Status                      string `json:"status,omitempty"`
	NextCharacterCountResetUnix int64  `json:"next_character_count_reset_unix,omitempty"`
}

func (p *ElevenLabsProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
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

func (p *ElevenLabsProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("  Fetching available voices from ElevenLabs API...")
	}

	// Call the /voices endpoint to get available voices (these act as models)
	url := p.baseURL + "/voices"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("xi-api-key", p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list voices: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var voicesResp elevenLabsVoicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&voicesResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Map voices to our Model structure
	models := make([]Model, 0, len(voicesResp.Voices))
	for _, voice := range voicesResp.Voices {
		model := Model{
			ID:          voice.VoiceID,
			Name:        voice.Name,
			Description: voice.Description,
			// ElevenLabs pricing is character-based: $180 per 1M characters for Creator tier
			CostPer1MIn:    180.0, // Character-based billing
			CostPer1MOut:   0.0,   // No output tokens, just audio
			ContextWindow:  5000,  // Character limit per request
			SupportsImages: false,
			SupportsTools:  false,
			CanReason:      false,
			CanStream:      true,
			Categories:     []string{"tts", "voice"},
		}

		if voice.Category != "" {
			model.Categories = append(model.Categories, voice.Category)
		}

		models = append(models, model)
	}

	if verbose {
		fmt.Printf("  Found %d voices\n", len(models))
	}

	return models, nil
}

func (p *ElevenLabsProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsChat:         false,
		SupportsFIM:          false,
		SupportsEmbeddings:   false,
		SupportsFineTuning:   false,
		SupportsAgents:       false,
		SupportsFileUpload:   true, // Voice cloning supports file upload
		SupportsStreaming:    true,
		SupportsJSONMode:     false,
		SupportsVision:       false,
		SupportsAudio:        true, // Primary capability
		SupportedParameters:  []string{"voice_id", "model_id", "voice_settings", "stability", "similarity_boost"},
		SecurityFeatures:     []string{"voice_verification", "rate_limiting"},
		MaxRequestsPerMinute: 20,
		MaxTokensPerRequest:  5000, // Character limit
	}
}

func (p *ElevenLabsProvider) GetEndpoints() []Endpoint {
	// Return cached endpoints if available
	if len(p.endpoints) > 0 {
		return p.endpoints
	}

	// Otherwise return fresh endpoints
	return []Endpoint{
		{
			Path:        "/voices",
			Method:      "GET",
			Description: "List available voices",
			Headers: map[string]string{
				"xi-api-key": p.apiKey,
			},
		},
		{
			Path:        "/models",
			Method:      "GET",
			Description: "List available TTS models",
			Headers: map[string]string{
				"xi-api-key": p.apiKey,
			},
		},
		{
			Path:        "/user/subscription",
			Method:      "GET",
			Description: "Get subscription and quota information",
			Headers: map[string]string{
				"xi-api-key": p.apiKey,
			},
		},
		{
			Path:        "/history",
			Method:      "GET",
			Description: "Get generation history",
			Headers: map[string]string{
				"xi-api-key": p.apiKey,
			},
		},
	}
}

func (p *ElevenLabsProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("  Testing voice: %s\n", modelID)
	}

	// Create test request for text-to-speech
	requestBody := map[string]interface{}{
		"text":     "Test",
		"model_id": "eleven_monolingual_v1",
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.baseURL + "/text-to-speech/" + modelID
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("xi-api-key", p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("voice test failed with status %d: %s", resp.StatusCode, string(body))
	}

	if verbose {
		fmt.Printf("    ✓ Voice is working\n")
	}

	return nil
}

func (p *ElevenLabsProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
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
