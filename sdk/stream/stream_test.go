package stream

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestNewStream_CreatesWithType(t *testing.T) {
	reader := strings.NewReader("test data")
	ctx := context.Background()

	stream := NewStream(ctx, reader, StreamTypeSSE)
	defer stream.Close()

	if stream.streamType != StreamTypeSSE {
		t.Errorf("Expected SSE stream type, got %s", stream.streamType)
	}
	if stream.chunks == nil {
		t.Error("Chunks channel not initialized")
	}
}

func TestStream_SSE_ParsesDataLines(t *testing.T) {
	sseData := `data: {"content": "Hello"}

data: {"content": " World"}

data: [DONE]

`
	reader := strings.NewReader(sseData)
	ctx := context.Background()

	stream := NewStream(ctx, reader, StreamTypeSSE)
	defer stream.Close()

	// Collect chunks
	var chunks []*Chunk
	for chunk := range stream.Chunks() {
		chunks = append(chunks, chunk)
	}

	if len(chunks) < 2 {
		t.Fatalf("Expected at least 2 chunks, got %d", len(chunks))
	}

	// First chunk
	if chunks[0].Type != ChunkTypeData {
		t.Errorf("Expected data chunk, got %s", chunks[0].Type)
	}

	// Last chunk should be DONE
	lastChunk := chunks[len(chunks)-1]
	if lastChunk.Type != ChunkTypeDone {
		t.Errorf("Expected done chunk, got %s", lastChunk.Type)
	}
}

func TestStream_SSE_ExtractsOpenAIContent(t *testing.T) {
	sseData := `data: {"choices":[{"delta":{"content":"Hello"}}]}

data: {"choices":[{"delta":{"content":" World"}}]}

data: [DONE]

`
	reader := strings.NewReader(sseData)
	ctx := context.Background()

	stream := NewStream(ctx, reader, StreamTypeSSE)
	defer stream.Close()

	// Collect content
	var content strings.Builder
	for chunk := range stream.Chunks() {
		if chunk.Type == ChunkTypeDone {
			break
		}
		content.WriteString(chunk.Data)
	}

	result := content.String()
	if result != "Hello World" {
		t.Errorf("Expected 'Hello World', got '%s'", result)
	}
}

func TestStream_SSE_ExtractsAnthropicContent(t *testing.T) {
	sseData := `data: {"delta":{"text":"Hello"}}

data: {"delta":{"text":" from"}}

data: {"delta":{"text":" Claude"}}

data: [DONE]

`
	reader := strings.NewReader(sseData)
	ctx := context.Background()

	stream := NewStream(ctx, reader, StreamTypeSSE)
	defer stream.Close()

	// Collect content
	var content strings.Builder
	for chunk := range stream.Chunks() {
		if chunk.Type == ChunkTypeDone {
			break
		}
		content.WriteString(chunk.Data)
	}

	result := content.String()
	if result != "Hello from Claude" {
		t.Errorf("Expected 'Hello from Claude', got '%s'", result)
	}
}

func TestStream_SSE_HandlesMetadata(t *testing.T) {
	sseData := `event: message
id: msg-123
data: {"content": "Hello"}

`
	reader := strings.NewReader(sseData)
	ctx := context.Background()

	stream := NewStream(ctx, reader, StreamTypeSSE)
	defer stream.Close()

	// Get first chunk
	chunk := <-stream.Chunks()

	if chunk.Metadata["event"] != "message" {
		t.Errorf("Expected event=message, got %v", chunk.Metadata["event"])
	}
	if chunk.Metadata["id"] != "msg-123" {
		t.Errorf("Expected id=msg-123, got %v", chunk.Metadata["id"])
	}
}

func TestStream_HTTP_ReadsChunkedData(t *testing.T) {
	httpData := "Hello World from HTTP chunked transfer"
	reader := strings.NewReader(httpData)
	ctx := context.Background()

	stream := NewStream(ctx, reader, StreamTypeHTTP)
	defer stream.Close()

	// Collect all data
	var collected strings.Builder
	for chunk := range stream.Chunks() {
		collected.WriteString(chunk.Data)
	}

	result := collected.String()
	if !strings.Contains(result, "Hello World") {
		t.Errorf("Expected 'Hello World' in result, got '%s'", result)
	}
}

func TestStream_Collect_AccumulatesAllChunks(t *testing.T) {
	sseData := `data: {"content": "Hello"}

data: {"content": " World"}

data: [DONE]

`
	reader := strings.NewReader(sseData)
	ctx := context.Background()

	stream := NewStream(ctx, reader, StreamTypeSSE)

	result, err := stream.Collect()
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if !strings.Contains(result, "Hello") || !strings.Contains(result, "World") {
		t.Errorf("Expected 'Hello World', got '%s'", result)
	}
}

func TestStream_Filter_OnlyMatchingChunks(t *testing.T) {
	sseData := `data: {"content": "Hello"}

data: {"content": " World"}

data: {"content": "!"}

data: [DONE]

`
	reader := strings.NewReader(sseData)
	ctx := context.Background()

	stream := NewStream(ctx, reader, StreamTypeSSE)
	defer stream.Close()

	// Filter only chunks with more than 5 characters
	filtered := stream.Filter(func(chunk *Chunk) bool {
		return len(chunk.Data) > 2
	})

	var count int
	for range filtered.Chunks() {
		count++
	}

	// Should filter out the "!" chunk and [DONE]
	if count != 2 {
		t.Errorf("Expected 2 filtered chunks, got %d", count)
	}
}

func TestStream_Map_TransformsChunks(t *testing.T) {
	sseData := `data: {"content": "hello"}

data: {"content": " world"}

data: [DONE]

`
	reader := strings.NewReader(sseData)
	ctx := context.Background()

	stream := NewStream(ctx, reader, StreamTypeSSE)
	defer stream.Close()

	// Map to uppercase
	mapped := stream.Map(func(chunk *Chunk) *Chunk {
		if chunk.Type == ChunkTypeData {
			chunk.Data = strings.ToUpper(chunk.Data)
		}
		return chunk
	})

	var collected strings.Builder
	for chunk := range mapped.Chunks() {
		if chunk.Type == ChunkTypeDone {
			break
		}
		collected.WriteString(chunk.Data)
	}

	result := collected.String()
	if result != "HELLO WORLD" {
		t.Errorf("Expected 'HELLO WORLD', got '%s'", result)
	}
}

func TestStream_Tap_ObservesWithoutModifying(t *testing.T) {
	sseData := `data: {"content": "Hello"}

data: {"content": " World"}

data: [DONE]

`
	reader := strings.NewReader(sseData)
	ctx := context.Background()

	stream := NewStream(ctx, reader, StreamTypeSSE)
	defer stream.Close()

	var observed []string
	tapped := stream.Tap(func(chunk *Chunk) {
		if chunk.Type == ChunkTypeData {
			observed = append(observed, chunk.Data)
		}
	})

	// Consume the stream
	for range tapped.Chunks() {
	}

	if len(observed) < 1 {
		t.Errorf("Expected to observe chunks, got %d", len(observed))
	}
}

func TestStream_Close_CancelsProcessing(t *testing.T) {
	// Slow reader that never completes
	reader := &slowReader{delay: 100 * time.Millisecond}
	ctx := context.Background()

	stream := NewStream(ctx, reader, StreamTypeHTTP)

	// Close immediately
	stream.Close()

	// Stream should be closed
	select {
	case _, ok := <-stream.Chunks():
		if ok {
			t.Error("Chunks channel still open after close")
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("Stream did not close within timeout")
	}

	// Error may be context.Canceled, which is expected
	if stream.Err() != nil && stream.Err() != context.Canceled {
		t.Errorf("Unexpected error: %v", stream.Err())
	}
}

func TestStream_ContextCancellation_StopsProcessing(t *testing.T) {
	reader := &slowReader{delay: 10 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())

	stream := NewStream(ctx, reader, StreamTypeHTTP)
	defer stream.Close()

	// Cancel context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// Try to read chunks
	chunkCount := 0
	for range stream.Chunks() {
		chunkCount++
	}

	// Should stop due to cancellation
	if stream.Err() != context.Canceled {
		t.Errorf("Expected context canceled error, got %v", stream.Err())
	}
}

func TestStream_ExtractContent_HandlesMultipleFormats(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		expected string
	}{
		{
			name: "OpenAI format",
			data: map[string]interface{}{
				"choices": []interface{}{
					map[string]interface{}{
						"delta": map[string]interface{}{
							"content": "Hello OpenAI",
						},
					},
				},
			},
			expected: "Hello OpenAI",
		},
		{
			name: "Anthropic format",
			data: map[string]interface{}{
				"delta": map[string]interface{}{
					"text": "Hello Anthropic",
				},
			},
			expected: "Hello Anthropic",
		},
		{
			name: "Google format",
			data: map[string]interface{}{
				"candidates": []interface{}{
					map[string]interface{}{
						"content": map[string]interface{}{
							"parts": []interface{}{
								map[string]interface{}{
									"text": "Hello Google",
								},
							},
						},
					},
				},
			},
			expected: "Hello Google",
		},
		{
			name: "Generic text",
			data: map[string]interface{}{
				"text": "Hello Generic",
			},
			expected: "Hello Generic",
		},
	}

	stream := &Stream{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stream.extractContent(tt.data)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// slowReader simulates a slow streaming response
type slowReader struct {
	delay time.Duration
	count int
}

func (sr *slowReader) Read(p []byte) (n int, err error) {
	if sr.count >= 10 {
		return 0, io.EOF
	}
	time.Sleep(sr.delay)
	copy(p, []byte("chunk "))
	sr.count++
	return 6, nil
}

func TestStream_ProcessWebSocket(t *testing.T) {
	// processWebSocket currently calls processHTTP internally
	// Test that it doesn't panic and works as expected
	responseBody := `{"choices":[{"delta":{"content":"Test"}}]}`
	stream := NewStream(context.Background(), io.NopCloser(strings.NewReader(responseBody)), StreamTypeWebSocket)
	defer stream.Close()

	// Collect chunks - just verify it works without panic
	var chunkCount int
	for chunk := range stream.Chunks() {
		if chunk.Type == ChunkTypeData {
			chunkCount++
		}
	}

	// WebSocket processing should work (at least 1 chunk)
	if chunkCount < 1 {
		t.Errorf("Expected at least 1 chunk from WebSocket processing, got %d", chunkCount)
	}
}
