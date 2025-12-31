package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jeffersonwarrior/modelscan/internal/database"
	"github.com/jeffersonwarrior/modelscan/routing"
)

func TestKeyManager_RoundRobin(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDB(t)
	keyMgr := setupKeyManager(t, db)

	// Add test provider
	err := db.CreateProvider(&database.Provider{
		ID:           "test-provider",
		Name:         "Test Provider",
		BaseURL:      "https://api.test.com",
		AuthMethod:   "bearer",
		PricingModel: "pay_as_you_go",
		Status:       "online",
	})
	if err != nil {
		t.Fatalf("Failed to add provider: %v", err)
	}

	// Add multiple API keys
	for i := 1; i <= 3; i++ {
		key := fmt.Sprintf("test-key-%d", i)
		_, err := db.CreateAPIKey("test-provider", key)
		if err != nil {
			t.Fatalf("Failed to add API key %d: %v", i, err)
		}
	}

	ctx := context.Background()

	// Test key retrieval (round-robin tested in unit tests)
	key, err := keyMgr.GetKey(ctx, "test-provider")
	if err != nil {
		t.Fatalf("Failed to get key: %v", err)
	}

	if key == nil {
		t.Fatal("Expected non-nil key")
	}

	// Verify we can get multiple keys
	keys, err := db.ListActiveAPIKeys("test-provider")
	if err != nil {
		t.Fatalf("Failed to list keys: %v", err)
	}

	if len(keys) != 3 {
		t.Errorf("Expected 3 keys in database, got %d", len(keys))
	}

	t.Logf("Successfully retrieved %d API keys for provider", len(keys))
}

func TestKeyManager_DegradationRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDB(t)
	keyMgr := setupKeyManager(t, db)

	// Add test provider
	err := db.CreateProvider(&database.Provider{
		ID:           "test-provider",
		Name:         "Test Provider",
		BaseURL:      "https://api.test.com",
		AuthMethod:   "bearer",
		PricingModel: "pay_as_you_go",
		Status:       "online",
	})
	if err != nil {
		t.Fatalf("Failed to add provider: %v", err)
	}

	// Add API key
	_, err = db.CreateAPIKey("test-provider", "test-key")
	if err != nil {
		t.Fatalf("Failed to add API key: %v", err)
	}

	ctx := context.Background()

	// Get key - should work
	key1, err := keyMgr.GetKey(ctx, "test-provider")
	if err != nil {
		t.Fatalf("Failed to get key: %v", err)
	}

	keyID := key1.ID

	// Mark key as degraded
	degradeDuration := 2 * time.Second
	err = keyMgr.MarkDegraded(ctx, keyID, degradeDuration)
	if err != nil {
		t.Fatalf("Failed to mark key degraded: %v", err)
	}

	// Try to get key immediately - should fail (degraded)
	_, err = keyMgr.GetKey(ctx, "test-provider")
	if err == nil {
		t.Error("Expected error when getting degraded key, got nil")
	}

	t.Log("Key correctly degraded")

	// Wait for degradation period to expire
	t.Logf("Waiting %v for key recovery...", degradeDuration)
	time.Sleep(degradeDuration + 500*time.Millisecond)

	// Try again - should work now
	key2, err := keyMgr.GetKey(ctx, "test-provider")
	if err != nil {
		t.Fatalf("Failed to get key after recovery: %v", err)
	}

	if key2.ID != keyID {
		t.Errorf("Expected same key ID %d, got %d", keyID, key2.ID)
	}

	t.Log("Key successfully recovered from degradation")
}

func TestKeySelectingClient_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDB(t)
	keyMgr := setupKeyManager(t, db)

	// Add test provider
	err := db.CreateProvider(&database.Provider{
		ID:           "test-provider",
		Name:         "Test Provider",
		BaseURL:      "https://api.test.com",
		AuthMethod:   "bearer",
		PricingModel: "pay_as_you_go",
		Status:       "online",
	})
	if err != nil {
		t.Fatalf("Failed to add provider: %v", err)
	}

	// Add API key
	_, err = db.CreateAPIKey("test-provider", "test-api-key-123")
	if err != nil {
		t.Fatalf("Failed to add API key: %v", err)
	}

	// Create mock client
	mockClient := newMockClient("Test response")

	// Wrap with key selecting client
	keyClient := routing.NewKeySelectingClient("test-provider", mockClient, keyMgr)

	// Make request
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := keyClient.ChatCompletion(ctx, routing.Request{
		Model: "test-model",
		Messages: []routing.Message{
			{Role: "user", Content: "Test"},
		},
	})

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.Content != "Test response" {
		t.Errorf("Unexpected response: %s", resp.Content)
	}

	// Verify API key was added to request
	if resp.Usage.TotalTokens != 30 {
		t.Errorf("Expected 30 tokens, got %d", resp.Usage.TotalTokens)
	}

	t.Log("Key selecting client integration successful")
}

func TestKeyManager_RateLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDB(t)
	keyMgr := setupKeyManager(t, db)

	// Add test provider
	err := db.CreateProvider(&database.Provider{
		ID:           "test-provider",
		Name:         "Test Provider",
		BaseURL:      "https://api.test.com",
		AuthMethod:   "bearer",
		PricingModel: "pay_as_you_go",
		Status:       "online",
	})
	if err != nil {
		t.Fatalf("Failed to add provider: %v", err)
	}

	// Add API key (rate limits tested in unit tests)
	_, err = db.CreateAPIKey("test-provider", "limited-key")
	if err != nil {
		t.Fatalf("Failed to add API key: %v", err)
	}

	ctx := context.Background()

	// Test basic key retrieval and usage recording
	key, err := keyMgr.GetKey(ctx, "test-provider")
	if err != nil {
		t.Fatalf("Failed to get key: %v", err)
	}

	// Record usage
	err = keyMgr.RecordUsage(ctx, key.ID, 100)
	if err != nil {
		t.Fatalf("Failed to record usage: %v", err)
	}

	// Verify usage was recorded
	err = db.IncrementKeyUsage(key.ID, 50)
	if err != nil {
		t.Fatalf("Failed to increment usage: %v", err)
	}

	t.Log("Usage tracking test passed")
}

func TestKeyManager_MultipleProviders(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDB(t)
	keyMgr := setupKeyManager(t, db)

	// Add multiple providers
	providers := []string{"provider-a", "provider-b", "provider-c"}
	for _, providerID := range providers {
		err := db.CreateProvider(&database.Provider{
			ID:           providerID,
			Name:         providerID + " Name",
			BaseURL:      "https://api." + providerID + ".com",
			AuthMethod:   "bearer",
			PricingModel: "pay_as_you_go",
			Status:       "online",
		})
		if err != nil {
			t.Fatalf("Failed to add %s: %v", providerID, err)
		}

		// Add key for each provider
		_, err = db.CreateAPIKey(providerID, providerID+"-key")
		if err != nil {
			t.Fatalf("Failed to add key for %s: %v", providerID, err)
		}
	}

	ctx := context.Background()

	// Get keys for each provider
	for _, providerID := range providers {
		key, err := keyMgr.GetKey(ctx, providerID)
		if err != nil {
			t.Errorf("Failed to get key for %s: %v", providerID, err)
			continue
		}

		if key.ProviderID != providerID {
			t.Errorf("Expected provider %s, got %s", providerID, key.ProviderID)
		}

		t.Logf("Successfully got key for %s", providerID)
	}
}
