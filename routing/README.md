# ModelScan Routing Package

Zero-dependency routing layer for ModelScan with support for direct SDK calls, Plano proxy, and embedded Plano deployment.

## Features

- **Direct Mode**: Route requests directly to SDK clients (current behavior)
- **Plano Proxy Mode**: Route through external Plano instance for intelligent provider selection
- **Plano Embedded Mode**: Automatically manage Plano Docker container
- **Policy-Based Routing**: Let Plano's 1.5B model select optimal provider based on task description
- **Fallback Support**: Automatic failover to direct mode if Plano unavailable
- **100% Stdlib**: No external dependencies (pure Go stdlib)

## Installation

```bash
go get github.com/jeffersonwarrior/modelscan/routing
```

## Quick Start

### Direct Mode (Default)

```go
import "github.com/jeffersonwarrior/modelscan/routing"

// Create direct router
config := routing.DefaultConfig()
router, _ := routing.NewRouter(config)
defer router.Close()

// Register SDK clients
directRouter := router.(*routing.DirectRouter)
directRouter.RegisterClient("openai", openaiClient)
directRouter.RegisterClient("anthropic", anthropicClient)

// Make request
resp, err := router.Route(ctx, routing.Request{
    Model:    "gpt-4o",
    Provider: "openai",
    Messages: []routing.Message{
        {Role: "user", Content: "Hello!"},
    },
})
```

### Plano Proxy Mode

```go
// Create proxy router pointing to external Plano
config := routing.NewProxyConfigFromURL("http://localhost:12000")
router, _ := routing.NewRouter(config)
defer router.Close()

// Let Plano route based on policy
resp, err := router.Route(ctx, routing.Request{
    Model: "none", // Plano decides
    Messages: []routing.Message{
        {Role: "user", Content: "Write a Python function"},
    },
})
```

### Plano Embedded Mode

```go
// Create embedded router (manages Docker container)
config := routing.NewEmbeddedConfigFromFile("./plano_config.yaml")
config.Embedded.Env = map[string]string{
    "OPENAI_API_KEY": os.Getenv("OPENAI_API_KEY"),
    "ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
}

router, _ := routing.NewRouter(config)
defer router.Close() // Stops and removes container

// Make request
resp, err := router.Route(ctx, routing.Request{
    Model: "none",
    Messages: []routing.Message{
        {Role: "user", Content: "Explain quantum computing"},
    },
})
```

## Configuration

### Direct Mode

```go
config := &routing.Config{
    Mode: routing.ModeDirect,
    Direct: &routing.DirectConfig{
        DefaultProvider: "openai",
    },
    Fallback: false,
}
```

### Proxy Mode

```go
config := &routing.Config{
    Mode: routing.ModeProxy,
    Proxy: &routing.ProxyConfig{
        BaseURL: "http://localhost:12000",
        Timeout: 30, // seconds
        APIKey:  "", // optional
    },
    Fallback: true, // fallback to direct mode on error
}
```

### Embedded Mode

```go
config := &routing.Config{
    Mode: routing.ModeEmbedded,
    Embedded: &routing.EmbeddedConfig{
        ConfigPath: "./plano_config.yaml",
        Image:      "katanemo/plano:0.4.0",
        Ports: map[string]int{
            "ingress": 10000,
            "egress":  12000,
        },
        Env: map[string]string{
            "OPENAI_API_KEY":    "sk-...",
            "ANTHROPIC_API_KEY": "sk-ant-...",
        },
    },
    Fallback: true,
}
```

## Plano Policy Configuration

Create `plano_config.yaml` to define routing policies:

```yaml
version: v0.1.0

listeners:
  - type: model
    name: model_gateway
    address: 0.0.0.0
    port: 10000

model_providers:
  - access_key: $OPENAI_API_KEY
    model: openai/gpt-4o
    routing_preferences:
      - name: general_tasks
        description: general conversation, data analysis, creative writing

  - access_key: $ANTHROPIC_API_KEY
    model: anthropic/claude-sonnet-4-5
    routing_preferences:
      - name: code_tasks
        description: code generation, debugging, technical documentation

  - access_key: $DEEPSEEK_API_KEY
    model: deepseek/deepseek-coder
    routing_preferences:
      - name: code_review
        description: code review, refactoring, security analysis
```

## API Reference

### Router Interface

```go
type Router interface {
    Route(ctx context.Context, req Request) (*Response, error)
    Close() error
}
```

### Request

```go
type Request struct {
    Model            string
    Messages         []Message
    Provider         string  // optional override
    Temperature      float64
    MaxTokens        int
    Stream           bool
    AdditionalParams map[string]interface{}
}
```

### Response

```go
type Response struct {
    Model        string
    Content      string
    Provider     string
    Usage        Usage
    Metadata     map[string]interface{}
    FinishReason string
    Latency      time.Duration
}
```

## Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific test
go test -run TestDirectRouter_Route

# Verbose output
go test -v ./...
```

## Examples

See `examples/routing/` for complete working examples:

- `direct_config.yaml` - Direct mode configuration
- `proxy_config.yaml` - Proxy mode configuration
- `embedded_config.yaml` - Embedded mode configuration
- `plano_config.yaml` - Plano routing policies
- `main.go` - Example usage of all modes

## Performance

| Mode | Latency | Overhead | Use Case |
|------|---------|----------|----------|
| Direct | ~100ms | None | Single provider, maximum performance |
| Proxy | ~120ms | ~20ms | Shared Plano instance, centralized policies |
| Embedded | ~150ms | ~50ms | Development, self-contained deployment |

## Requirements

- Go 1.23+
- Docker (for embedded mode only)
- Plano server (for proxy mode only)

## Dependencies

**Zero external dependencies** - 100% Go stdlib only.

Uses only:
- `context`
- `encoding/json`
- `errors`
- `fmt`
- `io`
- `net/http`
- `os`
- `os/exec`
- `path/filepath`
- `strings`
- `time`

## License

Same as parent project (see root LICENSE file)

## Contributing

See main project CONTRIBUTING.md

## Support

For questions and issues, see the main ModelScan repository.
