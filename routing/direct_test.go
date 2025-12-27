package routing

import (
	"context"
	"errors"
	"testing"
)

func TestNewDirectRouter(t *testing.T) {
	tests := []struct {
		name    string
		config  *DirectConfig
		wantErr bool
	}{
		{
			name:    "nil config uses defaults",
			config:  nil,
			wantErr: false,
		},
		{
			name: "custom config",
			config: &DirectConfig{
				DefaultProvider: "anthropic",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, err := NewDirectRouter(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDirectRouter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && router == nil {
				t.Error("NewDirectRouter() returned nil router")
			}
		})
	}
}

func TestDirectRouter_RegisterClient(t *testing.T) {
	router, err := NewDirectRouter(nil)
	if err != nil {
		t.Fatalf("NewDirectRouter() error = %v", err)
	}

	mockClient := &MockClient{
		response: &Response{
			Model:    "test-model",
			Content:  "test response",
			Provider: "test",
		},
	}

	router.RegisterClient("test", mockClient)

	if len(router.clients) != 1 {
		t.Errorf("Expected 1 client, got %d", len(router.clients))
	}

	if router.clients["test"] != mockClient {
		t.Error("Client not registered correctly")
	}
}

func TestDirectRouter_Route(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		mockResp *Response
		mockErr  error
		wantErr  bool
	}{
		{
			name:     "successful route",
			provider: "openai",
			mockResp: &Response{
				Model:    "gpt-4o",
				Content:  "Hello!",
				Provider: "openai",
			},
			mockErr: nil,
			wantErr: false,
		},
		{
			name:     "client error",
			provider: "openai",
			mockResp: nil,
			mockErr:  errors.New("client error"),
			wantErr:  true,
		},
		{
			name:     "unknown provider",
			provider: "unknown",
			mockResp: nil,
			mockErr:  nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, err := NewDirectRouter(&DirectConfig{
				DefaultProvider: "openai",
			})
			if err != nil {
				t.Fatalf("NewDirectRouter() error = %v", err)
			}

			if tt.provider == "openai" {
				mockClient := &MockClient{
					response: tt.mockResp,
					err:      tt.mockErr,
				}
				router.RegisterClient("openai", mockClient)
			}

			req := Request{
				Model:    "gpt-4o",
				Provider: tt.provider,
				Messages: []Message{
					{Role: "user", Content: "Test"},
				},
			}

			ctx := context.Background()
			resp, err := router.Route(ctx, req)

			if (err != nil) != tt.wantErr {
				t.Errorf("Route() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if resp == nil {
					t.Fatal("Route() returned nil response")
				}

				if resp.Provider != tt.provider {
					t.Errorf("Provider = %v, want %v", resp.Provider, tt.provider)
				}

				if resp.Latency == 0 {
					t.Error("Latency not set")
				}
			}
		})
	}
}

func TestDirectRouter_RouteWithFallback(t *testing.T) {
	// Create primary router that will fail
	primaryRouter, _ := NewDirectRouter(&DirectConfig{
		DefaultProvider: "openai",
	})

	failingClient := &MockClient{
		response: nil,
		err:      errors.New("primary error"),
	}
	primaryRouter.RegisterClient("openai", failingClient)

	// Create fallback router that will succeed
	fallbackRouter, _ := NewDirectRouter(&DirectConfig{
		DefaultProvider: "openai",
	})

	successClient := &MockClient{
		response: &Response{
			Model:    "gpt-4o",
			Content:  "Fallback response",
			Provider: "openai",
		},
	}
	fallbackRouter.RegisterClient("openai", successClient)

	// Set fallback
	primaryRouter.SetFallback(fallbackRouter)

	req := Request{
		Model:    "gpt-4o",
		Provider: "openai",
		Messages: []Message{
			{Role: "user", Content: "Test"},
		},
	}

	ctx := context.Background()
	resp, err := primaryRouter.Route(ctx, req)

	if err != nil {
		t.Fatalf("Route() with fallback should succeed, got error: %v", err)
	}

	if resp.Content != "Fallback response" {
		t.Errorf("Expected fallback response, got: %v", resp.Content)
	}
}

func TestDirectRouter_ListProviders(t *testing.T) {
	router, err := NewDirectRouter(nil)
	if err != nil {
		t.Fatalf("NewDirectRouter() error = %v", err)
	}

	router.RegisterClient("openai", &MockClient{})
	router.RegisterClient("anthropic", &MockClient{})
	router.RegisterClient("groq", &MockClient{})

	providers := router.ListProviders()

	if len(providers) != 3 {
		t.Errorf("Expected 3 providers, got %d", len(providers))
	}

	providerMap := make(map[string]bool)
	for _, p := range providers {
		providerMap[p] = true
	}

	expected := []string{"openai", "anthropic", "groq"}
	for _, e := range expected {
		if !providerMap[e] {
			t.Errorf("Expected provider %s not found", e)
		}
	}
}

func TestDirectRouter_Close(t *testing.T) {
	router, err := NewDirectRouter(nil)
	if err != nil {
		t.Fatalf("NewDirectRouter() error = %v", err)
	}

	router.RegisterClient("test", &MockClient{})

	if err := router.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}
