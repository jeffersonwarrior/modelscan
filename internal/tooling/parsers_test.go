package tooling

import (
	"testing"
)

// mockParser implements ToolParser for testing
type mockParser struct {
	providerID string
	format     ToolFormat
	caps       ProviderCapabilities
}

func (m *mockParser) Parse(response string) ([]ToolCall, error) {
	return []ToolCall{}, nil
}

func (m *mockParser) Format() ToolFormat {
	return m.format
}

func (m *mockParser) ProviderID() string {
	return m.providerID
}

func (m *mockParser) Capabilities() ProviderCapabilities {
	return m.caps
}

func TestRegisterAndGetParser(t *testing.T) {
	testFormat := ToolFormat("test_format_unique_1")
	parser := &mockParser{
		providerID: "test-provider-unique-1",
		format:     testFormat,
		caps: ProviderCapabilities{
			SupportsToolCalling: true,
			Format:              testFormat,
		},
	}

	RegisterParser("test-provider-unique-1", parser)

	retrieved, err := GetParser("test-provider-unique-1")
	if err != nil {
		t.Fatalf("Failed to get parser: %v", err)
	}

	if retrieved.ProviderID() != "test-provider-unique-1" {
		t.Errorf("Expected provider ID test-provider-unique-1, got %s", retrieved.ProviderID())
	}
}

func TestGetParserNotFound(t *testing.T) {
	_, err := GetParser("completely-nonexistent-parser-xyz")
	if err == nil {
		t.Error("Expected error for nonexistent parser")
	}
}

func TestHasParser(t *testing.T) {
	testFormat := ToolFormat("test_format_unique_2")
	parser := &mockParser{
		providerID: "test-provider-unique-2",
		format:     testFormat,
	}

	// Should not exist before registration
	if HasParser("test-provider-unique-2") {
		t.Error("Expected HasParser to return false before registration")
	}

	RegisterParser("test-provider-unique-2", parser)

	// Should exist after registration
	if !HasParser("test-provider-unique-2") {
		t.Error("Expected HasParser to return true after registration")
	}
}

func TestListParsers(t *testing.T) {
	// List should contain at least the parsers registered via init()
	list := ListParsers()
	if len(list) == 0 {
		t.Error("Expected at least some parsers to be registered")
	}

	// Check for known parsers from init()
	hasAnthropic := false
	hasOpenAI := false
	for _, id := range list {
		if id == "anthropic" {
			hasAnthropic = true
		}
		if id == "openai" {
			hasOpenAI = true
		}
	}

	if !hasAnthropic {
		t.Error("Expected anthropic parser to be registered")
	}
	if !hasOpenAI {
		t.Error("Expected openai parser to be registered")
	}
}

func TestGetParserByFormat(t *testing.T) {
	// Test with real parsers registered via init()
	parser, err := GetParserByFormat(FormatAnthropicJSON)
	if err != nil {
		t.Fatalf("Failed to get parser by format: %v", err)
	}

	if parser.ProviderID() != "anthropic" {
		t.Errorf("Expected anthropic parser, got %s", parser.ProviderID())
	}

	// Test with non-existent format (very unlikely to exist)
	_, err = GetParserByFormat(ToolFormat("completely-fake-format-xyz"))
	if err == nil {
		t.Error("Expected error when getting parser for non-existent format")
	}
}

func TestParserRegistryConcurrency(t *testing.T) {
	// Simulate concurrent registration with unique IDs and formats
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			testFormat := ToolFormat("test_concurrent_" + string(rune('α'+id)))
			parser := &mockParser{
				providerID: "concurrent_" + string(rune('α'+id)), // Use Greek letters to avoid conflicts
				format:     testFormat,
			}
			RegisterParser(parser.providerID, parser)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Test concurrent reads
	for i := 0; i < 10; i++ {
		go func(id int) {
			providerID := "concurrent_" + string(rune('α'+id))
			_, err := GetParser(providerID)
			if err != nil {
				t.Errorf("Failed to get parser %s: %v", providerID, err)
			}
			done <- true
		}(i)
	}

	// Wait for all reads
	for i := 0; i < 10; i++ {
		<-done
	}
}
