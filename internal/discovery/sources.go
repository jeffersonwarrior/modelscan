package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Source represents a data source for provider discovery
type Source interface {
	Fetch(ctx context.Context, identifier string) (SourceResult, error)
	Name() string
}

// SourceResult holds data from a single source
type SourceResult struct {
	SourceName       string
	ProviderID       string
	ProviderName     string
	BaseURL          string
	DocumentationURL string
	Pricing          *PricingData
	Capabilities     []string
	Models           []ModelData
	RawData          map[string]interface{}
}

// PricingData holds pricing information
type PricingData struct {
	InputPerM      float64
	OutputPerM     float64
	ReasoningPerM  float64
	CacheReadPerM  float64
	CacheWritePerM float64
}

// ModelData holds model information from source
type ModelData struct {
	ID            string
	Name          string
	ContextWindow int
	MaxTokens     int
	Capabilities  []string
}

// ModelsDevSource scrapes models.dev API
type ModelsDevSource struct {
	apiURL     string
	httpClient *http.Client
}

// NewModelsDevSource creates a models.dev source
func NewModelsDevSource() Source {
	return &ModelsDevSource{
		apiURL: "https://models.dev/api.json",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *ModelsDevSource) Name() string {
	return "models.dev"
}

func (s *ModelsDevSource) Fetch(ctx context.Context, identifier string) (SourceResult, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", s.apiURL, nil)
	if err != nil {
		return SourceResult{}, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return SourceResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return SourceResult{}, fmt.Errorf("models.dev returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return SourceResult{}, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return SourceResult{}, err
	}

	// Find model in data
	modelData, ok := data[identifier].(map[string]interface{})
	if !ok {
		return SourceResult{}, fmt.Errorf("model %s not found in models.dev", identifier)
	}

	result := SourceResult{
		SourceName: "models.dev",
		ProviderID: extractProvider(identifier),
		RawData:    modelData,
	}

	// Extract pricing if available
	if cost, ok := modelData["cost"].(map[string]interface{}); ok {
		pricing := &PricingData{}
		if input, ok := cost["input"].(float64); ok {
			pricing.InputPerM = input
		}
		if output, ok := cost["output"].(float64); ok {
			pricing.OutputPerM = output
		}
		result.Pricing = pricing
	}

	// Extract capabilities
	if toolCall, ok := modelData["tool_call"].(bool); ok && toolCall {
		result.Capabilities = append(result.Capabilities, "tool_call")
	}
	if reasoning, ok := modelData["reasoning"].(bool); ok && reasoning {
		result.Capabilities = append(result.Capabilities, "reasoning")
	}

	return result, nil
}

// GPUStackSource scrapes GPUStack model catalog
type GPUStackSource struct {
	catalogURL string
	httpClient *http.Client
}

// NewGPUStackSource creates a GPUStack source
func NewGPUStackSource() Source {
	return &GPUStackSource{
		catalogURL: "https://raw.githubusercontent.com/gpustack/gpustack/main/model-catalog.yaml",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *GPUStackSource) Name() string {
	return "GPUStack"
}

func (s *GPUStackSource) Fetch(ctx context.Context, identifier string) (SourceResult, error) {
	// TODO: Implement YAML parsing for GPUStack catalog
	// For now, return empty result
	return SourceResult{
		SourceName: "GPUStack",
		ProviderID: extractProvider(identifier),
	}, nil
}

// ModelScopeSource scrapes ModelScope
type ModelScopeSource struct {
	apiURL     string
	httpClient *http.Client
}

// NewModelScopeSource creates a ModelScope source
func NewModelScopeSource() Source {
	return &ModelScopeSource{
		apiURL: "https://www.modelscope.ai/api/v1",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *ModelScopeSource) Name() string {
	return "ModelScope"
}

func (s *ModelScopeSource) Fetch(ctx context.Context, identifier string) (SourceResult, error) {
	// TODO: Implement ModelScope API fetching
	// For now, return empty result
	return SourceResult{
		SourceName: "ModelScope",
		ProviderID: extractProvider(identifier),
	}, nil
}

// HuggingFaceSource scrapes HuggingFace
type HuggingFaceSource struct {
	apiURL     string
	httpClient *http.Client
}

// NewHuggingFaceSource creates a HuggingFace source
func NewHuggingFaceSource() Source {
	return &HuggingFaceSource{
		apiURL: "https://huggingface.co/api",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *HuggingFaceSource) Name() string {
	return "HuggingFace"
}

func (s *HuggingFaceSource) Fetch(ctx context.Context, identifier string) (SourceResult, error) {
	// Parse identifier (could be full URL or repo ID)
	repoID := identifier
	if len(repoID) > 0 && repoID[0] == 'h' {
		// Extract repo ID from URL
		// https://huggingface.co/openai/gpt-4 -> openai/gpt-4
		repoID = extractHFRepoID(identifier)
	}

	url := fmt.Sprintf("%s/models/%s", s.apiURL, repoID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return SourceResult{}, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return SourceResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return SourceResult{}, fmt.Errorf("HuggingFace returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return SourceResult{}, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return SourceResult{}, err
	}

	result := SourceResult{
		SourceName: "HuggingFace",
		ProviderID: extractProvider(repoID),
		RawData:    data,
	}

	return result, nil
}

// extractProvider extracts provider ID from model identifier
// e.g., "openai/gpt-4" -> "openai"
func extractProvider(identifier string) string {
	for i, ch := range identifier {
		if ch == '/' {
			return identifier[:i]
		}
	}
	return identifier
}

// extractHFRepoID extracts repo ID from HuggingFace URL
// e.g., "https://huggingface.co/openai/gpt-4" -> "openai/gpt-4"
func extractHFRepoID(url string) string {
	// Simple extraction - find last two path segments
	// This is a placeholder - would need more robust parsing
	return url
}
