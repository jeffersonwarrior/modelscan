package keymanager

import (
	"context"
	"testing"
	"time"
)

// MockDatabase implements Database interface for testing
type MockDatabase struct {
	keys        map[string][]*APIKey
	usageCount  map[int]int
	degraded    map[int]time.Time
	resetCalled map[int]bool
}

func NewMockDatabase() *MockDatabase {
	return &MockDatabase{
		keys:        make(map[string][]*APIKey),
		usageCount:  make(map[int]int),
		degraded:    make(map[int]time.Time),
		resetCalled: make(map[int]bool),
	}
}

func (m *MockDatabase) ListActiveAPIKeys(providerID string) ([]*APIKey, error) {
	keys, ok := m.keys[providerID]
	if !ok {
		return []*APIKey{}, nil
	}
	return keys, nil
}

func (m *MockDatabase) IncrementKeyUsage(keyID int, tokens int) error {
	m.usageCount[keyID] += tokens
	return nil
}

func (m *MockDatabase) MarkKeyDegraded(keyID int, until time.Time) error {
	m.degraded[keyID] = until
	return nil
}

func (m *MockDatabase) ResetKeyLimits(keyID int) error {
	m.resetCalled[keyID] = true
	return nil
}

func (m *MockDatabase) GetAPIKey(id int) (*APIKey, error) {
	for _, keys := range m.keys {
		for _, key := range keys {
			if key.ID == id {
				return key, nil
			}
		}
	}
	return nil, nil
}

func TestNewKeyManager(t *testing.T) {
	db := NewMockDatabase()
	cfg := Config{
		CacheTTL:        1 * time.Minute,
		DegradeDuration: 10 * time.Minute,
	}

	km := NewKeyManager(db, cfg)
	if km == nil {
		t.Fatal("expected non-nil key manager")
	}

	if km.cacheTTL != 1*time.Minute {
		t.Errorf("expected cache TTL 1m, got %v", km.cacheTTL)
	}
}

func TestGetKeyRoundRobin(t *testing.T) {
	db := NewMockDatabase()

	// Add test keys with varying usage
	rpm := 100
	db.keys["testprovider"] = []*APIKey{
		{ID: 1, ProviderID: "testprovider", RequestsCount: 10, TokensCount: 1000, RPMLimit: &rpm},
		{ID: 2, ProviderID: "testprovider", RequestsCount: 5, TokensCount: 500, RPMLimit: &rpm},
		{ID: 3, ProviderID: "testprovider", RequestsCount: 15, TokensCount: 1500, RPMLimit: &rpm},
	}

	km := NewKeyManager(db, Config{})
	ctx := context.Background()

	// Should select key with lowest usage (ID 2)
	key, err := km.GetKey(ctx, "testprovider")
	if err != nil {
		t.Fatalf("GetKey failed: %v", err)
	}

	if key.ID != 2 {
		t.Errorf("expected key ID 2 (lowest usage), got %d", key.ID)
	}
}

func TestGetKeyRateLimits(t *testing.T) {
	db := NewMockDatabase()

	rpm := 10
	db.keys["testprovider"] = []*APIKey{
		{ID: 1, ProviderID: "testprovider", RequestsCount: 15, RPMLimit: &rpm}, // Over limit
		{ID: 2, ProviderID: "testprovider", RequestsCount: 5, RPMLimit: &rpm},  // Under limit
	}

	km := NewKeyManager(db, Config{})
	ctx := context.Background()

	key, err := km.GetKey(ctx, "testprovider")
	if err != nil {
		t.Fatalf("GetKey failed: %v", err)
	}

	// Should skip key 1 (rate limited) and select key 2
	if key.ID != 2 {
		t.Errorf("expected key ID 2 (not rate limited), got %d", key.ID)
	}
}

func TestGetKeyDegraded(t *testing.T) {
	db := NewMockDatabase()

	db.keys["testprovider"] = []*APIKey{
		{ID: 1, ProviderID: "testprovider", Degraded: true}, // Degraded
		{ID: 2, ProviderID: "testprovider", Degraded: false},
	}

	km := NewKeyManager(db, Config{})
	ctx := context.Background()

	key, err := km.GetKey(ctx, "testprovider")
	if err != nil {
		t.Fatalf("GetKey failed: %v", err)
	}

	// Should skip degraded key and select key 2
	if key.ID != 2 {
		t.Errorf("expected key ID 2 (not degraded), got %d", key.ID)
	}
}

func TestGetKeyNoActiveKeys(t *testing.T) {
	db := NewMockDatabase()
	db.keys["testprovider"] = []*APIKey{}

	km := NewKeyManager(db, Config{})
	ctx := context.Background()

	_, err := km.GetKey(ctx, "testprovider")
	if err == nil {
		t.Error("expected error for no active keys")
	}
}

func TestGetKeyAllDegraded(t *testing.T) {
	db := NewMockDatabase()

	db.keys["testprovider"] = []*APIKey{
		{ID: 1, ProviderID: "testprovider", Degraded: true},
		{ID: 2, ProviderID: "testprovider", Degraded: true},
	}

	km := NewKeyManager(db, Config{})
	ctx := context.Background()

	_, err := km.GetKey(ctx, "testprovider")
	if err == nil {
		t.Error("expected error when all keys are degraded")
	}
}

func TestRecordUsage(t *testing.T) {
	db := NewMockDatabase()
	km := NewKeyManager(db, Config{})
	ctx := context.Background()

	err := km.RecordUsage(ctx, 1, 500)
	if err != nil {
		t.Fatalf("RecordUsage failed: %v", err)
	}

	if db.usageCount[1] != 500 {
		t.Errorf("expected usage count 500, got %d", db.usageCount[1])
	}
}

func TestMarkDegraded(t *testing.T) {
	db := NewMockDatabase()

	db.keys["testprovider"] = []*APIKey{
		{ID: 1, ProviderID: "testprovider"},
	}

	km := NewKeyManager(db, Config{})
	ctx := context.Background()

	duration := 10 * time.Minute
	err := km.MarkDegraded(ctx, 1, duration)
	if err != nil {
		t.Fatalf("MarkDegraded failed: %v", err)
	}

	until, ok := db.degraded[1]
	if !ok {
		t.Fatal("key was not marked as degraded")
	}

	// Check that until time is approximately now + duration
	expected := time.Now().Add(duration)
	diff := until.Sub(expected)
	if diff < -1*time.Second || diff > 1*time.Second {
		t.Errorf("degraded until time incorrect: expected ~%v, got %v", expected, until)
	}
}

func TestResetLimits(t *testing.T) {
	db := NewMockDatabase()
	km := NewKeyManager(db, Config{})
	ctx := context.Background()

	err := km.ResetLimits(ctx, 1)
	if err != nil {
		t.Fatalf("ResetLimits failed: %v", err)
	}

	if !db.resetCalled[1] {
		t.Error("ResetLimits was not called on database")
	}
}

func TestListKeys(t *testing.T) {
	db := NewMockDatabase()

	db.keys["testprovider"] = []*APIKey{
		{ID: 1, ProviderID: "testprovider"},
		{ID: 2, ProviderID: "testprovider"},
	}

	km := NewKeyManager(db, Config{})
	ctx := context.Background()

	keys, err := km.ListKeys(ctx, "testprovider")
	if err != nil {
		t.Fatalf("ListKeys failed: %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}
