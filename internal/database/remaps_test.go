package database

import (
	"os"
	"testing"
)

func TestRemapRuleRepository(t *testing.T) {
	// Create temp database
	tmpFile, err := os.CreateTemp("", "modelscan-test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create a client for foreign key
	clientRepo := NewClientRepository(db)
	client := &Client{
		ID:           "test-client-1",
		Name:         "Test Client",
		Version:      "1.0.0",
		Token:        "test-token-123",
		Capabilities: []string{"chat"},
		Config:       ClientConfig{},
	}
	if err := clientRepo.Create(client); err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	repo := NewRemapRuleRepository(db)

	t.Run("Create and Get", func(t *testing.T) {
		rule := &RemapRule{
			ClientID:   "test-client-1",
			FromModel:  "claude-*",
			ToModel:    "gpt-4",
			ToProvider: "openai",
			Priority:   10,
			Enabled:    true,
		}

		err := repo.Create(rule)
		if err != nil {
			t.Fatalf("failed to create remap rule: %v", err)
		}

		if rule.ID == 0 {
			t.Error("expected rule ID to be set after create")
		}

		got, err := repo.Get(rule.ID)
		if err != nil {
			t.Fatalf("failed to get remap rule: %v", err)
		}

		if got.ClientID != rule.ClientID {
			t.Errorf("got ClientID %q, want %q", got.ClientID, rule.ClientID)
		}
		if got.FromModel != rule.FromModel {
			t.Errorf("got FromModel %q, want %q", got.FromModel, rule.FromModel)
		}
		if got.ToModel != rule.ToModel {
			t.Errorf("got ToModel %q, want %q", got.ToModel, rule.ToModel)
		}
		if got.ToProvider != rule.ToProvider {
			t.Errorf("got ToProvider %q, want %q", got.ToProvider, rule.ToProvider)
		}
		if got.Priority != rule.Priority {
			t.Errorf("got Priority %d, want %d", got.Priority, rule.Priority)
		}
		if got.Enabled != rule.Enabled {
			t.Errorf("got Enabled %v, want %v", got.Enabled, rule.Enabled)
		}
	})

	t.Run("Update", func(t *testing.T) {
		rule := &RemapRule{
			ClientID:   "test-client-1",
			FromModel:  "gpt-*",
			ToModel:    "claude-3-opus",
			ToProvider: "anthropic",
			Priority:   5,
			Enabled:    true,
		}
		err := repo.Create(rule)
		if err != nil {
			t.Fatalf("failed to create remap rule: %v", err)
		}

		rule.ToModel = "claude-3-sonnet"
		rule.Priority = 20
		err = repo.Update(rule)
		if err != nil {
			t.Fatalf("failed to update remap rule: %v", err)
		}

		got, err := repo.Get(rule.ID)
		if err != nil {
			t.Fatalf("failed to get remap rule: %v", err)
		}

		if got.ToModel != "claude-3-sonnet" {
			t.Errorf("got ToModel %q, want %q", got.ToModel, "claude-3-sonnet")
		}
		if got.Priority != 20 {
			t.Errorf("got Priority %d, want %d", got.Priority, 20)
		}
	})

	t.Run("SetEnabled", func(t *testing.T) {
		rule := &RemapRule{
			ClientID:   "test-client-1",
			FromModel:  "test-model",
			ToModel:    "target-model",
			ToProvider: "provider",
			Priority:   1,
			Enabled:    true,
		}
		err := repo.Create(rule)
		if err != nil {
			t.Fatalf("failed to create remap rule: %v", err)
		}

		err = repo.SetEnabled(rule.ID, false)
		if err != nil {
			t.Fatalf("failed to set enabled: %v", err)
		}

		got, err := repo.Get(rule.ID)
		if err != nil {
			t.Fatalf("failed to get remap rule: %v", err)
		}

		if got.Enabled {
			t.Error("expected rule to be disabled")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		rule := &RemapRule{
			ClientID:   "test-client-1",
			FromModel:  "to-delete",
			ToModel:    "target",
			ToProvider: "provider",
			Priority:   1,
			Enabled:    true,
		}
		err := repo.Create(rule)
		if err != nil {
			t.Fatalf("failed to create remap rule: %v", err)
		}

		err = repo.Delete(rule.ID)
		if err != nil {
			t.Fatalf("failed to delete remap rule: %v", err)
		}

		got, err := repo.Get(rule.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Error("expected rule to be deleted")
		}
	})

	t.Run("Exists", func(t *testing.T) {
		rule := &RemapRule{
			ClientID:   "test-client-1",
			FromModel:  "exists-test",
			ToModel:    "target",
			ToProvider: "provider",
			Priority:   1,
			Enabled:    true,
		}
		err := repo.Create(rule)
		if err != nil {
			t.Fatalf("failed to create remap rule: %v", err)
		}

		exists, err := repo.Exists(rule.ID)
		if err != nil {
			t.Fatalf("failed to check exists: %v", err)
		}
		if !exists {
			t.Error("expected rule to exist")
		}

		exists, err = repo.Exists(99999)
		if err != nil {
			t.Fatalf("failed to check exists: %v", err)
		}
		if exists {
			t.Error("expected rule not to exist")
		}
	})

	t.Run("List", func(t *testing.T) {
		// Create another client
		client2 := &Client{
			ID:           "test-client-2",
			Name:         "Test Client 2",
			Version:      "1.0.0",
			Token:        "test-token-456",
			Capabilities: []string{"chat"},
			Config:       ClientConfig{},
		}
		if err := clientRepo.Create(client2); err != nil {
			t.Fatalf("failed to create client: %v", err)
		}

		// Create rules for client 2
		rules := []*RemapRule{
			{ClientID: "test-client-2", FromModel: "m1", ToModel: "t1", ToProvider: "p1", Priority: 10, Enabled: true},
			{ClientID: "test-client-2", FromModel: "m2", ToModel: "t2", ToProvider: "p2", Priority: 5, Enabled: true},
		}
		for _, r := range rules {
			if err := repo.Create(r); err != nil {
				t.Fatalf("failed to create rule: %v", err)
			}
		}

		// List all for client 2
		got, err := repo.ListByClientID("test-client-2")
		if err != nil {
			t.Fatalf("failed to list rules: %v", err)
		}

		if len(got) != 2 {
			t.Errorf("got %d rules, want 2", len(got))
		}

		// Should be ordered by priority DESC
		if got[0].Priority < got[1].Priority {
			t.Error("expected rules to be ordered by priority descending")
		}
	})
}

func TestGlobMatching(t *testing.T) {
	tests := []struct {
		pattern string
		input   string
		want    bool
	}{
		// Exact matches
		{"claude-3-opus", "claude-3-opus", true},
		{"claude-3-opus", "claude-3-sonnet", false},

		// Star wildcard
		{"claude-*", "claude-3-opus", true},
		{"claude-*", "claude-3-sonnet", true},
		{"claude-*", "gpt-4", false},
		{"*-opus", "claude-3-opus", true},
		{"*-opus", "gpt-4-opus", true},
		{"*-opus", "claude-3-sonnet", false},
		{"*", "anything", true},
		{"claude-*-*", "claude-3-opus", true},
		{"claude-*-*", "claude-opus", false},

		// Question mark wildcard
		{"claude-?-opus", "claude-3-opus", true},
		{"claude-?-opus", "claude-35-opus", false},
		{"gpt-?", "gpt-4", true},
		{"gpt-?", "gpt-4o", false},

		// Combined wildcards
		{"*-?", "gpt-4", true},
		{"*-?", "claude-3", true},
		{"claude-?-*", "claude-3-opus", true},
		{"claude-?-*", "claude-3-opus-latest", true},

		// Edge cases
		{"", "", true},
		{"*", "", true},
		{"?", "a", true},
		{"?", "", false},
		{"**", "anything", true},
		{"a*b*c", "abc", true},
		{"a*b*c", "aXXXbYYYc", true},
		{"a*b*c", "aXXXbYYYd", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.input, func(t *testing.T) {
			got := matchGlob(tt.pattern, tt.input)
			if got != tt.want {
				t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.pattern, tt.input, got, tt.want)
			}
		})
	}
}

func TestFindMatching(t *testing.T) {
	// Create temp database
	tmpFile, err := os.CreateTemp("", "modelscan-test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create a client
	clientRepo := NewClientRepository(db)
	client := &Client{
		ID:           "match-client",
		Name:         "Match Test Client",
		Version:      "1.0.0",
		Token:        "match-token-123",
		Capabilities: []string{},
		Config:       ClientConfig{},
	}
	if err := clientRepo.Create(client); err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	repo := NewRemapRuleRepository(db)

	// Create rules with different priorities
	rules := []*RemapRule{
		{ClientID: "match-client", FromModel: "claude-*", ToModel: "gpt-4", ToProvider: "openai", Priority: 10, Enabled: true},
		{ClientID: "match-client", FromModel: "claude-3-opus", ToModel: "gpt-4-turbo", ToProvider: "openai", Priority: 20, Enabled: true},
		{ClientID: "match-client", FromModel: "gpt-*", ToModel: "claude-3", ToProvider: "anthropic", Priority: 5, Enabled: false},
	}
	for _, r := range rules {
		if err := repo.Create(r); err != nil {
			t.Fatalf("failed to create rule: %v", err)
		}
	}

	t.Run("ExactMatchHigherPriority", func(t *testing.T) {
		// claude-3-opus should match the exact rule (priority 20) over wildcard (priority 10)
		got, err := repo.FindMatching("claude-3-opus", "match-client")
		if err != nil {
			t.Fatalf("failed to find matching: %v", err)
		}
		if got == nil {
			t.Fatal("expected to find a matching rule")
		}
		if got.ToModel != "gpt-4-turbo" {
			t.Errorf("got ToModel %q, want %q", got.ToModel, "gpt-4-turbo")
		}
	})

	t.Run("WildcardMatch", func(t *testing.T) {
		// claude-3-sonnet should match the wildcard rule
		got, err := repo.FindMatching("claude-3-sonnet", "match-client")
		if err != nil {
			t.Fatalf("failed to find matching: %v", err)
		}
		if got == nil {
			t.Fatal("expected to find a matching rule")
		}
		if got.ToModel != "gpt-4" {
			t.Errorf("got ToModel %q, want %q", got.ToModel, "gpt-4")
		}
	})

	t.Run("NoMatch", func(t *testing.T) {
		// gemini should not match any rule
		got, err := repo.FindMatching("gemini-pro", "match-client")
		if err != nil {
			t.Fatalf("failed to find matching: %v", err)
		}
		if got != nil {
			t.Errorf("expected no matching rule, got %+v", got)
		}
	})

	t.Run("DisabledRuleNotMatched", func(t *testing.T) {
		// gpt-4 should not match the disabled rule
		got, err := repo.FindMatching("gpt-4", "match-client")
		if err != nil {
			t.Fatalf("failed to find matching: %v", err)
		}
		if got != nil {
			t.Errorf("expected no matching rule for disabled, got %+v", got)
		}
	})

	t.Run("WrongClient", func(t *testing.T) {
		// Using a different client should not match
		got, err := repo.FindMatching("claude-3-opus", "other-client")
		if err != nil {
			t.Fatalf("failed to find matching: %v", err)
		}
		if got != nil {
			t.Errorf("expected no matching rule for wrong client, got %+v", got)
		}
	})
}
