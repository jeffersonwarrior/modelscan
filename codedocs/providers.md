# Providers Package Documentation

**Package**: `providers`
**Purpose**: Unified interface for LLM provider integrations with validation and model listing
**Stability**: Production
**Test Coverage**: Varied (46+ files with dedicated test files)

---

## Overview

The providers package implements a unified interface for 20+ AI/LLM providers, enabling endpoint validation, model discovery, and capability detection. Each provider implements the `Provider` interface for consistent interaction.

---

## Core Interface

**File**: `providers/interface.go` (108 lines)

```go
type Provider interface {
    ValidateEndpoints(ctx context.Context, verbose bool) error
    ListModels(ctx context.Context, verbose bool) ([]Model, error)
    GetCapabilities() ProviderCapabilities
    GetEndpoints() []Endpoint
    TestModel(ctx context.Context, modelID string, verbose bool) error
}
```

### Key Methods

**ValidateEndpoints** - Parallel endpoint validation with latency tracking
**ListModels** - Discover available models with pricing and capabilities
**GetCapabilities** - Provider feature matrix
**GetEndpoints** - Return configured API endpoints
**TestModel** - Validate specific model availability

---

## Provider Implementations

### Chat/Completion Providers

**OpenAI** (`openai.go`, `openai_extended.go`)
- GPT-4, GPT-3.5, GPT-4o models
- Full tool calling support
- Streaming and batch endpoints

**Anthropic** (`anthropic.go`, `anthropic_extended.go`)
- Claude 3.5 Sonnet, Opus, Haiku
- Extended tool use patterns
- Message batching API

**Mistral** (`mistral.go`)
- Mistral Large, Medium, Small
- Function calling
- European data residency

**Google Gemini** (`google.go`, `google_thinking.go`)
- Gemini Pro, Flash, Ultra
- Extended thinking mode
- Multimodal capabilities

**DeepSeek** (`deepseek_extended.go`)
- DeepSeek Coder, Chat
- Extended context support
- Code-specific optimizations

**Cerebras** (`cerebras_extended.go`)
- Llama 3.1 models on Cerebras hardware
- Ultra-low latency inference
- Extended parameter support

---

### Speech/Audio Providers

**Whisper** (`whisper.go`)
- OpenAI Whisper speech-to-text
- Multiple language support
- Transcription and translation

**Deepgram** (`deepgram.go`)
- Real-time speech recognition
- Streaming transcription
- Custom model training

**ElevenLabs** (`elevenlabs.go`)
- Text-to-speech synthesis
- Voice cloning
- Multi-language TTS

**PlayHT** (`playht.go`)
- Ultra-realistic voice synthesis
- Custom voice creation
- SSML support

**TTS** (`tts.go`)
- Generic text-to-speech interface
- Multiple provider backends

---

### Embeddings Providers

**Embeddings** (`embeddings.go`)
- Generic embedding interface
- Vector dimension configuration
- Batch processing

**Cohere Embeddings** (`cohere_embeddings.go`)
- Semantic search optimized
- Multilingual embeddings
- Classification support

**VoyageAI** (`voyageai.go`)
- Domain-specific embeddings
- Retrieval-optimized vectors
- Custom fine-tuning

---

### Specialized Providers

**Realtime** (`realtime.go`)
- OpenAI Realtime API
- Bidirectional streaming
- Function calling in real-time

**Fal** (`fal_extended.go`)
- AI media generation
- Image and video models
- Extended parameter control

**LumaAI** (`lumaai.go`)
- Video generation
- Scene understanding
- Creative automation

**RunwayML** (`runwayml.go`)
- Generative video tools
- Image-to-video
- Style transfer

**Midjourney** (`midjourney.go`)
- Image generation (unofficial API)
- Prompt optimization
- Style parameters

---

## Core Types

### Model

```go
type Model struct {
    ID             string            // Unique model identifier
    Name           string            // Display name
    Description    string            // Model description
    CostPer1MIn    float64          // Input cost per 1M tokens
    CostPer1MOut   float64          // Output cost per 1M tokens
    ContextWindow  int              // Max context tokens
    MaxTokens      int              // Max output tokens
    SupportsImages bool             // Vision capability
    SupportsTools  bool             // Function calling
    CanReason      bool             // Extended reasoning
    CanStream      bool             // Streaming support
    Categories     []string         // e.g., ["coding", "chat"]
    Capabilities   map[string]string // Feature flags
}
```

### Endpoint

```go
type Endpoint struct {
    Path        string            // API endpoint path
    Method      string            // HTTP method
    Description string            // Endpoint purpose
    Headers     map[string]string // Required headers
    TestParams  interface{}       // Validation payload
    Status      EndpointStatus    // Validation result
    Latency     time.Duration     // Response time
    Error       string            // Error message if failed
}

const (
    StatusUnknown    EndpointStatus = "unknown"
    StatusWorking    EndpointStatus = "working"
    StatusFailed     EndpointStatus = "failed"
    StatusDeprecated EndpointStatus = "deprecated"
)
```

### ProviderCapabilities

```go
type ProviderCapabilities struct {
    SupportsChat         bool // Chat completions
    SupportsFIM          bool // Fill-in-the-middle
    SupportsEmbeddings   bool // Vector embeddings
    SupportsFineTuning   bool // Custom model training
    SupportsAgents       bool // Agent frameworks
    SupportsFileUpload   bool // File attachments
    SupportsStreaming    bool // SSE streaming
    SupportsJSONMode     bool // Structured output
    SupportsVision       bool // Image input
    SupportsAudio        bool // Audio processing
    SupportsVideoInput   bool // Video understanding
    MaxToolsSupported    int  // Tool calling limit
    SupportedLanguages   []string // Model languages
}
```

---

## Factory Pattern

**File**: `providers/utils.go`

```go
// RegisterProvider registers a provider factory
func RegisterProvider(name string, factory func(apiKey string) Provider)

// GetProvider creates provider instance
func GetProvider(name string, apiKey string) (Provider, error)
```

**Usage:**
```go
provider, err := providers.GetProvider("openai", apiKey)
if err != nil {
    log.Fatal(err)
}

models, err := provider.ListModels(ctx, false)
```

---

## Validation Workflow

1. **Initialize Provider** - Create with API key
2. **Parallel Endpoint Check** - Test all endpoints concurrently
3. **Latency Measurement** - Track response times
4. **Model Discovery** - List available models
5. **Capability Detection** - Determine feature support
6. **Result Aggregation** - Combine validation data

**Example:**
```go
provider := openai.New(apiKey)

// Validate all endpoints
if err := provider.ValidateEndpoints(ctx, true); err != nil {
    log.Printf("Validation failed: %v", err)
}

// Get validated endpoints
endpoints := provider.GetEndpoints()
for _, ep := range endpoints {
    fmt.Printf("%s %s: %s (latency: %v)\n",
        ep.Method, ep.Path, ep.Status, ep.Latency)
}

// List models with pricing
models, _ := provider.ListModels(ctx, false)
for _, m := range models {
    fmt.Printf("Model: %s ($%.4f/$%.4f per 1M)\n",
        m.ID, m.CostPer1MIn, m.CostPer1MOut)
}
```

---

## Provider Categories

| Category | Count | Examples |
|----------|-------|----------|
| Chat/Completion | 6 | OpenAI, Anthropic, Google, Mistral, DeepSeek, Cerebras |
| Speech/Audio | 5 | Whisper, Deepgram, ElevenLabs, PlayHT, TTS |
| Embeddings | 3 | Embeddings, Cohere, VoyageAI |
| Media Generation | 4 | Fal, LumaAI, RunwayML, Midjourney |
| Specialized | 2 | Realtime (streaming), Extended variants |

---

## Testing

Each provider has dedicated test files (`*_test.go`):
- Unit tests with mock HTTP servers
- Integration tests with real APIs (when API keys available)
- Parallel execution safety
- Error handling coverage

**Run tests:**
```bash
# All provider tests
go test ./providers/... -v

# Specific provider
go test ./providers -run TestOpenAI -v

# With race detection
go test ./providers/... -race
```

---

## Extension Guide

### Adding a New Provider

1. **Create provider file** (`providers/newprovider.go`)
2. **Implement Provider interface**
3. **Add factory registration**
4. **Create test file** (`providers/newprovider_test.go`)
5. **Update documentation**

**Template:**
```go
package providers

type NewProvider struct {
    apiKey    string
    baseURL   string
    endpoints []Endpoint
}

func NewNewProvider(apiKey string) Provider {
    return &NewProvider{
        apiKey: apiKey,
        baseURL: "https://api.newprovider.com",
        endpoints: []Endpoint{
            {Path: "/v1/chat/completions", Method: "POST"},
        },
    }
}

func (p *NewProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
    // Implementation
}

// ... implement other interface methods
```

---

## Known Issues

1. **HTTP Timeouts** - Some providers may need custom timeout configs
2. **Rate Limiting** - No built-in rate limit handling per provider
3. **Connection Pooling** - Uses Go default (no custom pool configuration)
4. **Retry Logic** - Providers don't implement automatic retries (handled by internal/http layer)

---

## Dependencies

- `net/http` - HTTP client
- `context` - Cancellation support
- `encoding/json` - JSON marshaling
- `time` - Latency tracking
- `sync` - Parallel endpoint validation

**Zero external dependencies** - Pure Go stdlib

---

**Last Updated**: December 31, 2025
**Total Implementations**: 24 provider files
**Test Files**: 24 dedicated test files
