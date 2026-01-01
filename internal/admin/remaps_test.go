package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockRemapStore implements RemapStore for testing
type mockRemapStore struct {
	rules  map[int]*RemapRule
	nextID int
	err    error // if set, methods return this error
}

func newMockRemapStore() *mockRemapStore {
	return &mockRemapStore{
		rules:  make(map[int]*RemapRule),
		nextID: 1,
	}
}

func (m *mockRemapStore) Create(rule *RemapRule) error {
	if m.err != nil {
		return m.err
	}
	rule.ID = m.nextID
	m.nextID++
	m.rules[rule.ID] = rule
	return nil
}

func (m *mockRemapStore) Get(id int) (*RemapRule, error) {
	if m.err != nil {
		return nil, m.err
	}
	rule, ok := m.rules[id]
	if !ok {
		return nil, nil
	}
	return rule, nil
}

func (m *mockRemapStore) List(clientID *string) ([]*RemapRule, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []*RemapRule
	for _, rule := range m.rules {
		if clientID == nil || rule.ClientID == *clientID {
			result = append(result, rule)
		}
	}
	return result, nil
}

func (m *mockRemapStore) Update(rule *RemapRule) error {
	if m.err != nil {
		return m.err
	}
	if _, ok := m.rules[rule.ID]; !ok {
		return nil
	}
	m.rules[rule.ID] = rule
	return nil
}

func (m *mockRemapStore) Delete(id int) error {
	if m.err != nil {
		return m.err
	}
	delete(m.rules, id)
	return nil
}

func (m *mockRemapStore) SetEnabled(id int, enabled bool) error {
	if m.err != nil {
		return m.err
	}
	if rule, ok := m.rules[id]; ok {
		rule.Enabled = enabled
	}
	return nil
}

func TestRemapAPI_HandleListRemaps_Empty(t *testing.T) {
	store := newMockRemapStore()
	api := NewRemapAPI(store)

	req := httptest.NewRequest("GET", "/api/rules/remap", nil)
	w := httptest.NewRecorder()

	api.HandleRemaps(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["count"].(float64) != 0 {
		t.Errorf("expected 0 rules, got %v", response["count"])
	}
}

func TestRemapAPI_HandleCreateRemap(t *testing.T) {
	store := newMockRemapStore()
	api := NewRemapAPI(store)

	body := map[string]interface{}{
		"client_id":   "mclaude-123",
		"from_model":  "claude-*",
		"to_model":    "gpt-4o",
		"to_provider": "openai",
		"priority":    10,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/rules/remap", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.HandleRemaps(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var response RemapRule
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ClientID != "mclaude-123" {
		t.Errorf("expected client_id 'mclaude-123', got %s", response.ClientID)
	}
	if response.FromModel != "claude-*" {
		t.Errorf("expected from_model 'claude-*', got %s", response.FromModel)
	}
	if response.ToModel != "gpt-4o" {
		t.Errorf("expected to_model 'gpt-4o', got %s", response.ToModel)
	}
	if response.ToProvider != "openai" {
		t.Errorf("expected to_provider 'openai', got %s", response.ToProvider)
	}
	if response.Priority != 10 {
		t.Errorf("expected priority 10, got %d", response.Priority)
	}
	if !response.Enabled {
		t.Error("expected Enabled=true by default")
	}
	if response.ID != 1 {
		t.Errorf("expected ID 1, got %d", response.ID)
	}
}

func TestRemapAPI_HandleCreateRemap_Disabled(t *testing.T) {
	store := newMockRemapStore()
	api := NewRemapAPI(store)

	enabled := false
	body := map[string]interface{}{
		"client_id":   "mclaude-123",
		"from_model":  "claude-*",
		"to_model":    "gpt-4o",
		"to_provider": "openai",
		"enabled":     enabled,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/rules/remap", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.HandleRemaps(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var response RemapRule
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Enabled {
		t.Error("expected Enabled=false")
	}
}

func TestRemapAPI_HandleCreateRemap_MissingClientID(t *testing.T) {
	store := newMockRemapStore()
	api := NewRemapAPI(store)

	body := map[string]interface{}{
		"from_model":  "claude-*",
		"to_model":    "gpt-4o",
		"to_provider": "openai",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/rules/remap", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.HandleRemaps(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestRemapAPI_HandleCreateRemap_MissingFromModel(t *testing.T) {
	store := newMockRemapStore()
	api := NewRemapAPI(store)

	body := map[string]interface{}{
		"client_id":   "mclaude-123",
		"to_model":    "gpt-4o",
		"to_provider": "openai",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/rules/remap", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.HandleRemaps(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestRemapAPI_HandleCreateRemap_MissingToModel(t *testing.T) {
	store := newMockRemapStore()
	api := NewRemapAPI(store)

	body := map[string]interface{}{
		"client_id":   "mclaude-123",
		"from_model":  "claude-*",
		"to_provider": "openai",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/rules/remap", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.HandleRemaps(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestRemapAPI_HandleCreateRemap_MissingToProvider(t *testing.T) {
	store := newMockRemapStore()
	api := NewRemapAPI(store)

	body := map[string]interface{}{
		"client_id":  "mclaude-123",
		"from_model": "claude-*",
		"to_model":   "gpt-4o",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/rules/remap", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.HandleRemaps(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestRemapAPI_HandleListRemaps(t *testing.T) {
	store := newMockRemapStore()
	api := NewRemapAPI(store)

	// Add some rules
	store.Create(&RemapRule{ClientID: "client1", FromModel: "a", ToModel: "b", ToProvider: "p1", CreatedAt: time.Now()})
	store.Create(&RemapRule{ClientID: "client2", FromModel: "c", ToModel: "d", ToProvider: "p2", CreatedAt: time.Now()})

	req := httptest.NewRequest("GET", "/api/rules/remap", nil)
	w := httptest.NewRecorder()

	api.HandleRemaps(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["count"].(float64) != 2 {
		t.Errorf("expected 2 rules, got %v", response["count"])
	}
}

func TestRemapAPI_HandleListRemaps_FilterByClientID(t *testing.T) {
	store := newMockRemapStore()
	api := NewRemapAPI(store)

	// Add some rules
	store.Create(&RemapRule{ClientID: "client1", FromModel: "a", ToModel: "b", ToProvider: "p1", CreatedAt: time.Now()})
	store.Create(&RemapRule{ClientID: "client2", FromModel: "c", ToModel: "d", ToProvider: "p2", CreatedAt: time.Now()})

	req := httptest.NewRequest("GET", "/api/rules/remap?client_id=client1", nil)
	w := httptest.NewRecorder()

	api.HandleRemaps(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["count"].(float64) != 1 {
		t.Errorf("expected 1 rule, got %v", response["count"])
	}
}

func TestRemapAPI_HandleGetRemap(t *testing.T) {
	store := newMockRemapStore()
	api := NewRemapAPI(store)

	store.Create(&RemapRule{ClientID: "client1", FromModel: "a", ToModel: "b", ToProvider: "p1", Priority: 5, Enabled: true, CreatedAt: time.Now()})

	req := httptest.NewRequest("GET", "/api/rules/remap/1", nil)
	w := httptest.NewRecorder()

	api.HandleRemapByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response RemapRule
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ID != 1 {
		t.Errorf("expected ID 1, got %d", response.ID)
	}
	if response.ClientID != "client1" {
		t.Errorf("expected client_id 'client1', got %s", response.ClientID)
	}
}

func TestRemapAPI_HandleGetRemap_NotFound(t *testing.T) {
	store := newMockRemapStore()
	api := NewRemapAPI(store)

	req := httptest.NewRequest("GET", "/api/rules/remap/999", nil)
	w := httptest.NewRecorder()

	api.HandleRemapByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestRemapAPI_HandleUpdateRemap(t *testing.T) {
	store := newMockRemapStore()
	api := NewRemapAPI(store)

	store.Create(&RemapRule{ClientID: "client1", FromModel: "a", ToModel: "b", ToProvider: "p1", Priority: 5, Enabled: true, CreatedAt: time.Now()})

	newPriority := 20
	body := map[string]interface{}{
		"priority": newPriority,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PATCH", "/api/rules/remap/1", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.HandleRemapByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response RemapRule
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Priority != 20 {
		t.Errorf("expected priority 20, got %d", response.Priority)
	}
	// Other fields should be unchanged
	if response.ClientID != "client1" {
		t.Errorf("expected client_id 'client1', got %s", response.ClientID)
	}
}

func TestRemapAPI_HandleUpdateRemap_Multiple(t *testing.T) {
	store := newMockRemapStore()
	api := NewRemapAPI(store)

	store.Create(&RemapRule{ClientID: "client1", FromModel: "a", ToModel: "b", ToProvider: "p1", Priority: 5, Enabled: true, CreatedAt: time.Now()})

	enabled := false
	body := map[string]interface{}{
		"to_model":    "new-model",
		"to_provider": "new-provider",
		"enabled":     enabled,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PATCH", "/api/rules/remap/1", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.HandleRemapByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response RemapRule
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ToModel != "new-model" {
		t.Errorf("expected to_model 'new-model', got %s", response.ToModel)
	}
	if response.ToProvider != "new-provider" {
		t.Errorf("expected to_provider 'new-provider', got %s", response.ToProvider)
	}
	if response.Enabled {
		t.Error("expected Enabled=false")
	}
}

func TestRemapAPI_HandleUpdateRemap_NotFound(t *testing.T) {
	store := newMockRemapStore()
	api := NewRemapAPI(store)

	body := map[string]interface{}{
		"priority": 20,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PATCH", "/api/rules/remap/999", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.HandleRemapByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestRemapAPI_HandleDeleteRemap(t *testing.T) {
	store := newMockRemapStore()
	api := NewRemapAPI(store)

	store.Create(&RemapRule{ClientID: "client1", FromModel: "a", ToModel: "b", ToProvider: "p1", CreatedAt: time.Now()})

	req := httptest.NewRequest("DELETE", "/api/rules/remap/1", nil)
	w := httptest.NewRecorder()

	api.HandleRemapByID(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	// Verify deletion
	rule, _ := store.Get(1)
	if rule != nil {
		t.Error("expected rule to be deleted")
	}
}

func TestRemapAPI_HandleDeleteRemap_NotFound(t *testing.T) {
	store := newMockRemapStore()
	api := NewRemapAPI(store)

	req := httptest.NewRequest("DELETE", "/api/rules/remap/999", nil)
	w := httptest.NewRecorder()

	api.HandleRemapByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestRemapAPI_HandleRemaps_MethodNotAllowed(t *testing.T) {
	store := newMockRemapStore()
	api := NewRemapAPI(store)

	req := httptest.NewRequest("DELETE", "/api/rules/remap", nil)
	w := httptest.NewRecorder()

	api.HandleRemaps(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestRemapAPI_HandleRemapByID_MissingID(t *testing.T) {
	store := newMockRemapStore()
	api := NewRemapAPI(store)

	req := httptest.NewRequest("GET", "/api/rules/remap/", nil)
	w := httptest.NewRecorder()

	api.HandleRemapByID(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestRemapAPI_HandleRemapByID_InvalidID(t *testing.T) {
	store := newMockRemapStore()
	api := NewRemapAPI(store)

	req := httptest.NewRequest("GET", "/api/rules/remap/abc", nil)
	w := httptest.NewRecorder()

	api.HandleRemapByID(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestRemapAPI_HandleRemapByID_MethodNotAllowed(t *testing.T) {
	store := newMockRemapStore()
	api := NewRemapAPI(store)

	req := httptest.NewRequest("POST", "/api/rules/remap/1", nil)
	w := httptest.NewRecorder()

	api.HandleRemapByID(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}
