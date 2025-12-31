package tooling

import (
	"fmt"
	"regexp"
)

// FormatDetector identifies tool calling format from response content
type FormatDetector struct {
	patterns map[ToolFormat]*regexp.Regexp
}

// NewFormatDetector creates a new format detector with detection patterns
func NewFormatDetector() *FormatDetector {
	return &FormatDetector{
		patterns: map[ToolFormat]*regexp.Regexp{
			// Anthropic: Look for "tool_use" type in content array
			FormatAnthropicJSON: regexp.MustCompile(`"type"\s*:\s*"tool_use"`),

			// OpenAI: Look for tool_calls array in message
			FormatOpenAIJSON: regexp.MustCompile(`"tool_calls"\s*:\s*\[`),

			// xAI: Look for XML tool_call tags with arguments (multiline)
			FormatXAIXML: regexp.MustCompile(`(?s)<tool_call>.*<arguments>.*</arguments>.*</tool_call>`),

			// DeepSeek: Look for XML tool_call tags with parameters (multiline)
			FormatDeepSeekXML: regexp.MustCompile(`(?s)<tool_call>.*<parameters>.*</parameters>.*</tool_call>`),

			// Google: Look for functionCall in parts
			FormatGoogleJSON: regexp.MustCompile(`"functionCall"\s*:\s*\{`),
		},
	}
}

// DetectFormat analyzes a response and returns the detected format
func (fd *FormatDetector) DetectFormat(response string) (ToolFormat, error) {
	// Check each pattern
	for format, pattern := range fd.patterns {
		if pattern.MatchString(response) {
			return format, nil
		}
	}

	return "", fmt.Errorf("unable to detect tool calling format in response")
}

// DetectParser analyzes a response and returns the appropriate parser
func (fd *FormatDetector) DetectParser(response string) (ToolParser, error) {
	format, err := fd.DetectFormat(response)
	if err != nil {
		return nil, err
	}

	parser, err := GetParserByFormat(format)
	if err != nil {
		return nil, fmt.Errorf("no parser found for detected format %s: %w", format, err)
	}

	return parser, nil
}

// DetectAndParse combines detection and parsing in one step
func (fd *FormatDetector) DetectAndParse(response string) ([]ToolCall, ToolFormat, error) {
	parser, err := fd.DetectParser(response)
	if err != nil {
		return nil, "", err
	}

	toolCalls, err := parser.Parse(response)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse response: %w", err)
	}

	return toolCalls, parser.Format(), nil
}

// IsToolResponse checks if a response contains tool calls
func (fd *FormatDetector) IsToolResponse(response string) bool {
	_, err := fd.DetectFormat(response)
	return err == nil
}

// DetectProvider tries to identify the provider from the response format
func (fd *FormatDetector) DetectProvider(response string) (string, error) {
	parser, err := fd.DetectParser(response)
	if err != nil {
		return "", err
	}

	return parser.ProviderID(), nil
}
