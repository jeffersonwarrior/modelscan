# Discovery Package Documentation

**Package**: `internal/discovery`
**Purpose**: LLM-powered provider discovery with TDD validation
**Stability**: Beta
**Test Coverage**: 70%

---

## Overview

Auto-discovers LLM provider APIs by scraping metadata from 4 sources (models.dev, GPUStack, ModelScope, HuggingFace), using LLM synthesis (Claude 4.5/GPT-4o) to extract API structure, and validating with TDD 3-loop retry approach.

---

## Files

- `agent.go` (60+ lines) - Discovery orchestrator
- `sources.go` - Data source scrapers
- `validator.go` - TDD validation logic
- `cache.go` - Result caching layer
- `*_test.go` - Test files

---

## Discovery Flow

```
User Identifier (e.g., "openai/gpt-4")
    ↓
Parallel Scraping (4 sources)
    ↓
LLM Synthesis (Claude 4.5)
    ↓
TDD Validation (3-loop retry)
    ↓
Cache Result (7 days TTL)
    ↓
Return DiscoveryResult
```

---

## Core Types

### DiscoveryResult

```go
type DiscoveryResult struct {
    Provider    ProviderInfo `json:"provider"`
    Models      []ModelInfo  `json:"models"`
    SDK         SDKInfo      `json:"sdk"`
    Validated   bool         `json:"validated"`
    CachedAt    time.Time    `json:"cached_at"`
}
```

### ProviderInfo

```go
type ProviderInfo struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    BaseURL     string `json:"base_url"`
    AuthMethod  string `json:"auth_method"`
    APIFormat   string `json:"api_format"` // openai|anthropic|custom
}
```

### ModelInfo

```go
type ModelInfo struct {
    ID             string  `json:"id"`
    Name           string  `json:"name"`
    CostPer1MIn    float64 `json:"cost_per_1m_in"`
    CostPer1MOut   float64 `json:"cost_per_1m_out"`
    ContextWindow  int     `json:"context_window"`
    Capabilities   []string `json:"capabilities"`
}
```

---

## Data Sources

### 1. models.dev

**Priority**: Primary source for pricing and capabilities

```go
func scrapeModelsDE(identifier string) (*SourceData, error) {
    // Fetch from models.dev API
    // Extract: pricing, context window, capabilities
}
```

### 2. GPUStack

**Priority**: Deployment specs and quantization

```go
func scrapeGPUStack(identifier string) (*SourceData, error) {
    // Fetch deployment metadata
    // Extract: hardware requirements, quantization levels
}
```

### 3. ModelScope

**Priority**: Chinese model hub

```go
func scrapeModelScope(identifier string) (*SourceData, error) {
    // Fetch from ModelScope API
    // Extract: model cards, configurations
}
```

### 4. HuggingFace

**Priority**: Model cards and configs

```go
func scrapeHuggingFace(identifier string) (*SourceData, error) {
    // Fetch from HuggingFace Hub
    // Extract: model metadata, parameters
}
```

---

## LLM Synthesis

Uses Claude Sonnet 4.5 or GPT-4o to analyze scraped data:

```go
func synthesizeProviderInfo(sources []SourceData) (*ProviderInfo, error) {
    prompt := buildSynthesisPrompt(sources)

    resp, err := llm.Generate(ctx, prompt, llm.Options{
        Model: "claude-sonnet-4-5",
        Temperature: 0.0, // Deterministic
    })

    return parseStructuredResponse(resp)
}
```

**Synthesis Prompt** extracts:
- API endpoint URLs
- Authentication methods
- Request/response schemas
- Model identifiers
- Parameter specifications

---

## TDD Validation

3-loop retry approach:

```go
func validateDiscovery(result *DiscoveryResult) error {
    for attempt := 1; attempt <= 3; attempt++ {
        // Test 1: HTTP connectivity
        if err := testEndpoint(result.Provider.BaseURL); err != nil {
            return retry(attempt, err)
        }

        // Test 2: Auth validation
        if err := testAuth(result.Provider); err != nil {
            return retry(attempt, err)
        }

        // Test 3: Model availability
        if err := testModel(result.Models[0]); err != nil {
            return retry(attempt, err)
        }

        return nil // All tests passed
    }

    return ErrValidationFailed
}
```

---

## Caching

**TTL**: 7 days (configurable)

```go
type Cache struct {
    store  map[string]CacheEntry
    ttl    time.Duration
}

func (c *Cache) Get(identifier string) (*DiscoveryResult, bool) {
    entry, ok := c.store[identifier]
    if !ok || time.Since(entry.CachedAt) > c.ttl {
        return nil, false
    }
    return entry.Result, true
}

func (c *Cache) Set(identifier string, result *DiscoveryResult) {
    c.store[identifier] = CacheEntry{
        Result:   result,
        CachedAt: time.Now(),
    }
}
```

---

## Usage

### Basic Discovery

```go
import "github.com/jeffersonwarrior/modelscan/internal/discovery"

agent := discovery.NewAgent(discovery.Config{
    LLMModel:      "claude-sonnet-4-5",
    ParallelBatch: 5,
    CacheDays:     7,
})

result, err := agent.Discover(ctx, "deepseek/deepseek-coder")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Provider: %s\n", result.Provider.Name)
fmt.Printf("Base URL: %s\n", result.Provider.BaseURL)
fmt.Printf("Models: %d\n", len(result.Models))
```

### With Custom Config

```go
config := discovery.Config{
    LLMModel:      "gpt-4o",
    ParallelBatch: 10,
    CacheDays:     14,
    OutputDir:     "./generated",
}

agent := discovery.NewAgent(config)
result, err := agent.Discover(ctx, identifier)
```

---

## Error Handling

```go
var (
    ErrSourceUnavailable  = errors.New("data source unavailable")
    ErrLLMSynthesisFailed = errors.New("LLM synthesis failed")
    ErrValidationFailed   = errors.New("validation failed after 3 attempts")
    ErrCacheCorrupted     = errors.New("cache data corrupted")
)
```

---

## Performance

### Parallel Scraping

Scrapes 4 sources concurrently:

```go
var wg sync.WaitGroup
results := make(chan *SourceData, 4)

for _, source := range sources {
    wg.Add(1)
    go func(s DataSource) {
        defer wg.Done()
        data, _ := s.Scrape(identifier)
        results <- data
    }(source)
}

wg.Wait()
close(results)
```

### LLM Caching

Identical prompts return cached responses (provider-level caching).

---

## Testing

### Test Coverage

- Source scrapers: 75%
- LLM synthesis: 60%
- TDD validation: 80%
- Cache layer: 85%

**Run tests:**
```bash
go test ./internal/discovery/... -v
go test ./internal/discovery/... -race -cover
```

---

## Dependencies

- `net/http` - HTTP scraping
- `context` - Cancellation
- `encoding/json` - Data parsing
- `sync` - Parallel scraping
- `time` - Caching TTL

**External**: LLM API (Claude/GPT) for synthesis

---

**Last Updated**: December 31, 2025
**Status**: Beta (v0.3.1)
**Integration**: Wired to internal/service, internal/generator
