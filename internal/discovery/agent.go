package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Agent discovers provider information using LLM-powered analysis
type Agent struct {
	model         string     // claude-sonnet-4-5, gpt-4o, etc.
	sources       []Source   // Data sources to scrape
	validator     *Validator // TDD validation
	cache         *Cache     // Result cache
	parallelBatch int        // Concurrent scraping limit
	maxRetries    int        // Max validation retries
}

// Config holds agent configuration
type Config struct {
	Model         string
	ParallelBatch int
	CacheDays     int
	MaxRetries    int
}

// DiscoveryRequest represents a request to discover a provider
type DiscoveryRequest struct {
	Identifier string // model ID, HuggingFace URL, or provider name
	APIKey     string // optional API key for testing
}

// DiscoveryResult holds discovered provider information
type DiscoveryResult struct {
	Provider      ProviderInfo
	ModelFamilies []ModelFamilyInfo
	Models        []ModelInfo
	SDK           SDKInfo
	Validated     bool
	ValidationLog string
	Sources       []string // Sources used for discovery
	DiscoveredAt  time.Time
}

// ProviderInfo contains provider-level information
type ProviderInfo struct {
	ID            string
	Name          string
	BaseURL       string
	AuthMethod    string // bearer, api-key, oauth
	AuthHeader    string // e.g., "Authorization", "X-API-Key"
	PricingModel  string // pay-per-token, subscription, free
	Documentation string
}

// ModelFamilyInfo contains model family information
type ModelFamilyInfo struct {
	ID          string
	Name        string
	Description string
}

// ModelInfo contains model-level information
type ModelInfo struct {
	ID                 string
	FamilyID           string
	Name               string
	CostPer1MIn        *float64
	CostPer1MOut       *float64
	CostPer1MReasoning *float64
	ContextWindow      *int
	MaxTokens          *int
	Capabilities       []string
}

// SDKInfo contains SDK generation information
type SDKInfo struct {
	Type       string // openai-compatible, anthropic-compatible, custom
	Endpoints  []EndpointInfo
	Parameters map[string]ParameterInfo
}

// EndpointInfo describes an API endpoint
type EndpointInfo struct {
	Path    string
	Method  string
	Purpose string // chat, embeddings, models, etc.
}

// ParameterInfo describes API parameters
type ParameterInfo struct {
	Name     string
	Type     string
	Required bool
	Default  interface{}
}

// NewAgent creates a new discovery agent
func NewAgent(cfg Config) (*Agent, error) {
	if cfg.Model == "" {
		cfg.Model = "claude-sonnet-4-5"
	}
	if cfg.ParallelBatch == 0 {
		cfg.ParallelBatch = 5
	}
	if cfg.CacheDays == 0 {
		cfg.CacheDays = 7
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}

	cache := NewCache(time.Duration(cfg.CacheDays) * 24 * time.Hour)

	sources := []Source{
		NewModelsDevSource(),
		NewGPUStackSource(),
		NewModelScopeSource(),
		NewHuggingFaceSource(),
	}

	validator := NewValidator(cfg.MaxRetries)

	return &Agent{
		model:         cfg.Model,
		sources:       sources,
		validator:     validator,
		cache:         cache,
		parallelBatch: cfg.ParallelBatch,
		maxRetries:    cfg.MaxRetries,
	}, nil
}

// Discover discovers provider information from multiple sources
func (a *Agent) Discover(ctx context.Context, req DiscoveryRequest) (*DiscoveryResult, error) {
	// Check cache first
	if cached, ok := a.cache.Get(req.Identifier); ok {
		return cached, nil
	}

	// Scrape from all sources in parallel
	sourceCh := make(chan SourceResult, len(a.sources))
	errCh := make(chan error, len(a.sources))

	for _, source := range a.sources {
		go func(s Source) {
			result, err := s.Fetch(ctx, req.Identifier)
			if err != nil {
				errCh <- err
				return
			}
			sourceCh <- result
		}(source)
	}

	// Collect results
	var sourceResults []SourceResult
	var failedSources int
	for i := 0; i < len(a.sources); i++ {
		select {
		case result := <-sourceCh:
			sourceResults = append(sourceResults, result)
		case err := <-errCh:
			// Log error but continue with other sources
			failedSources++
			log.Printf("Discovery source failed for %s: %v", req.Identifier, err)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if failedSources > 0 {
		log.Printf("Discovery completed with %d/%d source failures for %s", failedSources, len(a.sources), req.Identifier)
	}

	if len(sourceResults) == 0 {
		return nil, fmt.Errorf("no sources returned data for %s", req.Identifier)
	}

	// Use LLM to synthesize information
	result, err := a.synthesize(ctx, sourceResults)
	if err != nil {
		return nil, fmt.Errorf("synthesis failed: %w", err)
	}

	// Validate with TDD approach
	if req.APIKey != "" {
		validated, log := a.validator.Validate(ctx, result, req.APIKey)
		result.Validated = validated
		result.ValidationLog = log
	}

	result.DiscoveredAt = time.Now()

	// Cache result
	a.cache.Set(req.Identifier, result)

	return result, nil
}

// synthesize uses LLM to combine information from multiple sources
func (a *Agent) synthesize(ctx context.Context, sources []SourceResult) (*DiscoveryResult, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("no sources available")
	}

	// Build prompt from source data
	prompt := a.buildSynthesisPrompt(sources)

	// Call LLM API
	response, err := a.callLLM(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// Parse LLM response into DiscoveryResult
	result, err := a.parseDiscoveryResult(response, sources)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return result, nil
}

// buildSynthesisPrompt creates a prompt from source results
func (a *Agent) buildSynthesisPrompt(sources []SourceResult) string {
	var sb strings.Builder
	sb.WriteString("You are an AI provider discovery agent. Analyze the following information about an AI provider and extract structured data.\n\n")
	sb.WriteString("Source Data:\n")
	for i, src := range sources {
		sb.WriteString(fmt.Sprintf("\n--- Source %d: %s ---\n", i+1, src.SourceName))
		sb.WriteString(fmt.Sprintf("Provider ID: %s\n", src.ProviderID))
		sb.WriteString(fmt.Sprintf("Provider Name: %s\n", src.ProviderName))
		sb.WriteString(fmt.Sprintf("Base URL: %s\n", src.BaseURL))
		sb.WriteString(fmt.Sprintf("Documentation: %s\n", src.DocumentationURL))
		if len(src.RawData) > 0 {
			if rawJSON, err := json.Marshal(src.RawData); err == nil {
				sb.WriteString(fmt.Sprintf("Additional Data: %s\n", string(rawJSON)))
			}
		}
	}

	sb.WriteString("\n\nExtract and return ONLY a JSON object with this structure (no markdown, no explanation):\n")
	sb.WriteString(`{
  "provider": {
    "id": "unique-provider-id",
    "name": "Provider Name",
    "base_url": "https://api.example.com/v1",
    "auth_method": "bearer",
    "auth_header": "Authorization",
    "pricing_model": "pay-per-token",
    "documentation": "https://docs.example.com"
  },
  "sdk_type": "openai-compatible"
}`)

	return sb.String()
}

// callLLM makes an API call to the configured LLM
func (a *Agent) callLLM(ctx context.Context, prompt string) (string, error) {
	// Get API key from environment (injected via psst)
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	// Build request
	reqBody := map[string]interface{}{
		"model": a.model,
		"max_tokens": 4096,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	// Make request
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(apiResp.Content) == 0 {
		return "", fmt.Errorf("empty response from LLM")
	}

	return apiResp.Content[0].Text, nil
}

// parseDiscoveryResult parses LLM JSON response into DiscoveryResult
func (a *Agent) parseDiscoveryResult(llmResponse string, sources []SourceResult) (*DiscoveryResult, error) {
	// Remove markdown code blocks if present
	llmResponse = strings.TrimPrefix(llmResponse, "```json\n")
	llmResponse = strings.TrimPrefix(llmResponse, "```\n")
	llmResponse = strings.TrimSuffix(llmResponse, "\n```")
	llmResponse = strings.TrimSpace(llmResponse)

	var parsed struct {
		Provider struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			BaseURL       string `json:"base_url"`
			AuthMethod    string `json:"auth_method"`
			AuthHeader    string `json:"auth_header"`
			PricingModel  string `json:"pricing_model"`
			Documentation string `json:"documentation"`
		} `json:"provider"`
		SDKType string `json:"sdk_type"`
	}

	if err := json.Unmarshal([]byte(llmResponse), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w (response: %s)", err, llmResponse)
	}

	// Build source names list
	sourceNames := make([]string, len(sources))
	for i, src := range sources {
		sourceNames[i] = src.SourceName
	}

	return &DiscoveryResult{
		Provider: ProviderInfo{
			ID:            parsed.Provider.ID,
			Name:          parsed.Provider.Name,
			BaseURL:       parsed.Provider.BaseURL,
			AuthMethod:    parsed.Provider.AuthMethod,
			AuthHeader:    parsed.Provider.AuthHeader,
			PricingModel:  parsed.Provider.PricingModel,
			Documentation: parsed.Provider.Documentation,
		},
		SDK: SDKInfo{
			Type: parsed.SDKType,
		},
		Sources: sourceNames,
	}, nil
}

// Close cleans up agent resources
func (a *Agent) Close() error {
	return nil
}
