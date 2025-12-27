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

// VoyageAIProvider implements the Provider interface for Voyage AI embeddings
type VoyageAIProvider struct {
	apiKey    string
	baseURL   string
	client    *http.Client
	endpoints []Endpoint
}

// NewVoyageAIProvider creates a new Voyage AI provider instance
func NewVoyageAIProvider(apiKey string) Provider {
	return &VoyageAIProvider{
		apiKey:  apiKey,
		baseURL: "https://api.voyageai.com/v1",
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func init() {
	RegisterProvider("voyageai", NewVoyageAIProvider)
}

// Voyage AI API request/response structures
type voyageEmbeddingRequest struct {
	Input          interface{} `json:"input"` // string or []string
	Model          string      `json:"model"`
	InputType      string      `json:"input_type,omitempty"`      // "query" or "document"
	TruncationType string      `json:"truncation_type,omitempty"` // "start" or "end"
}

type voyageEmbeddingResponse struct {
	Object string               `json:"object"`
	Data   []voyageEmbedding    `json:"data"`
	Model  string               `json:"model"`
	Usage  voyageEmbeddingUsage `json:"usage"`
}

type voyageEmbedding struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

type voyageEmbeddingUsage struct {
	TotalTokens int `json:"total_tokens"`
}

func (p *VoyageAIProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
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

func (p *VoyageAIProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
	url := p.baseURL + endpoint.Path

	var req *http.Request
	var err error

	if endpoint.Method == "POST" {
		// Create minimal test request for embeddings endpoint
		if strings.Contains(endpoint.Path, "/embeddings") {
			reqBody := voyageEmbeddingRequest{
				Input: "test",
				Model: "voyage-2",
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

func (p *VoyageAIProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("  Returning hardcoded Voyage AI embedding models...")
	}

	// Voyage AI doesn't have a models endpoint, so we return hardcoded models
	models := []Model{
		{
			ID:            "voyage-2",
			Name:          "Voyage 2: General-purpose embeddings",
			ContextWindow: 16000,
			SupportsTools: false,
			CanStream:     false,
			Categories:    []string{"embeddings", "general-purpose"},
			Capabilities: map[string]string{
				"embedding_dimension": "1024",
				"max_batch_size":      "128",
			},
		},
		{
			ID:            "voyage-code-2",
			Name:          "Voyage Code 2: Code-optimized embeddings",
			ContextWindow: 16000,
			SupportsTools: false,
			CanStream:     false,
			Categories:    []string{"embeddings", "code"},
			Capabilities: map[string]string{
				"embedding_dimension": "1536",
				"max_batch_size":      "128",
			},
		},
		{
			ID:            "voyage-large-2",
			Name:          "Voyage Large 2: High-performance embeddings",
			ContextWindow: 16000,
			SupportsTools: false,
			CanStream:     false,
			Categories:    []string{"embeddings", "high-performance"},
			Capabilities: map[string]string{
				"embedding_dimension": "1536",
				"max_batch_size":      "128",
			},
		},
	}

	if verbose {
		fmt.Printf("  Found %d models\n", len(models))
	}

	return models, nil
}

func (p *VoyageAIProvider) GetCapabilities() ProviderCapabilities {
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
		SupportedParameters:  []string{"input_type", "truncation_type"},
		SecurityFeatures:     []string{},
		MaxRequestsPerMinute: 60,
		MaxTokensPerRequest:  16000,
	}
}

func (p *VoyageAIProvider) GetEndpoints() []Endpoint {
	if p.endpoints != nil {
		return p.endpoints
	}

	return []Endpoint{
		{
			Path:        "/embeddings",
			Method:      "POST",
			Description: "Create embeddings",
		},
	}
}

func (p *VoyageAIProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("  Testing model: %s\n", modelID)
	}

	url := p.baseURL + "/embeddings"
	reqBody := voyageEmbeddingRequest{
		Input: "test embedding",
		Model: modelID,
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

	var embResp voyageEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if verbose && len(embResp.Data) > 0 {
		fmt.Printf("    Embedding dimension: %d\n", len(embResp.Data[0].Embedding))
		fmt.Printf("    Tokens used: %d\n", embResp.Usage.TotalTokens)
		fmt.Printf("    ✓ Model is working\n")
	}

	return nil
}
