package providers

import (
	"context"
	"time"
)

// Model represents an AI model available from a provider
type Model struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description,omitempty"`
	CostPer1MIn    float64           `json:"cost_per_1m_in"`
	CostPer1MOut   float64           `json:"cost_per_1m_out"`
	ContextWindow  int               `json:"context_window"`
	MaxTokens      int               `json:"max_tokens,omitempty"`
	SupportsImages bool              `json:"supports_images"`
	SupportsTools  bool              `json:"supports_tools"`
	CanReason      bool              `json:"can_reason"`
	CanStream      bool              `json:"can_stream"`
	CreatedAt      string            `json:"created_at,omitempty"`
	Deprecated     bool              `json:"deprecated,omitempty"`
	DeprecatedAt   *time.Time        `json:"deprecated_at,omitempty"`
	Categories     []string          `json:"categories,omitempty"`   // e.g., ["coding", "chat", "embedding"]
	Capabilities   map[string]string `json:"capabilities,omitempty"` // e.g., {"function_calling": "full", "vision": "high"}
}

// Endpoint represents an API endpoint that can be validated
type Endpoint struct {
	Path        string            `json:"path"`
	Method      string            `json:"method"`
	Description string            `json:"description"`
	Headers     map[string]string `json:"headers,omitempty"`
	TestParams  interface{}       `json:"test_params,omitempty"`
	Status      EndpointStatus    `json:"status"`
	Latency     time.Duration     `json:"latency,omitempty"`
	Error       string            `json:"error,omitempty"`
}

type EndpointStatus string

const (
	StatusUnknown    EndpointStatus = "unknown"
	StatusWorking    EndpointStatus = "working"
	StatusFailed     EndpointStatus = "failed"
	StatusDeprecated EndpointStatus = "deprecated"
)

// ProviderCapabilities describes what a provider supports
type ProviderCapabilities struct {
	SupportsChat         bool     `json:"supports_chat"`
	SupportsFIM          bool     `json:"supports_fim"` // Fill-in-the-middle
	SupportsEmbeddings   bool     `json:"supports_embeddings"`
	SupportsFineTuning   bool     `json:"supports_fine_tuning"`
	SupportsAgents       bool     `json:"supports_agents"`
	SupportsFileUpload   bool     `json:"supports_file_upload"`
	SupportsStreaming    bool     `json:"supports_streaming"`
	SupportsJSONMode     bool     `json:"supports_json_mode"`
	SupportsVision       bool     `json:"supports_vision"`
	SupportsAudio        bool     `json:"supports_audio"`
	SupportedParameters  []string `json:"supported_parameters"`
	SecurityFeatures     []string `json:"security_features"`
	MaxRequestsPerMinute int      `json:"max_requests_per_minute"`
	MaxTokensPerRequest  int      `json:"max_tokens_per_request"`
}

// Provider defines the interface for all provider validations
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

// ProviderFactory creates a new provider instance
type ProviderFactory func(apiKey string) Provider

var providerFactories = make(map[string]ProviderFactory)

// RegisterProvider registers a new provider factory
func RegisterProvider(name string, factory ProviderFactory) {
	providerFactories[name] = factory
}

// GetProviderFactory returns a provider factory by name
func GetProviderFactory(name string) (ProviderFactory, bool) {
	factory, exists := providerFactories[name]
	return factory, exists
}

// ListProviders returns all registered provider names
func ListProviders() []string {
	names := make([]string, 0, len(providerFactories))
	for name := range providerFactories {
		names = append(names, name)
	}
	return names
}
