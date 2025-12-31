package parsers

import (
	"encoding/json"
	"fmt"

	"github.com/jeffersonwarrior/modelscan/internal/tooling"
)

// OpenAIParser handles OpenAI's function calling JSON format.
type OpenAIParser struct{}

// openaiToolCall represents OpenAI's tool_call structure
type openaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON string
	} `json:"function"`
}

// openaiMessage represents OpenAI's message structure
type openaiMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
}

// openaiResponse represents the structure of OpenAI API responses
type openaiResponse struct {
	Choices []struct {
		Message openaiMessage `json:"message"`
	} `json:"choices"`
}

// Parse extracts tool calls from OpenAI's JSON response format.
func (p *OpenAIParser) Parse(response string) ([]tooling.ToolCall, error) {
	var resp openaiResponse
	if err := json.Unmarshal([]byte(response), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI response: %w", err)
	}

	if len(resp.Choices) == 0 {
		return []tooling.ToolCall{}, nil
	}

	var toolCalls []tooling.ToolCall
	for _, tc := range resp.Choices[0].Message.ToolCalls {
		// Parse the arguments JSON string
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
		}

		toolCall := tooling.ToolCall{
			ID:       tc.ID,
			Name:     tc.Function.Name,
			Args:     args,
			Type:     "function",
			RawInput: response,
		}
		toolCalls = append(toolCalls, toolCall)
	}

	return toolCalls, nil
}

// Format returns the format this parser handles.
func (p *OpenAIParser) Format() tooling.ToolFormat {
	return tooling.FormatOpenAIJSON
}

// ProviderID returns the provider identifier.
func (p *OpenAIParser) ProviderID() string {
	return "openai"
}

// Capabilities returns the tool calling capabilities for OpenAI.
func (p *OpenAIParser) Capabilities() tooling.ProviderCapabilities {
	return tooling.ProviderCapabilities{
		SupportsToolCalling: true,
		SupportsParallel:    true,
		SupportsStreaming:   true,
		Format:              tooling.FormatOpenAIJSON,
	}
}

func init() {
	tooling.RegisterParser("openai", &OpenAIParser{})
}
