package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockAliasStore implements AliasStore for testing
type mockAliasStore struct {
	aliases map[string]*Alias // keyed by name+clientID
}

func newMockAliasStore() *mockAliasStore {
	return &mockAliasStore{
		aliases: make(map[string]*Alias),
	}
}

func (m *mockAliasStore) key(name string, clientID *string) string {
	if clientID == nil {
		return name + ":global"
	}
	return name + ":" + *clientID
}

func (m *mockAliasStore) CreateAlias(alias *Alias) error {
	m.aliases[m.key(alias.Name, alias.ClientID)] = alias
	return nil
}

func (m *mockAliasStore) GetAlias(name string, clientID *string) (*Alias, error) {
	alias, ok := m.aliases[m.key(name, clientID)]
	if !ok {
		return nil, nil
	}
	return alias, nil
}

func (m *mockAliasStore) ListAllAliases() ([]*Alias, error) {
	var result []*Alias
	for _, alias := range m.aliases {
		result = append(result, alias)
	}
	return result, nil
}

func (m *mockAliasStore) ListAliases(clientID *string) ([]*Alias, error) {
	var result []*Alias
	for _, alias := range m.aliases {
		// Include if global or matches clientID
		if alias.ClientID == nil || (clientID != nil && alias.ClientID != nil && *alias.ClientID == *clientID) {
			result = append(result, alias)
		}
	}
	return result, nil
}

func (m *mockAliasStore) DeleteAlias(name string, clientID *string) error {
	delete(m.aliases, m.key(name, clientID))
	return nil
}

func (m *mockAliasStore) UpdateAlias(name string, clientID *string, newModelID string) error {
	alias := m.aliases[m.key(name, clientID)]
	if alias != nil {
		alias.ModelID = newModelID
	}
	return nil
}

func TestAliasAPI_HandleListAliases_Empty(t *testing.T) {
	store := newMockAliasStore()
	api := NewAliasAPI(store)

	req := httptest.NewRequest("GET", "/api/aliases", nil)
	w := httptest.NewRecorder()

	api.HandleAliases(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["count"].(float64) != 0 {
		t.Errorf("expected 0 aliases, got %v", response["count"])
	}
}

func TestAliasAPI_HandleCreateAlias(t *testing.T) {
	store := newMockAliasStore()
	api := NewAliasAPI(store)

	body := map[string]string{
		"name":     "sonnet",
		"model_id": "claude-sonnet-4-20250929",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/aliases", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.HandleAliases(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var response AliasResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Name != "sonnet" {
		t.Errorf("expected name 'sonnet', got %s", response.Name)
	}
	if response.ModelID != "claude-sonnet-4-20250929" {
		t.Errorf("expected model_id 'claude-sonnet-4-20250929', got %s", response.ModelID)
	}
	if !response.IsGlobal {
		t.Error("expected IsGlobal=true")
	}
}

func TestAliasAPI_HandleCreateAlias_WithClientID(t *testing.T) {
	store := newMockAliasStore()
	api := NewAliasAPI(store)

	clientID := "client-123"
	body := map[string]interface{}{
		"name":      "opus",
		"model_id":  "claude-opus-4-20250929",
		"client_id": clientID,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/aliases", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.HandleAliases(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var response AliasResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.IsGlobal {
		t.Error("expected IsGlobal=false for client-specific alias")
	}
	if response.ClientID == nil || *response.ClientID != clientID {
		t.Errorf("expected client_id '%s', got %v", clientID, response.ClientID)
	}
}

func TestAliasAPI_HandleCreateAlias_MissingName(t *testing.T) {
	store := newMockAliasStore()
	api := NewAliasAPI(store)

	body := map[string]string{
		"model_id": "claude-sonnet-4-20250929",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/aliases", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.HandleAliases(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestAliasAPI_HandleCreateAlias_MissingModelID(t *testing.T) {
	store := newMockAliasStore()
	api := NewAliasAPI(store)

	body := map[string]string{
		"name": "sonnet",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/aliases", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.HandleAliases(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestAliasAPI_HandleCreateAlias_Duplicate(t *testing.T) {
	store := newMockAliasStore()
	api := NewAliasAPI(store)

	// Create first alias
	store.CreateAlias(&Alias{
		Name:      "sonnet",
		ModelID:   "claude-sonnet-4-20250929",
		CreatedAt: time.Now(),
	})

	// Try to create duplicate
	body := map[string]string{
		"name":     "sonnet",
		"model_id": "claude-sonnet-4-20250930",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/aliases", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.HandleAliases(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}
}

func TestAliasAPI_HandleListAliases(t *testing.T) {
	store := newMockAliasStore()
	api := NewAliasAPI(store)

	// Add some aliases
	store.CreateAlias(&Alias{Name: "sonnet", ModelID: "claude-sonnet-4", CreatedAt: time.Now()})
	store.CreateAlias(&Alias{Name: "opus", ModelID: "claude-opus-4", CreatedAt: time.Now()})

	req := httptest.NewRequest("GET", "/api/aliases", nil)
	w := httptest.NewRecorder()

	api.HandleAliases(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["count"].(float64) != 2 {
		t.Errorf("expected 2 aliases, got %v", response["count"])
	}
}

func TestAliasAPI_HandleGetAlias(t *testing.T) {
	store := newMockAliasStore()
	api := NewAliasAPI(store)

	store.CreateAlias(&Alias{Name: "sonnet", ModelID: "claude-sonnet-4", CreatedAt: time.Now()})

	req := httptest.NewRequest("GET", "/api/aliases/sonnet", nil)
	w := httptest.NewRecorder()

	api.HandleAliasByName(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response AliasResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Name != "sonnet" {
		t.Errorf("expected name 'sonnet', got %s", response.Name)
	}
}

func TestAliasAPI_HandleGetAlias_NotFound(t *testing.T) {
	store := newMockAliasStore()
	api := NewAliasAPI(store)

	req := httptest.NewRequest("GET", "/api/aliases/nonexistent", nil)
	w := httptest.NewRecorder()

	api.HandleAliasByName(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestAliasAPI_HandleDeleteAlias(t *testing.T) {
	store := newMockAliasStore()
	api := NewAliasAPI(store)

	store.CreateAlias(&Alias{Name: "sonnet", ModelID: "claude-sonnet-4", CreatedAt: time.Now()})

	req := httptest.NewRequest("DELETE", "/api/aliases/sonnet", nil)
	w := httptest.NewRecorder()

	api.HandleAliasByName(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	// Verify deletion
	alias, _ := store.GetAlias("sonnet", nil)
	if alias != nil {
		t.Error("expected alias to be deleted")
	}
}

func TestAliasAPI_HandleDeleteAlias_NotFound(t *testing.T) {
	store := newMockAliasStore()
	api := NewAliasAPI(store)

	req := httptest.NewRequest("DELETE", "/api/aliases/nonexistent", nil)
	w := httptest.NewRecorder()

	api.HandleAliasByName(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestAliasAPI_HandleUpdateAlias(t *testing.T) {
	store := newMockAliasStore()
	api := NewAliasAPI(store)

	store.CreateAlias(&Alias{Name: "sonnet", ModelID: "claude-sonnet-4", CreatedAt: time.Now()})

	body := map[string]string{
		"model_id": "claude-sonnet-4-new",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/api/aliases/sonnet", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.HandleAliasByName(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response AliasResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ModelID != "claude-sonnet-4-new" {
		t.Errorf("expected model_id 'claude-sonnet-4-new', got %s", response.ModelID)
	}
}

func TestAliasAPI_HandleUpdateAlias_NotFound(t *testing.T) {
	store := newMockAliasStore()
	api := NewAliasAPI(store)

	body := map[string]string{
		"model_id": "claude-sonnet-4-new",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/api/aliases/nonexistent", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.HandleAliasByName(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestAliasAPI_HandleAliases_MethodNotAllowed(t *testing.T) {
	store := newMockAliasStore()
	api := NewAliasAPI(store)

	req := httptest.NewRequest("DELETE", "/api/aliases", nil)
	w := httptest.NewRecorder()

	api.HandleAliases(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestAliasAPI_HandleAliasByName_MissingName(t *testing.T) {
	store := newMockAliasStore()
	api := NewAliasAPI(store)

	req := httptest.NewRequest("GET", "/api/aliases/", nil)
	w := httptest.NewRecorder()

	api.HandleAliasByName(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestAliasAPI_HandleAliasByName_MethodNotAllowed(t *testing.T) {
	store := newMockAliasStore()
	api := NewAliasAPI(store)

	req := httptest.NewRequest("POST", "/api/aliases/sonnet", nil)
	w := httptest.NewRecorder()

	api.HandleAliasByName(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}
