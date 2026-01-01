// Package proxy provides HTTP proxy functionality for routing LLM API requests.
package proxy

import (
	"encoding/json"
	"fmt"
	"time"
)

// ====== Anthropic Request/Response Types ======

// AnthropicRequest represents an Anthropic Messages API request.
type AnthropicRequest struct {
	Model         string             `json:"model"`
	Messages      []AnthropicMessage `json:"messages"`
	MaxTokens     int                `json:"max_tokens"`
	System        string             `json:"system,omitempty"`
	Temperature   *float64           `json:"temperature,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	TopK          *int               `json:"top_k,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
	Tools         []AnthropicTool    `json:"tools,omitempty"`
	ToolChoice    *ToolChoice        `json:"tool_choice,omitempty"`
	Metadata      map[string]string  `json:"metadata,omitempty"`
}

// AnthropicMessage represents a message in the Anthropic format.
type AnthropicMessage struct {
	Role    string        `json:"role"`
	Content []ContentPart `json:"content"`
}

// ContentPart represents a content block in Anthropic messages.
type ContentPart struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text,omitempty"`
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
	Content   string                 `json:"content,omitempty"`
	Source    *ImageSource           `json:"source,omitempty"`
}

// ImageSource represents image data for vision requests.
type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// AnthropicTool represents a tool definition in Anthropic format.
type AnthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// ToolChoice specifies tool selection behavior.
type ToolChoice struct {
	Type string `json:"type"` // auto, any, tool
	Name string `json:"name,omitempty"`
}

// AnthropicResponse represents an Anthropic Messages API response.
type AnthropicResponse struct {
	ID           string        `json:"id"`
	Type         string        `json:"type"`
	Role         string        `json:"role"`
	Content      []ContentPart `json:"content"`
	Model        string        `json:"model"`
	StopReason   string        `json:"stop_reason,omitempty"`
	StopSequence string        `json:"stop_sequence,omitempty"`
	Usage        *Usage        `json:"usage,omitempty"`
}

// Usage tracks token usage for billing.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ====== OpenAI Request/Response Types ======

// OpenAIRequest represents an OpenAI Chat Completions API request.
type OpenAIRequest struct {
	Model               string          `json:"model"`
	Messages            []OpenAIMessage `json:"messages"`
	MaxTokens           *int            `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int            `json:"max_completion_tokens,omitempty"`
	Temperature         *float64        `json:"temperature,omitempty"`
	TopP                *float64        `json:"top_p,omitempty"`
	N                   *int            `json:"n,omitempty"`
	Stop                []string        `json:"stop,omitempty"`
	Stream              bool            `json:"stream,omitempty"`
	StreamOptions       *StreamOptions  `json:"stream_options,omitempty"`
	Tools               []OpenAITool    `json:"tools,omitempty"`
	ToolChoice          interface{}     `json:"tool_choice,omitempty"`
	FrequencyPenalty    *float64        `json:"frequency_penalty,omitempty"`
	PresencePenalty     *float64        `json:"presence_penalty,omitempty"`
	User                string          `json:"user,omitempty"`
}

// StreamOptions configures streaming behavior.
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// OpenAIMessage represents a message in the OpenAI format.
type OpenAIMessage struct {
	Role       string           `json:"role"`
	Content    interface{}      `json:"content"` // string or []ContentBlock
	Name       string           `json:"name,omitempty"`
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

// OpenAIToolCall represents a tool call in OpenAI format.
type OpenAIToolCall struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function OpenAIFunction `json:"function"`
}

// OpenAIFunction represents a function call.
type OpenAIFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// OpenAITool represents a tool definition in OpenAI format.
type OpenAITool struct {
	Type     string            `json:"type"`
	Function OpenAIFunctionDef `json:"function"`
}

// OpenAIFunctionDef is the function definition within a tool.
type OpenAIFunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// OpenAIResponse represents an OpenAI Chat Completions API response.
type OpenAIResponse struct {
	ID                string         `json:"id"`
	Object            string         `json:"object"`
	Created           int64          `json:"created"`
	Model             string         `json:"model"`
	Choices           []OpenAIChoice `json:"choices"`
	Usage             *OpenAIUsage   `json:"usage,omitempty"`
	SystemFingerprint string         `json:"system_fingerprint,omitempty"`
}

// OpenAIChoice represents a completion choice.
type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason,omitempty"`
}

// OpenAIUsage tracks token usage.
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ====== Streaming Chunk Types ======

// AnthropicStreamEvent represents a streaming event from Anthropic.
type AnthropicStreamEvent struct {
	Type         string             `json:"type"`
	Message      *AnthropicResponse `json:"message,omitempty"`
	Index        int                `json:"index,omitempty"`
	ContentBlock *ContentPart       `json:"content_block,omitempty"`
	Delta        *StreamDelta       `json:"delta,omitempty"`
	Usage        *Usage             `json:"usage,omitempty"`
}

// StreamDelta represents incremental content in a stream.
type StreamDelta struct {
	Type         string `json:"type,omitempty"`
	Text         string `json:"text,omitempty"`
	PartialJSON  string `json:"partial_json,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

// OpenAIStreamChunk represents a streaming chunk from OpenAI.
type OpenAIStreamChunk struct {
	ID                string               `json:"id"`
	Object            string               `json:"object"`
	Created           int64                `json:"created"`
	Model             string               `json:"model"`
	Choices           []OpenAIStreamChoice `json:"choices"`
	Usage             *OpenAIUsage         `json:"usage,omitempty"`
	SystemFingerprint string               `json:"system_fingerprint,omitempty"`
}

// OpenAIStreamChoice represents a choice in a stream chunk.
type OpenAIStreamChoice struct {
	Index        int               `json:"index"`
	Delta        OpenAIStreamDelta `json:"delta"`
	FinishReason *string           `json:"finish_reason,omitempty"`
}

// OpenAIStreamDelta represents the delta content in streaming.
type OpenAIStreamDelta struct {
	Role      string                `json:"role,omitempty"`
	Content   string                `json:"content,omitempty"`
	ToolCalls []OpenAIToolCallDelta `json:"tool_calls,omitempty"`
}

// OpenAIToolCallDelta represents a partial tool call in streaming.
type OpenAIToolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function,omitempty"`
}

// ====== Translation Functions ======

// ToOpenAI converts an Anthropic request to OpenAI format.
func ToOpenAI(req *AnthropicRequest) (*OpenAIRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("nil anthropic request")
	}

	openaiReq := &OpenAIRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stop:        req.StopSequences,
		Stream:      req.Stream,
	}

	// Convert max_tokens
	if req.MaxTokens > 0 {
		openaiReq.MaxTokens = &req.MaxTokens
	}

	// Convert messages
	messages := make([]OpenAIMessage, 0, len(req.Messages)+1)

	// Add system message if present
	if req.System != "" {
		messages = append(messages, OpenAIMessage{
			Role:    "system",
			Content: req.System,
		})
	}

	// Convert each Anthropic message
	for _, msg := range req.Messages {
		openaiMsg, err := convertAnthropicMessageToOpenAI(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to convert message: %w", err)
		}
		messages = append(messages, openaiMsg...)
	}

	openaiReq.Messages = messages

	// Convert tools
	if len(req.Tools) > 0 {
		openaiReq.Tools = make([]OpenAITool, len(req.Tools))
		for i, tool := range req.Tools {
			openaiReq.Tools[i] = OpenAITool{
				Type: "function",
				Function: OpenAIFunctionDef{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.InputSchema,
				},
			}
		}
	}

	// Convert tool_choice
	if req.ToolChoice != nil {
		openaiReq.ToolChoice = convertToolChoiceToOpenAI(req.ToolChoice)
	}

	return openaiReq, nil
}

// convertAnthropicMessageToOpenAI converts a single Anthropic message to OpenAI format.
// May return multiple messages (e.g., assistant with tool_use followed by tool results).
func convertAnthropicMessageToOpenAI(msg AnthropicMessage) ([]OpenAIMessage, error) {
	var result []OpenAIMessage

	switch msg.Role {
	case "user":
		openaiMsg := OpenAIMessage{Role: "user"}
		content, hasToolResult := convertContentToOpenAI(msg.Content)

		if hasToolResult {
			// Tool results become separate "tool" role messages in OpenAI
			for _, part := range msg.Content {
				if part.Type == "tool_result" {
					result = append(result, OpenAIMessage{
						Role:       "tool",
						Content:    part.Content,
						ToolCallID: part.ToolUseID,
					})
				}
			}
			// If there's also text content, add as user message
			if content != "" {
				openaiMsg.Content = content
				result = append(result, openaiMsg)
			}
		} else {
			openaiMsg.Content = content
			result = append(result, openaiMsg)
		}

	case "assistant":
		openaiMsg := OpenAIMessage{Role: "assistant"}

		// Extract text and tool_use blocks
		var textParts []string
		var toolCalls []OpenAIToolCall

		for _, part := range msg.Content {
			switch part.Type {
			case "text":
				textParts = append(textParts, part.Text)
			case "tool_use":
				argsJSON, err := json.Marshal(part.Input)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal tool input: %w", err)
				}
				toolCalls = append(toolCalls, OpenAIToolCall{
					ID:   part.ID,
					Type: "function",
					Function: OpenAIFunction{
						Name:      part.Name,
						Arguments: string(argsJSON),
					},
				})
			}
		}

		if len(textParts) > 0 {
			openaiMsg.Content = textParts[0]
			for i := 1; i < len(textParts); i++ {
				openaiMsg.Content = openaiMsg.Content.(string) + "\n" + textParts[i]
			}
		}

		if len(toolCalls) > 0 {
			openaiMsg.ToolCalls = toolCalls
		}

		result = append(result, openaiMsg)

	default:
		result = append(result, OpenAIMessage{
			Role:    msg.Role,
			Content: convertContentToString(msg.Content),
		})
	}

	return result, nil
}

// convertContentToOpenAI extracts text content and indicates if tool_result is present.
func convertContentToOpenAI(content []ContentPart) (string, bool) {
	var texts []string
	hasToolResult := false

	for _, part := range content {
		switch part.Type {
		case "text":
			texts = append(texts, part.Text)
		case "tool_result":
			hasToolResult = true
		}
	}

	return joinStrings(texts, "\n"), hasToolResult
}

// convertContentToString converts content parts to a string.
func convertContentToString(content []ContentPart) string {
	var texts []string
	for _, part := range content {
		if part.Type == "text" {
			texts = append(texts, part.Text)
		}
	}
	return joinStrings(texts, "\n")
}

// convertToolChoiceToOpenAI converts Anthropic tool_choice to OpenAI format.
func convertToolChoiceToOpenAI(tc *ToolChoice) interface{} {
	if tc == nil {
		return nil
	}

	switch tc.Type {
	case "auto":
		return "auto"
	case "any":
		return "required"
	case "tool":
		return map[string]interface{}{
			"type": "function",
			"function": map[string]string{
				"name": tc.Name,
			},
		}
	default:
		return "auto"
	}
}

// ToAnthropic converts an OpenAI request to Anthropic format.
func ToAnthropic(req *OpenAIRequest) (*AnthropicRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("nil openai request")
	}

	anthropicReq := &AnthropicRequest{
		Model:         req.Model,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		StopSequences: req.Stop,
		Stream:        req.Stream,
	}

	// Convert max_tokens (prefer max_completion_tokens if set)
	if req.MaxCompletionTokens != nil {
		anthropicReq.MaxTokens = *req.MaxCompletionTokens
	} else if req.MaxTokens != nil {
		anthropicReq.MaxTokens = *req.MaxTokens
	} else {
		// Anthropic requires max_tokens, default to reasonable value
		anthropicReq.MaxTokens = 4096
	}

	// Convert messages
	messages := make([]AnthropicMessage, 0, len(req.Messages))

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			// System messages become the system parameter
			content, ok := msg.Content.(string)
			if ok {
				if anthropicReq.System != "" {
					anthropicReq.System += "\n\n" + content
				} else {
					anthropicReq.System = content
				}
			}
			continue
		}

		anthropicMsg, err := convertOpenAIMessageToAnthropic(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to convert message: %w", err)
		}
		if anthropicMsg != nil {
			messages = append(messages, *anthropicMsg)
		}
	}

	anthropicReq.Messages = messages

	// Convert tools
	if len(req.Tools) > 0 {
		anthropicReq.Tools = make([]AnthropicTool, len(req.Tools))
		for i, tool := range req.Tools {
			anthropicReq.Tools[i] = AnthropicTool{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				InputSchema: tool.Function.Parameters,
			}
		}
	}

	// Convert tool_choice
	anthropicReq.ToolChoice = convertToolChoiceToAnthropic(req.ToolChoice)

	return anthropicReq, nil
}

// convertOpenAIMessageToAnthropic converts a single OpenAI message to Anthropic format.
func convertOpenAIMessageToAnthropic(msg OpenAIMessage) (*AnthropicMessage, error) {
	result := &AnthropicMessage{
		Role:    msg.Role,
		Content: []ContentPart{},
	}

	// Handle tool role (convert to tool_result)
	if msg.Role == "tool" {
		contentStr, _ := getStringContent(msg.Content)
		result.Role = "user"
		result.Content = []ContentPart{
			{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   contentStr,
			},
		}
		return result, nil
	}

	// Convert content
	contentStr, ok := getStringContent(msg.Content)
	if ok && contentStr != "" {
		result.Content = append(result.Content, ContentPart{
			Type: "text",
			Text: contentStr,
		})
	}

	// Convert tool_calls to tool_use content blocks
	for _, tc := range msg.ToolCalls {
		var input map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
			return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
		}

		result.Content = append(result.Content, ContentPart{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}

	// Don't return empty messages
	if len(result.Content) == 0 {
		return nil, nil
	}

	return result, nil
}

// getStringContent extracts string content from interface{}.
func getStringContent(content interface{}) (string, bool) {
	if content == nil {
		return "", false
	}
	if s, ok := content.(string); ok {
		return s, true
	}
	return "", false
}

// convertToolChoiceToAnthropic converts OpenAI tool_choice to Anthropic format.
func convertToolChoiceToAnthropic(tc interface{}) *ToolChoice {
	if tc == nil {
		return nil
	}

	switch v := tc.(type) {
	case string:
		switch v {
		case "auto":
			return &ToolChoice{Type: "auto"}
		case "required":
			return &ToolChoice{Type: "any"}
		case "none":
			return nil
		}
	case map[string]interface{}:
		if fn, ok := v["function"].(map[string]interface{}); ok {
			if name, ok := fn["name"].(string); ok {
				return &ToolChoice{Type: "tool", Name: name}
			}
		}
	}

	return nil
}

// ====== Response Translation ======

// TranslateResponseToOpenAI converts an Anthropic response to OpenAI format.
func TranslateResponseToOpenAI(resp *AnthropicResponse) *OpenAIResponse {
	if resp == nil {
		return nil
	}

	openaiResp := &OpenAIResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   resp.Model,
		Choices: []OpenAIChoice{
			{
				Index: 0,
				Message: OpenAIMessage{
					Role: "assistant",
				},
				FinishReason: mapStopReasonToOpenAI(resp.StopReason),
			},
		},
	}

	// Convert content blocks
	var textParts []string
	var toolCalls []OpenAIToolCall

	for _, part := range resp.Content {
		switch part.Type {
		case "text":
			textParts = append(textParts, part.Text)
		case "tool_use":
			argsJSON, err := json.Marshal(part.Input)
			if err != nil {
				// Log error but continue with empty args rather than failing entire translation
				argsJSON = []byte("{}")
			}
			toolCalls = append(toolCalls, OpenAIToolCall{
				ID:   part.ID,
				Type: "function",
				Function: OpenAIFunction{
					Name:      part.Name,
					Arguments: string(argsJSON),
				},
			})
		}
	}

	if len(textParts) > 0 {
		openaiResp.Choices[0].Message.Content = joinStrings(textParts, "\n")
	}
	if len(toolCalls) > 0 {
		openaiResp.Choices[0].Message.ToolCalls = toolCalls
	}

	// Convert usage
	if resp.Usage != nil {
		openaiResp.Usage = &OpenAIUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}

	return openaiResp
}

// TranslateResponseToAnthropic converts an OpenAI response to Anthropic format.
func TranslateResponseToAnthropic(resp *OpenAIResponse) *AnthropicResponse {
	if resp == nil || len(resp.Choices) == 0 {
		return nil
	}

	choice := resp.Choices[0]
	anthropicResp := &AnthropicResponse{
		ID:         resp.ID,
		Type:       "message",
		Role:       "assistant",
		Model:      resp.Model,
		StopReason: mapFinishReasonToAnthropic(choice.FinishReason),
		Content:    []ContentPart{},
	}

	// Convert message content
	if content, ok := choice.Message.Content.(string); ok && content != "" {
		anthropicResp.Content = append(anthropicResp.Content, ContentPart{
			Type: "text",
			Text: content,
		})
	}

	// Convert tool_calls to tool_use
	for _, tc := range choice.Message.ToolCalls {
		var input map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
			// If arguments are invalid, use empty object rather than failing entire translation
			input = make(map[string]interface{})
		}

		anthropicResp.Content = append(anthropicResp.Content, ContentPart{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}

	// Convert usage
	if resp.Usage != nil {
		anthropicResp.Usage = &Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		}
	}

	return anthropicResp
}

// ====== Streaming Chunk Translation ======

// TranslateStreamChunkToOpenAI converts an Anthropic stream event to OpenAI chunk.
func TranslateStreamChunkToOpenAI(event *AnthropicStreamEvent, id string) *OpenAIStreamChunk {
	if event == nil {
		return nil
	}

	chunk := &OpenAIStreamChunk{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Choices: []OpenAIStreamChoice{
			{Index: 0},
		},
	}

	switch event.Type {
	case "message_start":
		if event.Message != nil {
			chunk.Model = event.Message.Model
			chunk.Choices[0].Delta = OpenAIStreamDelta{Role: "assistant"}
		}

	case "content_block_start":
		if event.ContentBlock != nil {
			if event.ContentBlock.Type == "text" {
				// No content yet for text start
			} else if event.ContentBlock.Type == "tool_use" {
				argsJSON, _ := json.Marshal(event.ContentBlock.Input)
				chunk.Choices[0].Delta = OpenAIStreamDelta{
					ToolCalls: []OpenAIToolCallDelta{
						{
							Index: event.Index,
							ID:    event.ContentBlock.ID,
							Type:  "function",
							Function: struct {
								Name      string `json:"name,omitempty"`
								Arguments string `json:"arguments,omitempty"`
							}{
								Name:      event.ContentBlock.Name,
								Arguments: string(argsJSON),
							},
						},
					},
				}
			}
		}

	case "content_block_delta":
		if event.Delta != nil {
			if event.Delta.Type == "text_delta" {
				chunk.Choices[0].Delta = OpenAIStreamDelta{
					Content: event.Delta.Text,
				}
			} else if event.Delta.Type == "input_json_delta" {
				chunk.Choices[0].Delta = OpenAIStreamDelta{
					ToolCalls: []OpenAIToolCallDelta{
						{
							Index: event.Index,
							Function: struct {
								Name      string `json:"name,omitempty"`
								Arguments string `json:"arguments,omitempty"`
							}{
								Arguments: event.Delta.PartialJSON,
							},
						},
					},
				}
			}
		}

	case "message_delta":
		if event.Delta != nil && event.Delta.StopReason != "" {
			reason := mapStopReasonToOpenAI(event.Delta.StopReason)
			chunk.Choices[0].FinishReason = &reason
		}
		if event.Usage != nil {
			chunk.Usage = &OpenAIUsage{
				PromptTokens:     event.Usage.InputTokens,
				CompletionTokens: event.Usage.OutputTokens,
				TotalTokens:      event.Usage.InputTokens + event.Usage.OutputTokens,
			}
		}

	case "message_stop":
		// End of stream marker - return done
		return nil
	}

	return chunk
}

// TranslateStreamChunkToAnthropic converts an OpenAI stream chunk to Anthropic event.
func TranslateStreamChunkToAnthropic(chunk *OpenAIStreamChunk, eventIndex *int) []AnthropicStreamEvent {
	if chunk == nil || len(chunk.Choices) == 0 {
		return nil
	}

	var events []AnthropicStreamEvent
	choice := chunk.Choices[0]

	// Handle role (first chunk)
	if choice.Delta.Role == "assistant" && *eventIndex == 0 {
		events = append(events, AnthropicStreamEvent{
			Type: "message_start",
			Message: &AnthropicResponse{
				ID:    chunk.ID,
				Type:  "message",
				Role:  "assistant",
				Model: chunk.Model,
			},
		})
		*eventIndex++
	}

	// Handle text content
	if choice.Delta.Content != "" {
		events = append(events, AnthropicStreamEvent{
			Type: "content_block_delta",
			Delta: &StreamDelta{
				Type: "text_delta",
				Text: choice.Delta.Content,
			},
		})
	}

	// Handle tool calls
	for _, tc := range choice.Delta.ToolCalls {
		if tc.ID != "" {
			// New tool call
			events = append(events, AnthropicStreamEvent{
				Type:  "content_block_start",
				Index: tc.Index,
				ContentBlock: &ContentPart{
					Type: "tool_use",
					ID:   tc.ID,
					Name: tc.Function.Name,
				},
			})
		}
		if tc.Function.Arguments != "" {
			events = append(events, AnthropicStreamEvent{
				Type:  "content_block_delta",
				Index: tc.Index,
				Delta: &StreamDelta{
					Type:        "input_json_delta",
					PartialJSON: tc.Function.Arguments,
				},
			})
		}
	}

	// Handle finish reason
	if choice.FinishReason != nil && *choice.FinishReason != "" {
		events = append(events, AnthropicStreamEvent{
			Type: "message_delta",
			Delta: &StreamDelta{
				StopReason: mapFinishReasonToAnthropic(*choice.FinishReason),
			},
		})
	}

	// Handle usage
	if chunk.Usage != nil {
		events = append(events, AnthropicStreamEvent{
			Type: "message_delta",
			Usage: &Usage{
				InputTokens:  chunk.Usage.PromptTokens,
				OutputTokens: chunk.Usage.CompletionTokens,
			},
		})
	}

	return events
}

// ====== Helper Functions ======

// mapStopReasonToOpenAI converts Anthropic stop_reason to OpenAI finish_reason.
func mapStopReasonToOpenAI(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	case "tool_use":
		return "tool_calls"
	default:
		return "stop"
	}
}

// mapFinishReasonToAnthropic converts OpenAI finish_reason to Anthropic stop_reason.
func mapFinishReasonToAnthropic(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "content_filter":
		return "end_turn"
	default:
		return "end_turn"
	}
}

// joinStrings joins strings with a separator.
func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}

// ====== JSON Marshaling Helpers ======

// MarshalAnthropicRequest serializes an Anthropic request to JSON.
func MarshalAnthropicRequest(req *AnthropicRequest) ([]byte, error) {
	return json.Marshal(req)
}

// UnmarshalAnthropicRequest deserializes JSON to an Anthropic request.
func UnmarshalAnthropicRequest(data []byte) (*AnthropicRequest, error) {
	var req AnthropicRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// MarshalOpenAIRequest serializes an OpenAI request to JSON.
func MarshalOpenAIRequest(req *OpenAIRequest) ([]byte, error) {
	return json.Marshal(req)
}

// UnmarshalOpenAIRequest deserializes JSON to an OpenAI request.
func UnmarshalOpenAIRequest(data []byte) (*OpenAIRequest, error) {
	var req OpenAIRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// MarshalAnthropicResponse serializes an Anthropic response to JSON.
func MarshalAnthropicResponse(resp *AnthropicResponse) ([]byte, error) {
	return json.Marshal(resp)
}

// MarshalOpenAIResponse serializes an OpenAI response to JSON.
func MarshalOpenAIResponse(resp *OpenAIResponse) ([]byte, error) {
	return json.Marshal(resp)
}
