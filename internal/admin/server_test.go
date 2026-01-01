package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// Mock client counter for testing
type mockClientCounter struct {
	count int
}

func (m *mockClientCounter) Count() (int, error) {
	return m.count, nil
}

// Mock provider counter for testing
type mockProviderCounter struct {
	providers []*Provider
}

func (m *mockProviderCounter) ListProviders() ([]*Provider, error) {
	return m.providers, nil
}

// Mock key counter for testing
type mockKeyCounter struct {
	keys map[string][]*APIKey
}

func (m *mockKeyCounter) ListKeys(providerID string) ([]*APIKey, error) {
	if keys, ok := m.keys[providerID]; ok {
		return keys, nil
	}
	return nil, nil
}

func (m *mockKeyCounter) CountKeys() (int, error) {
	count := 0
	for _, keys := range m.keys {
		count += len(keys)
	}
	return count, nil
}

func TestNewServerAPI(t *testing.T) {
	api := NewServerAPI("1.0.0", 8080)

	if api == nil {
		t.Fatal("expected non-nil ServerAPI")
	}
	if api.version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", api.version)
	}
	if api.port != 8080 {
		t.Errorf("expected port 8080, got %d", api.port)
	}
}

func TestHandleServerInfo(t *testing.T) {
	api := NewServerAPI("0.5.5", 8080)

	// Set up mock counters
	api.SetClientCounter(&mockClientCounter{count: 3})
	api.SetProviderCounter(&mockProviderCounter{
		providers: []*Provider{
			{ID: "openai", Name: "OpenAI"},
			{ID: "anthropic", Name: "Anthropic"},
		},
	})
	api.SetKeyCounter(&mockKeyCounter{
		keys: map[string][]*APIKey{
			"openai":    {{ID: 1}, {ID: 2}},
			"anthropic": {{ID: 3}},
		},
	})

	// Increment request counter
	api.IncrementRequests()
	api.IncrementRequests()

	req := httptest.NewRequest("GET", "/api/server/info", nil)
	w := httptest.NewRecorder()

	api.HandleServerInfo(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ServerInfoResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Version != "0.5.5" {
		t.Errorf("expected version 0.5.5, got %s", response.Version)
	}
	if response.Port != 8080 {
		t.Errorf("expected port 8080, got %d", response.Port)
	}
	if response.PID != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), response.PID)
	}
	if response.ClientsConnected != 3 {
		t.Errorf("expected 3 clients, got %d", response.ClientsConnected)
	}
	if response.RequestsServed != 2 {
		t.Errorf("expected 2 requests, got %d", response.RequestsServed)
	}
	if response.ProvidersAvailable != 2 {
		t.Errorf("expected 2 providers, got %d", response.ProvidersAvailable)
	}
	if response.KeysConfigured != 3 {
		t.Errorf("expected 3 keys, got %d", response.KeysConfigured)
	}
	if response.UptimeSeconds < 0 {
		t.Error("expected non-negative uptime")
	}
}

func TestHandleServerInfo_MethodNotAllowed(t *testing.T) {
	api := NewServerAPI("0.5.5", 8080)

	req := httptest.NewRequest("POST", "/api/server/info", nil)
	w := httptest.NewRecorder()

	api.HandleServerInfo(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandleServerInfo_NoCounters(t *testing.T) {
	api := NewServerAPI("0.5.5", 8080)

	req := httptest.NewRequest("GET", "/api/server/info", nil)
	w := httptest.NewRecorder()

	api.HandleServerInfo(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ServerInfoResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should handle nil counters gracefully
	if response.ClientsConnected != 0 {
		t.Errorf("expected 0 clients, got %d", response.ClientsConnected)
	}
	if response.ProvidersAvailable != 0 {
		t.Errorf("expected 0 providers, got %d", response.ProvidersAvailable)
	}
	if response.KeysConfigured != 0 {
		t.Errorf("expected 0 keys, got %d", response.KeysConfigured)
	}
}

func TestIncrementRequests(t *testing.T) {
	api := NewServerAPI("0.5.5", 8080)

	// Verify initial state
	req := httptest.NewRequest("GET", "/api/server/info", nil)
	w := httptest.NewRecorder()
	api.HandleServerInfo(w, req)

	var response1 ServerInfoResponse
	json.NewDecoder(w.Body).Decode(&response1)

	if response1.RequestsServed != 0 {
		t.Errorf("expected 0 requests initially, got %d", response1.RequestsServed)
	}

	// Increment requests
	api.IncrementRequests()
	api.IncrementRequests()
	api.IncrementRequests()

	// Check updated count
	req2 := httptest.NewRequest("GET", "/api/server/info", nil)
	w2 := httptest.NewRecorder()
	api.HandleServerInfo(w2, req2)

	var response2 ServerInfoResponse
	json.NewDecoder(w2.Body).Decode(&response2)

	if response2.RequestsServed != 3 {
		t.Errorf("expected 3 requests, got %d", response2.RequestsServed)
	}
}

func TestHandleShutdown(t *testing.T) {
	api := NewServerAPI("0.5.5", 8080)

	// Set up shutdown callback
	api.SetShutdownFunc(func() error {
		return nil
	})

	req := httptest.NewRequest("POST", "/api/server/shutdown", nil)
	w := httptest.NewRecorder()

	api.HandleShutdown(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ShutdownResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "shutting_down" {
		t.Errorf("expected status 'shutting_down', got %s", response.Status)
	}
	if response.Message != "Server shutdown initiated" {
		t.Errorf("expected message 'Server shutdown initiated', got %s", response.Message)
	}
}

func TestHandleShutdown_MethodNotAllowed(t *testing.T) {
	api := NewServerAPI("0.5.5", 8080)
	api.SetShutdownFunc(func() error { return nil })

	req := httptest.NewRequest("GET", "/api/server/shutdown", nil)
	w := httptest.NewRecorder()

	api.HandleShutdown(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandleShutdown_NotConfigured(t *testing.T) {
	api := NewServerAPI("0.5.5", 8080)
	// Don't set shutdown func

	req := httptest.NewRequest("POST", "/api/server/shutdown", nil)
	w := httptest.NewRecorder()

	api.HandleShutdown(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}
}

func TestSetShutdownFunc(t *testing.T) {
	api := NewServerAPI("0.5.5", 8080)

	called := false
	fn := func() error {
		called = true
		return nil
	}

	api.SetShutdownFunc(fn)

	// Verify function is set and callable
	if api.shutdownFunc == nil {
		t.Fatal("expected shutdownFunc to be set")
	}

	if err := api.shutdownFunc(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !called {
		t.Error("expected shutdown function to be called")
	}
}
