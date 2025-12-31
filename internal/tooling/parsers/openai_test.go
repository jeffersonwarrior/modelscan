package parsers

import (
	"testing"

	"github.com/jeffersonwarrior/modelscan/internal/tooling"
)

func TestOpenAIParser_Parse(t *testing.T) {
	parser := &OpenAIParser{}

	// Example OpenAI response with tool_calls
	response := `{
		"choices": [
			{
				"message": {
					"role": "assistant",
					"content": null,
					"tool_calls": [
						{
							"id": "call_abc123",
							"type": "function",
							"function": {
								"name": "get_weather",
								"arguments": "{\"location\":\"San Francisco\",\"unit\":\"celsius\"}"
							}
						}
					]
				}
			}
		]
	}`

	toolCalls, err := parser.Parse(response)
	if err != nil {
		t.Fatalf("Failed to parse OpenAI response: %v", err)
	}

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	tc := toolCalls[0]
	if tc.ID != "call_abc123" {
		t.Errorf("Expected ID call_abc123, got %s", tc.ID)
	}
	if tc.Name != "get_weather" {
		t.Errorf("Expected Name get_weather, got %s", tc.Name)
	}
	if tc.Type != "function" {
		t.Errorf("Expected Type function, got %s", tc.Type)
	}

	// Check arguments
	location, ok := tc.Args["location"].(string)
	if !ok || location != "San Francisco" {
		t.Errorf("Expected location 'San Francisco', got %v", tc.Args["location"])
	}

	unit, ok := tc.Args["unit"].(string)
	if !ok || unit != "celsius" {
		t.Errorf("Expected unit 'celsius', got %v", tc.Args["unit"])
	}
}

func TestOpenAIParser_ParseMultiple(t *testing.T) {
	parser := &OpenAIParser{}

	// Multiple tool calls
	response := `{
		"choices": [
			{
				"message": {
					"role": "assistant",
					"content": null,
					"tool_calls": [
						{
							"id": "call_1",
							"type": "function",
							"function": {
								"name": "get_weather",
								"arguments": "{\"location\":\"NYC\"}"
							}
						},
						{
							"id": "call_2",
							"type": "function",
							"function": {
								"name": "get_time",
								"arguments": "{\"timezone\":\"EST\"}"
							}
						}
					]
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
	if toolCalls[0].ID != "call_1" {
		t.Errorf("Expected first ID call_1, got %s", toolCalls[0].ID)
	}

	// Verify second tool call
	if toolCalls[1].Name != "get_time" {
		t.Errorf("Expected second tool name get_time, got %s", toolCalls[1].Name)
	}
	if toolCalls[1].ID != "call_2" {
		t.Errorf("Expected second ID call_2, got %s", toolCalls[1].ID)
	}
}

func TestOpenAIParser_ParseEmpty(t *testing.T) {
	parser := &OpenAIParser{}

	// Response with no tool calls (text response)
	response := `{
		"choices": [
			{
				"message": {
					"role": "assistant",
					"content": "Here is my response without tools"
				}
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

func TestOpenAIParser_ParseNoChoices(t *testing.T) {
	parser := &OpenAIParser{}

	// Response with no choices
	response := `{
		"choices": []
	}`

	toolCalls, err := parser.Parse(response)
	if err != nil {
		t.Fatalf("Failed to parse empty choices: %v", err)
	}

	if len(toolCalls) != 0 {
		t.Errorf("Expected 0 tool calls for empty choices, got %d", len(toolCalls))
	}
}

func TestOpenAIParser_ParseInvalid(t *testing.T) {
	parser := &OpenAIParser{}

	// Invalid JSON
	response := `not valid json`

	_, err := parser.Parse(response)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestOpenAIParser_ParseInvalidArguments(t *testing.T) {
	parser := &OpenAIParser{}

	// Tool call with invalid arguments JSON
	response := `{
		"choices": [
			{
				"message": {
					"role": "assistant",
					"tool_calls": [
						{
							"id": "call_1",
							"type": "function",
							"function": {
								"name": "test",
								"arguments": "not valid json"
							}
						}
					]
				}
			}
		]
	}`

	_, err := parser.Parse(response)
	if err == nil {
		t.Error("Expected error for invalid arguments JSON")
	}
}

func TestOpenAIParser_Format(t *testing.T) {
	parser := &OpenAIParser{}

	if parser.Format() != tooling.FormatOpenAIJSON {
		t.Errorf("Expected format %s, got %s", tooling.FormatOpenAIJSON, parser.Format())
	}
}

func TestOpenAIParser_ProviderID(t *testing.T) {
	parser := &OpenAIParser{}

	if parser.ProviderID() != "openai" {
		t.Errorf("Expected provider ID openai, got %s", parser.ProviderID())
	}
}

func TestOpenAIParser_Capabilities(t *testing.T) {
	parser := &OpenAIParser{}

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
	if caps.Format != tooling.FormatOpenAIJSON {
		t.Errorf("Expected format %s, got %s", tooling.FormatOpenAIJSON, caps.Format)
	}
}

func TestOpenAIParser_Registration(t *testing.T) {
	// Parser should auto-register via init()
	parser, err := tooling.GetParser("openai")
	if err != nil {
		t.Fatalf("Failed to get registered OpenAI parser: %v", err)
	}

	if parser.ProviderID() != "openai" {
		t.Errorf("Expected openai parser, got %s", parser.ProviderID())
	}
}
