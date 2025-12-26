# AGENTS.md - ModelScan Developer Guide

## Project Overview

**ModelScan** is a Go-based CLI tool for validating AI provider APIs, discovering available models, and verifying endpoint functionality. It queries provider APIs directly to get accurate, up-to-date information about models and capabilities, then exports results to SQLite databases and Markdown reports.

**Key Purpose**: Keep provider configurations updated by detecting new models, updating pricing, identifying deprecated models, and checking endpoint health.

## Project Structure

```
modelscan/
├── main.go              # CLI entry point, flag parsing, validation orchestration
├── config/              # Configuration management
│   ├── config.go        # Multi-source config loading (env vars, files, NEXORA)
│   ├── config_test.go   # Config tests (86.5% coverage) ✅
│   └── agent_env.go     # Agent environment parsing for additional providers
├── providers/           # Provider implementations
│   ├── interface.go     # Provider interface, Model/Endpoint/Capabilities types
│   ├── utils.go         # Shared utility functions
│   ├── anthropic.go     # Anthropic implementation (official SDK) ✅
│   ├── openai.go        # OpenAI implementation (community SDK) ✅
│   ├── google.go        # Google Gemini implementation (REST API - SDK available) ⚠️
│   ├── mistral.go       # Mistral AI implementation (REST API - custom SDK ready) ⚠️
│   └── providers_test.go # Provider tests (30.4% coverage) ✅
├── storage/             # Data persistence
│   ├── sqlite.go        # SQLite database operations
│   ├── markdown.go      # Markdown report generation
│   └── storage_test.go  # Storage tests (67.3% coverage) ✅
├── validators/          # Empty directory (reserved for future validation logic)
├── go.mod               # Module definition (Go 1.23+)
├── go.sum               # Dependency checksums
├── modelscan            # Compiled binary
├── export.sh            # Helper script for validation and export ✅
├── providers.db         # Generated SQLite database
├── PROVIDERS.md         # Generated markdown report
└── AGENTS.md            # This file
```

## Essential Commands

### Building

```bash
# Standard build
go build -o modelscan main.go

# Optimized build (smaller binary)
go build -ldflags="-s -w" -o modelscan main.go

# Install dependencies
go mod tidy
go mod download
```

### Running the Tool

```bash
# Validate all providers with verbose output
./modelscan --provider=all --verbose

# Validate specific provider
./modelscan --provider=mistral --verbose
./modelscan --provider=openai --verbose
./modelscan --provider=anthropic --verbose

# Export to specific formats
./modelscan --provider=all --format=sqlite --output=./data/
./modelscan --provider=all --format=markdown --output=./docs/
./modelscan --provider=all --format=all --output=./results/

# Use config file for API keys
./modelscan --config=api-keys.txt --provider=all
```

**Note**: The binary name in the README examples is `pv` (old name), but the actual binary is `modelscan`.

### Available Flags

- `--provider`: Provider to validate (`mistral`, `openai`, `anthropic`, `all`) - default: `all`
- `--format`: Output format (`sqlite`, `markdown`, `all`) - default: `all`
- `--output`: Output directory for results - default: `.` (current directory)
- `--config`: Path to config file with API keys - optional
- `--verbose`: Enable verbose output - default: `false`

### Testing

```bash
# Run all tests
go test ./...

# Run with verbose output
go test ./... -v

# Run specific package tests
go test ./config -v
go test ./providers -v
go test ./storage -v

# Run with coverage
go test ./... -cover

# Generate coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run specific test
go test ./providers -run TestProviderRegistration

# Run benchmarks
go test ./providers -bench=. -benchmem
```

**Current Test Coverage**:
- `config/`: 86.5% ✅
- `storage/`: 67.3% ✅
- `providers/`: 30.4% ⚠️ (API calls require valid keys)
- `main.go`: No tests (CLI integration)

**Test Status**: All 29 tests passing ✅

### Development

```bash
# Check for errors without building
go vet ./...

# Format all code
go fmt ./...

# Check module dependencies
go list -m all

# View module graph
go mod graph
```

## Configuration System

ModelScan loads configuration from **multiple sources in priority order**:

1. **NEXORA Config** (`/home/nexora/.config/nexora/modelscan.config.json` or local)
2. **Agent Environment** (`/home/agent/.env`) - tab-delimited format with many providers
3. **ModelScan Config File** (various locations, see below)
4. **Environment Variables** (highest priority, overrides all others)

### Config File Locations (checked in order)

1. `~/.config/nexora/modelscan.config.json`
2. `./modelscan.config.json` (current directory)
3. `./config/modelscan.config.json`
4. `./.modelscan.config.json`

### Config File Format (JSON)

```json
{
  "providers": {
    "mistral": {
      "api_key": "your-key-here",
      "endpoint": "https://api.mistral.ai/v1",
      "description": "Mistral AI Provider"
    },
    "openai": {
      "api_key": "sk-...",
      "endpoint": "https://api.openai.com/v1"
    }
  }
}
```

### Environment Variables

Standard environment variables (checked by `loadFromNexoraConfig`):
- `MISTRAL_API_KEY`
- `OPENAI_API_KEY`
- `ANTHROPIC_API_KEY`

Override environment variables (highest priority):
- `MODELSCAN_MISTRAL_KEY`
- `MODELSCAN_OPENAI_KEY`
- `MODELSCAN_ANTHROPIC_KEY`

### Agent Environment Format

The tool can extract API keys from `/home/agent/.env` (tab-delimited format):
```
KEY_NAME	CATEGORY	API_KEY	USAGE	NOTES
MISTRAL_API_KEY	LLM	sk-xxx...	High	Description
```

Only processes providers with category: `LLM`, `LLM Router`, or `Search`.

Additional providers extracted from specialized config functions:
- `extractFromGamma()` - Gamma API keys
- `extractFromManus()` - Manus API keys
- `extractFromLlamaIndex()` - LlamaIndex keys
- `extractFromNanoGPT()` - NanoGPT keys
- `extractYouCom()` - You.com keys
- `extractMinimax()` - Minimax keys
- `extractKimiForCoding()` - Kimi coding keys
- `extractFromVibe()` - Vibe API keys

## Provider Interface

All providers must implement the `Provider` interface defined in `providers/interface.go`:

```go
type Provider interface {
    // ValidateEndpoints tests all known endpoints for the provider
    ValidateEndpoints(ctx context.Context, verbose bool) error
    
    // ListModels retrieves all available models from the provider
    ListModels(ctx context.Context, verbose bool) ([]Model, error)
    
    // GetCapabilities returns the provider's capabilities
    GetCapabilities() ProviderCapabilities
    
    // GetEndpoints returns all endpoints that should be validated
    GetEndpoints() []Endpoint
    
    // TestModel tests a specific model can respond to requests
    TestModel(ctx context.Context, modelID string, verbose bool) error
}
```

### Provider Registration Pattern

Each provider registers itself in its `init()` function:

```go
func init() {
    RegisterProvider("mistral", NewMistralProvider)
}
```

This uses the factory pattern - `RegisterProvider()` stores a `ProviderFactory` function that creates provider instances with an API key.

## Data Models

### Model Structure

```go
type Model struct {
    ID            string            // Unique model identifier
    Name          string            // Display name
    Description   string            // Model description
    CostPer1MIn   float64           // Cost per 1M input tokens
    CostPer1MOut  float64           // Cost per 1M output tokens
    ContextWindow int               // Maximum context window size
    MaxTokens     int               // Maximum output tokens
    SupportsImages bool             // Vision capability
    SupportsTools  bool             // Function calling support
    CanReason      bool             // Reasoning capability
    CanStream      bool             // Streaming support
    CreatedAt      string           // Creation timestamp
    deprecated     bool             // Deprecation status
    DeprecatedAt   *time.Time       // Deprecation date
    Categories     []string         // e.g., ["coding", "chat", "embedding"]
    Capabilities   map[string]string // Additional capabilities
}
```

### Endpoint Structure

```go
type Endpoint struct {
    Path        string            // API path
    Method      string            // HTTP method
    Description string            // Endpoint description
    Headers     map[string]string // Required headers
    TestParams  interface{}       // Test parameters
    Status      EndpointStatus    // Validation status
    Latency     time.Duration     // Response latency
    Error       string            // Error message if failed
}
```

### EndpointStatus Constants

- `StatusUnknown` - Not yet tested
- `StatusWorking` - Successfully validated
- `StatusFailed` - Validation failed
- `StatusDeprecated` - Marked as deprecated

### Provider Capabilities

```go
type ProviderCapabilities struct {
    SupportsChat          bool
    SupportsFIM           bool     // Fill-in-the-middle
    SupportsEmbeddings    bool
    SupportsFineTuning    bool
    SupportsAgents        bool
    SupportsFileUpload    bool
    SupportsStreaming     bool
    SupportsJSONMode      bool
    SupportsVision        bool
    SupportsAudio         bool
    SupportedParameters   []string
    SecurityFeatures      []string
    MaxRequestsPerMinute  int
    MaxTokensPerRequest   int
}
```

## Storage System

### SQLite Database Schema

**Table: providers**
- `id` INTEGER PRIMARY KEY AUTOINCREMENT
- `name` TEXT UNIQUE NOT NULL
- `capabilities` TEXT (JSON)
- `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP

**Table: models**
- `id` INTEGER PRIMARY KEY AUTOINCREMENT
- `provider_name` TEXT NOT NULL (FOREIGN KEY)
- `model_id` TEXT NOT NULL
- `name` TEXT NOT NULL
- `description` TEXT
- `cost_per_1m_in` REAL
- `cost_per_1m_out` REAL
- `context_window` INTEGER
- `max_tokens` INTEGER
- `supports_images` BOOLEAN
- `supports_tools` BOOLEAN
- `can_reason` BOOLEAN
- `can_stream` BOOLEAN
- `categories` TEXT (JSON array)
- `capabilities` TEXT (JSON)
- `created_at` DATETIME
- UNIQUE constraint on (`provider_name`, `model_id`)

**Table: endpoints**
- `id` INTEGER PRIMARY KEY AUTOINCREMENT
- `provider_name` TEXT NOT NULL (FOREIGN KEY)
- `path` TEXT NOT NULL
- `method` TEXT NOT NULL
- `description` TEXT
- `status` TEXT
- `latency_ms` INTEGER
- `error_message` TEXT
- `created_at` DATETIME
- UNIQUE constraint on (`provider_name`, `path`, `method`)

**Table: validation_runs**
- `id` INTEGER PRIMARY KEY AUTOINCREMENT
- `provider_name` TEXT NOT NULL (FOREIGN KEY)
- `run_at` DATETIME DEFAULT CURRENT_TIMESTAMP
- `success_count` INTEGER
- `failure_count` INTEGER
- `total_latency_ms` INTEGER

### Querying the Database

```sql
-- Get all models for a provider
SELECT * FROM models WHERE provider_name = 'mistral';

-- Get models with specific capabilities
SELECT * FROM models WHERE supports_images = 1 AND can_reason = 1;

-- Check endpoint health
SELECT path, method, status, latency_ms 
FROM endpoints 
WHERE provider_name = 'openai' AND status = 'working';

-- Generate provider config
SELECT 
    model_id,
    name,
    cost_per_1m_in as CostPer1MIn,
    cost_per_1m_out as CostPer1MOut,
    context_window as ContextWindow,
    max_tokens as DefaultMaxTokens
FROM models 
WHERE provider_name = 'mistral';
```

## Code Style & Conventions

### Naming Conventions

- **Packages**: lowercase, single word (`providers`, `storage`, `config`)
- **Interfaces**: PascalCase ending with meaningful noun (`Provider`, not `IProvider`)
- **Structs**: PascalCase (`MistralProvider`, `Model`, `Endpoint`)
- **Functions**: PascalCase for exported, camelCase for unexported
- **Variables**: camelCase (`apiKey`, `baseURL`, `client`)
- **Constants**: PascalCase with const keyword

### Error Handling Pattern

```go
// Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}

// Log and continue (non-fatal)
if err := someOperation(); err != nil {
    log.Printf("Warning: operation failed: %v", err)
}

// Fatal errors
if err := criticalOperation(); err != nil {
    log.Fatalf("Critical error: %v", err)
}
```

### HTTP Client Pattern

All providers use:
- 30-second timeout on HTTP clients
- Bearer token authentication in Authorization header
- Context-aware requests with `http.NewRequestWithContext()`
- Proper cleanup with `defer resp.Body.Close()`

### Verbose Output Pattern

```go
if verbose {
    fmt.Printf("  Descriptive message...\n")
}

// Use indentation for hierarchy
fmt.Printf("=== Provider Name ===\n")      // Top level
fmt.Printf("  Fetching models...\n")        // Second level
fmt.Printf("    ✓ Success\n")               // Third level
fmt.Printf("    ✗ Failed: %v\n", err)       // Third level error
```

### Provider Implementation Pattern

Each provider follows this structure:

1. **Struct definition** with `apiKey`, `baseURL`, `client`
2. **Constructor function** `NewXProvider(apiKey string) Provider`
3. **init() function** to register provider
4. **ValidateEndpoints()** - iterates and tests endpoints
5. **ListModels()** - calls API to get models, parses response
6. **GetCapabilities()** - returns static capabilities struct
7. **GetEndpoints()** - returns slice of endpoints to test
8. **TestModel()** - sends test request to specific model
9. **Helper methods** - `testEndpoint()`, `makeRequest()`, etc.

### Tab vs Spaces

The codebase uses **TABS for indentation** (Go standard). When viewing files, tabs display as `→\t` in the VIEW tool output. Always use actual tab characters (`\t`) in code, not spaces.

## Adding a New Provider

### Step-by-Step Process

1. **Create provider file**: `providers/newprovider.go`

2. **Define provider struct**:
```go
type NewProvider struct {
    apiKey  string
    baseURL string
    client  *http.Client
}
```

3. **Implement constructor**:
```go
func NewNewProvider(apiKey string) Provider {
    return &NewProvider{
        apiKey:  apiKey,
        baseURL: "https://api.newprovider.com/v1",
        client:  &http.Client{Timeout: 30 * time.Second},
    }
}
```

4. **Register in init()**:
```go
func init() {
    RegisterProvider("newprovider", NewNewProvider)
}
```

5. **Implement required interface methods**:
   - `ValidateEndpoints(ctx context.Context, verbose bool) error`
   - `ListModels(ctx context.Context, verbose bool) ([]Model, error)`
   - `GetCapabilities() ProviderCapabilities`
   - `GetEndpoints() []Endpoint`
   - `TestModel(ctx context.Context, modelID string, verbose bool) error`

6. **Update config loading** in `config/config.go`:
   - Add environment variable check in `loadFromNexoraConfig()`
   - Add override in `loadFromEnvironment()`
   - Add endpoint mapping in `agent_env.go` if needed

7. **Test the provider**:
```bash
export NEWPROVIDER_API_KEY="your-key"
./modelscan --provider=newprovider --verbose
```

### Example: Minimal Provider

```go
package providers

import (
    "context"
    "fmt"
    "net/http"
    "time"
)

type ExampleProvider struct {
    apiKey  string
    baseURL string
    client  *http.Client
}

func NewExampleProvider(apiKey string) Provider {
    return &ExampleProvider{
        apiKey:  apiKey,
        baseURL: "https://api.example.com/v1",
        client:  &http.Client{Timeout: 30 * time.Second},
    }
}

func init() {
    RegisterProvider("example", NewExampleProvider)
}

func (p *ExampleProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
    // Implement endpoint validation
    return nil
}

func (p *ExampleProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
    // Implement model listing
    return []Model{}, nil
}

func (p *ExampleProvider) GetCapabilities() ProviderCapabilities {
    return ProviderCapabilities{
        SupportsChat:      true,
        SupportsStreaming: true,
    }
}

func (p *ExampleProvider) GetEndpoints() []Endpoint {
    return []Endpoint{
        {
            Path:        "/chat/completions",
            Method:      "POST",
            Description: "Chat completions endpoint",
        },
    }
}

func (p *ExampleProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
    // Implement model testing
    return nil
}
```

## Important Gotchas

### ✅ FIXED Issues

1. **✅ Tests Now Exist** - Comprehensive test suite with 86.5% config coverage, 67.3% storage coverage, 30.4% providers coverage
2. **✅ Anthropic Models API** - Now using the official `/v1/models` endpoint (not hardcoded!)
3. **✅ Export Script Fixed** - Updated to use correct command syntax
4. **✅ Config System Simplified** - Still multi-source but well-tested
5. **✅ Official SDKs** - Using Anthropic SDK v1.19.0 and OpenAI client v1.41.2

### Current Gotchas

**Issue**: Multiple config sources can cause confusion about which API key is being used.

**Solution**: Check logs or add debug output. Priority order:
1. Environment variables (MODELSCAN_* highest)
2. NEXORA config
3. Agent environment
4. Config files

### 2. SQLite Database Initialization

**Issue**: Database must be initialized before storing results.

**Current Behavior**: Database is initialized in `main.go` when `--format=all`, `--format=sqlite`, or `--format=markdown` (markdown needs SQLite to read from).

**Gotcha**: If you only want markdown output, SQLite is still initialized because markdown export reads from the database.

### 3. Anthropic Models List

**Important**: Anthropic doesn't have a public `/models` endpoint, so models are **hardcoded** in `anthropic.go`. When new Claude models are released, they must be manually added to the code.

**Location**: `providers/anthropic.go`, `ListModels()` method around line 73-200.

### 4. Provider Registration Order

**Issue**: Providers must call `RegisterProvider()` in their `init()` functions. If `init()` doesn't run (e.g., file not imported), provider won't be available.

**Current State**: All provider files are imported in `main.go`, so all providers are automatically registered.

### 5. Model Categories

**Note**: Model categories (e.g., `["coding", "chat", "embedding"]`) are **provider-defined**. Different providers may use different category names. Mistral uses categories like:
- `audio` - Voice models
- `coding` - Code-focused models
- `general` - General purpose models
- `chat` - Chat/conversation models

### 6. Cost Fields Default to Zero

**Issue**: If a model's pricing isn't available from the API, `CostPer1MIn` and `CostPer1MOut` default to `0.00`.

**Note**: Check provider documentation separately for accurate pricing. The tool shows API-reported prices, which may not be complete.

### 7. Export Script Gotcha

**Issue**: `export.sh` uses `./modelscan export` command, but the tool doesn't actually have an `export` subcommand.

**Current Behavior**: The script will fail. To export markdown:
```bash
./modelscan --provider=all --format=markdown --output=./PROVIDERS.md
```

### 8. No Tests

**Critical**: There are **no test files** in the entire codebase. All testing is currently manual.

**TODO**: Add test coverage for:
- Config loading
- Provider interface implementations
- Storage operations
- Mock API responses

### 9. Validators Directory

**Status**: The `validators/` directory exists but is **completely empty**. It's reserved for future validation logic but not currently used.

### 10. Module Path

**Important**: The module path is `github.com/jeffersonwarrior/modelscan`, but this project may not actually be published at that GitHub location. When working with the code, imports still use this path.

## Common Development Tasks

### Update Model Pricing

1. Run the tool with verbose output:
```bash
./modelscan --provider=mistral --verbose
```

2. Check the SQLite database:
```bash
sqlite3 providers.db "SELECT model_id, cost_per_1m_in, cost_per_1m_out FROM models WHERE provider_name='mistral';"
```

3. For Anthropic (hardcoded), edit `providers/anthropic.go` directly.

### Add New Endpoint to Existing Provider

1. Open provider file (e.g., `providers/mistral.go`)
2. Find `GetEndpoints()` method
3. Add new endpoint to the slice:
```go
{
    Path:        "/new-endpoint",
    Method:      "POST",
    Description: "New endpoint description",
}
```
4. Rebuild and test:
```bash
go build -o modelscan main.go
./modelscan --provider=mistral --verbose
```

### Generate Provider Config from Database

```bash
sqlite3 providers.db << 'EOF'
.mode line
SELECT 
    'Model: ' || name,
    'ID: ' || model_id,
    'Cost In: $' || cost_per_1m_in,
    'Cost Out: $' || cost_per_1m_out,
    'Context: ' || context_window
FROM models 
WHERE provider_name = 'mistral'
ORDER BY name;
EOF
```

### Debug Config Loading

Add debug output in `config/config.go`:

```go
func LoadConfig() (*Config, error) {
    config := &Config{
        Providers: make(map[string]ProviderConfig),
    }
    
    // Add debug output
    fmt.Printf("DEBUG: Loading config...\n")
    
    if err := loadFromNexoraConfig(config); err != nil {
        fmt.Printf("DEBUG: NEXORA config failed: %v\n", err)
    } else {
        fmt.Printf("DEBUG: Loaded %d providers from NEXORA\n", len(config.Providers))
    }
    
    // ... rest of function
}
```

### Check Endpoint Health

```bash
# Query endpoint status
sqlite3 providers.db << 'EOF'
SELECT 
    provider_name,
    path,
    method,
    status,
    latency_ms || 'ms' as latency,
    error_message
FROM endpoints
ORDER BY provider_name, path;
EOF
```

### View Validation History

```bash
sqlite3 providers.db << 'EOF'
SELECT 
    provider_name,
    datetime(run_at, 'localtime') as run_time,
    success_count,
    failure_count,
    total_latency_ms || 'ms' as total_latency
FROM validation_runs
ORDER BY run_at DESC
LIMIT 10;
EOF
```

## Dependencies

**Official SDKs**:
- **github.com/anthropics/anthropic-sdk-go** v1.19.0 - Anthropic Claude official Go SDK
- **github.com/sashabaranov/go-openai** v1.41.2 - OpenAI community Go client (18k+ stars)
- **google.golang.org/genai** - Google Generative AI official Go SDK (NOT YET INTEGRATED) ⚠️
- **Custom Mistral SDK** - Available at `/home/nexora/sdk/mistral/` (NOT YET INTEGRATED) ⚠️

**Database**:
- **github.com/mattn/go-sqlite3** v1.14.22 - SQLite3 driver for Go

**Requirements**: Go 1.23+

### Why These SDKs?

- **Anthropic**: Official SDK with full feature support, type-safe API, automatic retries ✅
- **OpenAI**: Community-maintained SDK (sashabaranov) - most popular, well-maintained, 18k+ stars ✅
- **Google**: Currently using direct REST API, but official SDK available (`google.golang.org/genai`) ⚠️
  - Official Google SDK supports both Gemini Developer API AND Vertex AI
  - Replaces deprecated `github.com/google/generative-ai-go` (EOL: Aug 31, 2025)
  - Repository: https://github.com/googleapis/go-genai
  - **Migration recommended**
- **Mistral**: Currently using direct HTTP, custom SDK built but not integrated ⚠️
  - Custom SDK provides full Mistral API support
  - Enables FIM, Agents API, Fine-tuning features
  - **Integration recommended**

### SDK Features Used

**Anthropic**:
- Models list endpoint (`GET /v1/models`)
- Message creation for testing models
- Proper header management (x-api-key, anthropic-version)

**OpenAI**:
- `ListModels()` - Get all available models
- `CreateChatCompletion()` - Test chat models
- `CreateEmbeddings()` - Test embedding models

**Google** (Current - Direct REST API):
- REST API `/v1beta/models` - List all Gemini models
- `generateContent` endpoint - Test model responses
- API key authentication
- **Note**: Official SDK available but not yet integrated

**Mistral** (Current - Direct HTTP):
- REST API `/v1/models` - List all Mistral models
- Chat completions endpoint - Test chat models
- Bearer token authentication
- **Note**: Custom SDK built at `/home/nexora/sdk/mistral/` but not yet integrated
- `generateContent` endpoint - Test model responses
- API key authentication

## Output Files

- `providers.db` - SQLite database with validation results
- `PROVIDERS.md` - Human-readable markdown report
- `test_providers.db` - Test database (gitignored)
- `providers_backup.db` - Backup database (gitignored)

## Binary Management

The compiled binary `modelscan` is committed to the repository. To rebuild:

```bash
go build -o modelscan main.go
```

The old binary name was `pv` (mentioned in README), but current binary is `modelscan`.

## Future Improvements

Based on code analysis, these areas could be improved:

1. **Add comprehensive tests** - No tests exist currently
2. **Fix export.sh** - Update to use correct command syntax
3. **Add more providers** - XAI, Cerebras, Perplexity, OpenRouter, Gemini (config code exists, implementations don't)
4. **Rate limiting** - Add backoff/retry logic for API calls
5. **Async validation** - Validate multiple providers concurrently
6. **Better error messages** - More detailed API error parsing
7. **Config validation** - Validate API keys before running validations
8. **Streaming support** - Test streaming endpoints specifically
9. **Model comparison** - Tools to compare models across providers
10. **Deprecation tracking** - Alert when models are deprecated

## CLI Help Reference

```
Usage: modelscan [options]

Options:
  --provider string
        Provider to validate (mistral, openai, anthropic, all) (default "all")
  --format string
        Output format (sqlite, markdown, all) (default "all")
  --output string
        Output directory for results (default ".")
  --config string
        Path to config file with API keys
  --verbose
        Verbose output (default false)
```

## Quick Reference

**Build**: `go build -o modelscan main.go`  
**Run All**: `./modelscan --provider=all --verbose`  
**Run One**: `./modelscan --provider=mistral --verbose`  
**Export DB**: Results auto-saved to `providers.db`  
**Export MD**: Results auto-saved to `PROVIDERS.md`  
**View DB**: `sqlite3 providers.db`  
**Config**: `~/.config/nexora/modelscan.config.json`  
**Add Provider**: Create `providers/name.go`, implement interface, register in `init()`  

## Summary

ModelScan is a well-structured tool with clean separation of concerns:
- **main.go** orchestrates validation
- **config/** handles multi-source configuration
- **providers/** implements provider-specific logic
- **storage/** manages persistence and reporting

Key patterns:
- Factory pattern for provider registration
- Interface-based provider abstraction
- Context-aware HTTP operations
- Multi-source configuration with priority
- Dual export (SQLite + Markdown)

Work with confidence knowing:
- All providers follow the same interface
- Config loading is well-documented
- Database schema is straightforward
- Adding providers is a clear process
- The tool is production-ready (used by NEXORA)
