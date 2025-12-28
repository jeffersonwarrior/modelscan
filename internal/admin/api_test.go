package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Mock implementations for testing
type mockDB struct{}

func (m *mockDB) CreateProvider(p *Provider) error {
	return nil
}

func (m *mockDB) GetProvider(id string) (*Provider, error) {
	return &Provider{ID: id, Name: "Test Provider"}, nil
}

func (m *mockDB) ListProviders() ([]*Provider, error) {
	return []*Provider{
		{ID: "openai", Name: "OpenAI", Status: "online"},
		{ID: "anthropic", Name: "Anthropic", Status: "online"},
	}, nil
}

func (m *mockDB) CreateAPIKey(providerID, apiKey string) (*APIKey, error) {
	return &APIKey{ID: 1, ProviderID: providerID, Active: true}, nil
}

func (m *mockDB) ListActiveAPIKeys(providerID string) ([]*APIKey, error) {
	return []*APIKey{
		{ID: 1, ProviderID: providerID, Active: true},
	}, nil
}

func (m *mockDB) GetUsageStats(modelID string, since time.Time) (map[string]interface{}, error) {
	return map[string]interface{}{
		"total_requests": 1000,
		"total_tokens":   50000,
		"total_cost":     10.50,
	}, nil
}

type mockDiscovery struct{}

func (m *mockDiscovery) Discover(providerID string, apiKey string) (*DiscoveryResult, error) {
	return &DiscoveryResult{
		ProviderID: providerID,
		Success:    true,
		Message:    "Discovery successful",
	}, nil
}

type mockGenerator struct{}

func (m *mockGenerator) Generate(req GenerateRequest) (*GenerateResult, error) {
	return &GenerateResult{
		FilePath: "/tmp/test_generated.go",
		Success:  true,
	}, nil
}

func (m *mockGenerator) List() ([]string, error) {
	return []string{"openai_generated.go", "anthropic_generated.go"}, nil
}

func (m *mockGenerator) Delete(providerID string) error {
	return nil
}

type mockKeyManager struct{}

func (m *mockKeyManager) GetKey(providerID string) (*APIKey, error) {
	return &APIKey{ID: 1, ProviderID: providerID}, nil
}

func (m *mockKeyManager) ListKeys(providerID string) ([]*APIKey, error) {
	return []*APIKey{
		{ID: 1, ProviderID: providerID},
		{ID: 2, ProviderID: providerID},
	}, nil
}

func TestNewAPI(t *testing.T) {
	db := &mockDB{}
	discovery := &mockDiscovery{}
	generator := &mockGenerator{}
	keyManager := &mockKeyManager{}

	api := NewAPI(Config{Host: "127.0.0.1", Port: 8080}, db, discovery, generator, keyManager)
	if api == nil {
		t.Fatal("expected non-nil API")
	}

	if api.mux == nil {
		t.Error("mux was not initialized")
	}
}

func TestHandleHealth(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("expected status ok, got %v", response["status"])
	}
}

func TestHandleProviders(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("GET", "/api/providers", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["count"].(float64) != 2 {
		t.Errorf("expected 2 providers, got %v", response["count"])
	}
}

func TestHandleKeys(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("GET", "/api/keys?provider=openai", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["count"].(float64) != 2 {
		t.Errorf("expected 2 keys, got %v", response["count"])
	}
}

func TestHandleAddProvider(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	reqBody := map[string]string{
		"identifier": "openai/gpt-4",
		"api_key":    "sk-test-key",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/providers/add", bytes.NewReader(body))
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response DiscoveryResult
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success=true")
	}
}

func TestHandleSDKs(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("GET", "/api/sdks", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["count"].(float64) != 2 {
		t.Errorf("expected 2 SDKs, got %v", response["count"])
	}
}

func TestHandleStats(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("GET", "/api/stats?model=gpt-4", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["total_requests"].(float64) != 1000 {
		t.Errorf("expected 1000 requests, got %v", response["total_requests"])
	}
}

func TestMethodNotAllowed(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	// Try POST on GET-only endpoint
	req := httptest.NewRequest("POST", "/api/providers", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}
