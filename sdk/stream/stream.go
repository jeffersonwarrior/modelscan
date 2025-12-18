package stream

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

// StreamType indicates the streaming protocol
type StreamType string

const (
	StreamTypeSSE       StreamType = "sse"        // Server-Sent Events
	StreamTypeWebSocket StreamType = "websocket"  // WebSocket
	StreamTypeHTTP      StreamType = "http"       // HTTP chunked
)

// ChunkType indicates what kind of data is in the chunk
type ChunkType string

const (
	ChunkTypeData     ChunkType = "data"     // Content chunk
	ChunkTypeMetadata ChunkType = "metadata" // Metadata (usage, etc.)
	ChunkTypeError    ChunkType = "error"    // Error message
	ChunkTypeDone     ChunkType = "done"     // Stream complete
)

// Chunk represents a single piece of streamed data
type Chunk struct {
	Type     ChunkType              // Type of chunk
	Data     string                 // Content (for data chunks)
	Metadata map[string]interface{} // Additional metadata
	Raw      []byte                 // Raw bytes received
	Error    error                  // Error if Type == ChunkTypeError
}

// Stream represents a unified streaming interface
type Stream struct {
	streamType StreamType
	reader     io.Reader
	scanner    *bufio.Scanner
	chunks     chan *Chunk
	done       chan struct{}
	err        error
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewStream creates a new stream from a reader
func NewStream(ctx context.Context, reader io.Reader, streamType StreamType) *Stream {
	ctx, cancel := context.WithCancel(ctx)
	
	s := &Stream{
		streamType: streamType,
		reader:     reader,
		scanner:    bufio.NewScanner(reader),
		chunks:     make(chan *Chunk, 10),
		done:       make(chan struct{}),
		ctx:        ctx,
		cancel:     cancel,
	}

	go s.processStream()
	return s
}

// Chunks returns a channel that receives chunks as they arrive
func (s *Stream) Chunks() <-chan *Chunk {
	return s.chunks
}

// Close closes the stream and releases resources
func (s *Stream) Close() error {
	s.cancel()
	<-s.done
	return s.err
}

// Err returns any error that occurred during streaming
func (s *Stream) Err() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
}

// processStream reads from the stream and sends chunks
func (s *Stream) processStream() {
	defer close(s.chunks)
	defer close(s.done)

	switch s.streamType {
	case StreamTypeSSE:
		s.processSSE()
	case StreamTypeHTTP:
		s.processHTTP()
	case StreamTypeWebSocket:
		s.processWebSocket()
	default:
		s.setError(fmt.Errorf("unsupported stream type: %s", s.streamType))
	}
}

// processSSE handles Server-Sent Events format
func (s *Stream) processSSE() {
	var currentEvent strings.Builder
	
	for s.scanner.Scan() {
		select {
		case <-s.ctx.Done():
			s.setError(s.ctx.Err())
			return
		default:
		}

		line := s.scanner.Text()

		// Empty line signals end of event
		if line == "" {
			if currentEvent.Len() > 0 {
				s.parseSSEEvent(currentEvent.String())
				currentEvent.Reset()
			}
			continue
		}

		// Accumulate event lines
		if currentEvent.Len() > 0 {
			currentEvent.WriteString("\n")
		}
		currentEvent.WriteString(line)
	}

	if err := s.scanner.Err(); err != nil {
		s.setError(err)
	}
}

// parseSSEEvent parses a complete SSE event
func (s *Stream) parseSSEEvent(event string) {
	lines := strings.Split(event, "\n")
	chunk := &Chunk{
		Type:     ChunkTypeData,
		Metadata: make(map[string]interface{}),
	}

	var hasData bool

	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			hasData = true
			
			// Check for [DONE] marker (OpenAI convention)
			if data == "[DONE]" {
				chunk.Type = ChunkTypeDone
				s.sendChunk(chunk)
				return
			}

			// Try to parse as JSON
			var jsonData map[string]interface{}
			if err := json.Unmarshal([]byte(data), &jsonData); err == nil {
				// Extract content from various provider formats
				if content := s.extractContent(jsonData); content != "" {
					chunk.Data = content
				}
				// Merge JSON data into metadata
				for k, v := range jsonData {
					chunk.Metadata[k] = v
				}
			} else {
				// Plain text data
				chunk.Data = data
			}
			chunk.Raw = []byte(data)
		} else if strings.HasPrefix(line, "event: ") {
			chunk.Metadata["event"] = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "id: ") {
			chunk.Metadata["id"] = strings.TrimPrefix(line, "id: ")
		} else if strings.HasPrefix(line, "retry: ") {
			chunk.Metadata["retry"] = strings.TrimPrefix(line, "retry: ")
		}
	}

	// Only send if we got data or meaningful metadata
	if hasData || len(chunk.Metadata) > 0 {
		s.sendChunk(chunk)
	}
}

// processHTTP handles plain HTTP chunked responses
func (s *Stream) processHTTP() {
	buf := make([]byte, 4096)
	for {
		select {
		case <-s.ctx.Done():
			s.setError(s.ctx.Err())
			return
		default:
		}

		n, err := s.reader.Read(buf)
		if n > 0 {
			chunk := &Chunk{
				Type: ChunkTypeData,
				Data: string(buf[:n]),
				Raw:  buf[:n],
			}
			s.sendChunk(chunk)
		}

		if err != nil {
			if err != io.EOF {
				s.setError(err)
			}
			return
		}
	}
}

// processWebSocket handles WebSocket frames (placeholder)
func (s *Stream) processWebSocket() {
	// WebSocket implementation would go here
	// For now, treat like HTTP chunks
	s.processHTTP()
}

// extractContent extracts content from various provider response formats
func (s *Stream) extractContent(data map[string]interface{}) string {
	// OpenAI format: choices[0].delta.content
	if choices, ok := data["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if delta, ok := choice["delta"].(map[string]interface{}); ok {
				if content, ok := delta["content"].(string); ok {
					return content
				}
			}
		}
	}

	// Anthropic format: delta.text or content_block.text
	if delta, ok := data["delta"].(map[string]interface{}); ok {
		if text, ok := delta["text"].(string); ok {
			return text
		}
	}
	if contentBlock, ok := data["content_block"].(map[string]interface{}); ok {
		if text, ok := contentBlock["text"].(string); ok {
			return text
		}
	}

	// Google format: candidates[0].content.parts[0].text
	if candidates, ok := data["candidates"].([]interface{}); ok && len(candidates) > 0 {
		if candidate, ok := candidates[0].(map[string]interface{}); ok {
			if content, ok := candidate["content"].(map[string]interface{}); ok {
				if parts, ok := content["parts"].([]interface{}); ok && len(parts) > 0 {
					if part, ok := parts[0].(map[string]interface{}); ok {
						if text, ok := part["text"].(string); ok {
							return text
						}
					}
				}
			}
		}
	}

	// Generic fallback: look for "text", "content", or "message" keys
	if text, ok := data["text"].(string); ok {
		return text
	}
	if content, ok := data["content"].(string); ok {
		return content
	}
	if message, ok := data["message"].(string); ok {
		return message
	}

	return ""
}

// sendChunk sends a chunk to the channel
func (s *Stream) sendChunk(chunk *Chunk) {
	select {
	case s.chunks <- chunk:
	case <-s.ctx.Done():
		return
	}
}

// setError sets the error state
func (s *Stream) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err == nil {
		s.err = err
	}
}

// Collect accumulates all chunks into a single string
func (s *Stream) Collect() (string, error) {
	var builder strings.Builder
	for chunk := range s.chunks {
		if chunk.Type == ChunkTypeError {
			return "", chunk.Error
		}
		if chunk.Type == ChunkTypeDone {
			break
		}
		if chunk.Data != "" {
			builder.WriteString(chunk.Data)
		}
	}
	return builder.String(), s.Err()
}

// Filter creates a new stream with only chunks matching the predicate
func (s *Stream) Filter(predicate func(*Chunk) bool) *Stream {
	filtered := &Stream{
		streamType: s.streamType,
		chunks:     make(chan *Chunk, 10),
		done:       make(chan struct{}),
		ctx:        s.ctx,
	}

	go func() {
		defer close(filtered.chunks)
		defer close(filtered.done)

		for chunk := range s.chunks {
			if predicate(chunk) {
				select {
				case filtered.chunks <- chunk:
				case <-filtered.ctx.Done():
					return
				}
			}
		}
	}()

	return filtered
}

// Map transforms chunks using the provided function
func (s *Stream) Map(transform func(*Chunk) *Chunk) *Stream {
	mapped := &Stream{
		streamType: s.streamType,
		chunks:     make(chan *Chunk, 10),
		done:       make(chan struct{}),
		ctx:        s.ctx,
	}

	go func() {
		defer close(mapped.chunks)
		defer close(mapped.done)

		for chunk := range s.chunks {
			transformed := transform(chunk)
			if transformed != nil {
				select {
				case mapped.chunks <- transformed:
				case <-mapped.ctx.Done():
					return
				}
			}
		}
	}()

	return mapped
}

// Tap allows observing chunks without modifying the stream
func (s *Stream) Tap(observer func(*Chunk)) *Stream {
	return s.Map(func(chunk *Chunk) *Chunk {
		observer(chunk)
		return chunk
	})
}
