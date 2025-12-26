package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestComputeAcceptKey verifies RFC 6455 WebSocket key computation
func TestComputeAcceptKey(t *testing.T) {
	tests := []struct {
		key    string
		expect string
	}{
		{
			key:    "dGhlIHNhbXBsZSBub25jZQ==",
			expect: "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=",
		},
		{
			key:    "x3JJHMbDL1EzLkh9GBhXDw==",
			expect: "HSmrc0sMlYUkAGmm5OPpG2HaGWk=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := computeAcceptKey(tt.key)
			if result != tt.expect {
				t.Errorf("computeAcceptKey(%q) = %q, want %q", tt.key, result, tt.expect)
			}
		})
	}
}

// TestDialWebSocket_InvalidURL tests error handling for invalid URLs
func TestDialWebSocket_InvalidURL(t *testing.T) {
	ctx := context.Background()

	_, err := DialWebSocket(ctx, "://invalid", nil)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "invalid URL") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestDialWebSocket_ConnectionRefused tests connection failure handling
func TestDialWebSocket_ConnectionRefused(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Try to connect to a port that's definitely not listening
	_, err := DialWebSocket(ctx, "ws://localhost:9999", nil)
	if err == nil {
		t.Fatal("expected connection error")
	}
}

// TestWebSocket_WriteReadText tests text message exchange
func TestWebSocket_WriteReadText(t *testing.T) {
	server, client := createTestConnection(t)
	defer server.Close()
	defer client.Close()

	testMsg := []byte("Hello, WebSocket!")

	// Write from client
	if err := client.WriteText(testMsg); err != nil {
		t.Fatalf("WriteText failed: %v", err)
	}

	// Read on server
	data, opcode, err := server.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}
	if opcode != opcodeText {
		t.Errorf("opcode = %d, want %d (text)", opcode, opcodeText)
	}
	if string(data) != string(testMsg) {
		t.Errorf("received %q, want %q", data, testMsg)
	}
}

// TestWebSocket_WriteReadBinary tests binary message exchange
func TestWebSocket_WriteReadBinary(t *testing.T) {
	server, client := createTestConnection(t)
	defer server.Close()
	defer client.Close()

	testData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}

	// Write from client
	if err := client.WriteBinary(testData); err != nil {
		t.Fatalf("WriteBinary failed: %v", err)
	}

	// Read on server
	data, opcode, err := server.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}
	if opcode != opcodeBinary {
		t.Errorf("opcode = %d, want %d (binary)", opcode, opcodeBinary)
	}
	if len(data) != len(testData) {
		t.Fatalf("length = %d, want %d", len(data), len(testData))
	}
	for i := range data {
		if data[i] != testData[i] {
			t.Errorf("data[%d] = %02x, want %02x", i, data[i], testData[i])
		}
	}
}

// TestWebSocket_Ping tests ping/pong mechanism
func TestWebSocket_Ping(t *testing.T) {
	server, client := createTestConnection(t)
	defer server.Close()
	defer client.Close()

	// Start server pong responder
	go func() {
		for {
			_, _, err := server.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	// Client sends ping
	if err := client.Ping(); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	// Give time for pong response
	time.Sleep(100 * time.Millisecond)
}

// TestWebSocket_Close tests clean connection closure
func TestWebSocket_Close(t *testing.T) {
	server, client := createTestConnection(t)

	// Close client
	if err := client.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify closed state
	if err := client.WriteText([]byte("test")); err != ErrWebSocketClosed {
		t.Errorf("expected ErrWebSocketClosed, got %v", err)
	}

	server.Close()
}

// TestWebSocket_LargeMessage tests large message handling
func TestWebSocket_LargeMessage(t *testing.T) {
	server, client := createTestConnection(t)
	defer server.Close()
	defer client.Close()

	// Create 1MB message
	largeMsg := make([]byte, 1024*1024)
	for i := range largeMsg {
		largeMsg[i] = byte(i % 256)
	}

	// Write from client
	if err := client.WriteBinary(largeMsg); err != nil {
		t.Fatalf("WriteBinary failed: %v", err)
	}

	// Read on server
	data, opcode, err := server.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}
	if opcode != opcodeBinary {
		t.Errorf("opcode = %d, want %d", opcode, opcodeBinary)
	}
	if len(data) != len(largeMsg) {
		t.Fatalf("length = %d, want %d", len(data), len(largeMsg))
	}
}

// TestWebSocket_Concurrent tests thread-safety
func TestWebSocket_Concurrent(t *testing.T) {
	server, client := createTestConnection(t)
	defer server.Close()
	defer client.Close()

	// Start reader
	go func() {
		for {
			_, _, err := server.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	// Send messages concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			msg := []byte(fmt.Sprintf("message-%d", n))
			if err := client.WriteText(msg); err != nil {
				t.Errorf("WriteText failed: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all sends
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestWebSocket_EmptyMessage tests zero-length payload
func TestWebSocket_EmptyMessage(t *testing.T) {
	server, client := createTestConnection(t)
	defer server.Close()
	defer client.Close()

	if err := client.WriteText([]byte{}); err != nil {
		t.Fatalf("WriteText failed: %v", err)
	}

	data, opcode, err := server.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}
	if opcode != opcodeText {
		t.Errorf("opcode = %d, want %d", opcode, opcodeText)
	}
	if len(data) != 0 {
		t.Errorf("length = %d, want 0", len(data))
	}
}

// TestWebSocket_MultipleClose tests idempotent close
func TestWebSocket_MultipleClose(t *testing.T) {
	server, client := createTestConnection(t)
	defer server.Close()

	if err := client.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}

	// Second close should be no-op
	if err := client.Close(); err != nil {
		t.Errorf("second Close failed: %v", err)
	}
}

// TestWebSocket_ContextCancellation tests context cancellation during dial
func TestWebSocket_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := DialWebSocket(ctx, "ws://localhost:9999", nil)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// createTestConnection creates a connected client-server WebSocket pair for testing
func createTestConnection(t *testing.T) (*WebSocket, *WebSocket) {
	// Channel to pass server WebSocket to test
	serverChan := make(chan *WebSocket, 1)

	// Create HTTP server that upgrades to WebSocket
	upgradeHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify upgrade headers
		if r.Header.Get("Upgrade") != "websocket" {
			http.Error(w, "Expected websocket upgrade", http.StatusBadRequest)
			return
		}

		key := r.Header.Get("Sec-WebSocket-Key")
		if key == "" {
			http.Error(w, "Missing Sec-WebSocket-Key", http.StatusBadRequest)
			return
		}

		// Send upgrade response
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
			return
		}

		conn, bufrw, err := hijacker.Hijack()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		accept := computeAcceptKey(key)
		resp := fmt.Sprintf("HTTP/1.1 101 Switching Protocols\r\n"+
			"Upgrade: websocket\r\n"+
			"Connection: Upgrade\r\n"+
			"Sec-WebSocket-Accept: %s\r\n"+
			"\r\n", accept)

		if _, err := conn.Write([]byte(resp)); err != nil {
			conn.Close()
			return
		}

		// Create server-side WebSocket
		serverWS := &WebSocket{
			conn:   conn,
			reader: bufrw.Reader,
		}

		// Send to test
		serverChan <- serverWS
	})

	server := httptest.NewServer(upgradeHandler)

	// Convert http:// to ws://
	wsURL := "ws://" + strings.TrimPrefix(server.URL, "http://")

	// Client connects
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := DialWebSocket(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("DialWebSocket failed: %v", err)
	}

	// Wait for server WebSocket
	var serverWS *WebSocket
	select {
	case serverWS = <-serverChan:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for server WebSocket")
	}

	return serverWS, client
}

// TestWebSocket_MediumPayload tests 16-bit extended length encoding
func TestWebSocket_MediumPayload(t *testing.T) {
	server, client := createTestConnection(t)
	defer server.Close()
	defer client.Close()

	// Create message larger than 125 bytes but smaller than 65536
	mediumMsg := make([]byte, 1000)
	for i := range mediumMsg {
		mediumMsg[i] = byte(i % 256)
	}

	if err := client.WriteBinary(mediumMsg); err != nil {
		t.Fatalf("WriteBinary failed: %v", err)
	}

	data, opcode, err := server.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}
	if opcode != opcodeBinary {
		t.Errorf("opcode = %d, want %d", opcode, opcodeBinary)
	}
	if len(data) != len(mediumMsg) {
		t.Fatalf("length = %d, want %d", len(data), len(mediumMsg))
	}
}
