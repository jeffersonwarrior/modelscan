package routing

import (
	"fmt"
)

// NewRouter creates a router based on the provided configuration
func NewRouter(config *Config) (Router, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	var router Router
	var err error

	switch config.Mode {
	case ModeDirect:
		router, err = NewDirectRouter(config.Direct)
		if err != nil {
			return nil, fmt.Errorf("failed to create direct router: %w", err)
		}

	case ModeProxy:
		router, err = NewPlanoProxyRouter(config.Proxy)
		if err != nil {
			return nil, fmt.Errorf("failed to create proxy router: %w", err)
		}

	case ModeEmbedded:
		embeddedRouter, err := NewPlanoEmbeddedRouter(config.Embedded)
		if err != nil {
			return nil, fmt.Errorf("failed to create embedded router: %w", err)
		}

		// Start the embedded container
		if err := embeddedRouter.Start(); err != nil {
			return nil, fmt.Errorf("failed to start embedded plano: %w", err)
		}

		router = embeddedRouter

	default:
		return nil, fmt.Errorf("unsupported routing mode: %s", config.Mode)
	}

	// Setup fallback if enabled
	if config.Fallback {
		fallbackRouter, err := NewDirectRouter(config.Direct)
		if err == nil {
			switch r := router.(type) {
			case *DirectRouter:
				// No fallback for direct router
			case *PlanoProxyRouter:
				r.SetFallback(fallbackRouter)
			case *PlanoEmbeddedRouter:
				r.SetFallback(fallbackRouter)
			}
		}
	}

	return router, nil
}

// DefaultConfig returns a default configuration for direct routing
func DefaultConfig() *Config {
	return &Config{
		Mode: ModeDirect,
		Direct: &DirectConfig{
			DefaultProvider: "openai",
		},
		Fallback: false,
	}
}

// ProxyConfig returns a configuration for proxy routing
func NewProxyConfigFromURL(baseURL string) *Config {
	return &Config{
		Mode: ModeProxy,
		Proxy: &ProxyConfig{
			BaseURL: baseURL,
			Timeout: 30,
		},
		Fallback: true,
	}
}

// EmbeddedConfig returns a configuration for embedded routing
func NewEmbeddedConfigFromFile(configPath string) *Config {
	return &Config{
		Mode: ModeEmbedded,
		Embedded: &EmbeddedConfig{
			ConfigPath: configPath,
			Image:      "katanemo/plano:0.4.0",
			Ports: map[string]int{
				"ingress": 10000,
				"egress":  12000,
			},
		},
		Fallback: true,
	}
}
