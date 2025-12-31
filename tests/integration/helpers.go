package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jeffersonwarrior/modelscan/internal/database"
	"github.com/jeffersonwarrior/modelscan/internal/discovery"
	"github.com/jeffersonwarrior/modelscan/internal/keymanager"
	"github.com/jeffersonwarrior/modelscan/routing"
)

// setupTestDB creates a temporary test database
func setupTestDB(t *testing.T) *database.DB {
	t.Helper()

	tmpFile := fmt.Sprintf("/tmp/modelscan_test_%d.db", time.Now().UnixNano())
	t.Cleanup(func() {
		os.Remove(tmpFile)
	})

	db, err := database.Open(tmpFile)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

// setupDiscoveryAgent creates a test discovery agent
func setupDiscoveryAgent(t *testing.T, db *database.DB) *discovery.Agent {
	t.Helper()

	agent, err := discovery.NewAgent(discovery.Config{
		ParallelBatch: 2,
		CacheDays:     1,
		MaxRetries:    2,
		DB:            db,
	})
	if err != nil {
		t.Fatalf("Failed to create discovery agent: %v", err)
	}

	t.Cleanup(func() {
		agent.Close()
	})

	return agent
}

// setupKeyManager creates a test key manager
func setupKeyManager(t *testing.T, db *database.DB) *keymanager.KeyManager {
	t.Helper()

	adapter := &testKeyManagerAdapter{db: db}
	return keymanager.NewKeyManager(adapter, keymanager.Config{
		CacheTTL:        1 * time.Minute,
		DegradeDuration: 5 * time.Minute,
	})
}

// testKeyManagerAdapter adapts database.DB for keymanager
type testKeyManagerAdapter struct {
	db *database.DB
}

func (a *testKeyManagerAdapter) ListActiveAPIKeys(providerID string) ([]*keymanager.APIKey, error) {
	keys, err := a.db.ListActiveAPIKeys(providerID)
	if err != nil {
		return nil, err
	}

	result := make([]*keymanager.APIKey, len(keys))
	for i, k := range keys {
		result[i] = &keymanager.APIKey{
			ID:            k.ID,
			ProviderID:    k.ProviderID,
			KeyHash:       k.KeyHash,
			KeyPrefix:     k.KeyPrefix,
			Tier:          k.Tier,
			RPMLimit:      k.RPMLimit,
			TPMLimit:      k.TPMLimit,
			DailyLimit:    k.DailyLimit,
			ResetInterval: k.ResetInterval,
			LastReset:     k.LastReset,
			RequestsCount: k.RequestsCount,
			TokensCount:   k.TokensCount,
			Active:        k.Active,
			Degraded:      k.Degraded,
			DegradedUntil: k.DegradedUntil,
			CreatedAt:     k.CreatedAt,
		}
	}
	return result, nil
}

func (a *testKeyManagerAdapter) IncrementKeyUsage(keyID int, tokens int) error {
	return a.db.IncrementKeyUsage(keyID, tokens)
}

func (a *testKeyManagerAdapter) MarkKeyDegraded(keyID int, until time.Time) error {
	return a.db.MarkKeyDegraded(keyID, until)
}

func (a *testKeyManagerAdapter) ResetKeyLimits(keyID int) error {
	return a.db.ResetKeyLimits(keyID)
}

func (a *testKeyManagerAdapter) GetAPIKey(id int) (*keymanager.APIKey, error) {
	key, err := a.db.GetAPIKey(id)
	if err != nil || key == nil {
		return nil, err
	}

	return &keymanager.APIKey{
		ID:            key.ID,
		ProviderID:    key.ProviderID,
		KeyHash:       key.KeyHash,
		KeyPrefix:     key.KeyPrefix,
		Tier:          key.Tier,
		RPMLimit:      key.RPMLimit,
		TPMLimit:      key.TPMLimit,
		DailyLimit:    key.DailyLimit,
		ResetInterval: key.ResetInterval,
		LastReset:     key.LastReset,
		RequestsCount: key.RequestsCount,
		TokensCount:   key.TokensCount,
		Active:        key.Active,
		Degraded:      key.Degraded,
		DegradedUntil: key.DegradedUntil,
		CreatedAt:     key.CreatedAt,
	}, nil
}

// mockClient creates a mock routing client for testing
type mockClient struct {
	response *routing.Response
	err      error
}

func newMockClient(content string) *mockClient {
	return &mockClient{
		response: &routing.Response{
			Content: content,
			Model:   "mock-model",
			Usage: routing.Usage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
			FinishReason: "stop",
		},
	}
}

func (m *mockClient) ChatCompletion(ctx context.Context, req routing.Request) (*routing.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockClient) Close() error {
	return nil
}
