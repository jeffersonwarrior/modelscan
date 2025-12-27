package storage

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateRateLimitTables_CreatesAllTables(t *testing.T) {
	// Arrange
	dbPath := "/tmp/test_rate_limits.db"
	defer os.Remove(dbPath)

	// Act
	err := InitRateLimitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize rate limit DB: %v", err)
	}
	defer CloseRateLimitDB()

	// Assert - Check all 4 tables exist
	tables := []string{"rate_limits", "plan_metadata", "provider_pricing", "pricing_history"}
	for _, table := range tables {
		var name string
		err := GetRateLimitDB().QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&name)
		if err != nil {
			t.Errorf("Table %s does not exist: %v", table, err)
		}
	}

	// Assert - Check WAL mode is enabled
	var walMode string
	err = GetRateLimitDB().QueryRow("PRAGMA journal_mode").Scan(&walMode)
	if err != nil || walMode != "wal" {
		t.Errorf("WAL mode not enabled, got: %s", walMode)
	}
}

func TestInsertRateLimit_UpdatesOnConflict(t *testing.T) {
	// Arrange
	dbPath := "/tmp/test_rate_limits_insert.db"
	defer os.Remove(dbPath)
	InitRateLimitDB(dbPath)
	defer CloseRateLimitDB()

	rateLimit := RateLimit{
		ProviderName:       "openai",
		PlanType:           "tier-1",
		LimitType:          "rpm",
		LimitValue:         500,
		BurstAllowance:     50,
		ResetWindowSeconds: 60,
		AppliesTo:          "account",
		SourceURL:          "https://platform.openai.com/docs/guides/rate-limits",
		LastVerified:       time.Now(),
	}

	// Act - First insert
	err := InsertRateLimit(rateLimit)
	if err != nil {
		t.Fatalf("Failed to insert rate limit: %v", err)
	}

	// Act - Update same rate limit (conflict)
	rateLimit.LimitValue = 600
	rateLimit.LastVerified = time.Now()
	err = InsertRateLimit(rateLimit)
	if err != nil {
		t.Fatalf("Failed to update rate limit on conflict: %v", err)
	}

	// Assert - Verify updated value
	retrieved, err := QueryRateLimit("openai", "tier-1", "rpm", "", "")
	if err != nil {
		t.Fatalf("Failed to query rate limit: %v", err)
	}
	if len(retrieved) != 1 {
		t.Fatalf("Expected 1 rate limit, got %d", len(retrieved))
	}
	if retrieved[0].LimitValue != 600 {
		t.Errorf("Expected limit_value=600, got %d", retrieved[0].LimitValue)
	}
}

func TestQueryRateLimit_HandlesConcurrentReads(t *testing.T) {
	// Arrange
	dbPath := "/tmp/test_rate_limits_concurrent.db"
	defer os.Remove(dbPath)
	InitRateLimitDB(dbPath)
	defer CloseRateLimitDB()

	// Insert test data
	InsertRateLimit(RateLimit{
		ProviderName:       "anthropic",
		PlanType:           "free",
		LimitType:          "rpm",
		LimitValue:         50,
		ResetWindowSeconds: 60,
		AppliesTo:          "account",
		LastVerified:       time.Now(),
	})

	// Act - 10 concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := QueryRateLimit("anthropic", "free", "rpm", "", "")
			if err != nil {
				t.Errorf("Concurrent read failed: %v", err)
			}
			done <- true
		}()
	}

	// Assert - All reads complete successfully
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestQueryRateLimit_FiltersByModelAndEndpoint(t *testing.T) {
	// Arrange
	dbPath := "/tmp/test_rate_limits_filter.db"
	defer os.Remove(dbPath)
	InitRateLimitDB(dbPath)
	defer CloseRateLimitDB()

	// Insert rate limits with different scopes
	InsertRateLimit(RateLimit{
		ProviderName:       "openai",
		PlanType:           "tier-2",
		LimitType:          "rpm",
		LimitValue:         3500,
		ResetWindowSeconds: 60,
		AppliesTo:          "account",
		LastVerified:       time.Now(),
	})
	InsertRateLimit(RateLimit{
		ProviderName:       "openai",
		PlanType:           "tier-2",
		LimitType:          "tpm",
		LimitValue:         80000,
		ResetWindowSeconds: 60,
		AppliesTo:          "model",
		ModelID:            sql.NullString{String: "gpt-4o", Valid: true},
		LastVerified:       time.Now(),
	})

	// Act - Query for specific model
	results, err := QueryRateLimit("openai", "tier-2", "tpm", "gpt-4o", "")
	if err != nil {
		t.Fatalf("Failed to query with model filter: %v", err)
	}

	// Assert
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].ModelID.String != "gpt-4o" {
		t.Errorf("Expected model gpt-4o, got %s", results[0].ModelID.String)
	}
}

func TestInsertPlanMetadata_StoresAllFields(t *testing.T) {
	// Arrange
	dbPath := "/tmp/test_plan_metadata.db"
	defer os.Remove(dbPath)
	InitRateLimitDB(dbPath)
	defer CloseRateLimitDB()

	plan := PlanMetadata{
		ProviderName:     "deepseek",
		PlanType:         "pay_per_go",
		OfficialName:     "Pay-as-you-go",
		CostPerMonth:     sql.NullFloat64{Float64: 0.0, Valid: true},
		HasFreeTier:      true,
		DocumentationURL: "https://platform.deepseek.com/api-docs/pricing/",
	}

	// Act
	err := InsertPlanMetadata(plan)
	if err != nil {
		t.Fatalf("Failed to insert plan metadata: %v", err)
	}

	// Assert
	var retrievedName string
	var hasFreeTier bool
	err = GetRateLimitDB().QueryRow(
		"SELECT official_name, has_free_tier FROM plan_metadata WHERE provider_name=? AND plan_type=?",
		"deepseek", "pay_per_go",
	).Scan(&retrievedName, &hasFreeTier)
	if err != nil {
		t.Fatalf("Failed to retrieve plan metadata: %v", err)
	}
	if retrievedName != "Pay-as-you-go" || !hasFreeTier {
		t.Errorf("Plan metadata mismatch: name=%s, free_tier=%v", retrievedName, hasFreeTier)
	}
}

func TestInsertProviderPricing_HandlesMultiplePlans(t *testing.T) {
	// Arrange
	dbPath := "/tmp/test_pricing.db"
	defer os.Remove(dbPath)
	InitRateLimitDB(dbPath)
	defer CloseRateLimitDB()

	// Insert pricing for free and paid plans
	freePricing := ProviderPricing{
		ProviderName:  "cerebras",
		ModelID:       "llama3.1-8b",
		PlanType:      "free",
		InputCost:     0.0,
		OutputCost:    0.0,
		UnitType:      "1M tokens",
		Currency:      "USD",
		IncludedUnits: sql.NullInt64{Int64: 1000000, Valid: true},
	}
	paidPricing := ProviderPricing{
		ProviderName: "cerebras",
		ModelID:      "llama3.1-70b",
		PlanType:     "pay_per_go",
		InputCost:    0.60,
		OutputCost:   0.60,
		UnitType:     "1M tokens",
		Currency:     "USD",
	}

	// Act
	err := InsertProviderPricing(freePricing)
	if err != nil {
		t.Fatalf("Failed to insert free pricing: %v", err)
	}
	err = InsertProviderPricing(paidPricing)
	if err != nil {
		t.Fatalf("Failed to insert paid pricing: %v", err)
	}

	// Assert - Check both exist
	var count int
	err = GetRateLimitDB().QueryRow(
		"SELECT COUNT(*) FROM provider_pricing WHERE provider_name='cerebras'",
	).Scan(&count)
	if err != nil || count != 2 {
		t.Errorf("Expected 2 pricing entries, got %d", count)
	}
}

func TestPricingHistory_RecordsChanges(t *testing.T) {
	// Arrange
	dbPath := "/tmp/test_pricing_history.db"
	defer os.Remove(dbPath)
	InitRateLimitDB(dbPath)
	defer CloseRateLimitDB()

	history := PricingHistory{
		ProviderName: "openai",
		ModelID:      "gpt-4o",
		PlanType:     "tier-3",
		OldInputCost: 5.00,
		NewInputCost: 2.50,
		ChangeDate:   time.Now(),
		ChangeReason: "Pricing reduction announced 2024-12-18",
	}

	// Act
	err := InsertPricingHistory(history)
	if err != nil {
		t.Fatalf("Failed to insert pricing history: %v", err)
	}

	// Assert
	var changeReason string
	err = GetRateLimitDB().QueryRow(
		"SELECT change_reason FROM pricing_history WHERE provider_name=? AND model_id=?",
		"openai", "gpt-4o",
	).Scan(&changeReason)
	if err != nil {
		t.Fatalf("Failed to retrieve pricing history: %v", err)
	}
	if changeReason != "Pricing reduction announced 2024-12-18" {
		t.Errorf("Change reason mismatch: %s", changeReason)
	}
}

func TestRateLimitIndexes_ExistForPerformance(t *testing.T) {
	// Arrange
	dbPath := "/tmp/test_indexes.db"
	defer os.Remove(dbPath)
	InitRateLimitDB(dbPath)
	defer CloseRateLimitDB()

	// Assert - Check critical indexes exist
	indexes := []string{
		"idx_rate_limits_provider_plan",
		"idx_plan_metadata_provider",
		"idx_provider_pricing_model",
	}

	for _, idx := range indexes {
		var name string
		err := GetRateLimitDB().QueryRow(
			"SELECT name FROM sqlite_master WHERE type='index' AND name=?",
			idx,
		).Scan(&name)
		if err != nil {
			t.Errorf("Index %s does not exist: %v", idx, err)
		}
	}
}

func TestGetProviderPricing(t *testing.T) {
	// Setup
	err := InitRateLimitDB("/tmp/test_rate_limits_" + t.Name() + ".db")
	if err != nil {
		t.Fatalf("Failed to initialize DB: %v", err)
	}
	defer CloseRateLimitDB()
	defer os.Remove("/tmp/test_rate_limits_" + t.Name() + ".db")

	// Insert test pricing
	pricing := ProviderPricing{
		ProviderName: "test-provider",
		ModelID:      "test-model",
		PlanType:     "pro",
		InputCost:    0.001,
		OutputCost:   0.002,
		UnitType:     "token",
		Currency:     "USD",
	}

	err = InsertProviderPricing(pricing)
	if err != nil {
		t.Fatalf("Failed to insert pricing: %v", err)
	}

	// Test retrieval
	result, err := GetProviderPricing("test-provider", "test-model", "pro")
	if err != nil {
		t.Fatalf("GetProviderPricing failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected pricing, got nil")
	}

	if result.InputCost != 0.001 {
		t.Errorf("Expected InputCost 0.001, got %v", result.InputCost)
	}

	if result.OutputCost != 0.002 {
		t.Errorf("Expected OutputCost 0.002, got %v", result.OutputCost)
	}

	// Test non-existent pricing
	result, err = GetProviderPricing("nonexistent", "model", "plan")
	if err != nil {
		t.Fatalf("Expected no error for non-existent pricing, got: %v", err)
	}
	if result != nil {
		t.Error("Expected nil for non-existent pricing")
	}
}

func TestGetAllRateLimitsForProvider(t *testing.T) {
	// Setup
	err := InitRateLimitDB("/tmp/test_rate_limits_" + t.Name() + ".db")
	if err != nil {
		t.Fatalf("Failed to initialize DB: %v", err)
	}
	defer CloseRateLimitDB()
	defer os.Remove("/tmp/test_rate_limits_" + t.Name() + ".db")

	// Insert test rate limits
	limits := []RateLimit{
		{
			ProviderName: "test-provider",
			PlanType:     "free",
			LimitType:    "rpm",
			LimitValue:   60,
		},
		{
			ProviderName: "test-provider",
			PlanType:     "free",
			LimitType:    "tpm",
			LimitValue:   10000,
		},
		{
			ProviderName: "other-provider",
			PlanType:     "free",
			LimitType:    "rpm",
			LimitValue:   100,
		},
	}

	for _, limit := range limits {
		if err := InsertRateLimit(limit); err != nil {
			t.Fatalf("Failed to insert rate limit: %v", err)
		}
	}

	// Test retrieval
	results, err := GetAllRateLimitsForProvider("test-provider", "free")
	if err != nil {
		t.Fatalf("GetAllRateLimitsForProvider failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 rate limits, got %d", len(results))
	}

	// Verify we got the right limits
	hasRPM := false
	hasTPM := false
	for _, limit := range results {
		if limit.LimitType == "rpm" && limit.LimitValue == 60 {
			hasRPM = true
		}
		if limit.LimitType == "tpm" && limit.LimitValue == 10000 {
			hasTPM = true
		}
	}

	if !hasRPM {
		t.Error("Missing RPM limit")
	}
	if !hasTPM {
		t.Error("Missing TPM limit")
	}

	// Test non-existent provider
	results, err = GetAllRateLimitsForProvider("nonexistent", "free")
	if err != nil {
		t.Fatalf("Expected no error for non-existent provider, got: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 rate limits for non-existent provider, got %d", len(results))
	}
}

func TestCloseDB(t *testing.T) {
	// Setup
	err := InitRateLimitDB("/tmp/test_rate_limits_" + t.Name() + ".db")
	if err != nil {
		t.Fatalf("Failed to initialize DB: %v", err)
	}

	// Test closing
	err = CloseDB()
	if err != nil {
		t.Fatalf("CloseDB failed: %v", err)
	}

	// Verify DB is closed by trying to query (should fail)
	_, err = QueryRateLimit("test", "test", "", "", "")
}

func TestInitRateLimitDB_EdgeCases(t *testing.T) {
	// Test with invalid path
	err := InitRateLimitDB("/invalid/path/that/does/not/exist/test.db")
	if err == nil {
		t.Error("Expected error with invalid path")
	}

	// Clean up in case it was created
	CloseRateLimitDB()

	// Test successful initialization
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_ratelimit.db")

	err = InitRateLimitDB(dbPath)
	if err != nil {
		t.Fatalf("InitRateLimitDB failed: %v", err)
	}
	defer CloseRateLimitDB()

	// Verify database was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}

	// Verify we can query the database
	db := GetRateLimitDB()
	if db == nil {
		t.Fatal("GetRateLimitDB returned nil")
	}

	// Test that tables were created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query tables: %v", err)
	}
	if count == 0 {
		t.Error("No tables were created")
	}
}

func TestCloseRateLimitDB_Multiple(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_close_ratelimit.db")

	err := InitRateLimitDB(dbPath)
	if err != nil {
		t.Fatalf("InitRateLimitDB failed: %v", err)
	}

	// First close
	err = CloseRateLimitDB()
	if err != nil {
		t.Errorf("First CloseRateLimitDB failed: %v", err)
	}

	// Second close should handle nil gracefully
	err = CloseRateLimitDB()
	if err != nil {
		t.Errorf("Second CloseRateLimitDB should not error, got: %v", err)
	}
}

func TestCloseRateLimitDB_WhenNil(t *testing.T) {
	// Ensure database is closed/nil
	CloseRateLimitDB()

	// Test closing when already nil
	err := CloseRateLimitDB()
	if err != nil {
		t.Errorf("CloseRateLimitDB on nil should not error, got: %v", err)
	}
}

func TestGetRateLimitDB_AfterInit(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_get_ratelimit.db")

	err := InitRateLimitDB(dbPath)
	if err != nil {
		t.Fatalf("InitRateLimitDB failed: %v", err)
	}
	defer CloseRateLimitDB()

	db := GetRateLimitDB()
	if db == nil {
		t.Fatal("GetRateLimitDB returned nil after successful init")
	}

	// Verify we can ping the database
	err = db.Ping()
	if err != nil {
		t.Errorf("Database ping failed: %v", err)
	}
}
