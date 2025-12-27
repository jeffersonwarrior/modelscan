package routing

import (
	"context"
	"time"
)

// Router defines the interface for routing requests to LLM providers.
// Implementations can route directly to SDKs, through a Plano proxy, or via embedded Plano.
type Router interface {
	// Route sends a request and returns a response
	Route(ctx context.Context, req Request) (*Response, error)

	// Close cleans up any resources (connections, containers, etc.)
	Close() error
}

// Request represents a standardized LLM request
type Request struct {
	// Model name (e.g., "gpt-4o", "claude-sonnet-4-5")
	Model string

	// Messages in the conversation
	Messages []Message

	// Provider override (optional). If empty, router decides.
	Provider string

	// Temperature for response generation
	Temperature float64

	// MaxTokens limit for response
	MaxTokens int

	// Stream enables streaming responses
	Stream bool

	// AdditionalParams for provider-specific options
	AdditionalParams map[string]interface{}
}

// Message represents a single message in a conversation
type Message struct {
	Role    string // "system", "user", "assistant"
	Content string
}

// Response represents a standardized LLM response
type Response struct {
	// Model that generated the response
	Model string

	// Content of the response
	Content string

	// Provider that handled the request
	Provider string

	// Usage statistics
	Usage Usage

	// Metadata contains provider-specific information
	Metadata map[string]interface{}

	// FinishReason indicates why generation stopped
	FinishReason string

	// Latency of the request
	Latency time.Duration
}

// Usage tracks token usage
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// RoutingMode defines the type of routing
type RoutingMode string

const (
	ModeDirect   RoutingMode = "direct"
	ModeProxy    RoutingMode = "plano_proxy"
	ModeEmbedded RoutingMode = "plano_embedded"
)

// Config holds routing configuration
type Config struct {
	Mode     RoutingMode
	Direct   *DirectConfig
	Proxy    *ProxyConfig
	Embedded *EmbeddedConfig
	Fallback bool
}

// DirectConfig configures direct SDK routing
type DirectConfig struct {
	// DefaultProvider when no provider is specified
	DefaultProvider string
}

// ProxyConfig configures Plano proxy routing
type ProxyConfig struct {
	BaseURL string
	Timeout int // seconds
	APIKey  string
}

// EmbeddedConfig configures embedded Plano instance
type EmbeddedConfig struct {
	ConfigPath string
	Image      string
	Ports      map[string]int
	Env        map[string]string
}
