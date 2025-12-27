package routing

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// DirectRouter routes requests directly to SDK clients without any proxy
type DirectRouter struct {
	config   *DirectConfig
	clients  map[string]Client
	fallback Router // fallback router if direct fails
}

// Client represents a generic SDK client interface
// Implementations should wrap existing SDK clients (OpenAI, Anthropic, etc.)
type Client interface {
	ChatCompletion(ctx context.Context, req Request) (*Response, error)
	Close() error
}

// NewDirectRouter creates a new direct router
func NewDirectRouter(config *DirectConfig) (*DirectRouter, error) {
	if config == nil {
		config = &DirectConfig{
			DefaultProvider: "openai",
		}
	}

	return &DirectRouter{
		config:  config,
		clients: make(map[string]Client),
	}, nil
}

// RegisterClient registers an SDK client for a provider
func (r *DirectRouter) RegisterClient(provider string, client Client) {
	r.clients[provider] = client
}

// SetFallback sets a fallback router
func (r *DirectRouter) SetFallback(fallback Router) {
	r.fallback = fallback
}

// Route routes the request directly to the appropriate SDK client
func (r *DirectRouter) Route(ctx context.Context, req Request) (*Response, error) {
	start := time.Now()

	// Determine which provider to use
	provider := req.Provider
	if provider == "" {
		provider = r.config.DefaultProvider
	}

	// Get the client for this provider
	client, ok := r.clients[provider]
	if !ok {
		if r.fallback != nil {
			return r.fallback.Route(ctx, req)
		}
		return nil, fmt.Errorf("no client registered for provider: %s", provider)
	}

	// Make the request
	resp, err := client.ChatCompletion(ctx, req)
	if err != nil {
		if r.fallback != nil {
			return r.fallback.Route(ctx, req)
		}
		return nil, fmt.Errorf("chat completion failed: %w", err)
	}

	// Set latency
	if resp != nil {
		resp.Latency = time.Since(start)
		resp.Provider = provider
	}

	return resp, nil
}

// Close closes all registered clients
func (r *DirectRouter) Close() error {
	var errs []error
	for provider, client := range r.clients {
		if err := client.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close %s client: %w", provider, err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// ListProviders returns all registered providers
func (r *DirectRouter) ListProviders() []string {
	providers := make([]string, 0, len(r.clients))
	for provider := range r.clients {
		providers = append(providers, provider)
	}
	return providers
}
