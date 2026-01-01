// Package proxy provides HTTP proxy functionality for forwarding requests
// to LLM providers with SSE streaming support.
package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// sanitizeErrorMessage removes characters that could cause format string injection or JSON breaking
func sanitizeErrorMessage(msg string) string {
	// Replace quote characters to prevent JSON injection
	msg = strings.ReplaceAll(msg, `"`, `'`)
	msg = strings.ReplaceAll(msg, "\n", " ")
	msg = strings.ReplaceAll(msg, "\r", " ")
	// Limit length to prevent DoS
	if len(msg) > 500 {
		msg = msg[:500] + "..."
	}
	return msg
}

// StreamWriter wraps http.ResponseWriter with SSE streaming capabilities.
// It provides methods for writing Server-Sent Events with proper formatting
// and automatic flushing.
type StreamWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	closed  bool
}

// NewStreamWriter creates a new StreamWriter from an http.ResponseWriter.
// It sets the required SSE headers and returns an error if the ResponseWriter
// does not support flushing.
func NewStreamWriter(w http.ResponseWriter) (*StreamWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("response writer does not support flushing")
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	return &StreamWriter{
		w:       w,
		flusher: flusher,
		closed:  false,
	}, nil
}

// WriteEvent writes a data event to the SSE stream.
// The data is formatted as "data: <data>\n\n" per SSE specification.
func (sw *StreamWriter) WriteEvent(data []byte) error {
	if sw.closed {
		return fmt.Errorf("stream is closed")
	}

	// Write SSE data line
	if _, err := fmt.Fprintf(sw.w, "data: %s\n\n", data); err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	// Note: http.Flusher.Flush() doesn't return an error
	// Network errors are caught by the next write operation
	sw.flusher.Flush()
	return nil
}

// WriteEventWithType writes a named event to the SSE stream.
// The event is formatted as "event: <type>\ndata: <data>\n\n".
func (sw *StreamWriter) WriteEventWithType(eventType string, data []byte) error {
	if sw.closed {
		return fmt.Errorf("stream is closed")
	}

	if _, err := fmt.Fprintf(sw.w, "event: %s\ndata: %s\n\n", eventType, data); err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	sw.flusher.Flush()
	return nil
}

// WriteError writes an error event to the SSE stream.
// The error is formatted as a JSON object with an "error" field.
func (sw *StreamWriter) WriteError(err error) error {
	if sw.closed {
		return fmt.Errorf("stream is closed")
	}

	errObj := map[string]interface{}{
		"error": map[string]string{
			"type":    "stream_error",
			"message": sanitizeErrorMessage(err.Error()),
		},
	}

	data, marshalErr := json.Marshal(errObj)
	if marshalErr != nil {
		// Fallback to simple error message (sanitized to prevent injection)
		sanitized := sanitizeErrorMessage(err.Error())
		data = []byte(fmt.Sprintf(`{"error":{"type":"stream_error","message":"%s"}}`, sanitized))
	}

	return sw.WriteEventWithType("error", data)
}

// WriteComment writes an SSE comment line (for keep-alive pings).
// Comments start with ":" and are ignored by clients but keep the connection alive.
func (sw *StreamWriter) WriteComment(comment string) error {
	if sw.closed {
		return fmt.Errorf("stream is closed")
	}

	if _, err := fmt.Fprintf(sw.w, ": %s\n", comment); err != nil {
		return fmt.Errorf("failed to write comment: %w", err)
	}

	sw.flusher.Flush()
	return nil
}

// Close marks the stream as closed and writes the done event.
// After Close is called, no more events can be written.
func (sw *StreamWriter) Close() error {
	if sw.closed {
		return nil
	}

	sw.closed = true

	// Write the standard SSE done marker (used by OpenAI/Anthropic)
	if _, err := fmt.Fprint(sw.w, "data: [DONE]\n\n"); err != nil {
		return fmt.Errorf("failed to write done marker: %w", err)
	}

	sw.flusher.Flush()
	return nil
}

// IsClosed returns whether the stream has been closed.
func (sw *StreamWriter) IsClosed() bool {
	return sw.closed
}
