# Routing Package Documentation

**Package**: `routing`
**Purpose**: Multi-mode request routing with Plano integration (direct/proxy/embedded)
**Stability**: Production
**Test Coverage**: 80%+

---

## Overview

The routing package provides three routing modes for LLM requests: **Direct** (SDK pass-through), **Plano Proxy** (HTTP gateway), and **Plano Embedded** (containerized routing). Enables flexible deployment strategies with fallback support.

---

## Files

- `router.go` (116 lines) - Core router interface and types
- `factory.go` (101 lines) - Router factory with mode selection
- `direct.go` (108 lines) - Direct SDK routing implementation
- `plano_proxy.go` (221 lines) - HTTP proxy routing
- `plano_embedded.go` (250 lines) - Embedded Docker routing
- `*_test.go` - Test files (1,024 lines total)

---

## Core Interface

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
    Provider         string
    Temperature      float64
    MaxTokens        int
    Stream           bool
    AdditionalParams map[string]interface{}
}
```

### Response

```go
type Response struct {
    Content     string
    Usage       Usage
    Model       string
    Provider    string
    Latency     time.Duration
    Error       error
    Metadata    map[string]interface{}
}
```

---

## Routing Modes

### 1. Direct Mode

**File**: `routing/direct.go`

Routes requests directly to provider SDKs without intermediate layers.

**Features:**
- Zero latency overhead
- Direct SDK interaction
- Full control over requests
- No external dependencies

**Usage:**
```go
router := routing.NewDirect(providers)
resp, err := router.Route(ctx, req)
```

**Flow:**
```
Request → Direct Router → Provider SDK → HTTP Client → Provider API
```

---

### 2. Plano Proxy Mode

**File**: `routing/plano_proxy.go`

Routes through external Plano HTTP gateway for advanced features.

**Features:**
- Load balancing
- Request transformation
- Advanced caching
- Analytics collection
- Provider abstraction

**Configuration:**
```go
config := &PlanoProxyConfig{
    BaseURL:    "https://plano.gateway.com",
    APIKey:     "pk-...",
    Timeout:    30 * time.Second,
    RetryCount: 3,
}
router := routing.NewPlanoProxy(config)
```

**Flow:**
```
Request → Plano Proxy → HTTP Gateway → Provider API
```

---

### 3. Plano Embedded Mode

**File**: `routing/plano_embedded.go`

Spawns containerized Plano instance for self-hosted routing.

**Features:**
- Docker container management
- Auto-restart on failure
- Health monitoring
- Fallback to direct mode
- Zero external dependencies

**Configuration:**
```go
config := &PlanoEmbeddedConfig{
    ImageName:      "plano/gateway:latest",
    ContainerPort:  8080,
    HealthEndpoint: "/health",
    StartTimeout:   30 * time.Second,
    FallbackMode:   true, // Fall back to direct on container failure
}
router := routing.NewPlanoEmbedded(config, providers)
```

**Lifecycle:**
```
Initialize → Pull Image → Start Container → Health Check → Ready
```

**Fallback Strategy:**
```
Container fails → Log error → Switch to direct mode → Continue serving
```

---

## Factory Pattern

**File**: `routing/factory.go`

```go
type Config struct {
    Mode           string // "direct", "plano_proxy", "plano_embedded"
    PlanoProxyURL  string
    PlanoAPIKey    string
    PlanoImageName string
    EnableFallback bool
}

func NewRouter(config *Config, providers map[string]Provider) (Router, error)
```

**Example:**
```go
// Auto-select router based on config
config := &routing.Config{
    Mode:           "plano_embedded",
    PlanoImageName: "plano/gateway:v1.2.0",
    EnableFallback: true,
}

router, err := routing.NewRouter(config, providers)
if err != nil {
    log.Fatal(err)
}
defer router.Close()
```

---

## Advanced Features

### Health Monitoring

**Plano Embedded** includes health checks:

```go
func (r *PlanoEmbedded) healthCheck() error {
    resp, err := http.Get(r.healthURL)
    if err != nil || resp.StatusCode != 200 {
        return fmt.Errorf("unhealthy: %v", err)
    }
    return nil
}
```

### Auto-Restart

Failed containers automatically restart:

```go
if err := r.healthCheck(); err != nil {
    log.Printf("Container unhealthy, restarting...")
    r.stopContainer()
    r.startContainer()
}
```

### Graceful Degradation

On persistent failures, fall back to direct mode:

```go
if r.config.FallbackMode && r.restartCount > 3 {
    log.Warn("Switching to direct mode fallback")
    r.router = routing.NewDirect(r.providers)
}
```

---

## Usage Examples

### Direct Mode

```go
providers := map[string]Provider{
    "openai": openai.New(apiKey),
    "anthropic": anthropic.New(apiKey),
}

router := routing.NewDirect(providers)

req := routing.Request{
    Model:    "gpt-4",
    Provider: "openai",
    Messages: []routing.Message{
        {Role: "user", Content: "Hello"},
    },
}

resp, err := router.Route(context.Background(), req)
fmt.Println(resp.Content)
```

### Plano Proxy Mode

```go
config := &routing.PlanoProxyConfig{
    BaseURL: "https://plano.example.com",
    APIKey:  "pk-...",
    Timeout: 30 * time.Second,
}

router := routing.NewPlanoProxy(config)
resp, err := router.Route(ctx, req)
```

### Plano Embedded Mode

```go
config := &routing.PlanoEmbeddedConfig{
    ImageName:      "plano/gateway:latest",
    ContainerPort:  8080,
    HealthEndpoint: "/health",
    FallbackMode:   true,
}

router := routing.NewPlanoEmbedded(config, providers)
defer router.Close() // Stops container

resp, err := router.Route(ctx, req)
```

---

## Testing

### Test Coverage

- Direct routing: 85%
- Plano proxy: 80%
- Plano embedded: 75%
- Factory: 90%

### Test Strategy

**Direct Tests:**
- Mock provider SDKs
- Request transformation
- Error handling

**Proxy Tests:**
- Mock HTTP server
- Retry logic
- Timeout handling

**Embedded Tests:**
- Mock Docker API
- Container lifecycle
- Health check failures
- Fallback activation

**Run tests:**
```bash
go test ./routing/... -v
go test ./routing/... -race -cover
```

---

## Error Handling

```go
var (
    ErrProviderNotFound  = errors.New("provider not found")
    ErrInvalidRequest    = errors.New("invalid request")
    ErrContainerFailed   = errors.New("container failed to start")
    ErrHealthCheckFailed = errors.New("health check failed")
    ErrTimeout           = errors.New("request timeout")
)
```

**Error Recovery:**
- Retry transient errors (network, timeout)
- Fall back to direct mode on persistent failures
- Log all errors for debugging
- Return detailed error context

---

## Performance

### Latency by Mode

| Mode | Overhead | Use Case |
|------|----------|----------|
| Direct | 0ms | Production, low latency required |
| Plano Proxy | 5-20ms | Advanced features, analytics |
| Plano Embedded | 2-10ms | Self-hosted, data privacy |

### Connection Pooling

All modes use HTTP connection pooling:

```go
transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 10,
    IdleConnTimeout:     90 * time.Second,
}
```

---

## Docker Integration (Embedded Mode)

### Container Management

```go
// Start container
cmd := exec.Command("docker", "run", "-d",
    "-p", fmt.Sprintf("%d:8080", port),
    "--name", containerName,
    imageName)
cmd.Run()

// Stop container
exec.Command("docker", "stop", containerName).Run()

// Remove container
exec.Command("docker", "rm", containerName).Run()
```

### Image Pull

```go
// Pull latest image
exec.Command("docker", "pull", imageName).Run()
```

### Health Monitoring

Periodic health checks every 30 seconds:

```go
ticker := time.NewTicker(30 * time.Second)
for range ticker.C {
    if err := r.healthCheck(); err != nil {
        r.handleUnhealthy()
    }
}
```

---

## Configuration Examples

### Minimal (Direct)

```yaml
routing:
  mode: direct
```

### Plano Proxy

```yaml
routing:
  mode: plano_proxy
  plano_proxy:
    base_url: https://plano.example.com
    api_key: pk-...
    timeout: 30s
```

### Plano Embedded with Fallback

```yaml
routing:
  mode: plano_embedded
  plano_embedded:
    image: plano/gateway:v1.2.0
    port: 8080
    health_endpoint: /health
    fallback: true
```

---

## Dependencies

- `net/http` - HTTP client
- `context` - Cancellation support
- `os/exec` - Docker commands (embedded mode only)
- `encoding/json` - Request/response marshaling
- `time` - Timeout and retry logic

**Zero external Go dependencies** - Pure stdlib

---

**Last Updated**: December 31, 2025
**Total Lines**: 1,764 (including tests)
**Modes**: 3 (Direct, Proxy, Embedded)
