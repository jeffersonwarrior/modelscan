package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// RealtimeProvider implements the Provider interface for OpenAI Realtime API
type RealtimeProvider struct {
	apiKey    string
	baseURL   string
	wsURL     string
	client    *http.Client
	endpoints []Endpoint
}

// NewRealtimeProvider creates a new Realtime provider instance
func NewRealtimeProvider(apiKey string) Provider {
	return &RealtimeProvider{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1",
		wsURL:   "wss://api.openai.com/v1/realtime",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func init() {
	RegisterProvider("realtime", NewRealtimeProvider)
}

// realtimeModelResponse represents the response from /models endpoint
type realtimeModelResponse struct {
	Data []realtimeModel `json:"data"`
}

type realtimeModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// realtimeSessionRequest represents a session configuration request
type realtimeSessionRequest struct {
	Model        string         `json:"model"`
	Modalities   []string       `json:"modalities,omitempty"`
	Voice        string         `json:"voice,omitempty"`
	InputFormat  string         `json:"input_audio_format,omitempty"`
	OutputFormat string         `json:"output_audio_format,omitempty"`
	Tools        []realtimeTool `json:"tools,omitempty"`
	Temperature  float64        `json:"temperature,omitempty"`
}

type realtimeTool struct {
	Type        string      `json:"type"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

func (p *RealtimeProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
	endpoints := p.GetEndpoints()

	// Parallelize endpoint testing for better performance
	var wg sync.WaitGroup
	var mu sync.Mutex

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
			} else {
				endpoint.Status = StatusWorking
			}
			mu.Unlock()

			if verbose {
				mu.Lock()
				if err != nil {
					fmt.Printf("    ❌ Failed: %v (%.2fs)\n", err, latency.Seconds())
				} else {
					fmt.Printf("    ✅ OK (%.2fs)\n", latency.Seconds())
				}
				mu.Unlock()
			}
		}(&endpoints[i])
	}

	wg.Wait()

	// Store validated endpoints
	p.endpoints = endpoints

	// Check if any critical endpoints failed
	for _, endpoint := range endpoints {
		if endpoint.Status == StatusFailed && endpoint.Path == "/models" {
			return fmt.Errorf("critical endpoint failed: %s %s - %s", endpoint.Method, endpoint.Path, endpoint.Error)
		}
	}

	return nil
}

func (p *RealtimeProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
	url := p.baseURL + endpoint.Path

	req, err := http.NewRequestWithContext(ctx, endpoint.Method, url, nil)
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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (p *RealtimeProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	url := p.baseURL + "/models"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	if verbose {
		fmt.Printf("Fetching models from: %s\n", url)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var modelsResp realtimeModelResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var models []Model
	for _, m := range modelsResp.Data {
		// Filter to only realtime models
		if m.ID != "gpt-4o-realtime-preview" && m.ID != "gpt-4o-realtime-preview-2024-10-01" {
			continue
		}

		model := Model{
			ID:            m.ID,
			Name:          "GPT-4o Realtime",
			Categories:    []string{"realtime", "voice", "conversation", "audio"},
			ContextWindow: 128000,
			MaxTokens:     4096,
			// Realtime API pricing: $5/1M input tokens, $20/1M output tokens
			// Plus $100/1M audio input tokens, $200/1M audio output tokens
			CostPer1MIn:  5.00,  // Text input tokens
			CostPer1MOut: 20.00, // Text output tokens
		}

		models = append(models, model)
	}

	if verbose {
		fmt.Printf("Found %d realtime model(s)\n", len(models))
	}

	return models, nil
}

func (p *RealtimeProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsChat:         true,
		SupportsFIM:          false,
		SupportsEmbeddings:   false,
		SupportsFineTuning:   false,
		SupportsAgents:       true,
		SupportsFileUpload:   false,
		SupportsStreaming:    true, // Realtime is inherently streaming
		SupportsJSONMode:     false,
		SupportsVision:       false,
		SupportsAudio:        true,
		SupportedParameters:  []string{"temperature", "max_tokens"},
		SecurityFeatures:     []string{},
		MaxRequestsPerMinute: 60,
		MaxTokensPerRequest:  128000,
	}
}

func (p *RealtimeProvider) GetEndpoints() []Endpoint {
	return []Endpoint{
		{
			Path:        "/models",
			Method:      "GET",
			Description: "List available realtime models",
			Headers: map[string]string{
				"Authorization": "Bearer " + p.apiKey,
				"Content-Type":  "application/json",
			},
			Status: StatusUnknown,
		},
	}
}

func (p *RealtimeProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	// For realtime API, we validate that the model is available
	models, err := p.ListModels(ctx, verbose)
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}

	for _, model := range models {
		if model.ID == modelID {
			if verbose {
				fmt.Printf("Model %s is available for realtime API\n", modelID)
			}
			return nil
		}
	}

	return fmt.Errorf("model %s not found in realtime models", modelID)
}

// CreateSession creates a new realtime session configuration
func (p *RealtimeProvider) CreateSession(ctx context.Context, req realtimeSessionRequest) error {
	if req.Model == "" {
		req.Model = "gpt-4o-realtime-preview"
	}

	if len(req.Modalities) == 0 {
		req.Modalities = []string{"text", "audio"}
	}

	if req.Voice == "" {
		req.Voice = "alloy"
	}

	if req.InputFormat == "" {
		req.InputFormat = "pcm16"
	}

	if req.OutputFormat == "" {
		req.OutputFormat = "pcm16"
	}

	// Validate session request
	validVoices := map[string]bool{
		"alloy": true, "echo": true, "fable": true,
		"onyx": true, "nova": true, "shimmer": true,
	}

	if !validVoices[req.Voice] {
		return fmt.Errorf("invalid voice: %s", req.Voice)
	}

	validFormats := map[string]bool{
		"pcm16": true, "g711_ulaw": true, "g711_alaw": true,
	}

	if !validFormats[req.InputFormat] {
		return fmt.Errorf("invalid input format: %s", req.InputFormat)
	}

	if !validFormats[req.OutputFormat] {
		return fmt.Errorf("invalid output format: %s", req.OutputFormat)
	}

	return nil
}

// ConnectWebSocket simulates WebSocket connection validation
func (p *RealtimeProvider) ConnectWebSocket(ctx context.Context, model string) error {
	if model == "" {
		model = "gpt-4o-realtime-preview"
	}

	// Validate model is available
	models, err := p.ListModels(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}

	found := false
	for _, m := range models {
		if m.ID == model {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("model %s not available for realtime", model)
	}

	// In a real implementation, this would establish WebSocket connection
	// For testing purposes, we validate the configuration
	url := p.wsURL + "?model=" + model

	// Create HTTP request to validate URL formation (not actual WebSocket upgrade)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create WebSocket request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("OpenAI-Beta", "realtime=v1")

	// We don't actually execute the request, just validate it's well-formed
	if req.URL.Scheme != "wss" && req.URL.Scheme != "https" {
		return fmt.Errorf("invalid WebSocket URL scheme: %s", req.URL.Scheme)
	}

	return nil
}

// SendEvent simulates sending an event to the realtime API
func (p *RealtimeProvider) SendEvent(ctx context.Context, eventType string, eventData interface{}) error {
	validEvents := map[string]bool{
		"session.update":             true,
		"input_audio_buffer.append":  true,
		"input_audio_buffer.commit":  true,
		"input_audio_buffer.clear":   true,
		"conversation.item.create":   true,
		"conversation.item.truncate": true,
		"conversation.item.delete":   true,
		"response.create":            true,
		"response.cancel":            true,
	}

	if !validEvents[eventType] {
		return fmt.Errorf("invalid event type: %s", eventType)
	}

	// Validate event data can be marshaled
	_, err := json.Marshal(eventData)
	if err != nil {
		return fmt.Errorf("invalid event data: %w", err)
	}

	return nil
}

// ReceiveEvent simulates receiving an event from the realtime API
func (p *RealtimeProvider) ReceiveEvent(ctx context.Context) (map[string]interface{}, error) {
	// In a real implementation, this would read from WebSocket
	// For testing purposes, we return a mock event structure
	mockEvent := map[string]interface{}{
		"type": "session.created",
		"session": map[string]interface{}{
			"id":         "sess_001",
			"model":      "gpt-4o-realtime-preview",
			"modalities": []string{"text", "audio"},
			"voice":      "alloy",
		},
	}

	return mockEvent, nil
}
