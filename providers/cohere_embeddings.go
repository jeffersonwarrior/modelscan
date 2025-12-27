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

// CohereEmbeddingsProvider implements the Provider interface for Cohere embeddings using internal HTTP client
type CohereEmbeddingsProvider struct {
	apiKey    string
	baseURL   string
	client    *http.Client
	endpoints []Endpoint
}

// NewCohereEmbeddingsProvider creates a new Cohere Embeddings provider instance
func NewCohereEmbeddingsProvider(apiKey string) Provider {
	return &CohereEmbeddingsProvider{
		apiKey:  apiKey,
		baseURL: "https://api.cohere.ai/v1",
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func init() {
	RegisterProvider("cohere_embeddings", NewCohereEmbeddingsProvider)
}

// Cohere API request/response structures
type cohereEmbedRequest struct {
	Texts      []string `json:"texts"`
	Model      string   `json:"model"`
	InputType  string   `json:"input_type,omitempty"`
	Truncate   string   `json:"truncate,omitempty"`
	EmbedTypes []string `json:"embedding_types,omitempty"`
}

type cohereEmbedResponse struct {
	ID         string      `json:"id"`
	Embeddings [][]float64 `json:"embeddings"`
	Texts      []string    `json:"texts"`
	Meta       cohereMeta  `json:"meta,omitempty"`
}

type cohereMeta struct {
	APIVersion  cohereBilledUnits `json:"api_version,omitempty"`
	BilledUnits cohereBilledUnits `json:"billed_units,omitempty"`
}

type cohereBilledUnits struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}

type cohereModelsResponse struct {
	Models        []cohereModel `json:"models"`
	NextPageToken string        `json:"next_page_token,omitempty"`
}

type cohereModel struct {
	Name            string   `json:"name"`
	Endpoints       []string `json:"endpoints,omitempty"`
	FinetuneType    string   `json:"finetuned,omitempty"`
	ContextLength   int      `json:"context_length,omitempty"`
	TokenizerURL    string   `json:"tokenizer_url,omitempty"`
	DefaultEndpoint []string `json:"default_endpoints,omitempty"`
}

func (p *CohereEmbeddingsProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
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

func (p *CohereEmbeddingsProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
	url := p.baseURL + endpoint.Path

	var req *http.Request
	var err error

	if endpoint.Method == "POST" {
		// Create minimal test request for POST endpoints
		if strings.Contains(endpoint.Path, "/embed") {
			reqBody := cohereEmbedRequest{
				Texts:     []string{"test"},
				Model:     "embed-english-v3.0",
				InputType: "search_query",
			}
			body, _ := json.Marshal(reqBody)
			req, err = http.NewRequestWithContext(ctx, endpoint.Method, url, bytes.NewReader(body))
			if err == nil {
				req.Header.Set("Content-Type", "application/json")
			}
		} else {
			req, err = http.NewRequestWithContext(ctx, endpoint.Method, url, nil)
		}
	} else {
		req, err = http.NewRequestWithContext(ctx, endpoint.Method, url, nil)
	}

	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)

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

func (p *CohereEmbeddingsProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("  Fetching available models from Cohere API...")
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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var modelsResp cohereModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]Model, 0)
	for _, apiModel := range modelsResp.Models {
		// Filter for embedding models only
		isEmbedding := false
		for _, endpoint := range apiModel.Endpoints {
			if endpoint == "embed" {
				isEmbedding = true
				break
			}
		}

		if !isEmbedding {
			continue
		}

		model := Model{
			ID:   apiModel.Name,
			Name: p.formatModelName(apiModel.Name),
		}

		// Enrich with pricing and capabilities
		model = p.enrichModelDetails(model)
		models = append(models, model)
	}

	if verbose {
		fmt.Printf("  Found %d embedding models\n", len(models))
	}

	return models, nil
}

// formatModelName creates a human-readable name from model ID
func (p *CohereEmbeddingsProvider) formatModelName(modelID string) string {
	switch {
	case strings.HasPrefix(modelID, "embed-english-v3"):
		return "Cohere Embed English V3: " + modelID
	case strings.HasPrefix(modelID, "embed-multilingual-v3"):
		return "Cohere Embed Multilingual V3: " + modelID
	case strings.HasPrefix(modelID, "embed-english-light-v3"):
		return "Cohere Embed English Light V3: " + modelID
	case strings.HasPrefix(modelID, "embed-multilingual-light-v3"):
		return "Cohere Embed Multilingual Light V3: " + modelID
	default:
		return "Cohere Embedding: " + modelID
	}
}

// enrichModelDetails adds pricing, context window, and capability information
func (p *CohereEmbeddingsProvider) enrichModelDetails(model Model) Model {
	// Set common capabilities for embeddings
	model.SupportsTools = false
	model.CanStream = false
	model.SupportsImages = false
	model.CanReason = false

	// Determine specific details based on model ID
	switch {
	case strings.HasPrefix(model.ID, "embed-english-v3.0"):
		model.CostPer1MIn = 0.10
		model.CostPer1MOut = 0.0
		model.ContextWindow = 512
		model.MaxTokens = 512
		model.Categories = []string{"embeddings", "english", "high-quality"}

	case strings.HasPrefix(model.ID, "embed-multilingual-v3.0"):
		model.CostPer1MIn = 0.10
		model.CostPer1MOut = 0.0
		model.ContextWindow = 512
		model.MaxTokens = 512
		model.Categories = []string{"embeddings", "multilingual", "high-quality"}

	case strings.HasPrefix(model.ID, "embed-english-light-v3.0"):
		model.CostPer1MIn = 0.10
		model.CostPer1MOut = 0.0
		model.ContextWindow = 512
		model.MaxTokens = 512
		model.Categories = []string{"embeddings", "english", "light"}

	case strings.HasPrefix(model.ID, "embed-multilingual-light-v3.0"):
		model.CostPer1MIn = 0.10
		model.CostPer1MOut = 0.0
		model.ContextWindow = 512
		model.MaxTokens = 512
		model.Categories = []string{"embeddings", "multilingual", "light"}

	default:
		// Default values for unknown models
		model.CostPer1MIn = 0.10
		model.CostPer1MOut = 0.0
		model.ContextWindow = 512
		model.MaxTokens = 512
		model.Categories = []string{"embeddings"}
	}

	// Add capabilities metadata
	model.Capabilities = map[string]string{
		"embedding_types": "float,int8,uint8,binary,ubinary",
		"input_types":     "search_document,search_query,classification,clustering",
		"truncate":        "START,END,NONE",
	}

	return model
}

func (p *CohereEmbeddingsProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsChat:         false,
		SupportsFIM:          false,
		SupportsEmbeddings:   true,
		SupportsFineTuning:   false,
		SupportsAgents:       false,
		SupportsFileUpload:   false,
		SupportsStreaming:    false,
		SupportsJSONMode:     false,
		SupportsVision:       false,
		SupportsAudio:        false,
		SupportedParameters:  []string{"input_type", "truncate", "embedding_types"},
		SecurityFeatures:     []string{},
		MaxRequestsPerMinute: 1000,
		MaxTokensPerRequest:  96,
	}
}

func (p *CohereEmbeddingsProvider) GetEndpoints() []Endpoint {
	if p.endpoints != nil {
		return p.endpoints
	}

	return []Endpoint{
		{
			Path:        "/embed",
			Method:      "POST",
			Description: "Create embeddings for text inputs",
		},
		{
			Path:        "/models",
			Method:      "GET",
			Description: "List available models",
		},
	}
}

func (p *CohereEmbeddingsProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("  Testing model: %s\n", modelID)
	}

	url := p.baseURL + "/embed"
	reqBody := cohereEmbedRequest{
		Texts:     []string{"Hello, world!"},
		Model:     modelID,
		InputType: "search_query",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
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

	var embedResp cohereEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if verbose && len(embedResp.Embeddings) > 0 {
		fmt.Printf("    Generated %d embeddings with dimension %d\n", len(embedResp.Embeddings), len(embedResp.Embeddings[0]))
		fmt.Printf("    ✓ Model is working\n")
	}

	return nil
}
