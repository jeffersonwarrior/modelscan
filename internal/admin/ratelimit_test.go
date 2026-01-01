package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockRateLimitStore is a mock implementation of RateLimitStore for testing
type mockRateLimitStore struct {
	rateLimits map[string]*ClientRateLimit
	nextID     int
}

func newMockRateLimitStore() *mockRateLimitStore {
	return &mockRateLimitStore{
		rateLimits: make(map[string]*ClientRateLimit),
		nextID:     1,
	}
}

func (m *mockRateLimitStore) Get(clientID string) (*ClientRateLimit, error) {
	if rl, ok := m.rateLimits[clientID]; ok {
		return rl, nil
	}
	return nil, nil
}

func (m *mockRateLimitStore) GetOrCreate(clientID string) (*ClientRateLimit, error) {
	if rl, ok := m.rateLimits[clientID]; ok {
		return rl, nil
	}
	rl := &ClientRateLimit{
		ID:        m.nextID,
		ClientID:  clientID,
		LastReset: time.Now(),
	}
	m.nextID++
	m.rateLimits[clientID] = rl
	return rl, nil
}

func (m *mockRateLimitStore) Create(rl *ClientRateLimit) error {
	rl.ID = m.nextID
	m.nextID++
	m.rateLimits[rl.ClientID] = rl
	return nil
}

func (m *mockRateLimitStore) Update(rl *ClientRateLimit) error {
	m.rateLimits[rl.ClientID] = rl
	return nil
}

func (m *mockRateLimitStore) UpdateLimits(clientID string, rpmLimit, tpmLimit, dailyLimit *int) error {
	rl, ok := m.rateLimits[clientID]
	if !ok {
		return nil
	}
	if rpmLimit != nil {
		rl.RPMLimit = rpmLimit
	}
	if tpmLimit != nil {
		rl.TPMLimit = tpmLimit
	}
	if dailyLimit != nil {
		rl.DailyLimit = dailyLimit
	}
	return nil
}

func (m *mockRateLimitStore) Delete(clientID string) error {
	delete(m.rateLimits, clientID)
	return nil
}

func (m *mockRateLimitStore) List() ([]*ClientRateLimit, error) {
	var result []*ClientRateLimit
	for _, rl := range m.rateLimits {
		result = append(result, rl)
	}
	return result, nil
}

func (m *mockRateLimitStore) IncrementUsage(clientID string, requests, tokens int) error {
	if rl, ok := m.rateLimits[clientID]; ok {
		rl.CurrentRPM += requests
		rl.CurrentTPM += tokens
		rl.CurrentDaily += requests
	}
	return nil
}

func (m *mockRateLimitStore) CheckLimits(clientID string) (bool, string, error) {
	rl, ok := m.rateLimits[clientID]
	if !ok {
		return true, "", nil
	}
	if rl.RPMLimit != nil && rl.CurrentRPM >= *rl.RPMLimit {
		return false, "rpm", nil
	}
	if rl.TPMLimit != nil && rl.CurrentTPM >= *rl.TPMLimit {
		return false, "tpm", nil
	}
	if rl.DailyLimit != nil && rl.CurrentDaily >= *rl.DailyLimit {
		return false, "daily", nil
	}
	return true, "", nil
}

func (m *mockRateLimitStore) ResetMinuteCounters() error {
	for _, rl := range m.rateLimits {
		rl.CurrentRPM = 0
		rl.CurrentTPM = 0
		rl.LastReset = time.Now()
	}
	return nil
}

func (m *mockRateLimitStore) ResetDailyCounters() error {
	for _, rl := range m.rateLimits {
		rl.CurrentDaily = 0
		rl.LastReset = time.Now()
	}
	return nil
}

func (m *mockRateLimitStore) Exists(clientID string) (bool, error) {
	_, ok := m.rateLimits[clientID]
	return ok, nil
}

func TestRateLimitAPI_HandleRateLimits_List(t *testing.T) {
	store := newMockRateLimitStore()
	api := NewRateLimitAPI(store)

	// Add some rate limits
	rpm := 60
	store.rateLimits["client-1"] = &ClientRateLimit{
		ID:       1,
		ClientID: "client-1",
		RPMLimit: &rpm,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ratelimits", nil)
	w := httptest.NewRecorder()

	api.HandleRateLimits(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	count, ok := response["count"].(float64)
	if !ok || int(count) != 1 {
		t.Errorf("expected count 1, got %v", response["count"])
	}
}

func TestRateLimitAPI_HandleRateLimits_Create(t *testing.T) {
	store := newMockRateLimitStore()
	api := NewRateLimitAPI(store)

	rpm := 100
	tpm := 10000
	body := RateLimitCreateRequest{
		ClientID:   "new-client",
		RPMLimit:   &rpm,
		TPMLimit:   &tpm,
		DailyLimit: nil,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/ratelimits", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	api.HandleRateLimits(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var rl ClientRateLimit
	if err := json.NewDecoder(w.Body).Decode(&rl); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if rl.ClientID != "new-client" {
		t.Errorf("expected client_id 'new-client', got '%s'", rl.ClientID)
	}
	if rl.RPMLimit == nil || *rl.RPMLimit != 100 {
		t.Errorf("expected rpm_limit 100, got %v", rl.RPMLimit)
	}
}

func TestRateLimitAPI_HandleRateLimitByClientID_Get(t *testing.T) {
	store := newMockRateLimitStore()
	api := NewRateLimitAPI(store)

	rpm := 60
	store.rateLimits["test-client"] = &ClientRateLimit{
		ID:       1,
		ClientID: "test-client",
		RPMLimit: &rpm,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ratelimits/test-client", nil)
	w := httptest.NewRecorder()

	api.HandleRateLimitByClientID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var rl ClientRateLimit
	if err := json.NewDecoder(w.Body).Decode(&rl); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if rl.ClientID != "test-client" {
		t.Errorf("expected client_id 'test-client', got '%s'", rl.ClientID)
	}
}

func TestRateLimitAPI_HandleRateLimitByClientID_Update(t *testing.T) {
	store := newMockRateLimitStore()
	api := NewRateLimitAPI(store)

	rpm := 60
	store.rateLimits["test-client"] = &ClientRateLimit{
		ID:         1,
		ClientID:   "test-client",
		RPMLimit:   &rpm,
		CurrentRPM: 10,
	}

	newRpm := 120
	body := RateLimitUpdateRequest{
		RPMLimit: &newRpm,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/ratelimits/test-client", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	api.HandleRateLimitByClientID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var rl ClientRateLimit
	if err := json.NewDecoder(w.Body).Decode(&rl); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if rl.RPMLimit == nil || *rl.RPMLimit != 120 {
		t.Errorf("expected rpm_limit 120, got %v", rl.RPMLimit)
	}
}

func TestRateLimitAPI_HandleRateLimitByClientID_Delete(t *testing.T) {
	store := newMockRateLimitStore()
	api := NewRateLimitAPI(store)

	rpm := 60
	store.rateLimits["test-client"] = &ClientRateLimit{
		ID:       1,
		ClientID: "test-client",
		RPMLimit: &rpm,
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/ratelimits/test-client", nil)
	w := httptest.NewRecorder()

	api.HandleRateLimitByClientID(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	if _, exists := store.rateLimits["test-client"]; exists {
		t.Error("expected rate limit to be deleted")
	}
}

func TestRateLimitAPI_HandleRateLimitByClientID_Check(t *testing.T) {
	store := newMockRateLimitStore()
	api := NewRateLimitAPI(store)

	rpm := 5
	store.rateLimits["test-client"] = &ClientRateLimit{
		ID:         1,
		ClientID:   "test-client",
		RPMLimit:   &rpm,
		CurrentRPM: 3,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ratelimits/test-client/check", nil)
	w := httptest.NewRecorder()

	api.HandleRateLimitByClientID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp RateLimitCheckResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.WithinLimits {
		t.Error("expected within_limits to be true")
	}
}

func TestRateLimitAPI_HandleRateLimitByClientID_Check_Exceeded(t *testing.T) {
	store := newMockRateLimitStore()
	api := NewRateLimitAPI(store)

	rpm := 5
	store.rateLimits["test-client"] = &ClientRateLimit{
		ID:         1,
		ClientID:   "test-client",
		RPMLimit:   &rpm,
		CurrentRPM: 5,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ratelimits/test-client/check", nil)
	w := httptest.NewRecorder()

	api.HandleRateLimitByClientID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp RateLimitCheckResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.WithinLimits {
		t.Error("expected within_limits to be false")
	}
	if resp.LimitType != "rpm" {
		t.Errorf("expected limit_type 'rpm', got '%s'", resp.LimitType)
	}
}

func TestRateLimitAPI_HandleRateLimitByClientID_Reset(t *testing.T) {
	store := newMockRateLimitStore()
	api := NewRateLimitAPI(store)

	rpm := 60
	store.rateLimits["test-client"] = &ClientRateLimit{
		ID:           1,
		ClientID:     "test-client",
		RPMLimit:     &rpm,
		CurrentRPM:   50,
		CurrentTPM:   1000,
		CurrentDaily: 100,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/ratelimits/test-client/reset", nil)
	w := httptest.NewRecorder()

	api.HandleRateLimitByClientID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var rl ClientRateLimit
	if err := json.NewDecoder(w.Body).Decode(&rl); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if rl.CurrentRPM != 0 {
		t.Errorf("expected current_rpm 0, got %d", rl.CurrentRPM)
	}
	if rl.CurrentTPM != 0 {
		t.Errorf("expected current_tpm 0, got %d", rl.CurrentTPM)
	}
	if rl.CurrentDaily != 0 {
		t.Errorf("expected current_daily 0, got %d", rl.CurrentDaily)
	}
}

func TestRateLimitMiddleware_WithinLimits(t *testing.T) {
	store := newMockRateLimitStore()
	middleware := NewRateLimitMiddleware(store)

	rpm := 100
	store.rateLimits["test-client"] = &ClientRateLimit{
		ID:         1,
		ClientID:   "test-client",
		RPMLimit:   &rpm,
		CurrentRPM: 10,
	}

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// Add client to context
	ctx := req.Context()
	ctx = context.WithValue(ctx, ClientContextKey{}, &MiddlewareClient{ID: "test-client"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	middleware.Wrap(handler).ServeHTTP(w, req)

	if !called {
		t.Error("expected handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRateLimitMiddleware_Exceeded(t *testing.T) {
	store := newMockRateLimitStore()
	middleware := NewRateLimitMiddleware(store)

	rpm := 10
	store.rateLimits["test-client"] = &ClientRateLimit{
		ID:         1,
		ClientID:   "test-client",
		RPMLimit:   &rpm,
		CurrentRPM: 10,
	}

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// Add client to context
	ctx := req.Context()
	ctx = context.WithValue(ctx, ClientContextKey{}, &MiddlewareClient{ID: "test-client"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	middleware.Wrap(handler).ServeHTTP(w, req)

	if called {
		t.Error("expected handler NOT to be called")
	}
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, w.Code)
	}
}
