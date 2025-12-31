package tooling_test

import (
	"testing"

	"github.com/jeffersonwarrior/modelscan/internal/tooling"
	_ "github.com/jeffersonwarrior/modelscan/internal/tooling/parsers" // Register parsers
)

func TestDetectFormat_Anthropic(t *testing.T) {
	detector := tooling.NewFormatDetector()

	response := `{
		"content": [
			{
				"type": "tool_use",
				"id": "toolu_123",
				"name": "get_weather",
				"input": {"location": "SF"}
			}
		]
	}`

	format, err := detector.DetectFormat(response)
	if err != nil {
		t.Fatalf("Failed to detect Anthropic format: %v", err)
	}

	if format != tooling.FormatAnthropicJSON {
		t.Errorf("Expected %s, got %s", tooling.FormatAnthropicJSON, format)
	}
}

func TestDetectFormat_OpenAI(t *testing.T) {
	detector := tooling.NewFormatDetector()

	response := `{
		"choices": [
			{
				"message": {
					"role": "assistant",
					"tool_calls": [
						{
							"id": "call_123",
							"type": "function",
							"function": {
								"name": "get_weather",
								"arguments": "{\"location\":\"NYC\"}"
							}
						}
					]
				}
			}
		]
	}`

	format, err := detector.DetectFormat(response)
	if err != nil {
		t.Fatalf("Failed to detect OpenAI format: %v", err)
	}

	if format != tooling.FormatOpenAIJSON {
		t.Errorf("Expected %s, got %s", tooling.FormatOpenAIJSON, format)
	}
}

func TestDetectFormat_XAI(t *testing.T) {
	detector := tooling.NewFormatDetector()

	response := `Here is my response:
<tool_call>
<id>call_xai_123</id>
<name>get_weather</name>
<arguments>{"location":"Boston"}</arguments>
</tool_call>
Done.`

	format, err := detector.DetectFormat(response)
	if err != nil {
		t.Fatalf("Failed to detect xAI format: %v", err)
	}

	if format != tooling.FormatXAIXML {
		t.Errorf("Expected %s, got %s", tooling.FormatXAIXML, format)
	}
}

func TestDetectFormat_DeepSeek(t *testing.T) {
	detector := tooling.NewFormatDetector()

	response := `Let me use this tool:
<tool_call>
<id>call_ds_456</id>
<name>calculate</name>
<parameters>{"expression":"2+2"}</parameters>
</tool_call>
Result incoming.`

	format, err := detector.DetectFormat(response)
	if err != nil {
		t.Fatalf("Failed to detect DeepSeek format: %v", err)
	}

	if format != tooling.FormatDeepSeekXML {
		t.Errorf("Expected %s, got %s", tooling.FormatDeepSeekXML, format)
	}
}

func TestDetectFormat_Google(t *testing.T) {
	detector := tooling.NewFormatDetector()

	response := `{
		"candidates": [
			{
				"content": {
					"parts": [
						{
							"functionCall": {
								"name": "get_weather",
								"args": {"location": "Seattle"}
							}
						}
					]
				}
			}
		]
	}`

	format, err := detector.DetectFormat(response)
	if err != nil {
		t.Fatalf("Failed to detect Google format: %v", err)
	}

	if format != tooling.FormatGoogleJSON {
		t.Errorf("Expected %s, got %s", tooling.FormatGoogleJSON, format)
	}
}

func TestDetectFormat_NoToolCalls(t *testing.T) {
	detector := tooling.NewFormatDetector()

	response := `Just a regular text response without any tool calls.`

	_, err := detector.DetectFormat(response)
	if err == nil {
		t.Error("Expected error for response without tool calls")
	}
}

func TestDetectParser_Anthropic(t *testing.T) {
	detector := tooling.NewFormatDetector()

	response := `{
		"content": [
			{
				"type": "tool_use",
				"id": "toolu_123",
				"name": "test",
				"input": {}
			}
		]
	}`

	parser, err := detector.DetectParser(response)
	if err != nil {
		t.Fatalf("Failed to detect parser: %v", err)
	}

	if parser.ProviderID() != "anthropic" {
		t.Errorf("Expected anthropic parser, got %s", parser.ProviderID())
	}
}

func TestDetectParser_OpenAI(t *testing.T) {
	detector := tooling.NewFormatDetector()

	response := `{
		"choices": [
			{
				"message": {
					"tool_calls": [
						{
							"id": "call_1",
							"type": "function",
							"function": {
								"name": "test",
								"arguments": "{}"
							}
						}
					]
				}
			}
		]
	}`

	parser, err := detector.DetectParser(response)
	if err != nil {
		t.Fatalf("Failed to detect parser: %v", err)
	}

	if parser.ProviderID() != "openai" {
		t.Errorf("Expected openai parser, got %s", parser.ProviderID())
	}
}

func TestDetectAndParse_Anthropic(t *testing.T) {
	detector := tooling.NewFormatDetector()

	response := `{
		"content": [
			{
				"type": "tool_use",
				"id": "toolu_789",
				"name": "get_time",
				"input": {"timezone": "PST"}
			}
		]
	}`

	toolCalls, format, err := detector.DetectAndParse(response)
	if err != nil {
		t.Fatalf("Failed to detect and parse: %v", err)
	}

	if format != tooling.FormatAnthropicJSON {
		t.Errorf("Expected format %s, got %s", tooling.FormatAnthropicJSON, format)
	}

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].Name != "get_time" {
		t.Errorf("Expected name get_time, got %s", toolCalls[0].Name)
	}
}

func TestDetectAndParse_OpenAI(t *testing.T) {
	detector := tooling.NewFormatDetector()

	response := `{
		"choices": [
			{
				"message": {
					"tool_calls": [
						{
							"id": "call_456",
							"type": "function",
							"function": {
								"name": "search",
								"arguments": "{\"query\":\"AI\"}"
							}
						}
					]
				}
			}
		]
	}`

	toolCalls, format, err := detector.DetectAndParse(response)
	if err != nil {
		t.Fatalf("Failed to detect and parse: %v", err)
	}

	if format != tooling.FormatOpenAIJSON {
		t.Errorf("Expected format %s, got %s", tooling.FormatOpenAIJSON, format)
	}

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].Name != "search" {
		t.Errorf("Expected name search, got %s", toolCalls[0].Name)
	}
}

func TestIsToolResponse_True(t *testing.T) {
	detector := tooling.NewFormatDetector()

	response := `{
		"content": [
			{
				"type": "tool_use",
				"id": "toolu_123",
				"name": "test",
				"input": {}
			}
		]
	}`

	if !detector.IsToolResponse(response) {
		t.Error("Expected IsToolResponse to return true for tool response")
	}
}

func TestIsToolResponse_False(t *testing.T) {
	detector := tooling.NewFormatDetector()

	response := `Just regular text without tool calls.`

	if detector.IsToolResponse(response) {
		t.Error("Expected IsToolResponse to return false for text response")
	}
}

func TestDetectProvider_Anthropic(t *testing.T) {
	detector := tooling.NewFormatDetector()

	response := `{
		"content": [
			{
				"type": "tool_use",
				"id": "toolu_123",
				"name": "test",
				"input": {}
			}
		]
	}`

	provider, err := detector.DetectProvider(response)
	if err != nil {
		t.Fatalf("Failed to detect provider: %v", err)
	}

	if provider != "anthropic" {
		t.Errorf("Expected provider anthropic, got %s", provider)
	}
}

func TestDetectProvider_OpenAI(t *testing.T) {
	detector := tooling.NewFormatDetector()

	response := `{
		"choices": [
			{
				"message": {
					"tool_calls": [
						{
							"id": "call_1",
							"type": "function",
							"function": {
								"name": "test",
								"arguments": "{}"
							}
						}
					]
				}
			}
		]
	}`

	provider, err := detector.DetectProvider(response)
	if err != nil {
		t.Fatalf("Failed to detect provider: %v", err)
	}

	if provider != "openai" {
		t.Errorf("Expected provider openai, got %s", provider)
	}
}

func TestDetectProvider_NoMatch(t *testing.T) {
	detector := tooling.NewFormatDetector()

	response := `No tool calls here.`

	_, err := detector.DetectProvider(response)
	if err == nil {
		t.Error("Expected error when no provider detected")
	}
}
