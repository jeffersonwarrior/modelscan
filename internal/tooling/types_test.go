package tooling

import (
	"encoding/json"
	"testing"
)

func TestToolCallSerialization(t *testing.T) {
	tc := ToolCall{
		ID:   "call_123",
		Name: "get_weather",
		Args: map[string]interface{}{
			"location": "San Francisco",
			"units":    "celsius",
		},
		Type:     "function",
		RawInput: `{"location":"San Francisco","units":"celsius"}`,
	}

	// Test JSON marshaling
	data, err := json.Marshal(tc)
	if err != nil {
		t.Fatalf("Failed to marshal ToolCall: %v", err)
	}

	// Test JSON unmarshaling
	var decoded ToolCall
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal ToolCall: %v", err)
	}

	if decoded.ID != tc.ID {
		t.Errorf("Expected ID %s, got %s", tc.ID, decoded.ID)
	}
	if decoded.Name != tc.Name {
		t.Errorf("Expected Name %s, got %s", tc.Name, decoded.Name)
	}
	if decoded.Type != tc.Type {
		t.Errorf("Expected Type %s, got %s", tc.Type, decoded.Type)
	}
}

func TestToolResultSerialization(t *testing.T) {
	tr := ToolResult{
		ToolCallID: "call_123",
		Output:     "Temperature is 18°C",
		IsError:    false,
	}

	data, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("Failed to marshal ToolResult: %v", err)
	}

	var decoded ToolResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal ToolResult: %v", err)
	}

	if decoded.ToolCallID != tr.ToolCallID {
		t.Errorf("Expected ToolCallID %s, got %s", tr.ToolCallID, decoded.ToolCallID)
	}
	if decoded.Output != tr.Output {
		t.Errorf("Expected Output %s, got %s", tr.Output, decoded.Output)
	}
}

func TestToolSerialization(t *testing.T) {
	tool := Tool{
		Name:        "get_weather",
		Description: "Get current weather for a location",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "City name",
				},
			},
			"required": []string{"location"},
		},
	}

	data, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("Failed to marshal Tool: %v", err)
	}

	var decoded Tool
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal Tool: %v", err)
	}

	if decoded.Name != tool.Name {
		t.Errorf("Expected Name %s, got %s", tool.Name, decoded.Name)
	}
}

func TestProviderCapabilities(t *testing.T) {
	caps := ProviderCapabilities{
		SupportsToolCalling: true,
		SupportsParallel:    true,
		SupportsStreaming:   false,
		Format:              FormatAnthropicJSON,
	}

	if !caps.SupportsToolCalling {
		t.Error("Expected SupportsToolCalling to be true")
	}
	if caps.Format != FormatAnthropicJSON {
		t.Errorf("Expected Format %s, got %s", FormatAnthropicJSON, caps.Format)
	}
}

func TestToolFormats(t *testing.T) {
	formats := []ToolFormat{
		FormatAnthropicJSON,
		FormatOpenAIJSON,
		FormatXAIXML,
		FormatDeepSeekXML,
		FormatGoogleJSON,
	}

	expected := []string{
		"anthropic_json",
		"openai_json",
		"xai_xml",
		"deepseek_xml",
		"google_json",
	}

	for i, format := range formats {
		if string(format) != expected[i] {
			t.Errorf("Format %d: expected %s, got %s", i, expected[i], string(format))
		}
	}
}
