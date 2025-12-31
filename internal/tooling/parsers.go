package tooling

import (
	"fmt"
	"sync"
)

// ToolParser defines the interface all provider parsers must implement.
type ToolParser interface {
	// Parse extracts tool calls from a provider's response format
	Parse(response string) ([]ToolCall, error)

	// Format returns the format this parser handles
	Format() ToolFormat

	// ProviderID returns the provider identifier (e.g., "anthropic", "openai")
	ProviderID() string

	// Capabilities returns what tool features this provider supports
	Capabilities() ProviderCapabilities
}

// ParserRegistry manages the collection of registered tool parsers.
type ParserRegistry struct {
	mu      sync.RWMutex
	parsers map[string]ToolParser
}

var (
	// Global registry instance
	globalRegistry = &ParserRegistry{
		parsers: make(map[string]ToolParser),
	}
)

// RegisterParser registers a parser for a specific provider.
func RegisterParser(providerID string, parser ToolParser) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.parsers[providerID] = parser
}

// GetParser retrieves a parser by provider ID.
func GetParser(providerID string) (ToolParser, error) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	parser, ok := globalRegistry.parsers[providerID]
	if !ok {
		return nil, fmt.Errorf("no parser registered for provider: %s", providerID)
	}
	return parser, nil
}

// ListParsers returns all registered provider IDs.
func ListParsers() []string {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	providers := make([]string, 0, len(globalRegistry.parsers))
	for id := range globalRegistry.parsers {
		providers = append(providers, id)
	}
	return providers
}

// HasParser checks if a parser is registered for a provider.
func HasParser(providerID string) bool {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	_, ok := globalRegistry.parsers[providerID]
	return ok
}

// ClearParsers removes all registered parsers (primarily for testing).
func ClearParsers() {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.parsers = make(map[string]ToolParser)
}

// GetParserByFormat finds a parser that handles a specific format.
func GetParserByFormat(format ToolFormat) (ToolParser, error) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	for _, parser := range globalRegistry.parsers {
		if parser.Format() == format {
			return parser, nil
		}
	}
	return nil, fmt.Errorf("no parser found for format: %s", format)
}
