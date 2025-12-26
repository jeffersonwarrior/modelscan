package http

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// WebSocket implements RFC 6455 WebSocket protocol using only Go stdlib.
// This enables real-time bidirectional communication without external dependencies.
type WebSocket struct {
	conn   net.Conn
	reader *bufio.Reader
	mu     sync.Mutex // Protects writes
	closed bool
}

// WebSocket frame opcodes (RFC 6455)
const (
	opcodeContinuation = 0x0
	opcodeText         = 0x1
	opcodeBinary       = 0x2
	opcodeClose        = 0x8
	opcodePing         = 0x9
	opcodePong         = 0xA
)

var (
	// ErrWebSocketClosed is returned when operations are attempted on a closed WebSocket
	ErrWebSocketClosed = errors.New("websocket: connection closed")
	// ErrInvalidUpgrade is returned when the server doesn't accept the WebSocket upgrade
	ErrInvalidUpgrade = errors.New("websocket: invalid upgrade response")
)

// DialWebSocket establishes a WebSocket connection to the given URL.
// It performs the HTTP upgrade handshake per RFC 6455.
func DialWebSocket(ctx context.Context, urlStr string, headers map[string]string) (*WebSocket, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Determine host and port
	host := u.Host
	if !strings.Contains(host, ":") {
		if u.Scheme == "wss" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	// Establish TCP connection
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, fmt.Errorf("dial failed: %w", err)
	}

	// Generate WebSocket key (16 random bytes, base64 encoded)
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	wsKey := base64.StdEncoding.EncodeToString(key)

	// Build HTTP upgrade request
	path := u.Path
	if path == "" {
		path = "/"
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}

	req := fmt.Sprintf("GET %s HTTP/1.1\r\n", path)
	req += fmt.Sprintf("Host: %s\r\n", u.Host)
	req += "Upgrade: websocket\r\n"
	req += "Connection: Upgrade\r\n"
	req += fmt.Sprintf("Sec-WebSocket-Key: %s\r\n", wsKey)
	req += "Sec-WebSocket-Version: 13\r\n"

	// Add custom headers
	for k, v := range headers {
		req += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	req += "\r\n"

	// Send upgrade request
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send upgrade: %w", err)
	}

	// Read and validate response
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, &http.Request{Method: "GET"})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		conn.Close()
		return nil, fmt.Errorf("%w: got status %d", ErrInvalidUpgrade, resp.StatusCode)
	}

	// Verify WebSocket accept key
	expectedAccept := computeAcceptKey(wsKey)
	if resp.Header.Get("Sec-WebSocket-Accept") != expectedAccept {
		conn.Close()
		return nil, fmt.Errorf("%w: invalid accept key", ErrInvalidUpgrade)
	}

	return &WebSocket{
		conn:   conn,
		reader: reader,
	}, nil
}

// computeAcceptKey computes the Sec-WebSocket-Accept value per RFC 6455
func computeAcceptKey(key string) string {
	const magic = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	h := sha1.New()
	h.Write([]byte(key + magic))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// WriteText sends a text message over the WebSocket
func (ws *WebSocket) WriteText(data []byte) error {
	return ws.writeFrame(opcodeText, data)
}

// WriteBinary sends a binary message over the WebSocket
func (ws *WebSocket) WriteBinary(data []byte) error {
	return ws.writeFrame(opcodeBinary, data)
}

// ReadMessage reads the next message from the WebSocket.
// Returns the message data and opcode (text/binary).
func (ws *WebSocket) ReadMessage() ([]byte, int, error) {
	if ws.closed {
		return nil, 0, ErrWebSocketClosed
	}

	var message bytes.Buffer
	var opcode int

	for {
		frame, op, fin, err := ws.readFrame()
		if err != nil {
			return nil, 0, err
		}

		if opcode == 0 {
			opcode = op
		}

		// Handle control frames
		switch op {
		case opcodeClose:
			ws.Close()
			return nil, 0, io.EOF
		case opcodePing:
			// Respond with pong
			ws.writeFrame(opcodePong, frame)
			continue
		case opcodePong:
			// Ignore pongs
			continue
		}

		message.Write(frame)

		if fin {
			break
		}
	}

	return message.Bytes(), opcode, nil
}

// writeFrame writes a WebSocket frame with the given opcode and payload
func (ws *WebSocket) writeFrame(opcode int, payload []byte) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.closed {
		return ErrWebSocketClosed
	}

	// Frame header: FIN=1, RSV=0, opcode
	header := []byte{byte(0x80 | opcode)}

	// Payload length and masking
	length := len(payload)
	maskBit := byte(0x80) // Client must mask

	if length < 126 {
		header = append(header, maskBit|byte(length))
	} else if length < 65536 {
		header = append(header, maskBit|126)
		header = append(header, byte(length>>8), byte(length))
	} else {
		header = append(header, maskBit|127)
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(length))
		header = append(header, buf[:]...)
	}

	// Generate masking key
	maskKey := make([]byte, 4)
	rand.Read(maskKey)
	header = append(header, maskKey...)

	// Mask payload
	masked := make([]byte, length)
	for i := 0; i < length; i++ {
		masked[i] = payload[i] ^ maskKey[i%4]
	}

	// Send frame
	if _, err := ws.conn.Write(header); err != nil {
		return err
	}
	if _, err := ws.conn.Write(masked); err != nil {
		return err
	}

	return nil
}

// readFrame reads a single WebSocket frame
func (ws *WebSocket) readFrame() ([]byte, int, bool, error) {
	// Read first two bytes
	header := make([]byte, 2)
	if _, err := io.ReadFull(ws.reader, header); err != nil {
		return nil, 0, false, err
	}

	fin := header[0]&0x80 != 0
	opcode := int(header[0] & 0x0F)
	masked := header[1]&0x80 != 0
	length := int(header[1] & 0x7F)

	// Read extended payload length
	if length == 126 {
		buf := make([]byte, 2)
		if _, err := io.ReadFull(ws.reader, buf); err != nil {
			return nil, 0, false, err
		}
		length = int(binary.BigEndian.Uint16(buf))
	} else if length == 127 {
		buf := make([]byte, 8)
		if _, err := io.ReadFull(ws.reader, buf); err != nil {
			return nil, 0, false, err
		}
		length = int(binary.BigEndian.Uint64(buf))
	}

	// Read masking key if present (server frames shouldn't be masked)
	var maskKey []byte
	if masked {
		maskKey = make([]byte, 4)
		if _, err := io.ReadFull(ws.reader, maskKey); err != nil {
			return nil, 0, false, err
		}
	}

	// Read payload
	payload := make([]byte, length)
	if _, err := io.ReadFull(ws.reader, payload); err != nil {
		return nil, 0, false, err
	}

	// Unmask if needed
	if masked {
		for i := 0; i < length; i++ {
			payload[i] ^= maskKey[i%4]
		}
	}

	return payload, opcode, fin, nil
}

// Close closes the WebSocket connection
func (ws *WebSocket) Close() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.closed {
		return nil
	}

	ws.closed = true

	// Send close frame
	closeFrame := []byte{0x88, 0x00} // FIN=1, opcode=8, length=0, no mask
	ws.conn.Write(closeFrame)

	return ws.conn.Close()
}

// Ping sends a ping frame and waits for a pong response
func (ws *WebSocket) Ping() error {
	return ws.writeFrame(opcodePing, []byte{})
}
