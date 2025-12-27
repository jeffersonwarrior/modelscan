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

// TTSProvider implements the Provider interface for OpenAI Text-to-Speech
type TTSProvider struct {
	apiKey    string
	baseURL   string
	client    *http.Client
	endpoints []Endpoint
}

// NewTTSProvider creates a new TTS provider instance
func NewTTSProvider(apiKey string) Provider {
	return &TTSProvider{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func init() {
	RegisterProvider("tts", NewTTSProvider)
}

// ttsModelResponse represents the response from /models endpoint
type ttsModelResponse struct {
	Data []ttsModel `json:"data"`
}

type ttsModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ttsSpeechRequest represents the request to /audio/speech endpoint
type ttsSpeechRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
}

func (p *TTSProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
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
					fmt.Printf("    ✓ Working (latency: %v)\n", latency)
				}
			}
			mu.Unlock()
		}(&endpoints[i])
	}

	wg.Wait()
	p.endpoints = endpoints
	return nil
}

func (p *TTSProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
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

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return nil
}

func (p *TTSProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("Fetching TTS models from OpenAI API...")
	}

	url := p.baseURL + "/models"
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
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var modelsResp ttsModelResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	var models []Model
	for _, m := range modelsResp.Data {
		// Only include TTS models
		if m.ID != "tts-1" && m.ID != "tts-1-hd" {
			continue
		}

		var description string
		var cost float64

		if m.ID == "tts-1" {
			description = "Standard quality text-to-speech model"
			cost = 15000.0 // $0.015 per 1K chars = $15 per 1M chars
		} else if m.ID == "tts-1-hd" {
			description = "High definition text-to-speech model"
			cost = 30000.0 // $0.030 per 1K chars = $30 per 1M chars
		}

		model := Model{
			ID:             m.ID,
			Name:           "OpenAI TTS",
			Description:    description,
			CostPer1MIn:    cost,
			CostPer1MOut:   0, // No output cost
			ContextWindow:  4096,
			MaxTokens:      4096,
			SupportsImages: false,
			SupportsTools:  false,
			CanReason:      false,
			CanStream:      true,
			CreatedAt:      time.Unix(m.Created, 0).Format(time.RFC3339),
			Categories:     []string{"audio", "tts", "speech"},
			Capabilities: map[string]string{
				"voices":          "alloy,echo,fable,onyx,nova,shimmer",
				"formats":         "mp3,opus,aac,flac,wav,pcm",
				"speed_range":     "0.25-4.0",
				"max_input_chars": "4096",
			},
		}

		models = append(models, model)

		if verbose {
			fmt.Printf("  Found model: %s (%s)\n", model.ID, model.Name)
		}
	}

	return models, nil
}

func (p *TTSProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsChat:         false,
		SupportsFIM:          false,
		SupportsEmbeddings:   false,
		SupportsFineTuning:   false,
		SupportsAgents:       false,
		SupportsFileUpload:   false,
		SupportsStreaming:    true, // TTS supports streaming audio
		SupportsJSONMode:     false,
		SupportsVision:       false,
		SupportsAudio:        true, // Primary capability
		SupportedParameters:  []string{"model", "input", "voice", "response_format", "speed"},
		SecurityFeatures:     []string{"SOC2", "GDPR"},
		MaxRequestsPerMinute: 50,
		MaxTokensPerRequest:  4096, // Max input characters
	}
}

func (p *TTSProvider) GetEndpoints() []Endpoint {
	return []Endpoint{
		{
			Path:        "/models",
			Method:      "GET",
			Description: "List available models",
			Headers: map[string]string{
				"Authorization": "Bearer " + p.apiKey,
			},
			Status: StatusUnknown,
		},
		{
			Path:        "/audio/speech",
			Method:      "POST",
			Description: "Generate speech from text",
			Headers: map[string]string{
				"Authorization": "Bearer " + p.apiKey,
				"Content-Type":  "application/json",
			},
			Status: StatusUnknown,
		},
	}
}

func (p *TTSProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("Testing TTS model: %s\n", modelID)
	}

	// Create a simple TTS request
	request := ttsSpeechRequest{
		Model: modelID,
		Input: "Test",
		Voice: "alloy",
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseURL + "/audio/speech"
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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	if verbose {
		fmt.Printf("  ✓ Model %s is working\n", modelID)
	}

	return nil
}
