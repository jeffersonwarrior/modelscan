package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// Agent discovers provider information using LLM-powered analysis
type Agent struct {
	llm           *LLMSynthesizer // LLM client with fallback
	sources       []Source        // Data sources to scrape
	validator     *Validator      // TDD validation
	cache         *Cache          // In-memory cache (fast)
	db            DB              // Database persistence (durable)
	stats         *SourceStats    // Source scraping statistics
	cacheTTL      time.Duration
	parallelBatch int // Concurrent scraping limit
	maxRetries    int // Max validation retries
}

// DB interface for database operations
type DB interface {
	SaveDiscoveryResult(identifier string, result interface{}, ttl time.Duration) error
	GetDiscoveryResult(identifier string) (map[string]interface{}, bool, error)
}

// Config holds agent configuration
type Config struct {
	ParallelBatch int
	CacheDays     int
	MaxRetries    int
	DB            DB // Database for persistent cache
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
	if cfg.ParallelBatch == 0 {
		cfg.ParallelBatch = 5
	}
	if cfg.CacheDays == 0 {
		cfg.CacheDays = 7
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}

	cacheTTL := time.Duration(cfg.CacheDays) * 24 * time.Hour
	cache := NewCache(cacheTTL)

	sources := []Source{
		NewModelsDevSource(),
		NewGPUStackSource(),
		NewModelScopeSource(),
		NewHuggingFaceSource(),
	}

	validator := NewValidator(cfg.MaxRetries)
	llm := NewLLMSynthesizer()
	stats := NewSourceStats()

	return &Agent{
		llm:           llm,
		sources:       sources,
		validator:     validator,
		cache:         cache,
		db:            cfg.DB,
		stats:         stats,
		cacheTTL:      cacheTTL,
		parallelBatch: cfg.ParallelBatch,
		maxRetries:    cfg.MaxRetries,
	}, nil
}

// Discover discovers provider information from multiple sources
func (a *Agent) Discover(ctx context.Context, req DiscoveryRequest) (*DiscoveryResult, error) {
	// Check in-memory cache first (fastest)
	if cached, ok := a.cache.Get(req.Identifier); ok {
		return cached, nil
	}

	// Check database cache (persistent)
	if a.db != nil {
		if _, found, err := a.db.GetDiscoveryResult(req.Identifier); err == nil && found {
			// TODO: Reconstruct DiscoveryResult from database map
			// For now, just proceed to fresh discovery
			log.Printf("Found cached result in database for %s (reconstruction not yet implemented)", req.Identifier)
		} else if err != nil {
			log.Printf("Database cache lookup failed for %s: %v", req.Identifier, err)
		}
	}

	// Scrape from all sources in parallel
	sourceCh := make(chan SourceResult, len(a.sources))
	errCh := make(chan error, len(a.sources))

	for _, source := range a.sources {
		go func(s Source) {
			start := time.Now()
			result, err := s.Fetch(ctx, req.Identifier)
			latency := time.Since(start).Milliseconds()

			if err != nil {
				a.stats.RecordFailure(s.Name(), err)
				errCh <- err
				return
			}

			a.stats.RecordSuccess(s.Name(), latency)
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

	// Save to database (persistent cache)
	if a.db != nil {
		if err := a.db.SaveDiscoveryResult(req.Identifier, result, a.cacheTTL); err != nil {
			log.Printf("Failed to save discovery result to database for %s: %v", req.Identifier, err)
		}
	}

	// Cache result in memory
	a.cache.Set(req.Identifier, result)

	return result, nil
}

// synthesize uses LLM to combine information from multiple sources
func (a *Agent) synthesize(ctx context.Context, sources []SourceResult) (*DiscoveryResult, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("no sources available")
	}

	// Call LLM API with automatic fallback
	response, err := a.llm.Synthesize(ctx, sources)
	if err != nil {
		return nil, fmt.Errorf("LLM synthesis failed: %w", err)
	}

	// Parse LLM response into DiscoveryResult
	result, err := parseDiscoveryResult(response, sources)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return result, nil
}

// parseDiscoveryResult parses LLM JSON response into DiscoveryResult
func parseDiscoveryResult(llmResponse string, sources []SourceResult) (*DiscoveryResult, error) {
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

// GetSourceStats returns scraping statistics
func (a *Agent) GetSourceStats() map[string]SourceStat {
	return a.stats.GetStats()
}

// GetStatsSummary returns overall statistics summary
func (a *Agent) GetStatsSummary() map[string]interface{} {
	return a.stats.GetSummary()
}

// Close cleans up agent resources
func (a *Agent) Close() error {
	return nil
}
