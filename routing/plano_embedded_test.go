package routing

import (
	"testing"
)

func TestNewPlanoEmbeddedRouter(t *testing.T) {
	config := &EmbeddedConfig{
		ConfigPath: "/tmp/test-plano.yaml",
	}

	router, err := NewPlanoEmbeddedRouter(config)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if router == nil {
		t.Fatal("expected non-nil router")
	}
}

func TestNewPlanoEmbeddedRouter_NilConfig(t *testing.T) {
	_, err := NewPlanoEmbeddedRouter(nil)

	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestNewPlanoEmbeddedRouter_EmptyConfigPath(t *testing.T) {
	config := &EmbeddedConfig{}

	_, err := NewPlanoEmbeddedRouter(config)

	if err == nil {
		t.Error("expected error for empty config path")
	}
}

func TestGenerateContainerName(t *testing.T) {
	router := &PlanoEmbeddedRouter{}
	name := router.generateContainerName()

	if name == "" {
		t.Error("expected non-empty container name")
	}

	if len(name) < 5 {
		t.Errorf("expected container name length >= 5, got %d", len(name))
	}
}

func TestSetFallback(t *testing.T) {
	config := &EmbeddedConfig{
		ConfigPath: "/tmp/test-plano.yaml",
	}
	router, err := NewPlanoEmbeddedRouter(config)
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}

	directConfig := &DirectConfig{}
	fallback, err := NewDirectRouter(directConfig)
	if err != nil {
		t.Fatalf("failed to create fallback: %v", err)
	}

	router.SetFallback(fallback)

	if router.fallback == nil {
		t.Error("expected fallback to be set")
	}
}

func TestIsRunning(t *testing.T) {
	config := &EmbeddedConfig{
		ConfigPath: "/tmp/test-plano.yaml",
	}
	router, err := NewPlanoEmbeddedRouter(config)
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}

	// Should be false before starting
	if router.IsRunning() {
		t.Error("expected router to not be running initially")
	}
}

func TestGetContainerID(t *testing.T) {
	config := &EmbeddedConfig{
		ConfigPath: "/tmp/test-plano.yaml",
	}
	router, err := NewPlanoEmbeddedRouter(config)
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}

	id := router.GetContainerID()
	// Should be empty before starting
	if id != "" {
		t.Errorf("expected empty container ID before start, got %s", id)
	}
}
