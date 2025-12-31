package parsers

import (
	"encoding/json"
	"fmt"

	"github.com/jeffersonwarrior/modelscan/internal/tooling"
)

// AnthropicParser handles Anthropic's native JSON tool calling format.
type AnthropicParser struct{}

// anthropicToolUse represents Anthropic's tool_use content block
type anthropicToolUse struct {
	Type  string                 `json:"type"`
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// anthropicResponse represents the structure of Anthropic API responses
type anthropicResponse struct {
	Content []anthropicToolUse `json:"content"`
}

// Parse extracts tool calls from Anthropic's JSON response format.
func (p *AnthropicParser) Parse(response string) ([]tooling.ToolCall, error) {
	var resp anthropicResponse
	if err := json.Unmarshal([]byte(response), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic response: %w", err)
	}

	var toolCalls []tooling.ToolCall
	for _, content := range resp.Content {
		if content.Type == "tool_use" {
			toolCall := tooling.ToolCall{
				ID:       content.ID,
				Name:     content.Name,
				Args:     content.Input,
				Type:     "function",
				RawInput: response,
			}
			toolCalls = append(toolCalls, toolCall)
		}
	}

	return toolCalls, nil
}

// Format returns the format this parser handles.
func (p *AnthropicParser) Format() tooling.ToolFormat {
	return tooling.FormatAnthropicJSON
}

// ProviderID returns the provider identifier.
func (p *AnthropicParser) ProviderID() string {
	return "anthropic"
}

// Capabilities returns the tool calling capabilities for Anthropic.
func (p *AnthropicParser) Capabilities() tooling.ProviderCapabilities {
	return tooling.ProviderCapabilities{
		SupportsToolCalling: true,
		SupportsParallel:    true,
		SupportsStreaming:   true,
		Format:              tooling.FormatAnthropicJSON,
	}
}

func init() {
	tooling.RegisterParser("anthropic", &AnthropicParser{})
}
