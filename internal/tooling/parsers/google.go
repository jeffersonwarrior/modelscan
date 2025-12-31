package parsers

import (
	"encoding/json"
	"fmt"

	"github.com/jeffersonwarrior/modelscan/internal/tooling"
)

// GoogleParser handles Google Gemini's JSON tool calling format.
type GoogleParser struct{}

// googleFunctionCall represents Google's function_call structure
type googleFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// googleCandidate represents a response candidate
type googleCandidate struct {
	Content struct {
		Parts []struct {
			FunctionCall *googleFunctionCall `json:"functionCall,omitempty"`
		} `json:"parts"`
	} `json:"content"`
}

// googleResponse represents the structure of Google Gemini API responses
type googleResponse struct {
	Candidates []googleCandidate `json:"candidates"`
}

// Parse extracts tool calls from Google Gemini's JSON response format.
func (p *GoogleParser) Parse(response string) ([]tooling.ToolCall, error) {
	var resp googleResponse
	if err := json.Unmarshal([]byte(response), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse Google response: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return []tooling.ToolCall{}, nil
	}

	var toolCalls []tooling.ToolCall
	callIndex := 0

	for _, candidate := range resp.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.FunctionCall != nil {
				// Generate ID since Google doesn't provide one
				id := fmt.Sprintf("google_call_%d", callIndex)
				callIndex++

				toolCall := tooling.ToolCall{
					ID:       id,
					Name:     part.FunctionCall.Name,
					Args:     part.FunctionCall.Args,
					Type:     "function",
					RawInput: response,
				}
				toolCalls = append(toolCalls, toolCall)
			}
		}
	}

	return toolCalls, nil
}

// Format returns the format this parser handles.
func (p *GoogleParser) Format() tooling.ToolFormat {
	return tooling.FormatGoogleJSON
}

// ProviderID returns the provider identifier.
func (p *GoogleParser) ProviderID() string {
	return "google"
}

// Capabilities returns the tool calling capabilities for Google Gemini.
func (p *GoogleParser) Capabilities() tooling.ProviderCapabilities {
	return tooling.ProviderCapabilities{
		SupportsToolCalling: true,
		SupportsParallel:    true,
		SupportsStreaming:   true,
		Format:              tooling.FormatGoogleJSON,
	}
}

func init() {
	tooling.RegisterParser("google", &GoogleParser{})
}
