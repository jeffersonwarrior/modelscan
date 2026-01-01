package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// ClientContextKey is the key for storing client in request context
type ClientContextKey struct{}

// MiddlewareClient represents a registered client for middleware context
type MiddlewareClient struct {
	ID           string
	Name         string
	Version      string
	Token        string
	Capabilities []string
	Config       MiddlewareClientConfig
	CreatedAt    time.Time
	LastSeenAt   *time.Time
}

// MiddlewareClientConfig holds client-specific configuration
type MiddlewareClientConfig struct {
	DefaultModel     string   `json:"default_model,omitempty"`
	ThinkingModel    string   `json:"thinking_model,omitempty"`
	MaxOutputTokens  int      `json:"max_output_tokens,omitempty"`
	TimeoutMs        int      `json:"timeout_ms,omitempty"`
	ProviderPriority []string `json:"provider_priority,omitempty"`
}

// ClientStore interface for client operations needed by middleware
type ClientStore interface {
	GetByToken(token string) (*MiddlewareClient, error)
	UpdateLastSeen(id string) error
}

// ClientMiddleware provides X-Client-Token validation and client tracking
type ClientMiddleware struct {
	store    ClientStore
	optional bool // If true, requests without token are allowed
}

// NewClientMiddleware creates a new client middleware
func NewClientMiddleware(store ClientStore, optional bool) *ClientMiddleware {
	return &ClientMiddleware{
		store:    store,
		optional: optional,
	}
}

// Wrap wraps an http.Handler with client token validation
func (m *ClientMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Client-Token")

		// No token provided
		if token == "" {
			if m.optional {
				// Allow request without client context
				next.ServeHTTP(w, r)
				return
			}
			writeErrorJSON(w, http.StatusUnauthorized, "missing X-Client-Token header")
			return
		}

		// Validate token and get client
		client, err := m.store.GetByToken(token)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "failed to validate token")
			return
		}

		if client == nil {
			writeErrorJSON(w, http.StatusUnauthorized, "invalid X-Client-Token")
			return
		}

		// Update last seen timestamp (async, non-blocking with timeout)
		go func(clientID string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Create a channel to receive the error
			done := make(chan error, 1)
			go func() {
				done <- m.store.UpdateLastSeen(clientID)
			}()

			select {
			case err := <-done:
				if err != nil {
					log.Printf("Failed to update last_seen for client %s: %v", clientID, err)
				}
			case <-ctx.Done():
				log.Printf("Timeout updating last_seen for client %s", clientID)
			}
		}(client.ID)

		// Add client to request context
		ctx := context.WithValue(r.Context(), ClientContextKey{}, client)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// WrapFunc wraps an http.HandlerFunc with client token validation
func (m *ClientMiddleware) WrapFunc(next http.HandlerFunc) http.HandlerFunc {
	return m.Wrap(next).ServeHTTP
}

// GetClientFromContext retrieves the client from the request context
func GetClientFromContext(ctx context.Context) *MiddlewareClient {
	client, ok := ctx.Value(ClientContextKey{}).(*MiddlewareClient)
	if !ok {
		return nil
	}
	return client
}

// RequireClient is a middleware that requires a valid client token
func RequireClient(store ClientStore) func(http.Handler) http.Handler {
	mw := NewClientMiddleware(store, false)
	return mw.Wrap
}

// OptionalClient is a middleware that allows requests without client token
func OptionalClient(store ClientStore) func(http.Handler) http.Handler {
	mw := NewClientMiddleware(store, true)
	return mw.Wrap
}

// writeErrorJSON writes an error response as JSON
func writeErrorJSON(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   http.StatusText(statusCode),
		"message": message,
	})
}

// RemapContextKey is the key for storing remap result in request context
type RemapContextKey struct{}

// RemapResult represents the result of a model remapping
type RemapResult struct {
	OriginalModel  string
	RemappedModel  string
	TargetProvider string
	RuleID         int
}

// RemapRuleStore interface for remap rule lookup operations
type RemapRuleStore interface {
	FindMatching(model string, clientID string) (*RemapRule, error)
}

// RemapMiddleware provides model remapping for proxy requests
type RemapMiddleware struct {
	store RemapRuleStore
}

// NewRemapMiddleware creates a new remap middleware
func NewRemapMiddleware(store RemapRuleStore) *RemapMiddleware {
	return &RemapMiddleware{store: store}
}

// isValidModelName validates model name format
// Allows alphanumeric, dashes, underscores, dots, slashes, and asterisk wildcards
// Prevents path traversal by disallowing ".." sequences
var modelNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-_./\*]*$`)

func isValidModelName(model string) bool {
	if len(model) == 0 || len(model) >= 256 {
		return false
	}
	// Check regex first
	if !modelNameRegex.MatchString(model) {
		return false
	}
	// Explicitly reject path traversal patterns
	if strings.Contains(model, "..") {
		return false
	}
	return true
}

// RemapModel implements the proxy.ModelRemapper interface
// It looks up remap rules and returns the remapped model and target provider
func (m *RemapMiddleware) RemapModel(ctx context.Context, model string, clientID string) (remappedModel, targetProvider string, err error) {
	// Validate model name format
	if !isValidModelName(model) {
		return "", "", fmt.Errorf("invalid model name format")
	}

	if clientID == "" {
		// No client ID, no remapping possible
		return model, "", nil
	}

	rule, err := m.store.FindMatching(model, clientID)
	if err != nil {
		return "", "", err
	}

	if rule == nil {
		// No matching rule, return original model
		return model, "", nil
	}

	return rule.ToModel, rule.ToProvider, nil
}

// Wrap wraps an http.Handler with model remapping
// It extracts model from request body, applies remapping rules,
// and stores the remap result in the request context
func (m *RemapMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client from context (if available)
		client := GetClientFromContext(r.Context())
		if client == nil {
			// No client in context, skip remapping
			next.ServeHTTP(w, r)
			return
		}

		// Remapping is handled by the proxy handlers directly via RemapModel
		// This middleware can be used to pre-populate context with remap info
		// For now, just pass through - the proxy will call RemapModel directly
		next.ServeHTTP(w, r)
	})
}

// WrapFunc wraps an http.HandlerFunc with model remapping
func (m *RemapMiddleware) WrapFunc(next http.HandlerFunc) http.HandlerFunc {
	return m.Wrap(next).ServeHTTP
}

// GetRemapResultFromContext retrieves the remap result from the request context
func GetRemapResultFromContext(ctx context.Context) *RemapResult {
	result, ok := ctx.Value(RemapContextKey{}).(*RemapResult)
	if !ok {
		return nil
	}
	return result
}

// RemapHandler is a convenience function that creates a remapper for use with proxy
func RemapHandler(store RemapRuleStore) *RemapMiddleware {
	return NewRemapMiddleware(store)
}
