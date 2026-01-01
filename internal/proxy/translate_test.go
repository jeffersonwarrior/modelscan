package proxy

import (
	"encoding/json"
	"testing"
)

func TestToOpenAI_BasicRequest(t *testing.T) {
	temp := 0.7
	req := &AnthropicRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		System:    "You are a helpful assistant.",
		Messages: []AnthropicMessage{
			{
				Role: "user",
				Content: []ContentPart{
					{Type: "text", Text: "Hello, how are you?"},
				},
			},
		},
		Temperature: &temp,
		Stream:      false,
	}

	openaiReq, err := ToOpenAI(req)
	if err != nil {
		t.Fatalf("ToOpenAI failed: %v", err)
	}

	if openaiReq.Model != req.Model {
		t.Errorf("Model mismatch: got %s, want %s", openaiReq.Model, req.Model)
	}

	if *openaiReq.MaxTokens != req.MaxTokens {
		t.Errorf("MaxTokens mismatch: got %d, want %d", *openaiReq.MaxTokens, req.MaxTokens)
	}

	// Should have system + user messages
	if len(openaiReq.Messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(openaiReq.Messages))
	}

	if openaiReq.Messages[0].Role != "system" {
		t.Errorf("First message should be system, got %s", openaiReq.Messages[0].Role)
	}

	if openaiReq.Messages[0].Content != "You are a helpful assistant." {
		t.Errorf("System content mismatch")
	}

	if openaiReq.Messages[1].Role != "user" {
		t.Errorf("Second message should be user, got %s", openaiReq.Messages[1].Role)
	}
}

func TestToOpenAI_WithTools(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "claude-3-sonnet",
		MaxTokens: 512,
		Messages: []AnthropicMessage{
			{
				Role: "user",
				Content: []ContentPart{
					{Type: "text", Text: "What's the weather?"},
				},
			},
		},
		Tools: []AnthropicTool{
			{
				Name:        "get_weather",
				Description: "Get the current weather",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "City name",
						},
					},
					"required": []interface{}{"location"},
				},
			},
		},
	}

	openaiReq, err := ToOpenAI(req)
	if err != nil {
		t.Fatalf("ToOpenAI failed: %v", err)
	}

	if len(openaiReq.Tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(openaiReq.Tools))
	}

	tool := openaiReq.Tools[0]
	if tool.Type != "function" {
		t.Errorf("Tool type should be function, got %s", tool.Type)
	}

	if tool.Function.Name != "get_weather" {
		t.Errorf("Tool name mismatch: got %s", tool.Function.Name)
	}
}

func TestToOpenAI_ToolUseMessage(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "claude-3-opus",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{
				Role: "user",
				Content: []ContentPart{
					{Type: "text", Text: "Get weather for NYC"},
				},
			},
			{
				Role: "assistant",
				Content: []ContentPart{
					{Type: "text", Text: "I'll check the weather."},
					{
						Type:  "tool_use",
						ID:    "call_123",
						Name:  "get_weather",
						Input: map[string]interface{}{"location": "NYC"},
					},
				},
			},
		},
	}

	openaiReq, err := ToOpenAI(req)
	if err != nil {
		t.Fatalf("ToOpenAI failed: %v", err)
	}

	// User message + assistant message
	if len(openaiReq.Messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(openaiReq.Messages))
	}

	assistantMsg := openaiReq.Messages[1]
	if len(assistantMsg.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(assistantMsg.ToolCalls))
	}

	tc := assistantMsg.ToolCalls[0]
	if tc.ID != "call_123" {
		t.Errorf("Tool call ID mismatch: got %s", tc.ID)
	}
	if tc.Function.Name != "get_weather" {
		t.Errorf("Tool call name mismatch: got %s", tc.Function.Name)
	}
}

func TestToAnthropic_BasicRequest(t *testing.T) {
	maxTokens := 2048
	req := &OpenAIRequest{
		Model:     "gpt-4",
		MaxTokens: &maxTokens,
		Messages: []OpenAIMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello!"},
		},
	}

	anthropicReq, err := ToAnthropic(req)
	if err != nil {
		t.Fatalf("ToAnthropic failed: %v", err)
	}

	if anthropicReq.Model != "gpt-4" {
		t.Errorf("Model mismatch")
	}

	if anthropicReq.MaxTokens != 2048 {
		t.Errorf("MaxTokens mismatch: got %d", anthropicReq.MaxTokens)
	}

	if anthropicReq.System != "You are a helpful assistant." {
		t.Errorf("System mismatch")
	}

	if len(anthropicReq.Messages) != 1 {
		t.Fatalf("Expected 1 message (no system), got %d", len(anthropicReq.Messages))
	}

	if anthropicReq.Messages[0].Role != "user" {
		t.Errorf("First message should be user")
	}
}

func TestToAnthropic_WithToolCalls(t *testing.T) {
	req := &OpenAIRequest{
		Model: "gpt-4",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "What's the weather?"},
			{
				Role:    "assistant",
				Content: "Let me check.",
				ToolCalls: []OpenAIToolCall{
					{
						ID:   "call_456",
						Type: "function",
						Function: OpenAIFunction{
							Name:      "get_weather",
							Arguments: `{"location": "Paris"}`,
						},
					},
				},
			},
		},
	}

	anthropicReq, err := ToAnthropic(req)
	if err != nil {
		t.Fatalf("ToAnthropic failed: %v", err)
	}

	if len(anthropicReq.Messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(anthropicReq.Messages))
	}

	assistantMsg := anthropicReq.Messages[1]
	if len(assistantMsg.Content) != 2 {
		t.Fatalf("Expected 2 content blocks (text + tool_use), got %d", len(assistantMsg.Content))
	}

	// Check text block
	if assistantMsg.Content[0].Type != "text" {
		t.Errorf("First block should be text")
	}

	// Check tool_use block
	toolUse := assistantMsg.Content[1]
	if toolUse.Type != "tool_use" {
		t.Errorf("Second block should be tool_use, got %s", toolUse.Type)
	}
	if toolUse.ID != "call_456" {
		t.Errorf("Tool use ID mismatch")
	}
	if toolUse.Name != "get_weather" {
		t.Errorf("Tool use name mismatch")
	}
}

func TestToAnthropic_ToolMessage(t *testing.T) {
	req := &OpenAIRequest{
		Model: "gpt-4",
		Messages: []OpenAIMessage{
			{
				Role:       "tool",
				Content:    "72°F and sunny",
				ToolCallID: "call_789",
			},
		},
	}

	anthropicReq, err := ToAnthropic(req)
	if err != nil {
		t.Fatalf("ToAnthropic failed: %v", err)
	}

	if len(anthropicReq.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(anthropicReq.Messages))
	}

	msg := anthropicReq.Messages[0]
	if msg.Role != "user" {
		t.Errorf("Tool result should be user role in Anthropic, got %s", msg.Role)
	}

	if len(msg.Content) != 1 {
		t.Fatalf("Expected 1 content block, got %d", len(msg.Content))
	}

	result := msg.Content[0]
	if result.Type != "tool_result" {
		t.Errorf("Should be tool_result, got %s", result.Type)
	}
	if result.ToolUseID != "call_789" {
		t.Errorf("ToolUseID mismatch")
	}
	if result.Content != "72°F and sunny" {
		t.Errorf("Content mismatch")
	}
}

func TestTranslateResponseToOpenAI(t *testing.T) {
	resp := &AnthropicResponse{
		ID:         "msg_123",
		Type:       "message",
		Role:       "assistant",
		Model:      "claude-3-opus",
		StopReason: "end_turn",
		Content: []ContentPart{
			{Type: "text", Text: "Hello! How can I help you?"},
		},
		Usage: &Usage{
			InputTokens:  10,
			OutputTokens: 8,
		},
	}

	openaiResp := TranslateResponseToOpenAI(resp)

	if openaiResp.ID != "msg_123" {
		t.Errorf("ID mismatch")
	}
	if openaiResp.Object != "chat.completion" {
		t.Errorf("Object should be chat.completion")
	}
	if len(openaiResp.Choices) != 1 {
		t.Fatalf("Expected 1 choice")
	}

	choice := openaiResp.Choices[0]
	if choice.FinishReason != "stop" {
		t.Errorf("FinishReason should be stop, got %s", choice.FinishReason)
	}

	if openaiResp.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens mismatch")
	}
	if openaiResp.Usage.CompletionTokens != 8 {
		t.Errorf("CompletionTokens mismatch")
	}
	if openaiResp.Usage.TotalTokens != 18 {
		t.Errorf("TotalTokens mismatch")
	}
}

func TestTranslateResponseToAnthropic(t *testing.T) {
	resp := &OpenAIResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "gpt-4",
		Choices: []OpenAIChoice{
			{
				Index: 0,
				Message: OpenAIMessage{
					Role:    "assistant",
					Content: "Hello there!",
				},
				FinishReason: "stop",
			},
		},
		Usage: &OpenAIUsage{
			PromptTokens:     15,
			CompletionTokens: 5,
			TotalTokens:      20,
		},
	}

	anthropicResp := TranslateResponseToAnthropic(resp)

	if anthropicResp.ID != "chatcmpl-123" {
		t.Errorf("ID mismatch")
	}
	if anthropicResp.Type != "message" {
		t.Errorf("Type should be message")
	}
	if anthropicResp.Role != "assistant" {
		t.Errorf("Role should be assistant")
	}
	if anthropicResp.StopReason != "end_turn" {
		t.Errorf("StopReason should be end_turn, got %s", anthropicResp.StopReason)
	}

	if len(anthropicResp.Content) != 1 {
		t.Fatalf("Expected 1 content block")
	}
	if anthropicResp.Content[0].Text != "Hello there!" {
		t.Errorf("Text content mismatch")
	}

	if anthropicResp.Usage.InputTokens != 15 {
		t.Errorf("InputTokens mismatch")
	}
	if anthropicResp.Usage.OutputTokens != 5 {
		t.Errorf("OutputTokens mismatch")
	}
}

func TestMapStopReasons(t *testing.T) {
	tests := []struct {
		anthropic string
		openai    string
	}{
		{"end_turn", "stop"},
		{"max_tokens", "length"},
		{"stop_sequence", "stop"},
		{"tool_use", "tool_calls"},
	}

	for _, tt := range tests {
		got := mapStopReasonToOpenAI(tt.anthropic)
		if got != tt.openai {
			t.Errorf("mapStopReasonToOpenAI(%s) = %s, want %s", tt.anthropic, got, tt.openai)
		}
	}
}

func TestMapFinishReasons(t *testing.T) {
	tests := []struct {
		openai    string
		anthropic string
	}{
		{"stop", "end_turn"},
		{"length", "max_tokens"},
		{"tool_calls", "tool_use"},
		{"content_filter", "end_turn"},
	}

	for _, tt := range tests {
		got := mapFinishReasonToAnthropic(tt.openai)
		if got != tt.anthropic {
			t.Errorf("mapFinishReasonToAnthropic(%s) = %s, want %s", tt.openai, got, tt.anthropic)
		}
	}
}

func TestToolChoiceConversions(t *testing.T) {
	// Anthropic auto -> OpenAI auto
	tc := &ToolChoice{Type: "auto"}
	result := convertToolChoiceToOpenAI(tc)
	if result != "auto" {
		t.Errorf("auto should map to 'auto', got %v", result)
	}

	// Anthropic any -> OpenAI required
	tc = &ToolChoice{Type: "any"}
	result = convertToolChoiceToOpenAI(tc)
	if result != "required" {
		t.Errorf("any should map to 'required', got %v", result)
	}

	// Anthropic specific tool
	tc = &ToolChoice{Type: "tool", Name: "my_tool"}
	result = convertToolChoiceToOpenAI(tc)
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map for specific tool")
	}
	fn := resultMap["function"].(map[string]string)
	if fn["name"] != "my_tool" {
		t.Errorf("Tool name mismatch")
	}
}

func TestMarshalUnmarshal(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "claude-3",
		MaxTokens: 100,
		Messages: []AnthropicMessage{
			{
				Role: "user",
				Content: []ContentPart{
					{Type: "text", Text: "Hello"},
				},
			},
		},
	}

	data, err := MarshalAnthropicRequest(req)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	parsed, err := UnmarshalAnthropicRequest(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if parsed.Model != req.Model {
		t.Errorf("Model mismatch after roundtrip")
	}
	if parsed.MaxTokens != req.MaxTokens {
		t.Errorf("MaxTokens mismatch after roundtrip")
	}
}

func TestNilHandling(t *testing.T) {
	// ToOpenAI with nil
	_, err := ToOpenAI(nil)
	if err == nil {
		t.Error("ToOpenAI(nil) should return error")
	}

	// ToAnthropic with nil
	_, err = ToAnthropic(nil)
	if err == nil {
		t.Error("ToAnthropic(nil) should return error")
	}

	// Response translations with nil
	if TranslateResponseToOpenAI(nil) != nil {
		t.Error("TranslateResponseToOpenAI(nil) should return nil")
	}

	if TranslateResponseToAnthropic(nil) != nil {
		t.Error("TranslateResponseToAnthropic(nil) should return nil")
	}
}

func TestStreamChunkTranslation(t *testing.T) {
	// Test text delta
	event := &AnthropicStreamEvent{
		Type: "content_block_delta",
		Delta: &StreamDelta{
			Type: "text_delta",
			Text: "Hello",
		},
	}

	chunk := TranslateStreamChunkToOpenAI(event, "chunk_123")
	if chunk == nil {
		t.Fatal("Expected non-nil chunk")
	}

	if chunk.ID != "chunk_123" {
		t.Errorf("ID mismatch")
	}

	if len(chunk.Choices) != 1 {
		t.Fatalf("Expected 1 choice")
	}

	if chunk.Choices[0].Delta.Content != "Hello" {
		t.Errorf("Content mismatch: got %s", chunk.Choices[0].Delta.Content)
	}
}

func TestStreamChunkToAnthropic(t *testing.T) {
	finish := "stop"
	chunk := &OpenAIStreamChunk{
		ID:      "chatcmpl-stream",
		Object:  "chat.completion.chunk",
		Created: 1234567890,
		Model:   "gpt-4",
		Choices: []OpenAIStreamChoice{
			{
				Index: 0,
				Delta: OpenAIStreamDelta{
					Content: "World",
				},
				FinishReason: &finish,
			},
		},
	}

	eventIndex := 1 // Not first chunk
	events := TranslateStreamChunkToAnthropic(chunk, &eventIndex)

	// Should have text delta and message_delta for finish
	foundText := false
	foundDelta := false

	for _, e := range events {
		if e.Type == "content_block_delta" && e.Delta != nil && e.Delta.Text == "World" {
			foundText = true
		}
		if e.Type == "message_delta" && e.Delta != nil && e.Delta.StopReason == "end_turn" {
			foundDelta = true
		}
	}

	if !foundText {
		t.Error("Expected text delta event")
	}
	if !foundDelta {
		t.Error("Expected message_delta event with stop reason")
	}
}

func TestJSONRoundtrip(t *testing.T) {
	// Test that we can marshal and unmarshal without losing data
	original := &AnthropicRequest{
		Model:     "claude-3-opus",
		MaxTokens: 4096,
		System:    "You are helpful.",
		Messages: []AnthropicMessage{
			{
				Role: "user",
				Content: []ContentPart{
					{Type: "text", Text: "Question"},
				},
			},
		},
		Tools: []AnthropicTool{
			{
				Name:        "calculator",
				Description: "Math operations",
				InputSchema: map[string]interface{}{
					"type": "object",
				},
			},
		},
		Stream: true,
	}

	// Convert to OpenAI
	openaiReq, err := ToOpenAI(original)
	if err != nil {
		t.Fatalf("ToOpenAI failed: %v", err)
	}

	// Convert back to Anthropic
	backToAnthropic, err := ToAnthropic(openaiReq)
	if err != nil {
		t.Fatalf("ToAnthropic failed: %v", err)
	}

	// Verify key fields
	if backToAnthropic.Model != original.Model {
		t.Errorf("Model lost in roundtrip")
	}
	if backToAnthropic.System != original.System {
		t.Errorf("System lost in roundtrip")
	}
	if backToAnthropic.Stream != original.Stream {
		t.Errorf("Stream flag lost in roundtrip")
	}
	if len(backToAnthropic.Tools) != len(original.Tools) {
		t.Errorf("Tools lost in roundtrip")
	}
}

func TestContentAsStringAndArray(t *testing.T) {
	// OpenAI allows content as string or array
	req := &OpenAIRequest{
		Model: "gpt-4",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "Simple string content"},
		},
	}

	anthropicReq, err := ToAnthropic(req)
	if err != nil {
		t.Fatalf("ToAnthropic failed: %v", err)
	}

	if len(anthropicReq.Messages) != 1 {
		t.Fatalf("Expected 1 message")
	}

	if len(anthropicReq.Messages[0].Content) != 1 {
		t.Fatalf("Expected 1 content part")
	}

	if anthropicReq.Messages[0].Content[0].Text != "Simple string content" {
		t.Errorf("Content mismatch: %s", anthropicReq.Messages[0].Content[0].Text)
	}
}

// Benchmark translations
func BenchmarkToOpenAI(b *testing.B) {
	req := &AnthropicRequest{
		Model:     "claude-3-opus",
		MaxTokens: 4096,
		Messages: []AnthropicMessage{
			{
				Role: "user",
				Content: []ContentPart{
					{Type: "text", Text: "Hello, how are you?"},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ToOpenAI(req)
	}
}

func BenchmarkToAnthropic(b *testing.B) {
	maxTokens := 4096
	req := &OpenAIRequest{
		Model:     "gpt-4",
		MaxTokens: &maxTokens,
		Messages: []OpenAIMessage{
			{Role: "user", Content: "Hello, how are you?"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ToAnthropic(req)
	}
}

func BenchmarkMarshalResponse(b *testing.B) {
	resp := &AnthropicResponse{
		ID:         "msg_123",
		Type:       "message",
		Role:       "assistant",
		Model:      "claude-3-opus",
		StopReason: "end_turn",
		Content: []ContentPart{
			{Type: "text", Text: "This is a test response with some content."},
		},
		Usage: &Usage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(resp)
	}
}
