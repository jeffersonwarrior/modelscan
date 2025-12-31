package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Source represents a data source for provider discovery
type Source interface {
	Fetch(ctx context.Context, identifier string) (SourceResult, error)
	Name() string
	Priority() int // Lower number = higher priority for conflict resolution
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

// SourceStats tracks scraping statistics per source
type SourceStats struct {
	mu          sync.RWMutex
	stats       map[string]*SourceStat
	totalCalls  int
	totalErrors int
}

// SourceStat holds statistics for a single source
type SourceStat struct {
	SourceName   string
	TotalCalls   int
	SuccessCalls int
	FailedCalls  int
	LastSuccess  time.Time
	LastFailure  time.Time
	LastError    string
	AvgLatencyMS int64
	totalLatency int64
}

// NewSourceStats creates a new statistics tracker
func NewSourceStats() *SourceStats {
	return &SourceStats{
		stats: make(map[string]*SourceStat),
	}
}

// RecordSuccess records a successful fetch
func (ss *SourceStats) RecordSuccess(sourceName string, latencyMS int64) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if ss.stats[sourceName] == nil {
		ss.stats[sourceName] = &SourceStat{SourceName: sourceName}
	}

	stat := ss.stats[sourceName]
	stat.TotalCalls++
	stat.SuccessCalls++
	stat.LastSuccess = time.Now()
	stat.totalLatency += latencyMS
	stat.AvgLatencyMS = stat.totalLatency / int64(stat.TotalCalls)

	ss.totalCalls++
}

// RecordFailure records a failed fetch
func (ss *SourceStats) RecordFailure(sourceName string, err error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if ss.stats[sourceName] == nil {
		ss.stats[sourceName] = &SourceStat{SourceName: sourceName}
	}

	stat := ss.stats[sourceName]
	stat.TotalCalls++
	stat.FailedCalls++
	stat.LastFailure = time.Now()
	if err != nil {
		stat.LastError = err.Error()
	}

	ss.totalCalls++
	ss.totalErrors++
}

// GetStats returns a snapshot of current statistics
func (ss *SourceStats) GetStats() map[string]SourceStat {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	snapshot := make(map[string]SourceStat)
	for name, stat := range ss.stats {
		snapshot[name] = *stat
	}
	return snapshot
}

// GetSummary returns overall summary statistics
func (ss *SourceStats) GetSummary() map[string]interface{} {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	successRate := 0.0
	if ss.totalCalls > 0 {
		successRate = float64(ss.totalCalls-ss.totalErrors) / float64(ss.totalCalls) * 100
	}

	return map[string]interface{}{
		"total_calls":   ss.totalCalls,
		"total_errors":  ss.totalErrors,
		"success_rate":  successRate,
		"sources_count": len(ss.stats),
	}
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

func (s *ModelsDevSource) Priority() int {
	return 1 // Highest priority - most reliable pricing/capability data
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

func (s *GPUStackSource) Priority() int {
	return 2 // Hardware/deployment specs
}

func (s *GPUStackSource) Fetch(ctx context.Context, identifier string) (SourceResult, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", s.catalogURL, nil)
	if err != nil {
		return SourceResult{}, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return SourceResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return SourceResult{}, fmt.Errorf("GPUStack returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return SourceResult{}, err
	}

	// Store raw YAML as string in RawData for LLM synthesis
	// Note: We can't parse YAML without external dependencies,
	// but the LLM can extract information from the raw text
	result := SourceResult{
		SourceName: "GPUStack",
		ProviderID: extractProvider(identifier),
		RawData: map[string]interface{}{
			"yaml_catalog": string(body),
			"note":         "Raw YAML catalog - requires LLM synthesis to extract structured data",
		},
	}

	return result, nil
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

func (s *ModelScopeSource) Priority() int {
	return 3 // Chinese model hub - supplementary data
}

func (s *ModelScopeSource) Fetch(ctx context.Context, identifier string) (SourceResult, error) {
	// ModelScope uses format: namespace/model-name
	// API endpoint: /api/v1/models/{namespace}/{model-name}
	providerID := extractProvider(identifier)

	url := fmt.Sprintf("%s/models/%s", s.apiURL, identifier)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return SourceResult{}, err
	}

	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return SourceResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return SourceResult{}, fmt.Errorf("ModelScope returned %d", resp.StatusCode)
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
		SourceName: "ModelScope",
		ProviderID: providerID,
		RawData:    data,
	}

	// Extract basic info if available
	if name, ok := data["name"].(string); ok {
		result.ProviderName = name
	}
	if desc, ok := data["description"].(string); ok {
		result.RawData["description"] = desc
	}

	return result, nil
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

func (s *HuggingFaceSource) Priority() int {
	return 4 // Lowest priority - often requires auth, rate limited
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
	// Remove protocol and domain
	const prefix = "https://huggingface.co/"
	if len(url) > len(prefix) && url[:len(prefix)] == prefix {
		return url[len(prefix):]
	}

	// Try http as well
	const httpPrefix = "http://huggingface.co/"
	if len(url) > len(httpPrefix) && url[:len(httpPrefix)] == httpPrefix {
		return url[len(httpPrefix):]
	}

	// If no recognized prefix, return as-is (might already be repo ID)
	return url
}
