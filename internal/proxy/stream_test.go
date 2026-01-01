package proxy

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewStreamWriter(t *testing.T) {
	t.Run("creates stream writer with SSE headers", func(t *testing.T) {
		w := httptest.NewRecorder()
		sw, err := NewStreamWriter(w)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sw == nil {
			t.Fatal("expected stream writer, got nil")
		}

		// Check SSE headers
		if got := w.Header().Get("Content-Type"); got != "text/event-stream" {
			t.Errorf("Content-Type = %q, want %q", got, "text/event-stream")
		}
		if got := w.Header().Get("Cache-Control"); got != "no-cache" {
			t.Errorf("Cache-Control = %q, want %q", got, "no-cache")
		}
		if got := w.Header().Get("Connection"); got != "keep-alive" {
			t.Errorf("Connection = %q, want %q", got, "keep-alive")
		}
		if got := w.Header().Get("X-Accel-Buffering"); got != "no" {
			t.Errorf("X-Accel-Buffering = %q, want %q", got, "no")
		}
	})

	t.Run("returns error for non-flusher", func(t *testing.T) {
		w := &nonFlushingWriter{}
		_, err := NewStreamWriter(w)
		if err == nil {
			t.Fatal("expected error for non-flushing writer")
		}
		if !strings.Contains(err.Error(), "flushing") {
			t.Errorf("error should mention flushing: %v", err)
		}
	})
}

func TestStreamWriter_WriteEvent(t *testing.T) {
	t.Run("writes SSE formatted data", func(t *testing.T) {
		w := httptest.NewRecorder()
		sw, _ := NewStreamWriter(w)

		data := []byte(`{"text":"hello"}`)
		if err := sw.WriteEvent(data); err != nil {
			t.Fatalf("WriteEvent failed: %v", err)
		}

		want := "data: {\"text\":\"hello\"}\n\n"
		if got := w.Body.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("fails after close", func(t *testing.T) {
		w := httptest.NewRecorder()
		sw, _ := NewStreamWriter(w)
		sw.Close()

		err := sw.WriteEvent([]byte("test"))
		if err == nil {
			t.Fatal("expected error after close")
		}
	})
}

func TestStreamWriter_WriteEventWithType(t *testing.T) {
	t.Run("writes event with type", func(t *testing.T) {
		w := httptest.NewRecorder()
		sw, _ := NewStreamWriter(w)

		if err := sw.WriteEventWithType("message", []byte(`{"id":"123"}`)); err != nil {
			t.Fatalf("WriteEventWithType failed: %v", err)
		}

		want := "event: message\ndata: {\"id\":\"123\"}\n\n"
		if got := w.Body.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestStreamWriter_WriteError(t *testing.T) {
	t.Run("writes error as JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		sw, _ := NewStreamWriter(w)

		if err := sw.WriteError(errors.New("something went wrong")); err != nil {
			t.Fatalf("WriteError failed: %v", err)
		}

		body := w.Body.String()
		if !strings.Contains(body, "event: error") {
			t.Errorf("expected event type error, got: %s", body)
		}
		if !strings.Contains(body, "stream_error") {
			t.Errorf("expected stream_error type, got: %s", body)
		}
		if !strings.Contains(body, "something went wrong") {
			t.Errorf("expected error message, got: %s", body)
		}
	})
}

func TestStreamWriter_WriteComment(t *testing.T) {
	t.Run("writes SSE comment", func(t *testing.T) {
		w := httptest.NewRecorder()
		sw, _ := NewStreamWriter(w)

		if err := sw.WriteComment("keepalive"); err != nil {
			t.Fatalf("WriteComment failed: %v", err)
		}

		want := ": keepalive\n"
		if got := w.Body.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestStreamWriter_Close(t *testing.T) {
	t.Run("writes done marker", func(t *testing.T) {
		w := httptest.NewRecorder()
		sw, _ := NewStreamWriter(w)

		if err := sw.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}

		want := "data: [DONE]\n\n"
		if got := w.Body.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("is idempotent", func(t *testing.T) {
		w := httptest.NewRecorder()
		sw, _ := NewStreamWriter(w)

		sw.Close()
		initialBody := w.Body.String()

		sw.Close() // Second close should be no-op
		if got := w.Body.String(); got != initialBody {
			t.Errorf("second close changed output: got %q, want %q", got, initialBody)
		}
	})

	t.Run("sets closed state", func(t *testing.T) {
		w := httptest.NewRecorder()
		sw, _ := NewStreamWriter(w)

		if sw.IsClosed() {
			t.Error("stream should not be closed initially")
		}

		sw.Close()

		if !sw.IsClosed() {
			t.Error("stream should be closed after Close()")
		}
	})
}

func TestStreamWriter_FullSequence(t *testing.T) {
	w := httptest.NewRecorder()
	sw, _ := NewStreamWriter(w)

	// Simulate a streaming response
	sw.WriteComment("stream started")
	sw.WriteEvent([]byte(`{"delta":"Hello"}`))
	sw.WriteEvent([]byte(`{"delta":" world"}`))
	sw.Close()

	body := w.Body.String()
	lines := strings.Split(body, "\n")

	// Verify sequence
	expected := []string{
		": stream started",
		"data: {\"delta\":\"Hello\"}",
		"",
		"data: {\"delta\":\" world\"}",
		"",
		"data: [DONE]",
		"",
		"",
	}

	if len(lines) != len(expected) {
		t.Fatalf("got %d lines, want %d lines\nbody: %q", len(lines), len(expected), body)
	}

	for i, want := range expected {
		if lines[i] != want {
			t.Errorf("line %d: got %q, want %q", i, lines[i], want)
		}
	}
}

// nonFlushingWriter is a minimal ResponseWriter that doesn't implement http.Flusher
type nonFlushingWriter struct {
	buf bytes.Buffer
}

func (w *nonFlushingWriter) Header() http.Header {
	return http.Header{}
}

func (w *nonFlushingWriter) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}

func (w *nonFlushingWriter) WriteHeader(statusCode int) {}
