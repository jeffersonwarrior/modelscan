package integration

import (
	"context"
	"testing"
)

func TestNewIntegration(t *testing.T) {
	cfg := &Config{
		DatabasePath:  "/tmp/test.db",
		ServerHost:    "127.0.0.1",
		ServerPort:    8080,
		AgentModel:    "claude-sonnet-4-5",
		ParallelBatch: 5,
		CacheDays:     7,
		OutputDir:     "/tmp/generated",
	}

	integration, err := NewIntegration(cfg)
	if err != nil {
		t.Fatalf("NewIntegration failed: %v", err)
	}

	if integration == nil {
		t.Fatal("expected non-nil integration")
	}

	if integration.config.ServerPort != 8080 {
		t.Errorf("expected port 8080, got %d", integration.config.ServerPort)
	}
}

func TestIntegrationHealth(t *testing.T) {
	integration, err := NewIntegration(&Config{
		DatabasePath: "/tmp/test.db",
		ServerHost:   "127.0.0.1",
		ServerPort:   8080,
	})
	if err != nil {
		t.Fatalf("NewIntegration failed: %v", err)
	}

	health := integration.Health()

	if health["status"] != "ok" {
		t.Errorf("expected status ok, got %v", health["status"])
	}

	components, ok := health["components"].(map[string]string)
	if !ok {
		t.Fatal("expected components map")
	}

	expectedComponents := []string{"database", "discovery", "generator", "key_manager", "admin_api"}
	for _, comp := range expectedComponents {
		if status, exists := components[comp]; !exists {
			t.Errorf("missing component: %s", comp)
		} else if status != "ok" {
			t.Errorf("component %s not ok: %s", comp, status)
		}
	}
}

func TestIntegrationListProviders(t *testing.T) {
	integration, err := NewIntegration(&Config{
		DatabasePath: "/tmp/test.db",
	})
	if err != nil {
		t.Fatalf("NewIntegration failed: %v", err)
	}

	ctx := context.Background()
	providers, err := integration.ListProviders(ctx)
	if err != nil {
		t.Fatalf("ListProviders failed: %v", err)
	}

	if len(providers) < 1 {
		t.Error("expected at least 1 mock provider")
	}

	// Check provider structure
	p := providers[0]
	if _, ok := p["id"]; !ok {
		t.Error("provider missing 'id' field")
	}
	if _, ok := p["name"]; !ok {
		t.Error("provider missing 'name' field")
	}
	if _, ok := p["status"]; !ok {
		t.Error("provider missing 'status' field")
	}
}

func TestIntegrationGetProvider(t *testing.T) {
	integration, err := NewIntegration(&Config{
		DatabasePath: "/tmp/test.db",
	})
	if err != nil {
		t.Fatalf("NewIntegration failed: %v", err)
	}

	ctx := context.Background()
	provider, err := integration.GetProvider(ctx, "openai")
	if err != nil {
		t.Fatalf("GetProvider failed: %v", err)
	}

	if provider["id"] != "openai" {
		t.Errorf("expected id 'openai', got %v", provider["id"])
	}
}

func TestIntegrationGetUsageStats(t *testing.T) {
	integration, err := NewIntegration(&Config{
		DatabasePath: "/tmp/test.db",
	})
	if err != nil {
		t.Fatalf("NewIntegration failed: %v", err)
	}

	ctx := context.Background()
	stats, err := integration.GetUsageStats(ctx, "gpt-4", 7)
	if err != nil {
		t.Fatalf("GetUsageStats failed: %v", err)
	}

	if _, ok := stats["total_requests"]; !ok {
		t.Error("stats missing 'total_requests' field")
	}
	if _, ok := stats["total_tokens"]; !ok {
		t.Error("stats missing 'total_tokens' field")
	}
	if _, ok := stats["total_cost"]; !ok {
		t.Error("stats missing 'total_cost' field")
	}
}

func TestIntegrationRouteRequest(t *testing.T) {
	integration, err := NewIntegration(&Config{
		DatabasePath: "/tmp/test.db",
	})
	if err != nil {
		t.Fatalf("NewIntegration failed: %v", err)
	}

	ctx := context.Background()
	messages := []map[string]string{
		{"role": "user", "content": "Hello"},
	}

	response, err := integration.RouteRequest(ctx, "openai", "gpt-4", messages)
	if err != nil {
		t.Fatalf("RouteRequest failed: %v", err)
	}

	if response == "" {
		t.Error("expected non-empty response")
	}
}

func TestIntegrationClose(t *testing.T) {
	integration, err := NewIntegration(&Config{
		DatabasePath: "/tmp/test.db",
	})
	if err != nil {
		t.Fatalf("NewIntegration failed: %v", err)
	}

	err = integration.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}
