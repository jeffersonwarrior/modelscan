package routing

import (
	"context"

	"github.com/jeffersonwarrior/modelscan/internal/tooling"
)

// ToolingClient wraps a Client with tooling middleware for tool calling support
type ToolingClient struct {
	client     Client
	providerID string
	parser     tooling.ToolParser
	translator *tooling.SchemaTranslator
}

// NewToolingClient creates a new client with tooling middleware
func NewToolingClient(providerID string, client Client) (*ToolingClient, error) {
	// Try to get parser for this provider
	parser, err := tooling.GetParser(providerID)
	if err != nil {
		// No parser available - just use client without tooling
		return &ToolingClient{
			client:     client,
			providerID: providerID,
			parser:     nil,
			translator: nil,
		}, nil
	}

	return &ToolingClient{
		client:     client,
		providerID: providerID,
		parser:     parser,
		translator: &tooling.SchemaTranslator{},
	}, nil
}

// ChatCompletion processes request through tooling middleware
func (tc *ToolingClient) ChatCompletion(ctx context.Context, req Request) (*Response, error) {
	// TODO: Process tool schemas if present
	// This would translate tool definitions to provider-specific format

	// Call underlying client
	resp, err := tc.client.ChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	// TODO: Parse tool calls from response if parser available
	// This would extract tool calls into canonical format

	return resp, nil
}

// Close closes the underlying client
func (tc *ToolingClient) Close() error {
	return tc.client.Close()
}

// HasToolingSupport returns whether this client has tooling middleware
func (tc *ToolingClient) HasToolingSupport() bool {
	return tc.parser != nil
}

// GetProviderID returns the provider ID
func (tc *ToolingClient) GetProviderID() string {
	return tc.providerID
}
