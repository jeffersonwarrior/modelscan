package parsers

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/jeffersonwarrior/modelscan/internal/tooling"
)

// DeepSeekParser handles DeepSeek's XML-based tool calling format.
type DeepSeekParser struct{}

var (
	// Regex patterns for parsing DeepSeek XML tool calls
	// DeepSeek uses similar XML format to xAI
	deepseekToolCallPattern = regexp.MustCompile(`<tool_call>\s*<id>(.*?)</id>\s*<name>(.*?)</name>\s*<parameters>(.*?)</parameters>\s*</tool_call>`)
)

// Parse extracts tool calls from DeepSeek's XML response format.
func (p *DeepSeekParser) Parse(response string) ([]tooling.ToolCall, error) {
	matches := deepseekToolCallPattern.FindAllStringSubmatch(response, -1)
	if len(matches) == 0 {
		return []tooling.ToolCall{}, nil
	}

	var toolCalls []tooling.ToolCall
	for _, match := range matches {
		if len(match) != 4 {
			continue
		}

		id := match[1]
		name := match[2]
		paramsJSON := match[3]

		// Parse parameters JSON
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(paramsJSON), &args); err != nil {
			return nil, fmt.Errorf("failed to parse tool parameters JSON: %w", err)
		}

		toolCall := tooling.ToolCall{
			ID:       id,
			Name:     name,
			Args:     args,
			Type:     "function",
			RawInput: response,
		}
		toolCalls = append(toolCalls, toolCall)
	}

	return toolCalls, nil
}

// Format returns the format this parser handles.
func (p *DeepSeekParser) Format() tooling.ToolFormat {
	return tooling.FormatDeepSeekXML
}

// ProviderID returns the provider identifier.
func (p *DeepSeekParser) ProviderID() string {
	return "deepseek"
}

// Capabilities returns the tool calling capabilities for DeepSeek.
func (p *DeepSeekParser) Capabilities() tooling.ProviderCapabilities {
	return tooling.ProviderCapabilities{
		SupportsToolCalling: true,
		SupportsParallel:    true,
		SupportsStreaming:   false, // XML format harder to stream
		Format:              tooling.FormatDeepSeekXML,
	}
}

func init() {
	tooling.RegisterParser("deepseek", &DeepSeekParser{})
}
