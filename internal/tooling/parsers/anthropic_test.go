package parsers

import (
	"testing"

	"github.com/jeffersonwarrior/modelscan/internal/tooling"
)

func TestAnthropicParser_Parse(t *testing.T) {
	parser := &AnthropicParser{}

	// Example Anthropic response with tool_use
	response := `{
		"content": [
			{
				"type": "tool_use",
				"id": "toolu_01A09q90qw90lq917835lq9",
				"name": "get_weather",
				"input": {
					"location": "San Francisco, CA",
					"unit": "fahrenheit"
				}
			}
		]
	}`

	toolCalls, err := parser.Parse(response)
	if err != nil {
		t.Fatalf("Failed to parse Anthropic response: %v", err)
	}

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	tc := toolCalls[0]
	if tc.ID != "toolu_01A09q90qw90lq917835lq9" {
		t.Errorf("Expected ID toolu_01A09q90qw90lq917835lq9, got %s", tc.ID)
	}
	if tc.Name != "get_weather" {
		t.Errorf("Expected Name get_weather, got %s", tc.Name)
	}
	if tc.Type != "function" {
		t.Errorf("Expected Type function, got %s", tc.Type)
	}

	// Check arguments
	location, ok := tc.Args["location"].(string)
	if !ok || location != "San Francisco, CA" {
		t.Errorf("Expected location 'San Francisco, CA', got %v", tc.Args["location"])
	}

	unit, ok := tc.Args["unit"].(string)
	if !ok || unit != "fahrenheit" {
		t.Errorf("Expected unit 'fahrenheit', got %v", tc.Args["unit"])
	}
}

func TestAnthropicParser_ParseMultiple(t *testing.T) {
	parser := &AnthropicParser{}

	// Multiple tool calls in parallel
	response := `{
		"content": [
			{
				"type": "tool_use",
				"id": "toolu_01A",
				"name": "get_weather",
				"input": {
					"location": "New York"
				}
			},
			{
				"type": "tool_use",
				"id": "toolu_01B",
				"name": "get_time",
				"input": {
					"timezone": "EST"
				}
			}
		]
	}`

	toolCalls, err := parser.Parse(response)
	if err != nil {
		t.Fatalf("Failed to parse multiple tool calls: %v", err)
	}

	if len(toolCalls) != 2 {
		t.Fatalf("Expected 2 tool calls, got %d", len(toolCalls))
	}

	// Verify first tool call
	if toolCalls[0].Name != "get_weather" {
		t.Errorf("Expected first tool name get_weather, got %s", toolCalls[0].Name)
	}

	// Verify second tool call
	if toolCalls[1].Name != "get_time" {
		t.Errorf("Expected second tool name get_time, got %s", toolCalls[1].Name)
	}
}

func TestAnthropicParser_ParseEmpty(t *testing.T) {
	parser := &AnthropicParser{}

	// Response with no tool calls (text response)
	response := `{
		"content": [
			{
				"type": "text",
				"text": "Here is my response without tool use"
			}
		]
	}`

	toolCalls, err := parser.Parse(response)
	if err != nil {
		t.Fatalf("Failed to parse text-only response: %v", err)
	}

	if len(toolCalls) != 0 {
		t.Errorf("Expected 0 tool calls for text response, got %d", len(toolCalls))
	}
}

func TestAnthropicParser_ParseInvalid(t *testing.T) {
	parser := &AnthropicParser{}

	// Invalid JSON
	response := `not valid json at all`

	_, err := parser.Parse(response)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestAnthropicParser_Format(t *testing.T) {
	parser := &AnthropicParser{}

	if parser.Format() != tooling.FormatAnthropicJSON {
		t.Errorf("Expected format %s, got %s", tooling.FormatAnthropicJSON, parser.Format())
	}
}

func TestAnthropicParser_ProviderID(t *testing.T) {
	parser := &AnthropicParser{}

	if parser.ProviderID() != "anthropic" {
		t.Errorf("Expected provider ID anthropic, got %s", parser.ProviderID())
	}
}

func TestAnthropicParser_Capabilities(t *testing.T) {
	parser := &AnthropicParser{}

	caps := parser.Capabilities()

	if !caps.SupportsToolCalling {
		t.Error("Expected SupportsToolCalling to be true")
	}
	if !caps.SupportsParallel {
		t.Error("Expected SupportsParallel to be true")
	}
	if !caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be true")
	}
	if caps.Format != tooling.FormatAnthropicJSON {
		t.Errorf("Expected format %s, got %s", tooling.FormatAnthropicJSON, caps.Format)
	}
}

func TestAnthropicParser_Registration(t *testing.T) {
	// Parser should auto-register via init()
	parser, err := tooling.GetParser("anthropic")
	if err != nil {
		t.Fatalf("Failed to get registered Anthropic parser: %v", err)
	}

	if parser.ProviderID() != "anthropic" {
		t.Errorf("Expected anthropic parser, got %s", parser.ProviderID())
	}
}
