package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// mockClientStore implements ClientStore for testing
type mockClientStore struct {
	mu                sync.Mutex
	clients           map[string]*MiddlewareClient
	lastSeenUpdates   []string
	getByTokenErr     error
	updateLastSeenErr error
}

func newMockClientStore() *mockClientStore {
	return &mockClientStore{
		clients:         make(map[string]*MiddlewareClient),
		lastSeenUpdates: []string{},
	}
}

func (m *mockClientStore) GetByToken(token string) (*MiddlewareClient, error) {
	if m.getByTokenErr != nil {
		return nil, m.getByTokenErr
	}
	client, ok := m.clients[token]
	if !ok {
		return nil, nil
	}
	return client, nil
}

func (m *mockClientStore) UpdateLastSeen(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateLastSeenErr != nil {
		return m.updateLastSeenErr
	}
	m.lastSeenUpdates = append(m.lastSeenUpdates, id)
	return nil
}

func (m *mockClientStore) getLastSeenCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.lastSeenUpdates)
}

func (m *mockClientStore) addClient(token string, client *MiddlewareClient) {
	m.clients[token] = client
}

func TestClientMiddleware_ValidToken(t *testing.T) {
	store := newMockClientStore()
	client := &MiddlewareClient{
		ID:      "client-123",
		Name:    "test-client",
		Version: "1.0.0",
		Token:   "valid-token",
	}
	store.addClient("valid-token", client)

	mw := NewClientMiddleware(store, false)

	var capturedClient *MiddlewareClient
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedClient = GetClientFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Client-Token", "valid-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	if capturedClient == nil {
		t.Error("expected client in context, got nil")
	} else if capturedClient.ID != "client-123" {
		t.Errorf("expected client ID 'client-123', got %q", capturedClient.ID)
	}

	// Wait briefly for async update
	time.Sleep(10 * time.Millisecond)
	if store.getLastSeenCount() == 0 {
		t.Error("expected UpdateLastSeen to be called")
	}
}

func TestClientMiddleware_InvalidToken(t *testing.T) {
	store := newMockClientStore()
	mw := NewClientMiddleware(store, false)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Client-Token", "invalid-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestClientMiddleware_MissingToken_Required(t *testing.T) {
	store := newMockClientStore()
	mw := NewClientMiddleware(store, false)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestClientMiddleware_MissingToken_Optional(t *testing.T) {
	store := newMockClientStore()
	mw := NewClientMiddleware(store, true) // optional = true

	var called bool
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		client := GetClientFromContext(r.Context())
		if client != nil {
			t.Error("expected no client in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if !called {
		t.Error("expected handler to be called")
	}
}

func TestClientMiddleware_WrapFunc(t *testing.T) {
	store := newMockClientStore()
	client := &MiddlewareClient{
		ID:    "client-456",
		Name:  "func-test",
		Token: "func-token",
	}
	store.addClient("func-token", client)

	mw := NewClientMiddleware(store, false)

	var called bool
	handler := mw.WrapFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Client-Token", "func-token")
	rr := httptest.NewRecorder()

	handler(rr, req)

	if !called {
		t.Error("expected handler to be called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestRequireClient(t *testing.T) {
	store := newMockClientStore()
	client := &MiddlewareClient{
		ID:    "required-client",
		Token: "required-token",
	}
	store.addClient("required-token", client)

	wrap := RequireClient(store)
	handler := wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Without token - should fail
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("without token: expected status 401, got %d", rr.Code)
	}

	// With valid token - should succeed
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Client-Token", "required-token")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("with token: expected status 200, got %d", rr.Code)
	}
}

func TestOptionalClient(t *testing.T) {
	store := newMockClientStore()

	wrap := OptionalClient(store)
	handler := wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Without token - should succeed
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("without token: expected status 200, got %d", rr.Code)
	}
}

func TestGetClientFromContext_NoClient(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	client := GetClientFromContext(req.Context())
	if client != nil {
		t.Error("expected nil client for empty context")
	}
}

func TestClientMiddleware_StoreError(t *testing.T) {
	store := newMockClientStore()
	store.getByTokenErr = http.ErrHandlerTimeout // any error

	mw := NewClientMiddleware(store, false)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Client-Token", "some-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestClientMiddleware_ClientConfig(t *testing.T) {
	store := newMockClientStore()
	client := &MiddlewareClient{
		ID:           "config-client",
		Name:         "Config Test",
		Version:      "2.0.0",
		Token:        "config-token",
		Capabilities: []string{"streaming", "tools"},
		Config: MiddlewareClientConfig{
			DefaultModel:     "gpt-4o",
			ThinkingModel:    "o1-preview",
			MaxOutputTokens:  4096,
			TimeoutMs:        30000,
			ProviderPriority: []string{"openai", "anthropic"},
		},
	}
	store.addClient("config-token", client)

	mw := NewClientMiddleware(store, false)

	var capturedClient *MiddlewareClient
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedClient = GetClientFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Client-Token", "config-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if capturedClient == nil {
		t.Fatal("expected client in context")
	}

	if capturedClient.Config.DefaultModel != "gpt-4o" {
		t.Errorf("expected DefaultModel 'gpt-4o', got %q", capturedClient.Config.DefaultModel)
	}

	if capturedClient.Config.MaxOutputTokens != 4096 {
		t.Errorf("expected MaxOutputTokens 4096, got %d", capturedClient.Config.MaxOutputTokens)
	}

	if len(capturedClient.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(capturedClient.Capabilities))
	}
}

// mockRemapRuleStore implements RemapRuleStore for testing
type mockRemapRuleStore struct {
	rules          map[string]*RemapRule // key is "clientID:model"
	findMatchingErr error
}

func newMockRemapRuleStore() *mockRemapRuleStore {
	return &mockRemapRuleStore{
		rules: make(map[string]*RemapRule),
	}
}

func (m *mockRemapRuleStore) addRule(clientID, fromModel, toModel, toProvider string, priority int) {
	key := clientID + ":" + fromModel
	m.rules[key] = &RemapRule{
		ID:         len(m.rules) + 1,
		ClientID:   clientID,
		FromModel:  fromModel,
		ToModel:    toModel,
		ToProvider: toProvider,
		Priority:   priority,
		Enabled:    true,
	}
}

func (m *mockRemapRuleStore) FindMatching(model string, clientID string) (*RemapRule, error) {
	if m.findMatchingErr != nil {
		return nil, m.findMatchingErr
	}
	// Exact match first
	key := clientID + ":" + model
	if rule, ok := m.rules[key]; ok {
		return rule, nil
	}
	// Check wildcard
	wildcardKey := clientID + ":*"
	if rule, ok := m.rules[wildcardKey]; ok {
		return rule, nil
	}
	return nil, nil
}

func TestRemapMiddleware_RemapModel_ExactMatch(t *testing.T) {
	store := newMockRemapRuleStore()
	store.addRule("client-1", "claude-2", "claude-3-opus", "anthropic", 10)

	mw := NewRemapMiddleware(store)

	ctx := context.Background()
	remapped, provider, err := mw.RemapModel(ctx, "claude-2", "client-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if remapped != "claude-3-opus" {
		t.Errorf("expected remapped model 'claude-3-opus', got %q", remapped)
	}
	if provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", provider)
	}
}

func TestRemapMiddleware_RemapModel_NoMatch(t *testing.T) {
	store := newMockRemapRuleStore()
	store.addRule("client-1", "claude-2", "claude-3-opus", "anthropic", 10)

	mw := NewRemapMiddleware(store)

	ctx := context.Background()
	remapped, provider, err := mw.RemapModel(ctx, "gpt-4", "client-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return original model when no match
	if remapped != "gpt-4" {
		t.Errorf("expected original model 'gpt-4', got %q", remapped)
	}
	if provider != "" {
		t.Errorf("expected empty provider, got %q", provider)
	}
}

func TestRemapMiddleware_RemapModel_EmptyClientID(t *testing.T) {
	store := newMockRemapRuleStore()
	store.addRule("client-1", "claude-2", "claude-3-opus", "anthropic", 10)

	mw := NewRemapMiddleware(store)

	ctx := context.Background()
	remapped, provider, err := mw.RemapModel(ctx, "claude-2", "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return original model when no client ID
	if remapped != "claude-2" {
		t.Errorf("expected original model 'claude-2', got %q", remapped)
	}
	if provider != "" {
		t.Errorf("expected empty provider, got %q", provider)
	}
}

func TestRemapMiddleware_RemapModel_WildcardMatch(t *testing.T) {
	store := newMockRemapRuleStore()
	store.addRule("client-2", "*", "gpt-4o", "openai", 5)

	mw := NewRemapMiddleware(store)

	ctx := context.Background()
	remapped, provider, err := mw.RemapModel(ctx, "any-model", "client-2")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if remapped != "gpt-4o" {
		t.Errorf("expected remapped model 'gpt-4o', got %q", remapped)
	}
	if provider != "openai" {
		t.Errorf("expected provider 'openai', got %q", provider)
	}
}

func TestRemapMiddleware_RemapModel_StoreError(t *testing.T) {
	store := newMockRemapRuleStore()
	store.findMatchingErr = http.ErrHandlerTimeout

	mw := NewRemapMiddleware(store)

	ctx := context.Background()
	_, _, err := mw.RemapModel(ctx, "any-model", "client-1")

	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestRemapMiddleware_RemapModel_DifferentProvider(t *testing.T) {
	store := newMockRemapRuleStore()
	store.addRule("client-3", "claude-3", "llama-70b", "together", 20)

	mw := NewRemapMiddleware(store)

	ctx := context.Background()
	remapped, provider, err := mw.RemapModel(ctx, "claude-3", "client-3")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if remapped != "llama-70b" {
		t.Errorf("expected remapped model 'llama-70b', got %q", remapped)
	}
	if provider != "together" {
		t.Errorf("expected provider 'together', got %q", provider)
	}
}

func TestRemapMiddleware_Wrap(t *testing.T) {
	remapStore := newMockRemapRuleStore()
	remapStore.addRule("client-1", "model-a", "model-b", "provider-x", 10)

	clientStore := newMockClientStore()
	client := &MiddlewareClient{
		ID:    "client-1",
		Name:  "Test Client",
		Token: "test-token",
	}
	clientStore.addClient("test-token", client)

	clientMw := NewClientMiddleware(clientStore, false)
	remapMw := NewRemapMiddleware(remapStore)

	var called bool
	handler := clientMw.Wrap(remapMw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("X-Client-Token", "test-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("expected handler to be called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestRemapMiddleware_Wrap_NoClient(t *testing.T) {
	store := newMockRemapRuleStore()
	mw := NewRemapMiddleware(store)

	var called bool
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("expected handler to be called even without client")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestRemapMiddleware_WrapFunc(t *testing.T) {
	store := newMockRemapRuleStore()
	mw := NewRemapMiddleware(store)

	var called bool
	handler := mw.WrapFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	rr := httptest.NewRecorder()

	handler(rr, req)

	if !called {
		t.Error("expected handler to be called")
	}
}

func TestRemapHandler(t *testing.T) {
	store := newMockRemapRuleStore()
	store.addRule("client-1", "model-x", "model-y", "provider-z", 15)

	mw := RemapHandler(store)

	if mw == nil {
		t.Fatal("expected non-nil middleware")
	}

	ctx := context.Background()
	remapped, provider, err := mw.RemapModel(ctx, "model-x", "client-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if remapped != "model-y" {
		t.Errorf("expected remapped model 'model-y', got %q", remapped)
	}
	if provider != "provider-z" {
		t.Errorf("expected provider 'provider-z', got %q", provider)
	}
}

func TestGetRemapResultFromContext(t *testing.T) {
	// Test with no remap result
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	result := GetRemapResultFromContext(req.Context())
	if result != nil {
		t.Error("expected nil result for empty context")
	}

	// Test with remap result
	ctx := context.WithValue(req.Context(), RemapContextKey{}, &RemapResult{
		OriginalModel:  "claude-2",
		RemappedModel:  "claude-3-opus",
		TargetProvider: "anthropic",
		RuleID:         42,
	})
	req = req.WithContext(ctx)
	result = GetRemapResultFromContext(req.Context())
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.OriginalModel != "claude-2" {
		t.Errorf("expected OriginalModel 'claude-2', got %q", result.OriginalModel)
	}
	if result.RemappedModel != "claude-3-opus" {
		t.Errorf("expected RemappedModel 'claude-3-opus', got %q", result.RemappedModel)
	}
	if result.TargetProvider != "anthropic" {
		t.Errorf("expected TargetProvider 'anthropic', got %q", result.TargetProvider)
	}
	if result.RuleID != 42 {
		t.Errorf("expected RuleID 42, got %d", result.RuleID)
	}
}
