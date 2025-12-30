package database

import (
	"os"
	"testing"
	"time"
)

// TestDatabaseLifecycle tests database creation, migration, and cleanup
func TestDatabaseLifecycle(t *testing.T) {
	dbPath := "test_lifecycle.db"
	defer os.Remove(dbPath)

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Verify database file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("Database file not created")
	}

	// Verify schema_version table exists
	var version int
	err = db.conn.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version)
	if err != nil {
		t.Fatalf("schema_version query failed: %v", err)
	}

	if version != CurrentSchemaVersion {
		t.Errorf("Expected schema version %d, got %d", CurrentSchemaVersion, version)
	}
}

// TestProviderCRUD tests provider CRUD operations
func TestProviderCRUD(t *testing.T) {
	dbPath := "test_provider.db"
	defer os.Remove(dbPath)

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Create provider
	authHeader := "Authorization"
	provider := &Provider{
		ID:           "test-provider",
		Name:         "Test Provider",
		BaseURL:      "https://api.test.com",
		AuthMethod:   "bearer",
		AuthHeader:   &authHeader,
		PricingModel: "usage",
		Status:       "online",
	}

	if err := db.CreateProvider(provider); err != nil {
		t.Fatalf("CreateProvider failed: %v", err)
	}

	// Get provider
	retrieved, err := db.GetProvider("test-provider")
	if err != nil {
		t.Fatalf("GetProvider failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected provider, got nil")
	}
	if retrieved.Name != "Test Provider" {
		t.Errorf("Expected name 'Test Provider', got '%s'", retrieved.Name)
	}

	// List providers
	providers, err := db.ListProviders()
	if err != nil {
		t.Fatalf("ListProviders failed: %v", err)
	}
	if len(providers) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(providers))
	}

	// Update provider status
	errMsg := "connection timeout"
	if err := db.UpdateProviderStatus("test-provider", "offline", &errMsg); err != nil {
		t.Fatalf("UpdateProviderStatus failed: %v", err)
	}

	retrieved, _ = db.GetProvider("test-provider")
	if retrieved.Status != "offline" {
		t.Errorf("Expected status 'offline', got '%s'", retrieved.Status)
	}

	// Get non-existent provider
	notFound, err := db.GetProvider("nonexistent")
	if err != nil {
		t.Fatalf("GetProvider failed: %v", err)
	}
	if notFound != nil {
		t.Error("Expected nil for non-existent provider")
	}
}

// TestModelOperations tests model family and model CRUD
func TestModelOperations(t *testing.T) {
	dbPath := "test_models.db"
	defer os.Remove(dbPath)

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Create provider first (foreign key constraint)
	provider := &Provider{
		ID:           "openai",
		Name:         "OpenAI",
		BaseURL:      "https://api.openai.com",
		AuthMethod:   "bearer",
		PricingModel: "usage",
		Status:       "online",
	}
	if err := db.CreateProvider(provider); err != nil {
		t.Fatalf("CreateProvider failed: %v", err)
	}

	// Create model family
	desc := "GPT models"
	family := &ModelFamily{
		ID:          "gpt",
		ProviderID:  "openai",
		Name:        "GPT",
		Description: &desc,
	}
	if err := db.CreateModelFamily(family); err != nil {
		t.Fatalf("CreateModelFamily failed: %v", err)
	}

	// Create model
	costIn := 0.5
	costOut := 1.5
	ctxWindow := 128000
	maxTok := 16384
	model := &Model{
		ID:            "gpt-4-turbo",
		FamilyID:      "gpt",
		Name:          "GPT-4 Turbo",
		CostPer1MIn:   &costIn,
		CostPer1MOut:  &costOut,
		ContextWindow: &ctxWindow,
		MaxTokens:     &maxTok,
		Status:        "online",
	}
	if err := db.CreateModel(model); err != nil {
		t.Fatalf("CreateModel failed: %v", err)
	}

	// List models
	models, err := db.ListModels()
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(models))
	}

	// List models by status
	onlineModels, err := db.ListModelsByStatus("online")
	if err != nil {
		t.Fatalf("ListModelsByStatus failed: %v", err)
	}
	if len(onlineModels) != 1 {
		t.Errorf("Expected 1 online model, got %d", len(onlineModels))
	}

	// Update model status
	errMsg := "rate limited"
	if err := db.UpdateModelStatus("gpt-4-turbo", "degraded", &errMsg); err != nil {
		t.Fatalf("UpdateModelStatus failed: %v", err)
	}
}

// TestAPIKeyOperations tests API key CRUD and management
func TestAPIKeyOperations(t *testing.T) {
	dbPath := "test_apikeys.db"
	defer os.Remove(dbPath)

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Create provider
	provider := &Provider{
		ID:           "anthropic",
		Name:         "Anthropic",
		BaseURL:      "https://api.anthropic.com",
		AuthMethod:   "x-api-key",
		PricingModel: "usage",
		Status:       "online",
	}
	if err := db.CreateProvider(provider); err != nil {
		t.Fatalf("CreateProvider failed: %v", err)
	}

	// Create API key
	apiKey := "sk-ant-1234567890abcdef"
	key, err := db.CreateAPIKey("anthropic", apiKey)
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}
	if key == nil {
		t.Fatal("Expected key, got nil")
	}
	if key.ProviderID != "anthropic" {
		t.Errorf("Expected provider 'anthropic', got '%s'", key.ProviderID)
	}

	expectedHash := HashAPIKey(apiKey)
	if key.KeyHash != expectedHash {
		t.Error("Key hash mismatch")
	}

	// Get API key
	retrieved, err := db.GetAPIKey(key.ID)
	if err != nil {
		t.Fatalf("GetAPIKey failed: %v", err)
	}
	if retrieved.ID != key.ID {
		t.Errorf("Expected ID %d, got %d", key.ID, retrieved.ID)
	}

	// List active API keys
	activeKeys, err := db.ListActiveAPIKeys("anthropic")
	if err != nil {
		t.Fatalf("ListActiveAPIKeys failed: %v", err)
	}
	if len(activeKeys) != 1 {
		t.Errorf("Expected 1 active key, got %d", len(activeKeys))
	}

	// Increment usage
	if err := db.IncrementKeyUsage(key.ID, 1000); err != nil {
		t.Fatalf("IncrementKeyUsage failed: %v", err)
	}

	retrieved, _ = db.GetAPIKey(key.ID)
	if retrieved.RequestsCount != 1 {
		t.Errorf("Expected 1 request, got %d", retrieved.RequestsCount)
	}
	if retrieved.TokensCount != 1000 {
		t.Errorf("Expected 1000 tokens, got %d", retrieved.TokensCount)
	}

	// Mark degraded
	until := time.Now().Add(1 * time.Hour)
	if err := db.MarkKeyDegraded(key.ID, until); err != nil {
		t.Fatalf("MarkKeyDegraded failed: %v", err)
	}

	retrieved, _ = db.GetAPIKey(key.ID)
	if !retrieved.Degraded {
		t.Error("Expected key to be degraded")
	}

	// Should not appear in active keys
	activeKeys, _ = db.ListActiveAPIKeys("anthropic")
	if len(activeKeys) != 0 {
		t.Errorf("Expected 0 active keys, got %d", len(activeKeys))
	}

	// Reset limits
	if err := db.ResetKeyLimits(key.ID); err != nil {
		t.Fatalf("ResetKeyLimits failed: %v", err)
	}

	retrieved, _ = db.GetAPIKey(key.ID)
	if retrieved.RequestsCount != 0 {
		t.Errorf("Expected 0 requests after reset, got %d", retrieved.RequestsCount)
	}
	if retrieved.TokensCount != 0 {
		t.Errorf("Expected 0 tokens after reset, got %d", retrieved.TokensCount)
	}
	if retrieved.Degraded {
		t.Error("Expected key not to be degraded after reset")
	}
}

// TestUsageTracking tests usage record creation and statistics
func TestUsageTracking(t *testing.T) {
	dbPath := "test_usage.db"
	defer os.Remove(dbPath)

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Setup provider, family, model
	provider := &Provider{
		ID:           "openai",
		Name:         "OpenAI",
		BaseURL:      "https://api.openai.com",
		AuthMethod:   "bearer",
		PricingModel: "usage",
		Status:       "online",
	}
	db.CreateProvider(provider)

	family := &ModelFamily{
		ID:         "gpt",
		ProviderID: "openai",
		Name:       "GPT",
	}
	db.CreateModelFamily(family)

	model := &Model{
		ID:       "gpt-4",
		FamilyID: "gpt",
		Name:     "GPT-4",
		Status:   "online",
	}
	db.CreateModel(model)

	// Record usage
	latency := 250
	usage := &UsageRecord{
		ModelID:   "gpt-4",
		Timestamp: time.Now(),
		TokensIn:  100,
		TokensOut: 200,
		Requests:  1,
		Cost:      0.015,
		LatencyMS: &latency,
		Success:   true,
	}
	if err := db.RecordUsage(usage); err != nil {
		t.Fatalf("RecordUsage failed: %v", err)
	}

	// Get usage stats
	since := time.Now().Add(-1 * time.Hour)
	stats, err := db.GetUsageStats("gpt-4", since)
	if err != nil {
		t.Fatalf("GetUsageStats failed: %v", err)
	}

	if stats["total_requests"].(int) != 1 {
		t.Errorf("Expected 1 request, got %v", stats["total_requests"])
	}
	if stats["total_tokens_in"].(int) != 100 {
		t.Errorf("Expected 100 tokens in, got %v", stats["total_tokens_in"])
	}
	if stats["total_tokens_out"].(int) != 200 {
		t.Errorf("Expected 200 tokens out, got %v", stats["total_tokens_out"])
	}
	if stats["success_rate"].(float64) != 1.0 {
		t.Errorf("Expected 100%% success rate, got %v", stats["success_rate"])
	}
}

// TestDiscoveryLogs tests discovery log CRUD
func TestDiscoveryLogs(t *testing.T) {
	dbPath := "test_discovery.db"
	defer os.Remove(dbPath)

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Create provider
	provider := &Provider{
		ID:           "newprovider",
		Name:         "New Provider",
		BaseURL:      "https://api.new.com",
		AuthMethod:   "bearer",
		PricingModel: "usage",
		Status:       "online",
	}
	db.CreateProvider(provider)

	// Create discovery log
	logID, err := db.CreateDiscoveryLog("newprovider", "claude-3-5-sonnet-20241022")
	if err != nil {
		t.Fatalf("CreateDiscoveryLog failed: %v", err)
	}
	if logID == 0 {
		t.Error("Expected non-zero log ID")
	}

	// Increment retry
	if err := db.IncrementDiscoveryRetry(logID); err != nil {
		t.Fatalf("IncrementDiscoveryRetry failed: %v", err)
	}

	// Update discovery log
	findings := `{"models": ["model-1", "model-2"]}`
	sources := `["https://docs.new.com"]`
	errMsg := "timeout"
	if err := db.UpdateDiscoveryLog(logID, "failed", &findings, &sources, 0.05, &errMsg); err != nil {
		t.Fatalf("UpdateDiscoveryLog failed: %v", err)
	}
}

// TestSDKVersions tests SDK version tracking
func TestSDKVersions(t *testing.T) {
	dbPath := "test_sdk.db"
	defer os.Remove(dbPath)

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Create provider
	provider := &Provider{
		ID:           "groq",
		Name:         "Groq",
		BaseURL:      "https://api.groq.com",
		AuthMethod:   "bearer",
		PricingModel: "usage",
		Status:       "online",
	}
	db.CreateProvider(provider)

	// Create SDK versions
	versions := []string{"v1.0.0", "v1.1.0", "v1.2.0"}
	for _, ver := range versions {
		if err := db.CreateSDKVersion("groq", ver, "/sdk/groq/"+ver); err != nil {
			t.Fatalf("CreateSDKVersion failed: %v", err)
		}
	}

	// List SDK versions
	sdkVersions, err := db.ListSDKVersions("groq")
	if err != nil {
		t.Fatalf("ListSDKVersions failed: %v", err)
	}
	if len(sdkVersions) != 3 {
		t.Errorf("Expected 3 versions, got %d", len(sdkVersions))
	}

	// Deprecate old versions (keep last 1)
	if err := db.DeprecateOldSDKVersions("groq", 1); err != nil {
		t.Fatalf("DeprecateOldSDKVersions failed: %v", err)
	}

	// Should only have 1 non-deprecated version
	sdkVersions, _ = db.ListSDKVersions("groq")
	if len(sdkVersions) != 1 {
		t.Errorf("Expected 1 active version after deprecation, got %d", len(sdkVersions))
	}
	// The remaining version should be one of the created versions
	found := false
	for _, ver := range versions {
		if sdkVersions[0].Version == ver {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Active version %s not in original versions", sdkVersions[0].Version)
	}
}

// TestSettings tests settings CRUD
func TestSettings(t *testing.T) {
	dbPath := "test_settings.db"
	defer os.Remove(dbPath)

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Set setting
	if err := db.SetSetting("api_version", "v1"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}

	// Get setting
	value, err := db.GetSetting("api_version")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}
	if value != "v1" {
		t.Errorf("Expected 'v1', got '%s'", value)
	}

	// Update setting
	if err := db.SetSetting("api_version", "v2"); err != nil {
		t.Fatalf("SetSetting update failed: %v", err)
	}

	value, _ = db.GetSetting("api_version")
	if value != "v2" {
		t.Errorf("Expected 'v2' after update, got '%s'", value)
	}

	// Get non-existent setting
	value, err = db.GetSetting("nonexistent")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}
	if value != "" {
		t.Errorf("Expected empty string for non-existent setting, got '%s'", value)
	}
}

// TestHashAPIKey tests API key hashing
func TestHashAPIKey(t *testing.T) {
	key := "sk-test-key-123"
	hash1 := HashAPIKey(key)
	hash2 := HashAPIKey(key)

	// Same key should produce same hash
	if hash1 != hash2 {
		t.Error("Same key produced different hashes")
	}

	// Different key should produce different hash
	hash3 := HashAPIKey("sk-different-key")
	if hash1 == hash3 {
		t.Error("Different keys produced same hash")
	}

	// Hash should be hex encoded SHA256 (64 chars)
	if len(hash1) != 64 {
		t.Errorf("Expected hash length 64, got %d", len(hash1))
	}
}

// TestForeignKeyConstraints tests database integrity
func TestForeignKeyConstraints(t *testing.T) {
	dbPath := "test_fk.db"
	defer os.Remove(dbPath)

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Try to create model family without provider (should fail)
	family := &ModelFamily{
		ID:         "test",
		ProviderID: "nonexistent",
		Name:       "Test",
	}
	err = db.CreateModelFamily(family)
	if err == nil {
		t.Error("Expected foreign key error when creating family with invalid provider")
	}

	// Create provider first
	provider := &Provider{
		ID:           "valid",
		Name:         "Valid",
		BaseURL:      "https://api.valid.com",
		AuthMethod:   "bearer",
		PricingModel: "usage",
		Status:       "online",
	}
	db.CreateProvider(provider)

	// Now should succeed
	family.ProviderID = "valid"
	if err := db.CreateModelFamily(family); err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}
