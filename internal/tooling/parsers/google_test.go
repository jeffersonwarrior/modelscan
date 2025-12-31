package parsers

import (
	"testing"

	"github.com/jeffersonwarrior/modelscan/internal/tooling"
)

func TestGoogleParser_Parse(t *testing.T) {
	parser := &GoogleParser{}

	// Example Google Gemini response with function call
	response := `{
		"candidates": [
			{
				"content": {
					"parts": [
						{
							"functionCall": {
								"name": "get_weather",
								"args": {
									"location": "Mountain View",
									"unit": "celsius"
								}
							}
						}
					]
				}
			}
		]
	}`

	toolCalls, err := parser.Parse(response)
	if err != nil {
		t.Fatalf("Failed to parse Google response: %v", err)
	}

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	tc := toolCalls[0]
	if tc.Name != "get_weather" {
		t.Errorf("Expected Name get_weather, got %s", tc.Name)
	}
	if tc.Type != "function" {
		t.Errorf("Expected Type function, got %s", tc.Type)
	}

	// Check arguments
	location, ok := tc.Args["location"].(string)
	if !ok || location != "Mountain View" {
		t.Errorf("Expected location 'Mountain View', got %v", tc.Args["location"])
	}

	unit, ok := tc.Args["unit"].(string)
	if !ok || unit != "celsius" {
		t.Errorf("Expected unit 'celsius', got %v", tc.Args["unit"])
	}
}

func TestGoogleParser_ParseMultiple(t *testing.T) {
	parser := &GoogleParser{}

	// Multiple function calls across parts
	response := `{
		"candidates": [
			{
				"content": {
					"parts": [
						{
							"functionCall": {
								"name": "search",
								"args": {
									"query": "AI research"
								}
							}
						},
						{
							"functionCall": {
								"name": "translate",
								"args": {
									"text": "hello",
									"target": "es"
								}
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
	if toolCalls[0].Name != "search" {
		t.Errorf("Expected first tool name search, got %s", toolCalls[0].Name)
	}

	// Verify second tool call
	if toolCalls[1].Name != "translate" {
		t.Errorf("Expected second tool name translate, got %s", toolCalls[1].Name)
	}

	// Verify IDs are generated
	if toolCalls[0].ID == "" {
		t.Error("Expected generated ID for first tool call")
	}
	if toolCalls[1].ID == "" {
		t.Error("Expected generated ID for second tool call")
	}
}

func TestGoogleParser_ParseEmpty(t *testing.T) {
	parser := &GoogleParser{}

	// Response with no function calls (text response)
	response := `{
		"candidates": [
			{
				"content": {
					"parts": [
						{
							"text": "Here is my text response"
						}
					]
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

func TestGoogleParser_ParseNoCandidates(t *testing.T) {
	parser := &GoogleParser{}

	// Response with no candidates
	response := `{
		"candidates": []
	}`

	toolCalls, err := parser.Parse(response)
	if err != nil {
		t.Fatalf("Failed to parse empty candidates: %v", err)
	}

	if len(toolCalls) != 0 {
		t.Errorf("Expected 0 tool calls for empty candidates, got %d", len(toolCalls))
	}
}

func TestGoogleParser_ParseInvalid(t *testing.T) {
	parser := &GoogleParser{}

	// Invalid JSON
	response := `not valid json`

	_, err := parser.Parse(response)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestGoogleParser_ParseMultipleCandidates(t *testing.T) {
	parser := &GoogleParser{}

	// Multiple candidates with function calls
	response := `{
		"candidates": [
			{
				"content": {
					"parts": [
						{
							"functionCall": {
								"name": "func1",
								"args": {"key": "value1"}
							}
						}
					]
				}
			},
			{
				"content": {
					"parts": [
						{
							"functionCall": {
								"name": "func2",
								"args": {"key": "value2"}
							}
						}
					]
				}
			}
		]
	}`

	toolCalls, err := parser.Parse(response)
	if err != nil {
		t.Fatalf("Failed to parse multiple candidates: %v", err)
	}

	if len(toolCalls) != 2 {
		t.Fatalf("Expected 2 tool calls from multiple candidates, got %d", len(toolCalls))
	}

	if toolCalls[0].Name != "func1" {
		t.Errorf("Expected first function func1, got %s", toolCalls[0].Name)
	}
	if toolCalls[1].Name != "func2" {
		t.Errorf("Expected second function func2, got %s", toolCalls[1].Name)
	}
}

func TestGoogleParser_Format(t *testing.T) {
	parser := &GoogleParser{}

	if parser.Format() != tooling.FormatGoogleJSON {
		t.Errorf("Expected format %s, got %s", tooling.FormatGoogleJSON, parser.Format())
	}
}

func TestGoogleParser_ProviderID(t *testing.T) {
	parser := &GoogleParser{}

	if parser.ProviderID() != "google" {
		t.Errorf("Expected provider ID google, got %s", parser.ProviderID())
	}
}

func TestGoogleParser_Capabilities(t *testing.T) {
	parser := &GoogleParser{}

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
	if caps.Format != tooling.FormatGoogleJSON {
		t.Errorf("Expected format %s, got %s", tooling.FormatGoogleJSON, caps.Format)
	}
}

func TestGoogleParser_Registration(t *testing.T) {
	// Parser should auto-register via init()
	parser, err := tooling.GetParser("google")
	if err != nil {
		t.Fatalf("Failed to get registered Google parser: %v", err)
	}

	if parser.ProviderID() != "google" {
		t.Errorf("Expected google parser, got %s", parser.ProviderID())
	}
}
