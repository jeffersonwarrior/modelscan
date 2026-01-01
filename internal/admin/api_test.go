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

func (m *mockDB) GetAPIKey(id int) (*APIKey, error) {
	if id == 1 {
		prefix := "sk-test..."
		return &APIKey{ID: 1, ProviderID: "openai", KeyPrefix: &prefix, Active: true}, nil
	}
	return nil, nil // Key not found
}

func (m *mockDB) DeleteAPIKey(id int) error {
	return nil
}

func (m *mockDB) GetKeyStats(keyID int, since time.Time) (*KeyStats, error) {
	return &KeyStats{
		RequestsToday:    150,
		TokensToday:      45000,
		RateLimitPercent: 80,
		DegradationCount: 2,
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

func (m *mockKeyManager) CountKeys() (int, error) {
	return 2, nil
}

func (m *mockKeyManager) RegisterActualKey(keyHash, actualKey string) {
	// no-op for tests
}

func (m *mockKeyManager) TestKey(keyID int) (*KeyTestResult, error) {
	return &KeyTestResult{
		Valid:              true,
		RateLimitRemaining: 80,
		ModelsAccessible:   []string{"claude-opus-4", "claude-sonnet-4"},
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

func TestHandleAddKey(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	body := map[string]string{
		"provider_id": "openai",
		"api_key":     "sk-test-key",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/keys/add", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response APIKey
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ProviderID != "openai" {
		t.Errorf("expected provider_id openai, got %s", response.ProviderID)
	}
	if !response.Active {
		t.Error("expected active key")
	}
}

func TestHandleDiscover(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	body := map[string]string{
		"identifier": "test-provider",
		"api_key":    "sk-test",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/discover", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response DiscoveryResult
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ProviderID != "test-provider" {
		t.Errorf("expected provider_id test-provider, got %s", response.ProviderID)
	}
	if !response.Success {
		t.Error("expected success=true")
	}
}

func TestHandleGenerateSDK(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	body := map[string]string{
		"provider_id": "openai",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/sdks/generate", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response GenerateResult
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.FilePath != "/tmp/test_generated.go" {
		t.Errorf("expected file_path /tmp/test_generated.go, got %s", response.FilePath)
	}
	if !response.Success {
		t.Error("expected success=true")
	}
}

func TestHandleAddProvider_InvalidJSON(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("POST", "/api/providers/add", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleAddKey_InvalidJSON(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("POST", "/api/keys/add", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleDiscover_InvalidJSON(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("POST", "/api/discover", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleGenerateSDK_InvalidJSON(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("POST", "/api/sdks/generate", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleKeys_MethodNotAllowed(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("POST", "/api/keys", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandleSDKs_MethodNotAllowed(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("POST", "/api/sdks", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandleStats_MissingModel(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("GET", "/api/stats", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
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

func TestHandleKeyTest(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("POST", "/api/keys/1/test", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response KeyTestResult
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Valid {
		t.Error("expected valid=true")
	}
	if response.RateLimitRemaining != 80 {
		t.Errorf("expected rate_limit_remaining=80, got %d", response.RateLimitRemaining)
	}
	if len(response.ModelsAccessible) != 2 {
		t.Errorf("expected 2 models, got %d", len(response.ModelsAccessible))
	}
}

func TestHandleKeyTest_InvalidKeyID(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("POST", "/api/keys/invalid/test", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleKeyTest_MethodNotAllowed(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("GET", "/api/keys/1/test", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandleGetKey(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("GET", "/api/keys/1", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response APIKey
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ID != 1 {
		t.Errorf("expected id=1, got %d", response.ID)
	}
	if response.ProviderID != "openai" {
		t.Errorf("expected provider_id=openai, got %s", response.ProviderID)
	}
}

func TestHandleGetKey_NotFound(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("GET", "/api/keys/999", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleDeleteKey(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("DELETE", "/api/keys/1", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

func TestHandleDeleteKey_NotFound(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("DELETE", "/api/keys/999", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleDeleteKey_InvalidID(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("DELETE", "/api/keys/invalid", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleKeyStats(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("GET", "/api/keys/1/stats", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response KeyStats
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.RequestsToday != 150 {
		t.Errorf("expected requests_today=150, got %d", response.RequestsToday)
	}
	if response.TokensToday != 45000 {
		t.Errorf("expected tokens_today=45000, got %d", response.TokensToday)
	}
	if response.RateLimitPercent != 80 {
		t.Errorf("expected rate_limit_percent=80, got %f", response.RateLimitPercent)
	}
	if response.DegradationCount != 2 {
		t.Errorf("expected degradation_count=2, got %d", response.DegradationCount)
	}
}

func TestHandleKeyStats_NotFound(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("GET", "/api/keys/999/stats", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleKeyStats_InvalidID(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("GET", "/api/keys/invalid/stats", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleKeyStats_MethodNotAllowed(t *testing.T) {
	api := NewAPI(Config{}, &mockDB{}, &mockDiscovery{}, &mockGenerator{}, &mockKeyManager{})

	req := httptest.NewRequest("POST", "/api/keys/1/stats", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}
