package parsers

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/jeffersonwarrior/modelscan/internal/tooling"
)

// XAIParser handles xAI's XML-based tool calling format.
type XAIParser struct{}

var (
	// Regex patterns for parsing xAI XML tool calls
	xaiToolCallPattern = regexp.MustCompile(`<tool_call>\s*<id>(.*?)</id>\s*<name>(.*?)</name>\s*<arguments>(.*?)</arguments>\s*</tool_call>`)
	xaiIDPattern       = regexp.MustCompile(`<id>(.*?)</id>`)
	xaiNamePattern     = regexp.MustCompile(`<name>(.*?)</name>`)
	xaiArgsPattern     = regexp.MustCompile(`<arguments>(.*?)</arguments>`)
)

// Parse extracts tool calls from xAI's XML response format.
func (p *XAIParser) Parse(response string) ([]tooling.ToolCall, error) {
	matches := xaiToolCallPattern.FindAllStringSubmatch(response, -1)
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
		argsJSON := match[3]

		// Parse arguments JSON
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return nil, fmt.Errorf("failed to parse tool arguments JSON: %w", err)
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
func (p *XAIParser) Format() tooling.ToolFormat {
	return tooling.FormatXAIXML
}

// ProviderID returns the provider identifier.
func (p *XAIParser) ProviderID() string {
	return "xai"
}

// Capabilities returns the tool calling capabilities for xAI.
func (p *XAIParser) Capabilities() tooling.ProviderCapabilities {
	return tooling.ProviderCapabilities{
		SupportsToolCalling: true,
		SupportsParallel:    true,
		SupportsStreaming:   false, // XML format harder to stream
		Format:              tooling.FormatXAIXML,
	}
}

func init() {
	tooling.RegisterParser("xai", &XAIParser{})
}
