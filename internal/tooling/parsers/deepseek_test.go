package parsers

import (
	"testing"

	"github.com/jeffersonwarrior/modelscan/internal/tooling"
)

func TestDeepSeekParser_Parse(t *testing.T) {
	parser := &DeepSeekParser{}

	// Example DeepSeek XML response (uses <parameters> instead of <arguments>)
	response := `Here is my response with a tool call:
<tool_call>
<id>call_ds_456</id>
<name>get_weather</name>
<parameters>{"location":"Beijing","unit":"celsius"}</parameters>
</tool_call>
That's what I need.`

	toolCalls, err := parser.Parse(response)
	if err != nil {
		t.Fatalf("Failed to parse DeepSeek response: %v", err)
	}

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	tc := toolCalls[0]
	if tc.ID != "call_ds_456" {
		t.Errorf("Expected ID call_ds_456, got %s", tc.ID)
	}
	if tc.Name != "get_weather" {
		t.Errorf("Expected Name get_weather, got %s", tc.Name)
	}
	if tc.Type != "function" {
		t.Errorf("Expected Type function, got %s", tc.Type)
	}

	// Check parameters
	location, ok := tc.Args["location"].(string)
	if !ok || location != "Beijing" {
		t.Errorf("Expected location 'Beijing', got %v", tc.Args["location"])
	}

	unit, ok := tc.Args["unit"].(string)
	if !ok || unit != "celsius" {
		t.Errorf("Expected unit 'celsius', got %v", tc.Args["unit"])
	}
}

func TestDeepSeekParser_ParseMultiple(t *testing.T) {
	parser := &DeepSeekParser{}

	// Multiple tool calls
	response := `I'll use these tools:
<tool_call>
<id>call_1</id>
<name>search</name>
<parameters>{"query":"AI models"}</parameters>
</tool_call>
And also:
<tool_call>
<id>call_2</id>
<name>calculate</name>
<parameters>{"expression":"2+2"}</parameters>
</tool_call>
Done.`

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
	if toolCalls[0].ID != "call_1" {
		t.Errorf("Expected first ID call_1, got %s", toolCalls[0].ID)
	}

	// Verify second tool call
	if toolCalls[1].Name != "calculate" {
		t.Errorf("Expected second tool name calculate, got %s", toolCalls[1].Name)
	}
	if toolCalls[1].ID != "call_2" {
		t.Errorf("Expected second ID call_2, got %s", toolCalls[1].ID)
	}
}

func TestDeepSeekParser_ParseEmpty(t *testing.T) {
	parser := &DeepSeekParser{}

	// Response with no tool calls
	response := `Here is my text response without any tool calls.`

	toolCalls, err := parser.Parse(response)
	if err != nil {
		t.Fatalf("Failed to parse text-only response: %v", err)
	}

	if len(toolCalls) != 0 {
		t.Errorf("Expected 0 tool calls for text response, got %d", len(toolCalls))
	}
}

func TestDeepSeekParser_ParseCompact(t *testing.T) {
	parser := &DeepSeekParser{}

	// Compact XML without whitespace
	response := `<tool_call><id>compact_id</id><name>test_func</name><parameters>{"key":"value"}</parameters></tool_call>`

	toolCalls, err := parser.Parse(response)
	if err != nil {
		t.Fatalf("Failed to parse compact XML: %v", err)
	}

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].Name != "test_func" {
		t.Errorf("Expected name test_func, got %s", toolCalls[0].Name)
	}
}

func TestDeepSeekParser_ParseInvalidJSON(t *testing.T) {
	parser := &DeepSeekParser{}

	// Tool call with invalid JSON parameters
	response := `<tool_call>
<id>call_1</id>
<name>test</name>
<parameters>not valid json</parameters>
</tool_call>`

	_, err := parser.Parse(response)
	if err == nil {
		t.Error("Expected error for invalid JSON parameters")
	}
}

func TestDeepSeekParser_ParseMalformed(t *testing.T) {
	parser := &DeepSeekParser{}

	// Malformed XML (missing closing tag)
	response := `<tool_call>
<id>call_1</id>
<name>test</name>
<parameters>{"key":"value"}</parameters>`

	toolCalls, err := parser.Parse(response)
	if err != nil {
		t.Fatalf("Should not error on malformed XML, just return empty: %v", err)
	}

	if len(toolCalls) != 0 {
		t.Errorf("Expected 0 tool calls for malformed XML, got %d", len(toolCalls))
	}
}

func TestDeepSeekParser_Format(t *testing.T) {
	parser := &DeepSeekParser{}

	if parser.Format() != tooling.FormatDeepSeekXML {
		t.Errorf("Expected format %s, got %s", tooling.FormatDeepSeekXML, parser.Format())
	}
}

func TestDeepSeekParser_ProviderID(t *testing.T) {
	parser := &DeepSeekParser{}

	if parser.ProviderID() != "deepseek" {
		t.Errorf("Expected provider ID deepseek, got %s", parser.ProviderID())
	}
}

func TestDeepSeekParser_Capabilities(t *testing.T) {
	parser := &DeepSeekParser{}

	caps := parser.Capabilities()

	if !caps.SupportsToolCalling {
		t.Error("Expected SupportsToolCalling to be true")
	}
	if !caps.SupportsParallel {
		t.Error("Expected SupportsParallel to be true")
	}
	if caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be false for XML format")
	}
	if caps.Format != tooling.FormatDeepSeekXML {
		t.Errorf("Expected format %s, got %s", tooling.FormatDeepSeekXML, caps.Format)
	}
}

func TestDeepSeekParser_Registration(t *testing.T) {
	// Parser should auto-register via init()
	parser, err := tooling.GetParser("deepseek")
	if err != nil {
		t.Fatalf("Failed to get registered DeepSeek parser: %v", err)
	}

	if parser.ProviderID() != "deepseek" {
		t.Errorf("Expected deepseek parser, got %s", parser.ProviderID())
	}
}
