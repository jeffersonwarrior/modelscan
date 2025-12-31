package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// LLMClient interface for provider synthesis
type LLMClient interface {
	Synthesize(ctx context.Context, prompt string) (string, error)
	Name() string
}

// ClaudeClient implements LLMClient using Claude API
type ClaudeClient struct {
	apiKey  string
	model   string
	baseURL string
}

// NewClaudeClient creates a new Claude client
func NewClaudeClient() *ClaudeClient {
	return &ClaudeClient{
		apiKey:  os.Getenv("ANTHROPIC_API_KEY"),
		model:   "claude-sonnet-4-5",
		baseURL: "https://api.anthropic.com/v1/messages",
	}
}

// Name returns the client name
func (c *ClaudeClient) Name() string {
	return "claude-sonnet-4-5"
}

// Synthesize calls Claude API to synthesize provider information
func (c *ClaudeClient) Synthesize(ctx context.Context, prompt string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	reqBody := map[string]interface{}{
		"model":      c.model,
		"max_tokens": 4096,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(apiResp.Content) == 0 {
		return "", fmt.Errorf("empty response from Claude")
	}

	return apiResp.Content[0].Text, nil
}

// GPT4Client implements LLMClient using OpenAI GPT-4o API
type GPT4Client struct {
	apiKey  string
	model   string
	baseURL string
}

// NewGPT4Client creates a new GPT-4o client
func NewGPT4Client() *GPT4Client {
	return &GPT4Client{
		apiKey:  os.Getenv("OPENAI_API_KEY"),
		model:   "gpt-4o",
		baseURL: "https://api.openai.com/v1/chat/completions",
	}
}

// Name returns the client name
func (g *GPT4Client) Name() string {
	return "gpt-4o"
}

// Synthesize calls GPT-4o API to synthesize provider information
func (g *GPT4Client) Synthesize(ctx context.Context, prompt string) (string, error) {
	if g.apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY not set")
	}

	reqBody := map[string]interface{}{
		"model": g.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.0,
		"max_tokens":  4096,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", g.baseURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from GPT-4o")
	}

	return apiResp.Choices[0].Message.Content, nil
}

// LLMSynthesizer handles LLM calls with automatic fallback
type LLMSynthesizer struct {
	primary   LLMClient
	fallback  LLMClient
	retryable bool
}

// NewLLMSynthesizer creates a new synthesizer with Claude primary and GPT-4o fallback
func NewLLMSynthesizer() *LLMSynthesizer {
	return &LLMSynthesizer{
		primary:   NewClaudeClient(),
		fallback:  NewGPT4Client(),
		retryable: true,
	}
}

// Synthesize attempts synthesis with primary LLM, falls back to secondary on failure
func (s *LLMSynthesizer) Synthesize(ctx context.Context, sources []SourceResult) (string, error) {
	prompt := buildSynthesisPrompt(sources)

	// Try primary (Claude)
	response, err := s.primary.Synthesize(ctx, prompt)
	if err == nil {
		return response, nil
	}

	// Primary failed, check if we can fallback
	if !s.retryable {
		return "", fmt.Errorf("primary LLM (%s) failed: %w", s.primary.Name(), err)
	}

	// Try fallback (GPT-4o)
	response, fallbackErr := s.fallback.Synthesize(ctx, prompt)
	if fallbackErr == nil {
		return response, nil
	}

	// Both failed
	return "", fmt.Errorf("both LLMs failed - primary (%s): %w, fallback (%s): %v",
		s.primary.Name(), err, s.fallback.Name(), fallbackErr)
}

// buildSynthesisPrompt creates a prompt from source results
func buildSynthesisPrompt(sources []SourceResult) string {
	var sb strings.Builder
	sb.WriteString("You are an AI provider discovery agent. Analyze the following information about an AI provider and extract structured data.\n\n")
	sb.WriteString("Source Data:\n")
	for i, src := range sources {
		sb.WriteString(fmt.Sprintf("\n--- Source %d: %s ---\n", i+1, src.SourceName))
		sb.WriteString(fmt.Sprintf("Provider ID: %s\n", src.ProviderID))
		sb.WriteString(fmt.Sprintf("Provider Name: %s\n", src.ProviderName))
		sb.WriteString(fmt.Sprintf("Base URL: %s\n", src.BaseURL))
		sb.WriteString(fmt.Sprintf("Documentation: %s\n", src.DocumentationURL))
		if len(src.RawData) > 0 {
			if rawJSON, err := json.Marshal(src.RawData); err == nil {
				sb.WriteString(fmt.Sprintf("Additional Data: %s\n", string(rawJSON)))
			}
		}
	}

	sb.WriteString("\n\nExtract and return ONLY a JSON object with this structure (no markdown, no explanation):\n")
	sb.WriteString(`{
  "provider": {
    "id": "unique-provider-id",
    "name": "Provider Name",
    "base_url": "https://api.example.com/v1",
    "auth_method": "bearer",
    "auth_header": "Authorization",
    "pricing_model": "pay-per-token",
    "documentation": "https://docs.example.com"
  },
  "sdk_type": "openai-compatible"
}`)

	return sb.String()
}
