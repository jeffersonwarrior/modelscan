package service

import (
	"os"
	"testing"
	"time"

	"github.com/jeffersonwarrior/modelscan/internal/database"
)

func TestNewService(t *testing.T) {
	cfg := &Config{
		DatabasePath:  "/tmp/test.db",
		ServerHost:    "127.0.0.1",
		ServerPort:    8080,
		AgentModel:    "claude-sonnet-4-5",
		ParallelBatch: 5,
		CacheDays:     7,
		OutputDir:     "generated",
		RoutingMode:   "direct",
	}

	service := NewService(cfg)
	if service == nil {
		t.Fatal("expected non-nil service")
	}

	if service.config.ServerPort != 8080 {
		t.Errorf("expected port 8080, got %d", service.config.ServerPort)
	}

	if service.config.AgentModel != "claude-sonnet-4-5" {
		t.Errorf("expected agent model claude-sonnet-4-5, got %s", service.config.AgentModel)
	}
}

func TestServiceHealth_NotInitialized(t *testing.T) {
	service := NewService(&Config{})

	health := service.Health()

	if health["status"] != "not_initialized" {
		t.Errorf("expected status not_initialized, got %v", health["status"])
	}
	if health["initialized"] != false {
		t.Error("expected initialized=false")
	}
	if health["restarting"] != false {
		t.Error("expected restarting=false")
	}
}

func TestServiceHealth_Initialized(t *testing.T) {
	service := NewService(&Config{})
	service.initialized = true

	health := service.Health()

	if health["status"] != "ok" {
		t.Errorf("expected status ok, got %v", health["status"])
	}
	if health["initialized"] != true {
		t.Error("expected initialized=true")
	}
	if health["restarting"] != false {
		t.Error("expected restarting=false")
	}
}

func TestServiceHealth_Restarting(t *testing.T) {
	service := NewService(&Config{})
	service.initialized = true
	service.restarting = true

	health := service.Health()

	if health["status"] != "restarting" {
		t.Errorf("expected status restarting, got %v", health["status"])
	}
	if health["initialized"] != true {
		t.Error("expected initialized=true")
	}
	if health["restarting"] != true {
		t.Error("expected restarting=true")
	}
}

func TestServiceStop_NotInitialized(t *testing.T) {
	service := NewService(&Config{})

	err := service.Stop()
	if err != nil {
		t.Fatalf("Stop should not fail on non-initialized service: %v", err)
	}
}

func TestServiceInitialize(t *testing.T) {
	dbPath := "test_service_init.db"
	defer os.Remove(dbPath)
	defer os.RemoveAll("generated_test")

	cfg := &Config{
		DatabasePath:  dbPath,
		ServerHost:    "127.0.0.1",
		ServerPort:    9999,
		AgentModel:    "claude-sonnet-4-5",
		ParallelBatch: 5,
		CacheDays:     7,
		OutputDir:     "generated_test",
		RoutingMode:   "direct",
	}

	service := NewService(cfg)

	// Initialize service
	err := service.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Verify initialization state
	if !service.initialized {
		t.Error("Expected service to be initialized")
	}

	if service.db == nil {
		t.Error("Expected database to be initialized")
	}

	if service.discovery == nil {
		t.Error("Expected discovery agent to be initialized")
	}

	if service.generator == nil {
		t.Error("Expected generator to be initialized")
	}

	if service.keyManager == nil {
		t.Error("Expected key manager to be initialized")
	}

	if service.router == nil {
		t.Error("Expected router to be initialized")
	}

	if service.adminAPI == nil {
		t.Error("Expected admin API to be initialized")
	}

	// Cleanup
	service.Stop()
}

func TestServiceInitialize_AlreadyInitialized(t *testing.T) {
	dbPath := "test_service_reinit.db"
	defer os.Remove(dbPath)
	defer os.RemoveAll("generated_test2")

	cfg := &Config{
		DatabasePath:  dbPath,
		ServerHost:    "127.0.0.1",
		ServerPort:    9998,
		AgentModel:    "claude-sonnet-4-5",
		ParallelBatch: 5,
		CacheDays:     7,
		OutputDir:     "generated_test2",
		RoutingMode:   "direct",
	}

	service := NewService(cfg)
	service.Initialize()

	// Try to initialize again
	err := service.Initialize()
	if err == nil {
		t.Error("Expected error when initializing already initialized service")
	}

	service.Stop()
}

func TestServiceBootstrap(t *testing.T) {
	dbPath := "test_service_bootstrap.db"
	defer os.Remove(dbPath)
	defer os.RemoveAll("generated_test3")

	// Setup database with test provider
	db, _ := database.Open(dbPath)
	db.CreateProvider(&database.Provider{
		ID:           "test-provider",
		Name:         "Test Provider",
		BaseURL:      "https://api.test.com",
		AuthMethod:   "bearer",
		PricingModel: "usage",
		Status:       "online",
	})
	db.Close()

	cfg := &Config{
		DatabasePath:  dbPath,
		ServerHost:    "127.0.0.1",
		ServerPort:    9997,
		AgentModel:    "claude-sonnet-4-5",
		ParallelBatch: 5,
		CacheDays:     7,
		OutputDir:     "generated_test3",
		RoutingMode:   "direct",
	}

	service := NewService(cfg)
	service.Initialize()

	// Bootstrap should succeed
	err := service.Bootstrap()
	if err != nil {
		t.Fatalf("Bootstrap failed: %v", err)
	}

	service.Stop()
}

func TestServiceBootstrap_NotInitialized(t *testing.T) {
	service := NewService(&Config{})

	err := service.Bootstrap()
	if err == nil {
		t.Error("Expected error when bootstrapping non-initialized service")
	}
}

func TestServiceStart_NotInitialized(t *testing.T) {
	service := NewService(&Config{})

	err := service.Start()
	if err == nil {
		t.Error("Expected error when starting non-initialized service")
	}
}

func TestServiceStart(t *testing.T) {
	dbPath := "test_service_start.db"
	defer os.Remove(dbPath)
	defer os.RemoveAll("generated_test4")

	cfg := &Config{
		DatabasePath:  dbPath,
		ServerHost:    "127.0.0.1",
		ServerPort:    9996,
		AgentModel:    "claude-sonnet-4-5",
		ParallelBatch: 5,
		CacheDays:     7,
		OutputDir:     "generated_test4",
		RoutingMode:   "direct",
	}

	service := NewService(cfg)
	service.Initialize()

	err := service.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Give server a moment to start
	time.Sleep(100 * time.Millisecond)

	if service.httpServer == nil {
		t.Error("Expected HTTP server to be created")
	}

	service.Stop()
}

func TestServiceStopInitialized(t *testing.T) {
	dbPath := "test_service_stop.db"
	defer os.Remove(dbPath)
	defer os.RemoveAll("generated_test5")

	cfg := &Config{
		DatabasePath:  dbPath,
		ServerHost:    "127.0.0.1",
		ServerPort:    9995,
		AgentModel:    "claude-sonnet-4-5",
		ParallelBatch: 5,
		CacheDays:     7,
		OutputDir:     "generated_test5",
		RoutingMode:   "direct",
	}

	service := NewService(cfg)
	service.Initialize()
	service.Start()

	time.Sleep(100 * time.Millisecond)

	err := service.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if service.initialized {
		t.Error("Expected service to be not initialized after stop")
	}
}

func TestKeyManagerDatabaseAdapter(t *testing.T) {
	dbPath := "test_adapter.db"
	defer os.Remove(dbPath)

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Create test provider and API key
	db.CreateProvider(&database.Provider{
		ID:           "test",
		Name:         "Test",
		BaseURL:      "https://test.com",
		AuthMethod:   "bearer",
		PricingModel: "usage",
		Status:       "online",
	})

	key, _ := db.CreateAPIKey("test", "sk-test-123456")

	adapter := &keyManagerDatabaseAdapter{db: db}

	// Test ListActiveAPIKeys
	keys, err := adapter.ListActiveAPIKeys("test")
	if err != nil {
		t.Fatalf("ListActiveAPIKeys failed: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("Expected 1 key, got %d", len(keys))
	}

	// Test GetAPIKey
	retrieved, err := adapter.GetAPIKey(key.ID)
	if err != nil {
		t.Fatalf("GetAPIKey failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected key, got nil")
	}
	if retrieved.ID != key.ID {
		t.Errorf("Expected ID %d, got %d", key.ID, retrieved.ID)
	}

	// Test IncrementKeyUsage
	err = adapter.IncrementKeyUsage(key.ID, 100)
	if err != nil {
		t.Fatalf("IncrementKeyUsage failed: %v", err)
	}

	// Test MarkKeyDegraded
	until := time.Now().Add(1 * time.Hour)
	err = adapter.MarkKeyDegraded(key.ID, until)
	if err != nil {
		t.Fatalf("MarkKeyDegraded failed: %v", err)
	}

	// Test ResetKeyLimits
	err = adapter.ResetKeyLimits(key.ID)
	if err != nil {
		t.Fatalf("ResetKeyLimits failed: %v", err)
	}
}
