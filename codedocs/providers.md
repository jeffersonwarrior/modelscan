# Providers Package Documentation

## Package Overview

**Package**: `providers`  
**Purpose**: Unified interface for AI providers (OpenAI, Google, Anthropic, Mistral)  
**Stability**: Beta  
**Test Coverage**: 0% (critical)

---

## Provider Interface

Unified API for all providers.

```go
type Provider interface {
    Generate(ctx context.Context, prompt string, opts ...Option) (string, error)
    GenerateStream(ctx context.Context, prompt string, opts ...Option) (Stream, error)
    CallTool(ctx context.Context, toolName string, input map[string]interface{}) (interface{}, error)
    Close() error
}
```

**Options Pattern**:
```go
type Option func(*Request)

WithModel(model string) Option
WithTemperature(temp float64) Option
WithMaxTokens(tokens int) Option
WithStream() Option
```

---

## Implementations

### OpenAI Provider

**File**: `providers/openai.go`

Supports GPT models with streaming and tool calls.

**Key Features**:
- Automatic model detection
- Retry logic (3 attempts)
- Rate limiting awareness

### Google Provider

**File**: `providers/google.go`

Gemini models support.

### Anthropic Provider

**File**: `providers/anthropic.go`

Claude models with tool use.

### Mistral Provider

**File**: `providers/mistral.go`

Mistral models.

---

## Factory

```go
func NewProvider(name string, cfg ProviderConfig) (Provider, error)
```

Supported names: `openai`, `google`, `anthropic`, `mistral`.

---

## Issues & Recommendations

**High Priority**:
1. **No HTTP timeouts** - Add `http.Client` with 30s timeout
2. **No connection pooling** - Configure `MaxIdleConns`
3. **No tests** - Add mock HTTP servers
4. **No rate limiting** - Add token bucket

---

**Last Updated**: December 18, 2025