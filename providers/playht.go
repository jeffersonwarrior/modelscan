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

// PlayHTProvider implements the Provider interface for PlayHT TTS
type PlayHTProvider struct {
	userID    string
	apiKey    string
	baseURL   string
	client    *http.Client
	endpoints []Endpoint
}

// NewPlayHTProvider creates a new PlayHT provider instance
// apiKey should be in format "userID:apiKey"
func NewPlayHTProvider(apiKey string) Provider {
	// Split userID:apiKey format
	userID := ""
	authKey := apiKey

	// If apiKey contains ":", split it
	for i := 0; i < len(apiKey); i++ {
		if apiKey[i] == ':' {
			userID = apiKey[:i]
			authKey = apiKey[i+1:]
			break
		}
	}

	return &PlayHTProvider{
		userID:  userID,
		apiKey:  authKey,
		baseURL: "https://api.play.ht/api/v2",
		client: &http.Client{
			Timeout: 60 * time.Second, // Longer timeout for audio generation
		},
	}
}

func init() {
	RegisterProvider("playht", NewPlayHTProvider)
}

// playHTVoice represents a voice from the /voices endpoint
type playHTVoice struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Language string `json:"language,omitempty"`
	Gender   string `json:"gender,omitempty"`
	Age      string `json:"age,omitempty"`
	Accent   string `json:"accent,omitempty"`
	Style    string `json:"style,omitempty"`
}

// playHTTTSRequest represents the request to /tts endpoint
type playHTTTSRequest struct {
	Text         string  `json:"text"`
	Voice        string  `json:"voice,omitempty"`
	Quality      string  `json:"quality,omitempty"`
	OutputFormat string  `json:"output_format,omitempty"`
	Speed        float64 `json:"speed,omitempty"`
	SampleRate   int     `json:"sample_rate,omitempty"`
	VoiceEngine  string  `json:"voice_engine,omitempty"`
}

func (p *PlayHTProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
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

func (p *PlayHTProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
	url := p.baseURL + endpoint.Path

	req, err := http.NewRequestWithContext(ctx, endpoint.Method, url, nil)
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

	// Consider 2xx status codes as success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
}

func (p *PlayHTProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("Fetching PlayHT voices...")
	}

	url := p.baseURL + "/voices"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-USER-ID", p.userID)
	req.Header.Set("AUTHORIZATION", p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var voices []playHTVoice
	if err := json.NewDecoder(resp.Body).Decode(&voices); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Map voices to Model structure
	models := make([]Model, 0, len(voices))
	for _, voice := range voices {
		description := voice.Name
		if voice.Language != "" {
			description += " (" + voice.Language + ")"
		}
		if voice.Gender != "" {
			description += " - " + voice.Gender
		}

		model := Model{
			ID:             voice.ID,
			Name:           voice.Name,
			Description:    description,
			CostPer1MIn:    40.0, // $0.04 per 1K characters = $40 per 1M characters
			CostPer1MOut:   0.0,  // No output cost
			ContextWindow:  5000, // Character limit per request
			MaxTokens:      5000,
			SupportsImages: false,
			SupportsTools:  false,
			CanReason:      false,
			CanStream:      true, // PlayHT supports streaming
			Categories:     []string{"audio", "tts", "voice"},
			Capabilities: map[string]string{
				"language": voice.Language,
				"gender":   voice.Gender,
				"accent":   voice.Accent,
				"style":    voice.Style,
				"age":      voice.Age,
			},
		}

		models = append(models, model)

		if verbose && len(models) <= 5 {
			fmt.Printf("  Found voice: %s (%s)\n", model.Name, voice.Language)
		}
	}

	if verbose {
		fmt.Printf("  Total voices: %d\n", len(models))
	}

	return models, nil
}

func (p *PlayHTProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsChat:         false,
		SupportsFIM:          false,
		SupportsEmbeddings:   false,
		SupportsFineTuning:   false,
		SupportsAgents:       false,
		SupportsFileUpload:   true, // Voice cloning supports file upload
		SupportsStreaming:    true, // Streaming audio output
		SupportsJSONMode:     false,
		SupportsVision:       false,
		SupportsAudio:        true, // Primary capability
		SupportedParameters:  []string{"text", "voice", "quality", "output_format", "speed", "sample_rate", "voice_engine"},
		SecurityFeatures:     []string{"API_key_authentication", "rate_limiting"},
		MaxRequestsPerMinute: 60,
		MaxTokensPerRequest:  5000, // Character limit
	}
}

func (p *PlayHTProvider) GetEndpoints() []Endpoint {
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
				"X-USER-ID":     p.userID,
				"AUTHORIZATION": p.apiKey,
			},
			Status: StatusUnknown,
		},
		{
			Path:        "/tts",
			Method:      "POST",
			Description: "Generate speech from text",
			Headers: map[string]string{
				"X-USER-ID":     p.userID,
				"AUTHORIZATION": p.apiKey,
				"Content-Type":  "application/json",
			},
			Status: StatusUnknown,
		},
	}
}

func (p *PlayHTProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("Testing PlayHT voice: %s\n", modelID)
	}

	// Create a simple TTS request
	request := playHTTTSRequest{
		Text:         "Test",
		Voice:        modelID,
		Quality:      "medium",
		OutputFormat: "mp3",
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseURL + "/tts"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-USER-ID", p.userID)
	req.Header.Set("AUTHORIZATION", p.apiKey)
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

	if verbose {
		fmt.Printf("  ✓ Voice %s is working\n", modelID)
	}

	return nil
}
