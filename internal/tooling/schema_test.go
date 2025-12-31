package tooling

import (
	"encoding/json"
	"testing"
)

func TestAnthropicToOpenAI(t *testing.T) {
	st := &SchemaTranslator{}

	tools := []Tool{
		{
			Name:        "get_weather",
			Description: "Get weather for a location",
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
		},
	}

	openaiTools, err := st.AnthropicToOpenAI(tools)
	if err != nil {
		t.Fatalf("Failed to convert to OpenAI format: %v", err)
	}

	if len(openaiTools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(openaiTools))
	}

	tool := openaiTools[0]

	// Verify type
	if tool["type"] != "function" {
		t.Errorf("Expected type 'function', got %v", tool["type"])
	}

	// Verify function object
	funcObj, ok := tool["function"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected function object")
	}

	if funcObj["name"] != "get_weather" {
		t.Errorf("Expected name 'get_weather', got %v", funcObj["name"])
	}

	if funcObj["description"] != "Get weather for a location" {
		t.Errorf("Expected description, got %v", funcObj["description"])
	}

	// Verify parameters preserved
	params, ok := funcObj["parameters"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected parameters object")
	}

	if params["type"] != "object" {
		t.Errorf("Expected type 'object', got %v", params["type"])
	}
}

func TestOpenAIToAnthropic(t *testing.T) {
	st := &SchemaTranslator{}

	openaiTools := []map[string]interface{}{
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "calculate",
				"description": "Perform calculation",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"expression": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		},
	}

	tools, err := st.OpenAIToAnthropic(openaiTools)
	if err != nil {
		t.Fatalf("Failed to convert to Anthropic format: %v", err)
	}

	if len(tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(tools))
	}

	tool := tools[0]

	if tool.Name != "calculate" {
		t.Errorf("Expected name 'calculate', got %s", tool.Name)
	}

	if tool.Description != "Perform calculation" {
		t.Errorf("Expected description, got %s", tool.Description)
	}

	if tool.InputSchema["type"] != "object" {
		t.Errorf("Expected schema type 'object', got %v", tool.InputSchema["type"])
	}
}

func TestRoundTripConversion(t *testing.T) {
	st := &SchemaTranslator{}

	// Start with Anthropic format
	original := []Tool{
		{
			Name:        "test_tool",
			Description: "Test tool",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"param": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
	}

	// Convert to OpenAI
	openaiTools, err := st.AnthropicToOpenAI(original)
	if err != nil {
		t.Fatalf("Failed to convert to OpenAI: %v", err)
	}

	// Convert back to Anthropic
	result, err := st.OpenAIToAnthropic(openaiTools)
	if err != nil {
		t.Fatalf("Failed to convert back to Anthropic: %v", err)
	}

	// Verify round trip preserved data
	if result[0].Name != original[0].Name {
		t.Errorf("Name changed during round trip: %s -> %s", original[0].Name, result[0].Name)
	}

	if result[0].Description != original[0].Description {
		t.Errorf("Description changed during round trip")
	}
}

func TestValidateAnthropicSchema(t *testing.T) {
	st := &SchemaTranslator{}

	// Valid schema
	validTool := Tool{
		Name:        "test",
		Description: "Test tool",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"param": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}

	if err := st.ValidateAnthropicSchema(validTool); err != nil {
		t.Errorf("Valid schema failed validation: %v", err)
	}

	// Missing name
	invalidTool := Tool{
		Description: "Test",
		InputSchema: map[string]interface{}{
			"type": "object",
		},
	}

	if err := st.ValidateAnthropicSchema(invalidTool); err == nil {
		t.Error("Expected error for missing name")
	}

	// Missing input_schema
	invalidTool2 := Tool{
		Name:        "test",
		Description: "Test",
	}

	if err := st.ValidateAnthropicSchema(invalidTool2); err == nil {
		t.Error("Expected error for missing input_schema")
	}

	// Missing schema type
	invalidTool3 := Tool{
		Name:        "test",
		Description: "Test",
		InputSchema: map[string]interface{}{
			"properties": map[string]interface{}{},
		},
	}

	if err := st.ValidateAnthropicSchema(invalidTool3); err == nil {
		t.Error("Expected error for missing schema type")
	}
}

func TestValidateOpenAISchema(t *testing.T) {
	st := &SchemaTranslator{}

	// Valid schema
	validTool := map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "test",
			"description": "Test",
			"parameters": map[string]interface{}{
				"type": "object",
			},
		},
	}

	if err := st.ValidateOpenAISchema(validTool); err != nil {
		t.Errorf("Valid schema failed validation: %v", err)
	}

	// Missing type
	invalidTool := map[string]interface{}{
		"function": map[string]interface{}{
			"name":       "test",
			"parameters": map[string]interface{}{"type": "object"},
		},
	}

	if err := st.ValidateOpenAISchema(invalidTool); err == nil {
		t.Error("Expected error for missing type")
	}

	// Wrong type
	invalidTool2 := map[string]interface{}{
		"type": "invalid",
		"function": map[string]interface{}{
			"name":       "test",
			"parameters": map[string]interface{}{"type": "object"},
		},
	}

	if err := st.ValidateOpenAISchema(invalidTool2); err == nil {
		t.Error("Expected error for wrong type")
	}

	// Missing function object
	invalidTool3 := map[string]interface{}{
		"type": "function",
	}

	if err := st.ValidateOpenAISchema(invalidTool3); err == nil {
		t.Error("Expected error for missing function object")
	}
}

func TestNormalizeSchema(t *testing.T) {
	st := &SchemaTranslator{}

	// Schema without type
	schema := map[string]interface{}{
		"properties": map[string]interface{}{},
	}

	normalized := st.NormalizeSchema(schema)

	if normalized["type"] != "object" {
		t.Errorf("Expected default type 'object', got %v", normalized["type"])
	}

	// Object schema without properties
	schema2 := map[string]interface{}{
		"type": "object",
	}

	normalized2 := st.NormalizeSchema(schema2)

	if _, ok := normalized2["properties"]; !ok {
		t.Error("Expected properties to be added to object schema")
	}
}

func TestToolToJSON(t *testing.T) {
	st := &SchemaTranslator{}

	tool := Tool{
		Name:        "test",
		Description: "Test tool",
		InputSchema: map[string]interface{}{
			"type": "object",
		},
	}

	jsonStr, err := st.ToolToJSON(tool)
	if err != nil {
		t.Fatalf("Failed to serialize tool: %v", err)
	}

	// Verify it's valid JSON
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &decoded); err != nil {
		t.Fatalf("Produced invalid JSON: %v", err)
	}

	if decoded["name"] != "test" {
		t.Errorf("Expected name 'test', got %v", decoded["name"])
	}
}

func TestToolFromJSON(t *testing.T) {
	st := &SchemaTranslator{}

	jsonStr := `{
		"name": "test",
		"description": "Test tool",
		"input_schema": {
			"type": "object"
		}
	}`

	tool, err := st.ToolFromJSON(jsonStr)
	if err != nil {
		t.Fatalf("Failed to deserialize tool: %v", err)
	}

	if tool.Name != "test" {
		t.Errorf("Expected name 'test', got %s", tool.Name)
	}

	if tool.Description != "Test tool" {
		t.Errorf("Expected description 'Test tool', got %s", tool.Description)
	}

	if tool.InputSchema["type"] != "object" {
		t.Errorf("Expected schema type 'object', got %v", tool.InputSchema["type"])
	}
}

func TestToolFromJSON_Invalid(t *testing.T) {
	st := &SchemaTranslator{}

	invalidJSON := `not valid json`

	_, err := st.ToolFromJSON(invalidJSON)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestOpenAIToAnthropic_InvalidFormat(t *testing.T) {
	st := &SchemaTranslator{}

	// Missing function object
	invalidTools := []map[string]interface{}{
		{
			"type": "function",
		},
	}

	_, err := st.OpenAIToAnthropic(invalidTools)
	if err == nil {
		t.Error("Expected error for invalid OpenAI format")
	}

	// Missing name
	invalidTools2 := []map[string]interface{}{
		{
			"type": "function",
			"function": map[string]interface{}{
				"description": "Test",
				"parameters":  map[string]interface{}{},
			},
		},
	}

	_, err = st.OpenAIToAnthropic(invalidTools2)
	if err == nil {
		t.Error("Expected error for missing name")
	}
}
