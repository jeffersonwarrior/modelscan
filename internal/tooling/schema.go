package tooling

import (
	"encoding/json"
	"fmt"
)

// SchemaTranslator handles bidirectional translation between provider schemas
type SchemaTranslator struct{}

// anthropicToolSchema represents Anthropic's tool definition format
type anthropicToolSchema struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// openaiToolSchema represents OpenAI's tool definition format
type openaiToolSchema struct {
	Type     string `json:"type"`
	Function struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description,omitempty"`
		Parameters  map[string]interface{} `json:"parameters"`
	} `json:"function"`
}

// AnthropicToOpenAI converts Anthropic tool schema to OpenAI format
func (st *SchemaTranslator) AnthropicToOpenAI(tools []Tool) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, len(tools))

	for i, tool := range tools {
		openaiTool := map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.InputSchema,
			},
		}
		result[i] = openaiTool
	}

	return result, nil
}

// OpenAIToAnthropic converts OpenAI tool schema to Anthropic format
func (st *SchemaTranslator) OpenAIToAnthropic(openaiTools []map[string]interface{}) ([]Tool, error) {
	result := make([]Tool, 0, len(openaiTools))

	for _, openaiTool := range openaiTools {
		// Extract function object
		funcObj, ok := openaiTool["function"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid OpenAI tool format: missing or invalid function object")
		}

		name, _ := funcObj["name"].(string)
		description, _ := funcObj["description"].(string)
		parameters, _ := funcObj["parameters"].(map[string]interface{})

		if name == "" {
			return nil, fmt.Errorf("tool name is required")
		}

		tool := Tool{
			Name:        name,
			Description: description,
			InputSchema: parameters,
		}
		result = append(result, tool)
	}

	return result, nil
}

// ValidateAnthropicSchema validates Anthropic tool schema
func (st *SchemaTranslator) ValidateAnthropicSchema(tool Tool) error {
	if tool.Name == "" {
		return fmt.Errorf("tool name is required")
	}

	if tool.InputSchema == nil {
		return fmt.Errorf("input_schema is required")
	}

	// Check for required JSON schema fields
	schemaType, ok := tool.InputSchema["type"].(string)
	if !ok || schemaType == "" {
		return fmt.Errorf("input_schema must have a 'type' field")
	}

	return nil
}

// ValidateOpenAISchema validates OpenAI tool schema
func (st *SchemaTranslator) ValidateOpenAISchema(openaiTool map[string]interface{}) error {
	toolType, ok := openaiTool["type"].(string)
	if !ok || toolType != "function" {
		return fmt.Errorf("tool type must be 'function'")
	}

	funcObj, ok := openaiTool["function"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("function object is required")
	}

	name, ok := funcObj["name"].(string)
	if !ok || name == "" {
		return fmt.Errorf("function name is required")
	}

	parameters, ok := funcObj["parameters"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("parameters object is required")
	}

	// Check for required JSON schema fields
	paramType, ok := parameters["type"].(string)
	if !ok || paramType == "" {
		return fmt.Errorf("parameters must have a 'type' field")
	}

	return nil
}

// NormalizeSchema ensures a schema has required JSON Schema fields
func (st *SchemaTranslator) NormalizeSchema(schema map[string]interface{}) map[string]interface{} {
	// Ensure type exists
	if _, ok := schema["type"]; !ok {
		schema["type"] = "object"
	}

	// Ensure properties exists for object type
	if schemaType, ok := schema["type"].(string); ok && schemaType == "object" {
		if _, ok := schema["properties"]; !ok {
			schema["properties"] = make(map[string]interface{})
		}
	}

	return schema
}

// ToolToJSON serializes a tool to JSON for API requests
func (st *SchemaTranslator) ToolToJSON(tool Tool) (string, error) {
	data, err := json.Marshal(tool)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tool: %w", err)
	}
	return string(data), nil
}

// ToolFromJSON deserializes a tool from JSON
func (st *SchemaTranslator) ToolFromJSON(data string) (*Tool, error) {
	var tool Tool
	if err := json.Unmarshal([]byte(data), &tool); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tool: %w", err)
	}
	return &tool, nil
}
