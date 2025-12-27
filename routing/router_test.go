package routing

import (
	"context"
	"testing"
)

func TestRoutingMode(t *testing.T) {
	tests := []struct {
		name string
		mode RoutingMode
		want string
	}{
		{"Direct", ModeDirect, "direct"},
		{"Proxy", ModeProxy, "plano_proxy"},
		{"Embedded", ModeEmbedded, "plano_embedded"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.mode) != tt.want {
				t.Errorf("RoutingMode = %v, want %v", tt.mode, tt.want)
			}
		})
	}
}

func TestMessage(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hello, world!",
	}

	if msg.Role != "user" {
		t.Errorf("Role = %v, want user", msg.Role)
	}

	if msg.Content != "Hello, world!" {
		t.Errorf("Content = %v, want Hello, world!", msg.Content)
	}
}

func TestRequest(t *testing.T) {
	req := Request{
		Model:    "gpt-4o",
		Provider: "openai",
		Messages: []Message{
			{Role: "user", Content: "Test"},
		},
		Temperature: 0.7,
		MaxTokens:   100,
	}

	if req.Model != "gpt-4o" {
		t.Errorf("Model = %v, want gpt-4o", req.Model)
	}

	if req.Provider != "openai" {
		t.Errorf("Provider = %v, want openai", req.Provider)
	}

	if len(req.Messages) != 1 {
		t.Errorf("Messages length = %v, want 1", len(req.Messages))
	}

	if req.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want 0.7", req.Temperature)
	}

	if req.MaxTokens != 100 {
		t.Errorf("MaxTokens = %v, want 100", req.MaxTokens)
	}
}

func TestResponse(t *testing.T) {
	resp := Response{
		Model:    "gpt-4o",
		Content:  "Hello!",
		Provider: "openai",
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
		FinishReason: "stop",
	}

	if resp.Model != "gpt-4o" {
		t.Errorf("Model = %v, want gpt-4o", resp.Model)
	}

	if resp.Content != "Hello!" {
		t.Errorf("Content = %v, want Hello!", resp.Content)
	}

	if resp.Provider != "openai" {
		t.Errorf("Provider = %v, want openai", resp.Provider)
	}

	if resp.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %v, want 15", resp.Usage.TotalTokens)
	}

	if resp.FinishReason != "stop" {
		t.Errorf("FinishReason = %v, want stop", resp.FinishReason)
	}
}

// MockClient implements the Client interface for testing
type MockClient struct {
	response *Response
	err      error
}

func (m *MockClient) ChatCompletion(ctx context.Context, req Request) (*Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *MockClient) Close() error {
	return nil
}
