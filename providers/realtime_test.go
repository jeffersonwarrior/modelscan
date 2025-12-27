package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewRealtimeProvider(t *testing.T) {
	provider := NewRealtimeProvider("test-key")
	if provider == nil {
		t.Fatal("Expected provider to be created")
	}

	realtimeProvider, ok := provider.(*RealtimeProvider)
	if !ok {
		t.Fatal("Expected provider to be of type *RealtimeProvider")
	}

	if realtimeProvider.apiKey != "test-key" {
		t.Errorf("Expected apiKey 'test-key', got '%s'", realtimeProvider.apiKey)
	}

	if realtimeProvider.baseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected baseURL 'https://api.openai.com/v1', got '%s'", realtimeProvider.baseURL)
	}

	if realtimeProvider.wsURL != "wss://api.openai.com/v1/realtime" {
		t.Errorf("Expected wsURL 'wss://api.openai.com/v1/realtime', got '%s'", realtimeProvider.wsURL)
	}
}

func TestNewRealtimeProvider_EmptyKey(t *testing.T) {
	provider := NewRealtimeProvider("")
	if provider == nil {
		t.Fatal("Expected provider to be created even with empty key")
	}

	realtimeProvider := provider.(*RealtimeProvider)
	if realtimeProvider.apiKey != "" {
		t.Errorf("Expected empty apiKey, got '%s'", realtimeProvider.apiKey)
	}
}

func TestRealtimeProvider_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("Expected path /models, got %s", r.URL.Path)
		}

		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": [
				{
					"id": "gpt-4o-realtime-preview",
					"object": "model",
					"created": 1728933352,
					"owned_by": "openai"
				},
				{
					"id": "gpt-4o-realtime-preview-2024-10-01",
					"object": "model",
					"created": 1728933352,
					"owned_by": "openai"
				},
				{
					"id": "gpt-4o",
					"object": "model",
					"created": 1677649963,
					"owned_by": "openai"
				}
			]
		}`))
	}))
	defer server.Close()

	provider := &RealtimeProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	// Should only return realtime models, not gpt-4o
	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}

	// Verify first model details
	if models[0].ID != "gpt-4o-realtime-preview" {
		t.Errorf("Expected ID 'gpt-4o-realtime-preview', got '%s'", models[0].ID)
	}
	if models[0].Name != "GPT-4o Realtime" {
		t.Errorf("Expected name 'GPT-4o Realtime', got '%s'", models[0].Name)
	}

	// Verify pricing
	if models[0].CostPer1MIn != 5.00 {
		t.Errorf("Expected CostPer1MIn 5.00, got %f", models[0].CostPer1MIn)
	}
	if models[0].CostPer1MOut != 20.00 {
		t.Errorf("Expected CostPer1MOut 20.00, got %f", models[0].CostPer1MOut)
	}

	// Verify categories
	foundRealtime := false
	foundVoice := false
	foundAudio := false
	for _, cat := range models[0].Categories {
		if cat == "realtime" {
			foundRealtime = true
		}
		if cat == "voice" {
			foundVoice = true
		}
		if cat == "audio" {
			foundAudio = true
		}
	}

	if !foundRealtime {
		t.Error("Expected 'realtime' category")
	}
	if !foundVoice {
		t.Error("Expected 'voice' category")
	}
	if !foundAudio {
		t.Error("Expected 'audio' category")
	}

	// Verify context window
	if models[0].ContextWindow != 128000 {
		t.Errorf("Expected ContextWindow 128000, got %d", models[0].ContextWindow)
	}
}

func TestRealtimeProvider_ListModels_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": [{"id": "gpt-4o-realtime-preview", "object": "model", "created": 1728933352, "owned_by": "openai"}]}`))
	}))
	defer server.Close()

	provider := &RealtimeProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	_, err := provider.ListModels(ctx, true)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
}

func TestRealtimeProvider_ListModels_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Internal server error"}`))
	}))
	defer server.Close()

	provider := &RealtimeProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Fatal("Expected error for HTTP 500")
	}

	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Expected error to mention status 500, got: %v", err)
	}
}

func TestRealtimeProvider_ListModels_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	provider := &RealtimeProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	_, err := provider.ListModels(ctx, false)
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("Expected decode error, got: %v", err)
	}
}

func TestRealtimeProvider_GetCapabilities(t *testing.T) {
	provider := NewRealtimeProvider("test-key")
	caps := provider.GetCapabilities()

	if !caps.SupportsChat {
		t.Error("Expected SupportsChat to be true")
	}

	if !caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be true")
	}

	if !caps.SupportsAudio {
		t.Error("Expected SupportsAudio to be true")
	}

	if !caps.SupportsAgents {
		t.Error("Expected SupportsAgents to be true")
	}

	if caps.SupportsEmbeddings {
		t.Error("Expected SupportsEmbeddings to be false")
	}

	if caps.SupportsFIM {
		t.Error("Expected SupportsFIM to be false")
	}

	if caps.SupportsVision {
		t.Error("Expected SupportsVision to be false")
	}

	if caps.MaxTokensPerRequest != 128000 {
		t.Errorf("Expected MaxTokensPerRequest 128000, got %d", caps.MaxTokensPerRequest)
	}
}

func TestRealtimeProvider_GetEndpoints(t *testing.T) {
	provider := NewRealtimeProvider("test-key")
	endpoints := provider.GetEndpoints()

	if len(endpoints) == 0 {
		t.Fatal("Expected at least one endpoint")
	}

	// Verify /models endpoint
	foundModels := false
	for _, ep := range endpoints {
		if ep.Path == "/models" {
			foundModels = true
			if ep.Method != "GET" {
				t.Errorf("Expected /models to use GET method, got %s", ep.Method)
			}
			if ep.Status != StatusUnknown {
				t.Errorf("Expected initial status to be unknown, got %s", ep.Status)
			}
		}
	}

	if !foundModels {
		t.Error("Expected /models endpoint to be present")
	}
}

func TestRealtimeProvider_ValidateEndpoints(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)

		if r.URL.Path != "/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": [{"id": "gpt-4o-realtime-preview", "object": "model"}]}`))
	}))
	defer server.Close()

	provider := &RealtimeProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err != nil {
		t.Fatalf("ValidateEndpoints failed: %v", err)
	}

	if atomic.LoadInt32(&requestCount) == 0 {
		t.Error("Expected at least one request to be made")
	}

	// Verify endpoints were stored
	if len(provider.endpoints) == 0 {
		t.Error("Expected endpoints to be stored after validation")
	}

	// Check endpoint status
	for _, ep := range provider.endpoints {
		if ep.Status != StatusWorking {
			t.Errorf("Expected endpoint %s to have status working, got %s", ep.Path, ep.Status)
		}
	}
}

func TestRealtimeProvider_ValidateEndpoints_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": []}`))
	}))
	defer server.Close()

	provider := &RealtimeProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, true)
	if err != nil {
		t.Fatalf("ValidateEndpoints verbose failed: %v", err)
	}
}

func TestRealtimeProvider_ValidateEndpoints_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "Invalid API key"}`))
	}))
	defer server.Close()

	provider := &RealtimeProvider{
		apiKey:  "invalid-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.ValidateEndpoints(ctx, false)
	if err == nil {
		t.Fatal("Expected error for failed endpoint validation")
	}

	if !strings.Contains(err.Error(), "critical endpoint failed") {
		t.Errorf("Expected critical endpoint error, got: %v", err)
	}
}

func TestRealtimeProvider_TestModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": [
				{"id": "gpt-4o-realtime-preview", "object": "model", "created": 1728933352, "owned_by": "openai"}
			]
		}`))
	}))
	defer server.Close()

	provider := &RealtimeProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "gpt-4o-realtime-preview", false)
	if err != nil {
		t.Fatalf("TestModel failed: %v", err)
	}
}

func TestRealtimeProvider_TestModel_Verbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": [{"id": "gpt-4o-realtime-preview", "object": "model"}]}`))
	}))
	defer server.Close()

	provider := &RealtimeProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "gpt-4o-realtime-preview", true)
	if err != nil {
		t.Fatalf("TestModel verbose failed: %v", err)
	}
}

func TestRealtimeProvider_TestModel_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": [{"id": "gpt-4o-realtime-preview", "object": "model"}]}`))
	}))
	defer server.Close()

	provider := &RealtimeProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "nonexistent-model", false)
	if err == nil {
		t.Fatal("Expected error for nonexistent model")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func TestRealtimeProvider_TestModel_ListModelsFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	provider := &RealtimeProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.TestModel(ctx, "gpt-4o-realtime-preview", false)
	if err == nil {
		t.Fatal("Expected error when ListModels fails")
	}

	if !strings.Contains(err.Error(), "failed to list models") {
		t.Errorf("Expected 'failed to list models' error, got: %v", err)
	}
}

func TestRealtimeProvider_CreateSession(t *testing.T) {
	provider := NewRealtimeProvider("test-key").(*RealtimeProvider)

	tests := []struct {
		name    string
		req     realtimeSessionRequest
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid session with defaults",
			req:     realtimeSessionRequest{},
			wantErr: false,
		},
		{
			name: "valid session with custom voice",
			req: realtimeSessionRequest{
				Voice: "nova",
			},
			wantErr: false,
		},
		{
			name: "invalid voice",
			req: realtimeSessionRequest{
				Voice: "invalid-voice",
			},
			wantErr: true,
			errMsg:  "invalid voice",
		},
		{
			name: "invalid input format",
			req: realtimeSessionRequest{
				Voice:       "alloy",
				InputFormat: "invalid-format",
			},
			wantErr: true,
			errMsg:  "invalid input format",
		},
		{
			name: "invalid output format",
			req: realtimeSessionRequest{
				Voice:        "alloy",
				InputFormat:  "pcm16",
				OutputFormat: "invalid-format",
			},
			wantErr: true,
			errMsg:  "invalid output format",
		},
		{
			name: "valid session with all fields",
			req: realtimeSessionRequest{
				Model:        "gpt-4o-realtime-preview",
				Modalities:   []string{"text", "audio"},
				Voice:        "echo",
				InputFormat:  "g711_ulaw",
				OutputFormat: "g711_alaw",
				Temperature:  0.8,
			},
			wantErr: false,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.CreateSession(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSession() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("CreateSession() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}

func TestRealtimeProvider_ConnectWebSocket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": [{"id": "gpt-4o-realtime-preview", "object": "model"}]}`))
	}))
	defer server.Close()

	provider := &RealtimeProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		wsURL:   "wss://api.openai.com/v1/realtime",
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.ConnectWebSocket(ctx, "gpt-4o-realtime-preview")
	if err != nil {
		t.Fatalf("ConnectWebSocket failed: %v", err)
	}
}

func TestRealtimeProvider_ConnectWebSocket_DefaultModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": [{"id": "gpt-4o-realtime-preview", "object": "model"}]}`))
	}))
	defer server.Close()

	provider := &RealtimeProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		wsURL:   "wss://api.openai.com/v1/realtime",
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.ConnectWebSocket(ctx, "")
	if err != nil {
		t.Fatalf("ConnectWebSocket with default model failed: %v", err)
	}
}

func TestRealtimeProvider_ConnectWebSocket_ModelNotAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": [{"id": "gpt-4o-realtime-preview", "object": "model"}]}`))
	}))
	defer server.Close()

	provider := &RealtimeProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		wsURL:   "wss://api.openai.com/v1/realtime",
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.ConnectWebSocket(ctx, "nonexistent-model")
	if err == nil {
		t.Fatal("Expected error for unavailable model")
	}

	if !strings.Contains(err.Error(), "not available") {
		t.Errorf("Expected 'not available' error, got: %v", err)
	}
}

func TestRealtimeProvider_ConnectWebSocket_ListModelsFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	provider := &RealtimeProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		wsURL:   "wss://api.openai.com/v1/realtime",
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	err := provider.ConnectWebSocket(ctx, "gpt-4o-realtime-preview")
	if err == nil {
		t.Fatal("Expected error when ListModels fails")
	}

	if !strings.Contains(err.Error(), "failed to list models") {
		t.Errorf("Expected 'failed to list models' error, got: %v", err)
	}
}

func TestRealtimeProvider_SendEvent(t *testing.T) {
	provider := NewRealtimeProvider("test-key").(*RealtimeProvider)

	tests := []struct {
		name      string
		eventType string
		eventData interface{}
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid session update",
			eventType: "session.update",
			eventData: map[string]interface{}{"temperature": 0.8},
			wantErr:   false,
		},
		{
			name:      "valid input audio buffer append",
			eventType: "input_audio_buffer.append",
			eventData: map[string]interface{}{"audio": "base64data"},
			wantErr:   false,
		},
		{
			name:      "valid response create",
			eventType: "response.create",
			eventData: map[string]interface{}{},
			wantErr:   false,
		},
		{
			name:      "invalid event type",
			eventType: "invalid.event",
			eventData: map[string]interface{}{},
			wantErr:   true,
			errMsg:    "invalid event type",
		},
		{
			name:      "invalid event data",
			eventType: "session.update",
			eventData: make(chan int), // channels can't be marshaled
			wantErr:   true,
			errMsg:    "invalid event data",
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.SendEvent(ctx, tt.eventType, tt.eventData)
			if (err != nil) != tt.wantErr {
				t.Errorf("SendEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("SendEvent() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}

func TestRealtimeProvider_ReceiveEvent(t *testing.T) {
	provider := NewRealtimeProvider("test-key").(*RealtimeProvider)

	ctx := context.Background()
	event, err := provider.ReceiveEvent(ctx)
	if err != nil {
		t.Fatalf("ReceiveEvent failed: %v", err)
	}

	if event == nil {
		t.Fatal("Expected non-nil event")
	}

	eventType, ok := event["type"].(string)
	if !ok {
		t.Fatal("Expected event to have 'type' field")
	}

	if eventType != "session.created" {
		t.Errorf("Expected event type 'session.created', got '%s'", eventType)
	}

	session, ok := event["session"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected event to have 'session' field")
	}

	if session["model"] != "gpt-4o-realtime-preview" {
		t.Errorf("Expected model 'gpt-4o-realtime-preview', got '%v'", session["model"])
	}
}

func TestRealtimeProvider_testEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	provider := &RealtimeProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	endpoint := &Endpoint{
		Path:   "/models",
		Method: "GET",
	}

	ctx := context.Background()
	err := provider.testEndpoint(ctx, endpoint)
	if err != nil {
		t.Fatalf("testEndpoint failed: %v", err)
	}
}

func TestRealtimeProvider_testEndpoint_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "Unauthorized"}`))
	}))
	defer server.Close()

	provider := &RealtimeProvider{
		apiKey:  "invalid-key",
		baseURL: server.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	endpoint := &Endpoint{
		Path:   "/models",
		Method: "GET",
	}

	ctx := context.Background()
	err := provider.testEndpoint(ctx, endpoint)
	if err == nil {
		t.Fatal("Expected error for unauthorized request")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Errorf("Expected error to mention 401, got: %v", err)
	}
}

func TestRealtimeProvider_ProviderRegistration(t *testing.T) {
	// Test that the provider is registered
	factory, exists := providerFactories["realtime"]
	if !exists {
		t.Fatal("Expected 'realtime' provider to be registered")
	}

	provider := factory("test-key")
	if provider == nil {
		t.Fatal("Expected factory to create provider")
	}

	_, ok := provider.(*RealtimeProvider)
	if !ok {
		t.Fatal("Expected provider to be *RealtimeProvider")
	}
}
