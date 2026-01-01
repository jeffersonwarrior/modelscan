package admin

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// RateLimitStore interface for rate limit data operations
type RateLimitStore interface {
	Get(clientID string) (*ClientRateLimit, error)
	GetOrCreate(clientID string) (*ClientRateLimit, error)
	Create(rl *ClientRateLimit) error
	Update(rl *ClientRateLimit) error
	UpdateLimits(clientID string, rpmLimit, tpmLimit, dailyLimit *int) error
	Delete(clientID string) error
	List() ([]*ClientRateLimit, error)
	IncrementUsage(clientID string, requests, tokens int) error
	CheckLimits(clientID string) (bool, string, error)
	ResetMinuteCounters() error
	ResetDailyCounters() error
	Exists(clientID string) (bool, error)
}

// ClientRateLimit represents a client's rate limit configuration
type ClientRateLimit struct {
	ID           int        `json:"id"`
	ClientID     string     `json:"client_id"`
	RPMLimit     *int       `json:"rpm_limit,omitempty"`     // Requests per minute limit
	TPMLimit     *int       `json:"tpm_limit,omitempty"`     // Tokens per minute limit
	DailyLimit   *int       `json:"daily_limit,omitempty"`   // Daily request limit
	CurrentRPM   int        `json:"current_rpm"`             // Current requests this minute
	CurrentTPM   int        `json:"current_tpm"`             // Current tokens this minute
	CurrentDaily int        `json:"current_daily"`           // Current requests today
	LastReset    time.Time  `json:"last_reset"`              // When counters were last reset
}

// RateLimitAPI handles client rate limit management endpoints
type RateLimitAPI struct {
	store RateLimitStore
}

// NewRateLimitAPI creates a new RateLimitAPI
func NewRateLimitAPI(store RateLimitStore) *RateLimitAPI {
	return &RateLimitAPI{store: store}
}

// RateLimitCreateRequest represents the request body for creating/updating rate limits
type RateLimitCreateRequest struct {
	ClientID   string `json:"client_id"`
	RPMLimit   *int   `json:"rpm_limit,omitempty"`
	TPMLimit   *int   `json:"tpm_limit,omitempty"`
	DailyLimit *int   `json:"daily_limit,omitempty"`
}

// RateLimitUpdateRequest represents the request body for updating rate limits
type RateLimitUpdateRequest struct {
	RPMLimit   *int `json:"rpm_limit,omitempty"`
	TPMLimit   *int `json:"tpm_limit,omitempty"`
	DailyLimit *int `json:"daily_limit,omitempty"`
}

// RateLimitCheckResponse represents the response for rate limit check
type RateLimitCheckResponse struct {
	WithinLimits bool   `json:"within_limits"`
	LimitType    string `json:"limit_type,omitempty"` // "rpm", "tpm", or "daily" if exceeded
	RateLimit    *ClientRateLimit `json:"rate_limit,omitempty"`
}

// HandleRateLimits handles GET /api/ratelimits (list) and POST /api/ratelimits (create)
func (a *RateLimitAPI) HandleRateLimits(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleListRateLimits(w, r)
	case http.MethodPost:
		a.handleCreateRateLimit(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleListRateLimits handles GET /api/ratelimits
func (a *RateLimitAPI) handleListRateLimits(w http.ResponseWriter, r *http.Request) {
	limits, err := a.store.List()
	if err != nil {
		http.Error(w, "Failed to list rate limits: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if limits == nil {
		limits = []*ClientRateLimit{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"rate_limits": limits,
		"count":       len(limits),
	})
}

// handleCreateRateLimit handles POST /api/ratelimits
func (a *RateLimitAPI) handleCreateRateLimit(w http.ResponseWriter, r *http.Request) {
	var req RateLimitCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.ClientID == "" {
		http.Error(w, "client_id is required", http.StatusBadRequest)
		return
	}

	// Check if rate limit already exists
	exists, err := a.store.Exists(req.ClientID)
	if err != nil {
		http.Error(w, "Failed to check existing rate limit: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, "Rate limit already exists for client", http.StatusConflict)
		return
	}

	// Create rate limit
	rl := &ClientRateLimit{
		ClientID:     req.ClientID,
		RPMLimit:     req.RPMLimit,
		TPMLimit:     req.TPMLimit,
		DailyLimit:   req.DailyLimit,
		CurrentRPM:   0,
		CurrentTPM:   0,
		CurrentDaily: 0,
		LastReset:    time.Now(),
	}

	if err := a.store.Create(rl); err != nil {
		http.Error(w, "Failed to create rate limit: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rl)
}

// HandleRateLimitByClientID handles GET/PATCH/DELETE /api/ratelimits/{client_id}
// and GET /api/ratelimits/{client_id}/check
func (a *RateLimitAPI) HandleRateLimitByClientID(w http.ResponseWriter, r *http.Request) {
	// Extract client ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/ratelimits/")
	if path == "" {
		http.Error(w, "Client ID required", http.StatusBadRequest)
		return
	}

	// Check for /check suffix
	if strings.HasSuffix(path, "/check") {
		clientID := strings.TrimSuffix(path, "/check")
		a.handleCheckRateLimit(w, r, clientID)
		return
	}

	// Check for /reset suffix
	if strings.HasSuffix(path, "/reset") {
		clientID := strings.TrimSuffix(path, "/reset")
		a.handleResetRateLimit(w, r, clientID)
		return
	}

	clientID := path

	switch r.Method {
	case http.MethodGet:
		a.handleGetRateLimit(w, r, clientID)
	case http.MethodPatch:
		a.handleUpdateRateLimit(w, r, clientID)
	case http.MethodDelete:
		a.handleDeleteRateLimit(w, r, clientID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetRateLimit handles GET /api/ratelimits/{client_id}
func (a *RateLimitAPI) handleGetRateLimit(w http.ResponseWriter, r *http.Request, clientID string) {
	rl, err := a.store.Get(clientID)
	if err != nil {
		http.Error(w, "Failed to get rate limit: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if rl == nil {
		http.Error(w, "Rate limit not found for client", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rl)
}

// handleUpdateRateLimit handles PATCH /api/ratelimits/{client_id}
func (a *RateLimitAPI) handleUpdateRateLimit(w http.ResponseWriter, r *http.Request, clientID string) {
	// Get existing rate limit
	rl, err := a.store.Get(clientID)
	if err != nil {
		http.Error(w, "Failed to get rate limit: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if rl == nil {
		http.Error(w, "Rate limit not found for client", http.StatusNotFound)
		return
	}

	var req RateLimitUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Apply updates
	if req.RPMLimit != nil {
		rl.RPMLimit = req.RPMLimit
	}
	if req.TPMLimit != nil {
		rl.TPMLimit = req.TPMLimit
	}
	if req.DailyLimit != nil {
		rl.DailyLimit = req.DailyLimit
	}

	// Update rate limit
	if err := a.store.Update(rl); err != nil {
		http.Error(w, "Failed to update rate limit: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rl)
}

// handleDeleteRateLimit handles DELETE /api/ratelimits/{client_id}
func (a *RateLimitAPI) handleDeleteRateLimit(w http.ResponseWriter, r *http.Request, clientID string) {
	// Check if rate limit exists
	exists, err := a.store.Exists(clientID)
	if err != nil {
		http.Error(w, "Failed to check rate limit: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "Rate limit not found for client", http.StatusNotFound)
		return
	}

	// Delete rate limit
	if err := a.store.Delete(clientID); err != nil {
		http.Error(w, "Failed to delete rate limit: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleCheckRateLimit handles GET /api/ratelimits/{client_id}/check
func (a *RateLimitAPI) handleCheckRateLimit(w http.ResponseWriter, r *http.Request, clientID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	withinLimits, limitType, err := a.store.CheckLimits(clientID)
	if err != nil {
		http.Error(w, "Failed to check rate limits: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rl, err := a.store.Get(clientID)
	if err != nil {
		http.Error(w, "Failed to get rate limit: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := RateLimitCheckResponse{
		WithinLimits: withinLimits,
		LimitType:    limitType,
		RateLimit:    rl,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleResetRateLimit handles POST /api/ratelimits/{client_id}/reset
func (a *RateLimitAPI) handleResetRateLimit(w http.ResponseWriter, r *http.Request, clientID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get existing rate limit
	rl, err := a.store.Get(clientID)
	if err != nil {
		http.Error(w, "Failed to get rate limit: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if rl == nil {
		http.Error(w, "Rate limit not found for client", http.StatusNotFound)
		return
	}

	// Reset counters
	rl.CurrentRPM = 0
	rl.CurrentTPM = 0
	rl.CurrentDaily = 0
	rl.LastReset = time.Now()

	if err := a.store.Update(rl); err != nil {
		http.Error(w, "Failed to reset rate limit: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rl)
}

// RateLimitMiddleware provides rate limiting enforcement
type RateLimitMiddleware struct {
	store RateLimitStore
}

// NewRateLimitMiddleware creates a new rate limit middleware
func NewRateLimitMiddleware(store RateLimitStore) *RateLimitMiddleware {
	return &RateLimitMiddleware{store: store}
}

// Wrap wraps an http.Handler with rate limiting
func (m *RateLimitMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client from context (if using ClientMiddleware)
		client := GetClientFromContext(r.Context())
		if client == nil {
			// No client in context, allow request without rate limiting
			next.ServeHTTP(w, r)
			return
		}

		// Check rate limits
		withinLimits, limitType, err := m.store.CheckLimits(client.ID)
		if err != nil {
			// On error, allow request but log
			next.ServeHTTP(w, r)
			return
		}

		if !withinLimits {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-RateLimit-Type", limitType)
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":      "rate_limit_exceeded",
				"limit_type": limitType,
				"message":    "Rate limit exceeded for client",
			})
			return
		}

		// Increment usage counter (1 request, 0 tokens - tokens counted later)
		if err := m.store.IncrementUsage(client.ID, 1, 0); err != nil {
			// Log but don't block request - usage tracking is best-effort
			log.Printf("Warning: failed to increment usage for client %s: %v", client.ID, err)
		}

		next.ServeHTTP(w, r)
	})
}

// WrapFunc wraps an http.HandlerFunc with rate limiting
func (m *RateLimitMiddleware) WrapFunc(next http.HandlerFunc) http.HandlerFunc {
	return m.Wrap(next).ServeHTTP
}

// RecordTokens records token usage for a client after a request completes
func (m *RateLimitMiddleware) RecordTokens(clientID string, tokens int) error {
	return m.store.IncrementUsage(clientID, 0, tokens)
}
