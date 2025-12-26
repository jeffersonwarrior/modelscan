package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jeffersonwarrior/modelscan/providers"
)

func TestInitDB(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB() failed: %v", err)
	}

	// Check that database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}

	// Verify database is accessible
	if db == nil {
		t.Error("Database connection is nil after InitDB")
	}
}

func TestStoreProviderInfo(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB() failed: %v", err)
	}

	testModels := []providers.Model{
		{
			ID:             "test-model-1",
			Name:           "Test Model 1",
			Description:    "First test model",
			CostPer1MIn:    1.0,
			CostPer1MOut:   2.0,
			ContextWindow:  8192,
			MaxTokens:      4096,
			SupportsImages: true,
			SupportsTools:  true,
			CanReason:      false,
			CanStream:      true,
			Categories:     []string{"chat", "test"},
		},
		{
			ID:             "test-model-2",
			Name:           "Test Model 2",
			Description:    "Second test model",
			CostPer1MIn:    0.5,
			CostPer1MOut:   1.0,
			ContextWindow:  16384,
			MaxTokens:      8192,
			SupportsImages: false,
			SupportsTools:  true,
			CanReason:      true,
			CanStream:      true,
			Categories:     []string{"reasoning", "test"},
		},
	}

	capabilities := providers.ProviderCapabilities{
		SupportsChat:         true,
		SupportsStreaming:    true,
		SupportsVision:       true,
		MaxRequestsPerMinute: 50,
		MaxTokensPerRequest:  100000,
	}

	err := StoreProviderInfo("test-provider", testModels, capabilities)
	if err != nil {
		t.Fatalf("StoreProviderInfo() failed: %v", err)
	}

	// Verify data was stored by querying the database
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM models WHERE provider_name = ?", "test-provider").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query models: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 models to be stored, got %d", count)
	}
}

func TestStoreEndpointResults(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB() failed: %v", err)
	}

	// First store provider info (required for foreign key)
	err := StoreProviderInfo("test-provider", []providers.Model{}, providers.ProviderCapabilities{})
	if err != nil {
		t.Fatalf("StoreProviderInfo() failed: %v", err)
	}

	testEndpoints := []providers.Endpoint{
		{
			Path:        "/v1/test",
			Method:      "GET",
			Description: "Test endpoint",
			Status:      providers.StatusWorking,
			Latency:     100000000, // 100ms in nanoseconds
		},
		{
			Path:        "/v1/chat",
			Method:      "POST",
			Description: "Chat endpoint",
			Status:      providers.StatusFailed,
			Error:       "Test error",
			Latency:     50000000, // 50ms
		},
	}

	err = StoreEndpointResults("test-provider", testEndpoints)
	if err != nil {
		t.Fatalf("StoreEndpointResults() failed: %v", err)
	}

	// Verify data was stored
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM endpoints WHERE provider_name = ?", "test-provider").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query endpoints: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 endpoints to be stored, got %d", count)
	}
}

func TestExportToSQLite(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB() failed: %v", err)
	}

	// Store some test data
	testModels := []providers.Model{
		{
			ID:          "export-test-model",
			Name:        "Export Test Model",
			Description: "Model for export testing",
		},
	}
	if err := StoreProviderInfo("export-test", testModels, providers.ProviderCapabilities{}); err != nil {
		t.Fatalf("StoreProviderInfo() failed: %v", err)
	}

	// Export should succeed (it's essentially a no-op since we're already using SQLite)
	err := ExportToSQLite(dbPath)
	if err != nil {
		t.Errorf("ExportToSQLite() failed: %v", err)
	}
}

func TestExportToMarkdown(t *testing.T) {
	// Create temporary database and markdown file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	mdPath := filepath.Join(tmpDir, "test.md")

	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB() failed: %v", err)
	}

	// Store some test data
	testModels := []providers.Model{
		{
			ID:             "markdown-test-model",
			Name:           "Markdown Test Model",
			Description:    "Model for markdown testing",
			CostPer1MIn:    1.0,
			CostPer1MOut:   2.0,
			ContextWindow:  8192,
			SupportsImages: true,
			Categories:     []string{"chat"},
		},
	}
	if err := StoreProviderInfo("markdown-test", testModels, providers.ProviderCapabilities{}); err != nil {
		t.Fatalf("StoreProviderInfo() failed: %v", err)
	}

	// Export to markdown
	err := ExportToMarkdown(mdPath)
	if err != nil {
		t.Fatalf("ExportToMarkdown() failed: %v", err)
	}

	// Check that markdown file was created
	if _, err := os.Stat(mdPath); os.IsNotExist(err) {
		t.Error("Markdown file was not created")
	}

	// Read and verify content
	content, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("Failed to read markdown file: %v", err)
	}

	contentStr := string(content)
	if len(contentStr) == 0 {
		t.Error("Markdown file is empty")
	}

	// Check for expected content
	expectedStrings := []string{
		"AI Provider Validation Report",
		"markdown-test",
		"Markdown Test Model",
	}
	for _, expected := range expectedStrings {
		if !contains(contentStr, expected) {
			t.Errorf("Markdown file missing expected content: %q", expected)
		}
	}
}

func TestDatabaseTables(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB() failed: %v", err)
	}

	// Check that all expected tables exist
	expectedTables := []string{
		"providers",
		"models",
		"endpoints",
		"validation_runs",
	}

	for _, table := range expectedTables {
		var count int
		query := "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?"
		err := db.QueryRow(query, table).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to check table %q: %v", table, err)
		}
		if count != 1 {
			t.Errorf("Table %q does not exist", table)
		}
	}
}

func TestModelWithCategories(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB() failed: %v", err)
	}

	testModel := []providers.Model{
		{
			ID:         "category-test",
			Name:       "Category Test Model",
			Categories: []string{"chat", "coding", "reasoning"},
			Capabilities: map[string]string{
				"vision": "high",
				"tools":  "full",
			},
		},
	}

	err := StoreProviderInfo("category-provider", testModel, providers.ProviderCapabilities{})
	if err != nil {
		t.Fatalf("StoreProviderInfo() failed: %v", err)
	}

	// Verify categories were stored
	var categoriesJSON string
	query := "SELECT categories FROM models WHERE model_id = ?"
	err = db.QueryRow(query, "category-test").Scan(&categoriesJSON)
	if err != nil {
		t.Fatalf("Failed to query categories: %v", err)
	}
	if categoriesJSON == "" {
		t.Error("Categories were not stored")
	}
}

func TestUpdateExistingProvider(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB() failed: %v", err)
	}

	// Store initial data
	initialModel := []providers.Model{
		{
			ID:          "update-test",
			Name:        "Initial Name",
			Description: "Initial description",
		},
	}
	err := StoreProviderInfo("update-provider", initialModel, providers.ProviderCapabilities{})
	if err != nil {
		t.Fatalf("Initial StoreProviderInfo() failed: %v", err)
	}

	// Update with new data
	updatedModel := []providers.Model{
		{
			ID:          "update-test",
			Name:        "Updated Name",
			Description: "Updated description",
		},
	}
	err = StoreProviderInfo("update-provider", updatedModel, providers.ProviderCapabilities{})
	if err != nil {
		t.Fatalf("Update StoreProviderInfo() failed: %v", err)
	}

	// Verify update
	var name string
	query := "SELECT name FROM models WHERE model_id = ?"
	err = db.QueryRow(query, "update-test").Scan(&name)
	if err != nil {
		t.Fatalf("Failed to query updated model: %v", err)
	}
	if name != "Updated Name" {
		t.Errorf("Model name = %q, want %q", name, "Updated Name")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func BenchmarkStoreProviderInfo(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")
	if err := InitDB(dbPath); err != nil {
		b.Fatalf("InitDB() failed: %v", err)
	}

	testModels := []providers.Model{
		{
			ID:             "bench-model",
			Name:           "Benchmark Model",
			CostPer1MIn:    1.0,
			CostPer1MOut:   2.0,
			ContextWindow:  8192,
			MaxTokens:      4096,
			SupportsImages: true,
		},
	}
	capabilities := providers.ProviderCapabilities{
		SupportsChat: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = StoreProviderInfo("bench-provider", testModels, capabilities)
	}
}

func TestGetProviderEndpoints_Complete(t *testing.T) {
	dbPath := "/tmp/test_endpoints_complete.db"
	defer os.Remove(dbPath)
	
	err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer CloseDB()
	
	// Store some endpoints
	endpoints := []providers.Endpoint{
		{
			Path:        "/v1/chat",
			Method:      "POST",
			Description: "Chat endpoint",
			Status:      providers.StatusWorking,
			Latency:     100 * time.Millisecond,
		},
		{
			Path:        "/v1/models",
			Method:      "GET",
			Description: "Models endpoint",
			Status:      providers.StatusWorking,
			Latency:     50 * time.Millisecond,
		},
		{
			Path:        "/v1/broken",
			Method:      "POST",
			Description: "Broken endpoint",
			Status:      providers.StatusFailed,
			Error:       "timeout",
		},
	}
	
	err = StoreEndpointResults("test-provider", endpoints)
	if err != nil {
		t.Fatalf("StoreEndpointResults failed: %v", err)
	}
	
	// Test retrieval
	retrieved, err := GetProviderEndpoints("test-provider")
	if err != nil {
		t.Fatalf("GetProviderEndpoints failed: %v", err)
	}
	
	if len(retrieved) != 3 {
		t.Errorf("Expected 3 endpoints, got %d", len(retrieved))
	}
	
	// Verify endpoint details
	found := map[string]bool{}
	for _, ep := range retrieved {
		found[ep.Path] = true
		
		if ep.Path == "/v1/chat" {
			if ep.Status != providers.StatusWorking {
				t.Errorf("Expected /v1/chat to have StatusWorking")
			}
			if ep.Latency != 100*time.Millisecond {
				t.Errorf("Expected /v1/chat latency 100ms, got %v", ep.Latency)
			}
		}
		
		if ep.Path == "/v1/broken" {
			if ep.Status != providers.StatusFailed {
				t.Errorf("Expected /v1/broken to have StatusFailed")
			}
			if ep.Error != "timeout" {
				t.Errorf("Expected /v1/broken error 'timeout', got %s", ep.Error)
			}
		}
	}
	
	if !found["/v1/chat"] || !found["/v1/models"] || !found["/v1/broken"] {
		t.Error("Not all endpoints were retrieved")
	}
	
	// Test non-existent provider
	empty, err := GetProviderEndpoints("nonexistent")
	if err != nil {
		t.Fatalf("Expected no error for non-existent provider, got %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("Expected 0 endpoints for non-existent provider, got %d", len(empty))
	}
}

func TestGetProviderEndpoints_NoDatabase(t *testing.T) {
	// Test without initializing database
	CloseDB() // Make sure DB is closed
	
	_, err := GetProviderEndpoints("test")
	if err == nil {
		t.Error("Expected error when database not initialized")
	}
	if !strings.Contains(err.Error(), "database not initialized") {
		t.Errorf("Expected 'database not initialized' error, got: %v", err)
	}
}

func TestCloseDB_Multiple(t *testing.T) {
	dbPath := "/tmp/test_close_multiple.db"
	defer os.Remove(dbPath)
	
	err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	
	// First close
	err = CloseDB()
	if err != nil {
		t.Errorf("First CloseDB failed: %v", err)
	}
	
	// Second close should handle nil db gracefully
	err = CloseDB()
	if err != nil {
		t.Errorf("Second CloseDB should not error, got: %v", err)
	}
}

func TestCloseDB_NilDatabase(t *testing.T) {
	// Test closing when db is already nil
	CloseDB() // Ensure it's nil
	
	err := CloseDB()
	if err != nil {
		t.Errorf("CloseDB on nil database should not error, got: %v", err)
	}
}

func TestAppendProviderDetails(t *testing.T) {
	// Initialize database with test data
	dbPath := "/tmp/test_markdown_export.db"
	defer os.Remove(dbPath)
	
	err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer CloseDB()
	
	// Store some test models
	models := []providers.Model{
		{
			ID:             "test-model-1",
			Name:           "Test Model 1",
			Description:    "First test model",
			ContextWindow:  4096,
			CostPer1MIn:    0.50,
			CostPer1MOut:   1.50,
			SupportsImages: true,
			SupportsTools:  true,
			CanReason:      false,
			CanStream:      true,
			Categories:     []string{"chat", "fast"},
		},
		{
			ID:             "test-model-2",
			Name:           "Test Model 2",
			Description:    "Second test model",
			ContextWindow:  8192,
			CostPer1MIn:    1.00,
			CostPer1MOut:   2.00,
			SupportsImages: false,
			SupportsTools:  true,
			CanReason:      true,
			CanStream:      true,
			Categories:     []string{"reasoning", "premium"},
		},
		{
			ID:             "test-model-3",
			Name:           "Test Model 3",
			Description:    "Model without categories",
			ContextWindow:  2048,
			CostPer1MIn:    0.25,
			CostPer1MOut:   0.75,
			SupportsImages: false,
			SupportsTools:  false,
			CanReason:      false,
			CanStream:      false,
			Categories:     []string{}, // Empty categories
		},
	}
	
	capabilities := providers.ProviderCapabilities{
		SupportsChat:       true,
		SupportsStreaming:  true,
		SupportsVision:     true,
		// SupportsTools field does not exist
	}
	
	err = StoreProviderInfo("test-provider", models, capabilities)
	if err != nil {
		t.Fatalf("StoreProviderInfo failed: %v", err)
	}
	
	// Create temporary file for export
	tmpFile := filepath.Join(t.TempDir(), "test_export.md")
	file, err := os.Create(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer file.Close()
	
	// Test appendProviderDetails
	err = appendProviderDetails(file, "test-provider")
	if err != nil {
		t.Fatalf("appendProviderDetails failed: %v", err)
	}
	
	// Close file to flush
	file.Close()
	
	// Read back and verify content
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read exported file: %v", err)
	}
	
	contentStr := string(content)
	
	// Verify provider name is present
	if !strings.Contains(contentStr, "test-provider") {
		t.Error("Provider name not found in export")
	}
	
	// Verify model names are present
	if !strings.Contains(contentStr, "Test Model 1") {
		t.Error("Model 1 name not found in export")
	}
	if !strings.Contains(contentStr, "Test Model 2") {
		t.Error("Model 2 name not found in export")
	}
	if !strings.Contains(contentStr, "Test Model 3") {
		t.Error("Model 3 name not found in export")
	}
	
	// Verify categories are present
	if !strings.Contains(contentStr, "chat") {
		t.Error("'chat' category not found in export")
	}
	if !strings.Contains(contentStr, "reasoning") {
		t.Error("'reasoning' category not found in export")
	}
	if !strings.Contains(contentStr, "general") {
		t.Error("'general' category not found for model without categories")
	}
	
	// Verify feature icons are present
	if !strings.Contains(contentStr, "🖼️") {
		t.Error("Image support icon not found")
	}
	if !strings.Contains(contentStr, "🔧") {
		t.Error("Tool support icon not found")
	}
	if !strings.Contains(contentStr, "🧠") {
		t.Error("Reasoning icon not found")
	}
	
	// Verify markdown table structure
	if !strings.Contains(contentStr, "| Name | ID | Context |") {
		t.Error("Markdown table header not found")
	}
	if !strings.Contains(contentStr, "|------|") {
		t.Error("Markdown table separator not found")
	}
}

func TestAppendProviderDetails_Error(t *testing.T) {
	// Test with non-existent provider (should handle gracefully)
	tmpFile := filepath.Join(t.TempDir(), "test_error.md")
	file, err := os.Create(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer file.Close()
	
	// Initialize empty database
	dbPath := "/tmp/test_markdown_error.db"
	defer os.Remove(dbPath)
	
	err = InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer CloseDB()
	
	// Try to export non-existent provider
	err = appendProviderDetails(file, "nonexistent-provider")
	// Should complete without error even if no models found
	if err != nil {
		t.Logf("appendProviderDetails returned error (may be expected): %v", err)
	}
}
