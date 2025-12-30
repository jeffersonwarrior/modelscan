# SDK Auto-Generation Architecture

Auto-generate working Go provider SDKs from model identifiers or URLs.

## Vision

```bash
# User provides model identifier
modelscan add deepseek/deepseek-coder

# System performs:
# 1. Query models.dev → get metadata (pricing, capabilities)
# 2. Scan API endpoint → discover schema (endpoints, parameters)
# 3. Generate Go code → providers/deepseek_generated.go
# 4. Register with routing → auto-available for use
```

## Current Architecture

### Existing Provider Interface

ModelScan already has a clean provider abstraction in `providers/interface.go`:

```go
type Provider interface {
    ValidateEndpoints(ctx context.Context, verbose bool) error
    ListModels(ctx context.Context, verbose bool) ([]Model, error)
    GetCapabilities() ProviderCapabilities
    GetEndpoints() []Endpoint
    TestModel(ctx context.Context, modelID string, verbose bool) error
}
```

### Existing Implementation Pattern

Providers like `OpenAIProvider`:
1. Wrap external SDK (go-openai)
2. Implement Provider interface
3. Register via `init()` function
4. Enrich models with pricing/capabilities

**Problem**: Each provider requires manual implementation.

**Solution**: Auto-generate providers from model metadata + API scanning.

## Auto-Generation Workflow

```
User Input (deepseek/deepseek-coder)
         │
         ▼
   ┌─────────────────┐
   │ 1. DISCOVER     │  Query models.dev API
   │    Metadata     │  → pricing, capabilities, limits
   └────────┬────────┘
            │
            ▼
   ┌─────────────────┐
   │ 2. DETECT       │  Identify API type
   │    API Type     │  → OpenAI-compatible? Custom?
   └────────┬────────┘
            │
            ▼
   ┌─────────────────┐
   │ 3. SCAN         │  Probe API endpoints
   │    Schema       │  → /v1/models, /v1/chat/completions
   └────────┬────────┘
            │
            ▼
   ┌─────────────────┐
   │ 4. GENERATE     │  Generate Go provider code
   │    Code         │  → providers/{provider}_generated.go
   └────────┬────────┘
            │
            ▼
   ┌─────────────────┐
   │ 5. REGISTER     │  Compile and register
   │    Provider     │  → Available in routing layer
   └─────────────────┘
```

## Key Components

### 1. Metadata Discovery (`catalog/`)

**Purpose**: Fetch model metadata from multiple sources

**Sources**:
- `models.dev` API: pricing, capabilities, limits
- HuggingFace: model cards, configurations
- Provider docs: official API specifications

**Output**:
```go
type ModelMetadata struct {
    ID              string            // "deepseek/deepseek-coder"
    Provider        string            // "deepseek"
    Name            string            // "DeepSeek Coder"
    Pricing         PricingInfo       // from models.dev
    Capabilities    CapabilityInfo    // from models.dev
    APIEndpoint     string            // discovered or inferred
    Documentation   string            // link to docs
}
```

### 2. API Type Detection (`scanner/detector.go`)

**Purpose**: Identify which API pattern the model uses

**Detection Logic**:
```go
func DetectAPIType(endpoint string) APIType {
    // 1. Try OpenAI-compatible endpoints
    if exists("/v1/chat/completions") && exists("/v1/models") {
        return OpenAICompatible
    }

    // 2. Try Anthropic pattern
    if exists("/v1/messages") {
        return AnthropicPattern
    }

    // 3. Try Google Gemini pattern
    if contains("generativelanguage.googleapis.com") {
        return GoogleGeminiPattern
    }

    // 4. Fallback to generic REST
    return CustomREST
}
```

**API Types**:
- `OpenAICompatible` (90% of models)
- `AnthropicPattern`
- `GoogleGeminiPattern`
- `CustomREST`

### 3. API Schema Scanner (`scanner/schema.go`)

**Purpose**: Probe API to discover exact schema

**Approach**:
```go
type APISchema struct {
    BaseURL      string
    Endpoints    []EndpointSpec
    AuthMethod   AuthType        // Bearer, API-Key, OAuth
    Parameters   []ParamSpec
    Responses    []ResponseSpec
}

func ScanAPI(baseURL string, apiKey string) (*APISchema, error) {
    // 1. Discover endpoints
    endpoints := discoverEndpoints(baseURL)

    // 2. For each endpoint, probe with minimal request
    for _, endpoint := range endpoints {
        spec := probeEndpoint(endpoint, apiKey)
        schema.Endpoints = append(schema.Endpoints, spec)
    }

    return schema, nil
}
```

**Probing Strategy**:
- OPTIONS requests for supported methods
- Minimal POST to discover required parameters
- Error response analysis to infer schema
- OpenAPI/Swagger spec if available

### 4. Code Generator (`generator/`)

**Purpose**: Generate Go provider implementation

**Templates**:

#### Template A: OpenAI-Compatible Provider
```go
// providers/{provider}_generated.go
package providers

import (
    "context"
    "github.com/sashabaranov/go-openai"
)

type {{.ProviderName}}Provider struct {
    apiKey  string
    baseURL string
    client  *openai.Client
}

func New{{.ProviderName}}Provider(apiKey string) Provider {
    config := openai.DefaultConfig(apiKey)
    config.BaseURL = "{{.BaseURL}}"

    return &{{.ProviderName}}Provider{
        apiKey:  apiKey,
        baseURL: config.BaseURL,
        client:  openai.NewClientWithConfig(config),
    }
}

func init() {
    RegisterProvider("{{.ProviderID}}", New{{.ProviderName}}Provider)
}

func (p *{{.ProviderName}}Provider) GetCapabilities() ProviderCapabilities {
    return ProviderCapabilities{
        SupportsChat:       {{.Capabilities.Chat}},
        SupportsStreaming:  {{.Capabilities.Streaming}},
        SupportsVision:     {{.Capabilities.Vision}},
        // ... auto-populated from metadata
    }
}

// ListModels, ValidateEndpoints, etc. auto-generated
```

#### Template B: Custom REST Provider
```go
type {{.ProviderName}}Provider struct {
    apiKey     string
    baseURL    string
    httpClient *http.Client
}

func (p *{{.ProviderName}}Provider) ChatCompletion(
    ctx context.Context,
    req ChatRequest,
) (*ChatResponse, error) {
    // Auto-generated based on scanned schema
    payload := map[string]interface{}{
        {{range .Parameters}}
        "{{.Name}}": req.{{.GoField}},
        {{end}}
    }

    resp, err := p.post(ctx, "{{.ChatEndpoint}}", payload)
    // ... error handling and response parsing
}
```

## 3 Key Suggestions

### Suggestion 1: **Leverage OpenAI Compatibility (90% Coverage)**

**Insight**: Most modern LLM providers expose OpenAI-compatible APIs.

**Recommendation**:
- **Primary Path**: Use OpenAI SDK with custom base URL
- **Fast Path**: For OpenAI-compatible models, generation is trivial
- **Fallback**: Only generate custom HTTP clients for non-compatible APIs

**Benefits**:
- Handles 90% of models with minimal code
- Leverages battle-tested go-openai SDK
- Reduces maintenance burden
- Faster generation (template-based)

**Implementation**:
```go
func GenerateProvider(metadata ModelMetadata) (string, error) {
    apiType := scanner.DetectAPIType(metadata.APIEndpoint)

    switch apiType {
    case OpenAICompatible:
        // Use simple template - just change baseURL
        return templates.RenderOpenAICompat(metadata)

    case CustomREST:
        // Full code generation required
        schema := scanner.ScanAPI(metadata.APIEndpoint, apiKey)
        return generator.GenerateCustomClient(schema)
    }
}
```

**Example**:
```bash
# DeepSeek uses OpenAI-compatible API
modelscan add deepseek/deepseek-coder
# → Generates 30-line wrapper using go-openai
# → Works immediately

# Anthropic uses custom API
modelscan add anthropic/claude-sonnet-4-5
# → Full schema scan
# → Generates 200-line custom client
```

### Suggestion 2: **models.dev as Primary Metadata Source**

**Insight**: models.dev is community-maintained, comprehensive, and structured.

**Recommendation**:
- **Use models.dev API as ground truth** for pricing, capabilities, limits
- **Enrich with HuggingFace** for open-weights models
- **Fallback to provider docs** only when needed

**Benefits**:
- Single source of truth for 200+ models
- Active community (196 contributors)
- Structured TOML schema
- Already tracks what we need

**Implementation**:
```go
type CatalogClient struct {
    ModelsDevAPI string // https://models.dev/api.json
    Cache        time.Duration
}

func (c *CatalogClient) GetModel(modelID string) (*ModelMetadata, error) {
    // 1. Fetch from models.dev API
    apiData := fetchModelsDevAPI()
    metadata := parseModelFromAPI(modelID, apiData)

    // 2. Enrich with HuggingFace if open-weights
    if metadata.OpenWeights {
        hfData := fetchHuggingFaceCard(modelID)
        metadata = enrichWithHF(metadata, hfData)
    }

    // 3. Detect API endpoint
    metadata.APIEndpoint = inferAPIEndpoint(metadata.Provider)

    return metadata, nil
}
```

**models.dev API Structure**:
```json
{
  "deepseek/deepseek-coder": {
    "name": "DeepSeek Coder",
    "cost": {
      "input": 0.14,
      "output": 0.28
    },
    "limit": {
      "context": 32000
    },
    "tool_call": true,
    "reasoning": false,
    "open_weights": true
  }
}
```

### Suggestion 3: **Two-Phase Generation (Quick Start + Deep Scan)**

**Insight**: Most users want quick results, but some need full control.

**Recommendation**:
- **Phase 1: Quick Start** - Generate minimal working provider (5 seconds)
- **Phase 2: Deep Scan** - Optional detailed schema analysis (30 seconds)

**Quick Start Workflow**:
```bash
modelscan add deepseek/deepseek-coder
# → Fetches models.dev metadata (1s)
# → Detects OpenAI-compatible (1s)
# → Generates minimal provider (1s)
# → Compiles and registers (2s)
# ✓ Ready to use in 5 seconds
```

**Deep Scan Workflow**:
```bash
modelscan add deepseek/deepseek-coder --deep-scan
# Quick Start (5s)
# → Plus: Full endpoint discovery (10s)
# → Plus: Parameter validation (10s)
# → Plus: Response schema analysis (10s)
# → Generates optimized provider with all features (5s)
# ✓ Complete provider in 40 seconds
```

**Benefits**:
- **Quick Start**: Immediate usability for 90% of use cases
- **Deep Scan**: Full feature coverage when needed
- **Incremental**: Can upgrade Quick → Deep later
- **User Control**: Choose speed vs completeness

**Implementation**:
```go
type GenerationMode int

const (
    QuickStart GenerationMode = iota
    DeepScan
)

func Generate(modelID string, mode GenerationMode) error {
    // Quick Start always happens
    metadata := catalog.GetModel(modelID)
    apiType := scanner.QuickDetect(metadata.APIEndpoint)
    provider := generator.GenerateQuick(metadata, apiType)

    if mode == DeepScan {
        // Additional analysis
        schema := scanner.FullScan(metadata.APIEndpoint, apiKey)
        provider = generator.OptimizeWithSchema(provider, schema)
    }

    return compile(provider)
}
```

## Bonus: Integration with Routing

Auto-generated providers integrate seamlessly:

```go
// After generation, auto-register
provider := NewDeepSeekProvider(apiKey)

// Option 1: Direct routing
router.RegisterClient("deepseek-coder", &DirectClient{
    Provider: provider,
})

// Option 2: Through Plano proxy
config := routing.Config{
    Mode: routing.ModeDirect,
    Direct: &routing.DirectConfig{
        DefaultProvider: "deepseek-coder",
    },
}
```

## Implementation Phases

### Phase 1: Metadata Discovery
- Implement models.dev API client
- HuggingFace API client
- Metadata merging logic

### Phase 2: API Detection
- OpenAI-compatible detector
- Custom API detector
- Fallback logic

### Phase 3: Quick Start Generator
- OpenAI-compatible template
- Basic provider template
- Auto-registration

### Phase 4: Deep Scanner
- Endpoint discovery
- Schema inference
- Parameter validation

### Phase 5: Advanced Generator
- Custom client generation
- Response parsing
- Error handling

## Example: End-to-End

```bash
# Add new provider
$ modelscan add deepseek/deepseek-coder

Discovering model metadata...
✓ Found in models.dev
  - Name: DeepSeek Coder
  - Pricing: $0.14/M in, $0.28/M out
  - Context: 32K tokens
  - Capabilities: Chat, Tools, Streaming

Detecting API type...
✓ OpenAI-compatible API detected
  - Endpoint: https://api.deepseek.com/v1

Generating provider...
✓ Generated providers/deepseek_generated.go (47 lines)

Compiling...
✓ Build successful

Registering...
✓ Provider 'deepseek' registered

Ready to use! Try:
  modelscan route --provider deepseek --model deepseek-coder \
    --prompt "Write a hello world in Go"
```

## Conclusion

This approach provides:
1. **90% automation** via OpenAI compatibility detection
2. **Single source of truth** via models.dev API
3. **Quick Start + Deep Scan** for speed vs completeness tradeoff

Zero external dependencies (except go-openai for OpenAI-compatible providers).
