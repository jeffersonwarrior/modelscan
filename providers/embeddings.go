package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// EmbeddingsProvider implements the Provider interface for OpenAI Embeddings
type EmbeddingsProvider struct {
	apiKey    string
	baseURL   string
	client    *http.Client
	endpoints []Endpoint
}

// NewEmbeddingsProvider creates a new Embeddings provider instance
func NewEmbeddingsProvider(apiKey string) Provider {
	return &EmbeddingsProvider{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func init() {
	RegisterProvider("embeddings", NewEmbeddingsProvider)
}

// embeddingsModelResponse represents the response from /models endpoint
type embeddingsModelResponse struct {
	Data []embeddingsModel `json:"data"`
}

type embeddingsModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// embeddingsRequest represents the request to /embeddings endpoint
type embeddingsRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions,omitempty"`
}

// embeddingsResponse represents the response from /embeddings endpoint
type embeddingsResponse struct {
	Object string            `json:"object"`
	Data   []embeddingObject `json:"data"`
	Model  string            `json:"model"`
	Usage  embeddingsUsage   `json:"usage"`
}

type embeddingObject struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

type embeddingsUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

func (p *EmbeddingsProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
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

func (p *EmbeddingsProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
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

func (p *EmbeddingsProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("Fetching embedding models from OpenAI API...")
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

	var modelsResp embeddingsModelResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	var models []Model
	for _, m := range modelsResp.Data {
		// Only include embedding models
		if m.ID != "text-embedding-3-small" &&
			m.ID != "text-embedding-3-large" &&
			m.ID != "text-embedding-ada-002" {
			continue
		}

		model := Model{
			ID:             m.ID,
			Name:           getEmbeddingModelName(m.ID),
			Description:    getEmbeddingModelDescription(m.ID),
			CostPer1MIn:    getEmbeddingModelCost(m.ID),
			CostPer1MOut:   0, // No output cost for embeddings
			ContextWindow:  8191,
			MaxTokens:      0, // Not applicable for embeddings
			SupportsImages: false,
			SupportsTools:  false,
			CanReason:      false,
			CanStream:      false,
			CreatedAt:      time.Unix(m.Created, 0).Format(time.RFC3339),
			Categories:     []string{"embeddings", "text"},
			Capabilities: map[string]string{
				"dimensions":  getEmbeddingDimensions(m.ID),
				"max_input":   "8191 tokens",
				"use_case":    getEmbeddingUseCase(m.ID),
				"output_dims": getEmbeddingDimensions(m.ID),
			},
		}

		models = append(models, model)

		if verbose {
			fmt.Printf("  Found model: %s (%s)\n", model.ID, model.Name)
		}
	}

	return models, nil
}

func (p *EmbeddingsProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsChat:         false,
		SupportsFIM:          false,
		SupportsEmbeddings:   true, // Primary capability
		SupportsFineTuning:   false,
		SupportsAgents:       false,
		SupportsFileUpload:   false,
		SupportsStreaming:    false,
		SupportsJSONMode:     false,
		SupportsVision:       false,
		SupportsAudio:        false,
		SupportedParameters:  []string{"model", "input", "dimensions", "encoding_format", "user"},
		SecurityFeatures:     []string{"SOC2", "GDPR"},
		MaxRequestsPerMinute: 3000,
		MaxTokensPerRequest:  8191,
	}
}

func (p *EmbeddingsProvider) GetEndpoints() []Endpoint {
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
			Path:        "/embeddings",
			Method:      "POST",
			Description: "Create embeddings",
			Headers: map[string]string{
				"Authorization": "Bearer " + p.apiKey,
				"Content-Type":  "application/json",
			},
			Status: StatusUnknown,
		},
	}
}

func (p *EmbeddingsProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("Testing embedding model: %s\n", modelID)
	}

	reqBody := embeddingsRequest{
		Model: modelID,
		Input: []string{"test"},
	}

	// Add dimensions parameter for new models
	if modelID == "text-embedding-3-small" || modelID == "text-embedding-3-large" {
		if modelID == "text-embedding-3-small" {
			reqBody.Dimensions = 1536
		} else {
			reqBody.Dimensions = 3072
		}
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseURL + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
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
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var embResp embeddingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if len(embResp.Data) == 0 {
		return fmt.Errorf("no embeddings in response")
	}

	if verbose {
		fmt.Printf("  ✓ Model %s is working (dimension: %d)\n", modelID, len(embResp.Data[0].Embedding))
	}

	return nil
}

// Helper functions for model metadata
func getEmbeddingModelName(modelID string) string {
	switch modelID {
	case "text-embedding-3-small":
		return "Text Embedding 3 Small"
	case "text-embedding-3-large":
		return "Text Embedding 3 Large"
	case "text-embedding-ada-002":
		return "Text Embedding Ada 002"
	default:
		return modelID
	}
}

func getEmbeddingModelDescription(modelID string) string {
	switch modelID {
	case "text-embedding-3-small":
		return "Smaller, more efficient embedding model with 1536 dimensions"
	case "text-embedding-3-large":
		return "Most capable embedding model with up to 3072 dimensions"
	case "text-embedding-ada-002":
		return "Legacy embedding model with 1536 dimensions"
	default:
		return "OpenAI embedding model"
	}
}

func getEmbeddingModelCost(modelID string) float64 {
	switch modelID {
	case "text-embedding-3-small":
		return 20.0 // $0.00002 per 1K tokens = $0.02 per 1M tokens
	case "text-embedding-3-large":
		return 130.0 // $0.00013 per 1K tokens = $0.13 per 1M tokens
	case "text-embedding-ada-002":
		return 100.0 // $0.0001 per 1K tokens = $0.10 per 1M tokens
	default:
		return 0.0
	}
}

func getEmbeddingDimensions(modelID string) string {
	switch modelID {
	case "text-embedding-3-small":
		return "1536 (configurable: 512-1536)"
	case "text-embedding-3-large":
		return "3072 (configurable: 256-3072)"
	case "text-embedding-ada-002":
		return "1536"
	default:
		return "unknown"
	}
}

func getEmbeddingUseCase(modelID string) string {
	switch modelID {
	case "text-embedding-3-small":
		return "semantic search, clustering, recommendations (cost-effective)"
	case "text-embedding-3-large":
		return "semantic search, clustering, recommendations (highest quality)"
	case "text-embedding-ada-002":
		return "semantic search, clustering, recommendations (legacy)"
	default:
		return "general purpose"
	}
}
