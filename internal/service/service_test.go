package service

import (
	"context"
	"net/http"
	"testing"
)

// Mock implementations for testing
type mockDB struct{}

func (m *mockDB) Close() error { return nil }
func (m *mockDB) ListProviders() ([]*Provider, error) {
	return []*Provider{
		{ID: "openai", Name: "OpenAI"},
	}, nil
}
func (m *mockDB) ListActiveAPIKeys(providerID string) ([]*APIKey, error) {
	return []*APIKey{
		{ID: 1, ProviderID: providerID},
	}, nil
}

type mockDiscovery struct{}

func (m *mockDiscovery) Close() error { return nil }

type mockGenerator struct{}

func (m *mockGenerator) GenerateBatch(requests []GenerateRequest) []*GenerateResult {
	results := make([]*GenerateResult, len(requests))
	for i := range results {
		results[i] = &GenerateResult{Success: true}
	}
	return results
}

type mockKeyManager struct{}

func (m *mockKeyManager) Close() error { return nil }

type mockAdminAPI struct{}

func (m *mockAdminAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

type mockRouter struct{}

func (m *mockRouter) Route(ctx context.Context, req Request) (*Response, error) {
	return &Response{Content: "test response"}, nil
}
func (m *mockRouter) Close() error { return nil }

func TestNewService(t *testing.T) {
	cfg := &Config{
		DatabasePath:  "/tmp/test.db",
		ServerHost:    "127.0.0.1",
		ServerPort:    8080,
		AgentModel:    "claude-sonnet-4-5",
		ParallelBatch: 5,
		CacheDays:     7,
	}

	service := NewService(cfg)
	if service == nil {
		t.Fatal("expected non-nil service")
	}

	if service.config.ServerPort != 8080 {
		t.Errorf("expected port 8080, got %d", service.config.ServerPort)
	}
}

func TestServiceInitialize(t *testing.T) {
	service := NewService(&Config{
		DatabasePath: "/tmp/test.db",
		ServerHost:   "127.0.0.1",
		ServerPort:   8080,
	})

	// Mock components
	service.db = &mockDB{}
	service.discovery = &mockDiscovery{}
	service.generator = &mockGenerator{}
	service.keyManager = &mockKeyManager{}
	service.adminAPI = &mockAdminAPI{}
	service.router = &mockRouter{}

	err := service.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if !service.initialized {
		t.Error("expected initialized=true")
	}

	// Second call should be idempotent
	err = service.Initialize()
	if err != nil {
		t.Error("second Initialize should succeed")
	}
}

func TestServiceBootstrap(t *testing.T) {
	service := NewService(&Config{})
	service.db = &mockDB{}

	err := service.Bootstrap()
	if err != nil {
		t.Fatalf("Bootstrap failed: %v", err)
	}
}

func TestServiceHealth(t *testing.T) {
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

func TestServiceIsRestarting(t *testing.T) {
	service := NewService(&Config{})

	if service.IsRestarting() {
		t.Error("expected not restarting initially")
	}

	service.restarting = true
	if !service.IsRestarting() {
		t.Error("expected restarting after setting flag")
	}
}

func TestServiceStop(t *testing.T) {
	service := NewService(&Config{})
	service.db = &mockDB{}
	service.discovery = &mockDiscovery{}
	service.keyManager = &mockKeyManager{}
	service.router = &mockRouter{}

	err := service.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}
