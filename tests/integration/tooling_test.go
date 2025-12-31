package integration

import (
	"testing"

	"github.com/jeffersonwarrior/modelscan/internal/tooling"
	"github.com/jeffersonwarrior/modelscan/internal/tooling/parsers"
)

func TestToolCalling_AnthropicParser(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	parser, err := tooling.GetParser("anthropic")
	if err != nil {
		t.Fatalf("Failed to get Anthropic parser: %v", err)
	}

	response := `{
		"content": [
			{
				"type": "tool_use",
				"id": "toolu_123",
				"name": "get_weather",
				"input": {
					"city": "San Francisco",
					"unit": "celsius"
				}
			}
		]
	}`

	calls, err := parser.Parse(response)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(calls))
	}

	call := calls[0]
	if call.ID != "toolu_123" {
		t.Errorf("Expected ID 'toolu_123', got '%s'", call.ID)
	}

	if call.Name != "get_weather" {
		t.Errorf("Expected name 'get_weather', got '%s'", call.Name)
	}

	if city, ok := call.Args["city"].(string); !ok || city != "San Francisco" {
		t.Errorf("Expected city 'San Francisco', got '%v'", call.Args["city"])
	}

	t.Log("Anthropic parser test passed")
}

func TestToolCalling_OpenAIParser(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	parser, err := tooling.GetParser("openai")
	if err != nil {
		t.Fatalf("Failed to get OpenAI parser: %v", err)
	}

	response := `{
		"choices": [{
			"message": {
				"tool_calls": [{
					"id": "call_abc123",
					"type": "function",
					"function": {
						"name": "get_weather",
						"arguments": "{\"city\":\"London\",\"unit\":\"fahrenheit\"}"
					}
				}]
			}
		}]
	}`

	calls, err := parser.Parse(response)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(calls))
	}

	call := calls[0]
	if call.ID != "call_abc123" {
		t.Errorf("Expected ID 'call_abc123', got '%s'", call.ID)
	}

	if call.Name != "get_weather" {
		t.Errorf("Expected name 'get_weather', got '%s'", call.Name)
	}

	if city, ok := call.Args["city"].(string); !ok || city != "London" {
		t.Errorf("Expected city 'London', got '%v'", call.Args["city"])
	}

	t.Log("OpenAI parser test passed")
}

func TestToolCalling_XAIParser(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	parser, err := tooling.GetParser("xai")
	if err != nil {
		t.Fatalf("Failed to get xAI parser: %v", err)
	}

	response := `<tool_call>
<id>xai_001</id>
<name>calculate</name>
<arguments>{"operation":"add","a":5,"b":3}</arguments>
</tool_call>`

	calls, err := parser.Parse(response)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(calls))
	}

	call := calls[0]
	if call.Name != "calculate" {
		t.Errorf("Expected name 'calculate', got '%s'", call.Name)
	}

	if op, ok := call.Args["operation"].(string); !ok || op != "add" {
		t.Errorf("Expected operation 'add', got '%v'", call.Args["operation"])
	}

	t.Log("xAI parser test passed")
}

func TestToolCalling_DeepSeekParser(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	parser, err := tooling.GetParser("deepseek")
	if err != nil {
		t.Fatalf("Failed to get DeepSeek parser: %v", err)
	}

	response := `<tool_call>
<id>ds_123</id>
<name>search_database</name>
<parameters>{"query":"machine learning","limit":10}</parameters>
</tool_call>`

	calls, err := parser.Parse(response)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(calls))
	}

	call := calls[0]
	if call.Name != "search_database" {
		t.Errorf("Expected name 'search_database', got '%s'", call.Name)
	}

	if query, ok := call.Args["query"].(string); !ok || query != "machine learning" {
		t.Errorf("Expected query 'machine learning', got '%v'", call.Args["query"])
	}

	t.Log("DeepSeek parser test passed")
}

func TestToolCalling_GoogleParser(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	parser, err := tooling.GetParser("google")
	if err != nil {
		t.Fatalf("Failed to get Google parser: %v", err)
	}

	response := `{
		"candidates": [{
			"content": {
				"parts": [{
					"functionCall": {
						"name": "get_current_time",
						"args": {
							"timezone": "UTC"
						}
					}
				}]
			}
		}]
	}`

	calls, err := parser.Parse(response)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(calls))
	}

	call := calls[0]
	if call.Name != "get_current_time" {
		t.Errorf("Expected name 'get_current_time', got '%s'", call.Name)
	}

	if tz, ok := call.Args["timezone"].(string); !ok || tz != "UTC" {
		t.Errorf("Expected timezone 'UTC', got '%v'", call.Args["timezone"])
	}

	// Google generates IDs automatically
	if call.ID == "" {
		t.Error("Expected non-empty generated ID")
	}

	t.Log("Google parser test passed")
}

func TestToolCalling_FormatDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	detector := tooling.NewFormatDetector()

	testCases := []struct {
		name           string
		response       string
		expectedFormat tooling.ToolFormat
	}{
		{
			name:           "Anthropic JSON",
			response:       `{"content":[{"type":"tool_use","id":"123","name":"test","input":{}}]}`,
			expectedFormat: tooling.FormatAnthropicJSON,
		},
		{
			name:           "OpenAI JSON",
			response:       `{"choices":[{"message":{"tool_calls":[{"id":"123","function":{"name":"test"}}]}}]}`,
			expectedFormat: tooling.FormatOpenAIJSON,
		},
		{
			name:           "xAI XML",
			response:       `<tool_call><id>123</id><name>test</name><arguments>{}</arguments></tool_call>`,
			expectedFormat: tooling.FormatXAIXML,
		},
		{
			name:           "DeepSeek XML",
			response:       `<tool_call><id>123</id><name>test</name><parameters>{}</parameters></tool_call>`,
			expectedFormat: tooling.FormatDeepSeekXML,
		},
		{
			name:           "Google JSON",
			response:       `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"test"}}]}}]}`,
			expectedFormat: tooling.FormatGoogleJSON,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser, err := detector.DetectParser(tc.response)
			if err != nil {
				t.Fatalf("Failed to detect format: %v", err)
			}

			if parser.Format() != tc.expectedFormat {
				t.Errorf("Expected format %s, got %s", tc.expectedFormat, parser.Format())
			}

			t.Logf("Correctly detected format: %s", tc.expectedFormat)
		})
	}
}

func TestToolCalling_SchemaTranslation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	translator := &tooling.SchemaTranslator{}

	// Define Anthropic-style tools
	anthropicTools := []tooling.Tool{
		{
			Name:        "get_weather",
			Description: "Get weather for a location",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"city": map[string]interface{}{
						"type":        "string",
						"description": "City name",
					},
				},
				"required": []string{"city"},
			},
		},
	}

	// Convert to OpenAI format
	openaiTools, err := translator.AnthropicToOpenAI(anthropicTools)
	if err != nil {
		t.Fatalf("Failed to convert to OpenAI format: %v", err)
	}

	if len(openaiTools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(openaiTools))
	}

	openaiTool := openaiTools[0]
	if toolType, ok := openaiTool["type"].(string); !ok || toolType != "function" {
		t.Error("Expected type 'function'")
	}

	// Convert back to Anthropic format
	backToAnthropic, err := translator.OpenAIToAnthropic(openaiTools)
	if err != nil {
		t.Fatalf("Failed to convert to Anthropic format: %v", err)
	}

	if len(backToAnthropic) != 1 {
		t.Fatalf("Expected 1 tool after round-trip, got %d", len(backToAnthropic))
	}

	if backToAnthropic[0].Name != anthropicTools[0].Name {
		t.Errorf("Name mismatch after round-trip: %s != %s",
			backToAnthropic[0].Name, anthropicTools[0].Name)
	}

	t.Log("Schema translation round-trip successful")
}

func TestToolCalling_ParserRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test that all parsers are registered
	expectedParsers := []string{"anthropic", "openai", "xai", "deepseek", "google"}

	for _, providerID := range expectedParsers {
		parser, err := tooling.GetParser(providerID)
		if err != nil {
			t.Errorf("Parser for %s not registered: %v", providerID, err)
			continue
		}

		if parser == nil {
			t.Errorf("Parser for %s is nil", providerID)
			continue
		}

		if parser.ProviderID() != providerID {
			t.Errorf("Parser provider ID mismatch: expected %s, got %s",
				providerID, parser.ProviderID())
		}

		t.Logf("✓ %s parser registered and functional", providerID)
	}
}

func TestToolCalling_MultipleCallsInResponse(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test OpenAI parallel tool calls
	response := `{
		"choices": [{
			"message": {
				"tool_calls": [
					{
						"id": "call_1",
						"type": "function",
						"function": {
							"name": "get_weather",
							"arguments": "{\"city\":\"NYC\"}"
						}
					},
					{
						"id": "call_2",
						"type": "function",
						"function": {
							"name": "get_weather",
							"arguments": "{\"city\":\"LA\"}"
						}
					},
					{
						"id": "call_3",
						"type": "function",
						"function": {
							"name": "get_time",
							"arguments": "{\"timezone\":\"UTC\"}"
						}
					}
				]
			}
		}]
	}`

	parser, _ := tooling.GetParser("openai")
	calls, err := parser.Parse(response)

	if err != nil {
		t.Fatalf("Failed to parse multiple calls: %v", err)
	}

	if len(calls) != 3 {
		t.Fatalf("Expected 3 tool calls, got %d", len(calls))
	}

	// Verify all calls were parsed
	expectedNames := []string{"get_weather", "get_weather", "get_time"}
	for i, call := range calls {
		if call.Name != expectedNames[i] {
			t.Errorf("Call %d: expected name '%s', got '%s'",
				i, expectedNames[i], call.Name)
		}
	}

	t.Log("Multiple tool calls parsed successfully")
}

// Ensure all parser packages are imported for registration
var _ = parsers.AnthropicParser{}
