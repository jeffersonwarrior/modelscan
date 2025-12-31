package routing

import (
	"context"
	"fmt"
	"time"

	"github.com/jeffersonwarrior/modelscan/internal/keymanager"
)

// KeySelectingClient wraps a Client with automatic key selection and rotation
type KeySelectingClient struct {
	client     Client
	keyManager *keymanager.KeyManager
	providerID string
}

// NewKeySelectingClient creates a new client with automatic key management
func NewKeySelectingClient(providerID string, client Client, keyMgr *keymanager.KeyManager) *KeySelectingClient {
	return &KeySelectingClient{
		client:     client,
		keyManager: keyMgr,
		providerID: providerID,
	}
}

// ChatCompletion performs a chat completion with automatic key selection and rotation
func (ksc *KeySelectingClient) ChatCompletion(ctx context.Context, req Request) (*Response, error) {
	// Get next available key using round-robin selection
	key, err := ksc.keyManager.GetKey(ctx, ksc.providerID)
	if err != nil {
		return nil, fmt.Errorf("no API keys available for %s: %w", ksc.providerID, err)
	}

	// Add API key to request additional params
	if req.AdditionalParams == nil {
		req.AdditionalParams = make(map[string]interface{})
	}
	req.AdditionalParams["api_key"] = key.KeyHash

	// Make the request
	resp, err := ksc.client.ChatCompletion(ctx, req)

	// Record usage with key manager
	if err != nil {
		// Mark key as degraded on error (degraded for 15 minutes by default)
		_ = ksc.keyManager.MarkDegraded(ctx, key.ID, 15*time.Minute)
	} else if resp != nil {
		// Record successful usage
		_ = ksc.keyManager.RecordUsage(ctx, key.ID, resp.Usage.TotalTokens)
	}

	return resp, err
}

// Close closes the underlying client
func (ksc *KeySelectingClient) Close() error {
	return ksc.client.Close()
}

// GetProviderID returns the provider ID
func (ksc *KeySelectingClient) GetProviderID() string {
	return ksc.providerID
}

// HasKeyManager returns whether this client has key management
func (ksc *KeySelectingClient) HasKeyManager() bool {
	return ksc.keyManager != nil
}
