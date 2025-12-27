package providers

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestProviderRegistration(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		wantOk   bool
	}{
		{"Anthropic registered", "anthropic", true},
		{"OpenAI registered", "openai", true},
		{"Google registered", "google", true},
		{"Mistral registered", "mistral", true},
		{"Unknown provider", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := GetProviderFactory(tt.provider)
			if ok != tt.wantOk {
				t.Errorf("GetProviderFactory(%q) = %v, want %v", tt.provider, ok, tt.wantOk)
			}
		})
	}
}

func TestProviderCreation(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		apiKey   string
	}{
		{"Create Anthropic", "anthropic", "test-key"},
		{"Create OpenAI", "openai", "test-key"},
		{"Create Google", "google", "test-key"},
		{"Create Mistral", "mistral", "test-key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory, ok := GetProviderFactory(tt.provider)
			if !ok {
				t.Fatalf("Provider %q not registered", tt.provider)
			}

			provider := factory(tt.apiKey)
			if provider == nil {
				t.Errorf("Factory returned nil provider for %q", tt.provider)
			}
		})
	}
}

func TestListProviders(t *testing.T) {
	providers := ListProviders()

	if len(providers) == 0 {
		t.Error("ListProviders() returned empty list")
	}

	// Check that expected providers are in the list
	expectedProviders := []string{"anthropic", "openai", "google", "mistral"}
	for _, expected := range expectedProviders {
		found := false
		for _, p := range providers {
			if p == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected provider %q not found in list", expected)
		}
	}
}

func TestProviderInterfaces(t *testing.T) {
	tests := []struct {
		name     string
		provider string
	}{
		{"Anthropic implements Provider", "anthropic"},
		{"OpenAI implements Provider", "openai"},
		{"Google implements Provider", "google"},
		{"Mistral implements Provider", "mistral"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory, ok := GetProviderFactory(tt.provider)
			if !ok {
				t.Fatalf("Provider %q not registered", tt.provider)
			}

			provider := factory("test-key")

			// Test that all interface methods exist
			ctx := context.Background()

			// GetCapabilities should not panic
			caps := provider.GetCapabilities()
			if caps.MaxTokensPerRequest == 0 {
				t.Errorf("GetCapabilities() returned zero MaxTokensPerRequest")
			}

			// GetEndpoints should return at least one endpoint
			endpoints := provider.GetEndpoints()
			if len(endpoints) == 0 {
				t.Errorf("GetEndpoints() returned no endpoints")
			}

			// ValidateEndpoints should not panic (but may fail due to invalid API key)
			_ = provider.ValidateEndpoints(ctx, false)

			// ListModels should not panic (but may fail due to invalid API key)
			_, _ = provider.ListModels(ctx, false)
		})
	}
}

func TestEndpointStructure(t *testing.T) {
	providers := []string{"anthropic", "openai", "google", "mistral"}

	for _, provName := range providers {
		t.Run(provName, func(t *testing.T) {
			factory, ok := GetProviderFactory(provName)
			if !ok {
				t.Fatalf("Provider %q not registered", provName)
			}

			provider := factory("test-key")
			endpoints := provider.GetEndpoints()

			for i, endpoint := range endpoints {
				if endpoint.Path == "" {
					t.Errorf("Endpoint %d has empty Path", i)
				}
				if endpoint.Method == "" {
					t.Errorf("Endpoint %d has empty Method", i)
				}
				if endpoint.Method != "GET" && endpoint.Method != "POST" && endpoint.Method != "PUT" && endpoint.Method != "DELETE" {
					t.Errorf("Endpoint %d has invalid Method: %q", i, endpoint.Method)
				}
			}
		})
	}
}

func TestCapabilitiesStructure(t *testing.T) {
	providers := []string{"anthropic", "openai", "google", "mistral"}

	for _, provName := range providers {
		t.Run(provName, func(t *testing.T) {
			factory, ok := GetProviderFactory(provName)
			if !ok {
				t.Fatalf("Provider %q not registered", provName)
			}

			provider := factory("test-key")
			caps := provider.GetCapabilities()

			// All providers should support chat
			if !caps.SupportsChat {
				t.Errorf("Provider %q should support chat", provName)
			}

			// Should have reasonable rate limits
			if caps.MaxRequestsPerMinute <= 0 {
				t.Errorf("Provider %q has invalid MaxRequestsPerMinute: %d", provName, caps.MaxRequestsPerMinute)
			}

			if caps.MaxTokensPerRequest <= 0 {
				t.Errorf("Provider %q has invalid MaxTokensPerRequest: %d", provName, caps.MaxTokensPerRequest)
			}
		})
	}
}

func TestModelStructure(t *testing.T) {
	model := Model{
		ID:             "test-model",
		Name:           "Test Model",
		Description:    "A test model",
		CostPer1MIn:    1.0,
		CostPer1MOut:   2.0,
		ContextWindow:  8192,
		MaxTokens:      4096,
		SupportsImages: true,
		SupportsTools:  true,
		CanReason:      false,
		CanStream:      true,
		Categories:     []string{"chat", "test"},
		Capabilities:   map[string]string{"test": "value"},
	}

	if model.ID == "" {
		t.Error("Model ID should not be empty")
	}
	if model.Name == "" {
		t.Error("Model Name should not be empty")
	}
	if model.CostPer1MIn < 0 {
		t.Error("CostPer1MIn should not be negative")
	}
	if model.ContextWindow <= 0 {
		t.Error("ContextWindow should be positive")
	}
}

func TestEndpointStatus(t *testing.T) {
	tests := []struct {
		status EndpointStatus
		want   string
	}{
		{StatusUnknown, "unknown"},
		{StatusWorking, "working"},
		{StatusFailed, "failed"},
		{StatusDeprecated, "deprecated"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if string(tt.status) != tt.want {
				t.Errorf("EndpointStatus = %q, want %q", tt.status, tt.want)
			}
		})
	}
}

func TestOpenAIProvider_IsUsableModel(t *testing.T) {
	provider := NewOpenAIProvider("test-key").(*OpenAIProvider)

	tests := []struct {
		name     string
		modelID  string
		expected bool
	}{
		{"GPT-4", "gpt-4", true},
		{"GPT-4 Turbo", "gpt-4-turbo", true},
		{"GPT-3.5", "gpt-3.5-turbo", true},
		{"Embedding model", "text-embedding-ada-002", false},
		{"Whisper", "whisper-1", false},
		{"TTS", "tts-1", false},
		{"DALL-E", "dall-e-3", false},
		{"Moderation", "text-moderation-latest", false},
		{"Old davinci", "text-davinci-003", false},
		{"Old curie", "text-curie-001", false},
		{"Old babbage", "text-babbage-001", false},
		{"Old ada", "text-ada-001", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.isUsableModel(tt.modelID)
			if result != tt.expected {
				t.Errorf("isUsableModel(%q) = %v, want %v", tt.modelID, result, tt.expected)
			}
		})
	}
}

func TestOpenAIProvider_FormatModelName(t *testing.T) {
	provider := NewOpenAIProvider("test-key").(*OpenAIProvider)

	tests := []struct {
		name     string
		modelID  string
		contains string
	}{
		{"GPT-4 Omni", "gpt-4o", "GPT-4 Omni"},
		{"GPT-4 Turbo", "gpt-4-turbo", "GPT-4 Turbo"},
		{"GPT-4", "gpt-4", "GPT-4"},
		{"GPT-3.5", "gpt-3.5-turbo", "GPT-3.5 Turbo"},
		{"O1", "o1-preview", "O-Series Reasoning"},
		{"O3", "o3-mini", "O-Series Reasoning"},
		{"Unknown", "unknown-model", "Unknown Model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.formatModelName(tt.modelID)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("formatModelName(%q) = %q, want to contain %q", tt.modelID, result, tt.contains)
			}
		})
	}
}

func TestOpenAIProvider_EnrichModelDetails(t *testing.T) {
	provider := NewOpenAIProvider("test-key").(*OpenAIProvider)

	tests := []struct {
		name          string
		modelID       string
		checkSupports bool
		checkContext  bool
		expectVision  bool
		expectReason  bool
	}{
		{"GPT-4", "gpt-4", true, true, false, true},
		{"GPT-4 Turbo", "gpt-4-turbo-preview", true, true, true, true},
		{"GPT-3.5", "gpt-3.5-turbo", true, true, false, false},
		{"GPT-4o", "gpt-4o", true, true, true, true},
		{"GPT-4o-mini", "gpt-4o-mini", true, true, true, false},
		{"O1", "o1-2024-12-17", true, true, false, true},
		{"O1-mini", "o1-mini", true, true, false, true},
		{"O3", "o3-mini", true, true, true, true},
		{"Unknown", "unknown-model", true, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := Model{
				ID: tt.modelID,
			}

			enriched := provider.enrichModelDetails(model)

			if tt.checkSupports {
				if !enriched.SupportsTools {
					t.Error("Expected SupportsTools to be true")
				}
				if !enriched.CanStream {
					t.Error("Expected CanStream to be true")
				}
			}

			if tt.checkContext && enriched.ContextWindow == 0 {
				t.Error("Expected ContextWindow to be set")
			}

			// Check vision and reasoning
			if enriched.SupportsImages != tt.expectVision {
				t.Errorf("Expected SupportsImages=%v, got %v", tt.expectVision, enriched.SupportsImages)
			}
			if enriched.CanReason != tt.expectReason {
				t.Errorf("Expected CanReason=%v, got %v", tt.expectReason, enriched.CanReason)
			}
		})
	}
}

func TestAnthropicProvider_EnrichModelDetails(t *testing.T) {
	provider := NewAnthropicProvider("test-key").(*AnthropicProvider)

	tests := []struct {
		name          string
		modelID       string
		expectContext bool
	}{
		{"Claude 3 Opus", "claude-3-opus-20240229", true},
		{"Claude 3 Sonnet", "claude-3-sonnet-20240229", true},
		{"Claude 3.5 Sonnet", "claude-3-5-sonnet-20240620", true},
		{"Claude 2", "claude-2", true},
		{"Unknown", "unknown-model", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := Model{
				ID: tt.modelID,
			}

			enriched := provider.enrichModelDetails(model)

			if tt.expectContext && enriched.ContextWindow == 0 {
				t.Errorf("Expected ContextWindow to be set for %s", tt.modelID)
			}
			if !enriched.CanStream {
				t.Error("Expected CanStream to be true")
			}
			if !enriched.SupportsTools {
				t.Error("Expected SupportsTools to be true")
			}
		})
	}
}

func TestProvider_GetEndpoints(t *testing.T) {
	providers := map[string]Provider{
		"openai":    NewOpenAIProvider("test-key"),
		"anthropic": NewAnthropicProvider("test-key"),
		"google":    NewGoogleProvider("test-key"),
		"mistral":   NewMistralProvider("test-key"),
	}

	for name, provider := range providers {
		t.Run(name, func(t *testing.T) {
			endpoints := provider.GetEndpoints()
			if len(endpoints) == 0 {
				t.Errorf("%s: expected endpoints, got none", name)
			}

			// Verify endpoint structure
			for _, ep := range endpoints {
				if ep.Path == "" {
					t.Errorf("%s: endpoint has empty path", name)
				}
				if ep.Method == "" {
					t.Errorf("%s: endpoint has empty method", name)
				}
			}
		})
	}
}

func TestProvider_GetCapabilities(t *testing.T) {
	providers := map[string]Provider{
		"openai":    NewOpenAIProvider("test-key"),
		"anthropic": NewAnthropicProvider("test-key"),
		"google":    NewGoogleProvider("test-key"),
		"mistral":   NewMistralProvider("test-key"),
	}

	for name, provider := range providers {
		t.Run(name, func(t *testing.T) {
			caps := provider.GetCapabilities()

			// All providers should support at least chat
			if !caps.SupportsChat {
				t.Errorf("%s: should support chat", name)
			}

			// All providers should support streaming
			if !caps.SupportsStreaming {
				t.Errorf("%s: should support streaming", name)
			}
		})
	}
}

func TestOpenAIProvider_SpecificEndpoints(t *testing.T) {
	provider := NewOpenAIProvider("test-key")
	endpoints := provider.GetEndpoints()

	// Look for specific OpenAI endpoints
	hasChatCompletions := false
	hasModels := false

	for _, ep := range endpoints {
		if strings.Contains(ep.Path, "chat/completions") {
			hasChatCompletions = true
			if ep.Method != "POST" {
				t.Errorf("chat/completions should be POST, got %s", ep.Method)
			}
		}
		if strings.Contains(ep.Path, "models") {
			hasModels = true
			if ep.Method != "GET" {
				t.Errorf("models should be GET, got %s", ep.Method)
			}
		}
	}

	if !hasChatCompletions {
		t.Error("Missing chat/completions endpoint")
	}
	if !hasModels {
		t.Error("Missing models endpoint")
	}
}

func TestProviderCapabilities_Fields(t *testing.T) {
	// Test that we can access capability fields
	caps := ProviderCapabilities{
		SupportsChat:       true,
		SupportsStreaming:  true,
		SupportsVision:     true,
		SupportsJSONMode:   true,
		SupportsEmbeddings: true,
	}

	if !caps.SupportsChat {
		t.Error("Expected SupportsChat to be true")
	}
	if !caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be true")
	}
	if !caps.SupportsVision {
		t.Error("Expected SupportsVision to be true")
	}
}

func TestProvider_ValidateEndpoints_Structure(t *testing.T) {
	providers := map[string]Provider{
		"openai":    NewOpenAIProvider("test-key"),
		"anthropic": NewAnthropicProvider("test-key"),
		"google":    NewGoogleProvider("test-key"),
		"mistral":   NewMistralProvider("test-key"),
	}

	for name, provider := range providers {
		t.Run(name, func(t *testing.T) {
			// Test ValidateEndpoints with cancelled context
			// This will exercise the function without making real HTTP calls
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel immediately

			err := provider.ValidateEndpoints(ctx, false)
			// We expect either an error or success (depending on implementation)
			// The important thing is the function doesn't panic
			_ = err // Function executed without panic
		})
	}
}

func TestProvider_ListModels_WithInvalidKey(t *testing.T) {
	// Skip this test in CI/CD environments
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	providers := map[string]Provider{
		"openai":    NewOpenAIProvider("invalid-key-test"),
		"anthropic": NewAnthropicProvider("invalid-key-test"),
		"google":    NewGoogleProvider("invalid-key-test"),
		"mistral":   NewMistralProvider("invalid-key-test"),
	}

	for name, provider := range providers {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			models, err := provider.ListModels(ctx, false)
			// With invalid key, we expect an error
			// But the function should not panic
			if err == nil && len(models) == 0 {
				t.Logf("%s: No models returned (expected with invalid key)", name)
			}
			// Function executed without panic
		})
	}
}

func TestModel_Structure(t *testing.T) {
	// Test that Model struct can be created and accessed
	model := Model{
		ID:             "test-model",
		Name:           "Test Model",
		Description:    "A test model",
		CostPer1MIn:    0.001,
		CostPer1MOut:   0.002,
		ContextWindow:  4096,
		MaxTokens:      2048,
		SupportsImages: true,
		SupportsTools:  true,
		CanReason:      false,
		CanStream:      true,
		Categories:     []string{"chat", "text"},
	}

	if model.ID != "test-model" {
		t.Error("Model ID not set correctly")
	}
	if model.ContextWindow != 4096 {
		t.Error("Model ContextWindow not set correctly")
	}
	if !model.SupportsTools {
		t.Error("Model SupportsTools not set correctly")
	}
	if len(model.Categories) != 2 {
		t.Error("Model Categories not set correctly")
	}
}

func TestEndpoint_Structure(t *testing.T) {
	// Test Endpoint struct
	endpoint := Endpoint{
		Path:        "/v1/chat/completions",
		Method:      "POST",
		Description: "Chat completions endpoint",
		Status:      StatusWorking,
		Latency:     100 * time.Millisecond,
	}

	if endpoint.Path != "/v1/chat/completions" {
		t.Error("Endpoint Path not set correctly")
	}
	if endpoint.Method != "POST" {
		t.Error("Endpoint Method not set correctly")
	}
	if endpoint.Status != StatusWorking {
		t.Error("Endpoint Status not set correctly")
	}
}

func TestProviderMetadata_Complete(t *testing.T) {
	providers := map[string]Provider{
		"openai":    NewOpenAIProvider("test-key"),
		"anthropic": NewAnthropicProvider("test-key"),
		"google":    NewGoogleProvider("test-key"),
		"mistral":   NewMistralProvider("test-key"),
	}

	for name, provider := range providers {
		t.Run(name, func(t *testing.T) {
			// Test GetEndpoints
			endpoints := provider.GetEndpoints()
			if len(endpoints) == 0 {
				t.Errorf("%s: No endpoints defined", name)
			}

			// Test GetCapabilities
			caps := provider.GetCapabilities()

			// Verify capabilities struct has some true fields
			hasAnyCapability := caps.SupportsChat ||
				caps.SupportsStreaming ||
				caps.SupportsEmbeddings ||
				caps.SupportsVision ||
				caps.SupportsJSONMode ||
				caps.SupportsFIM ||
				caps.SupportsAgents

			if !hasAnyCapability {
				t.Errorf("%s: Provider has no capabilities enabled", name)
			}
		})
	}
}
