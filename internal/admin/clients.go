package admin

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jeffersonwarrior/modelscan/internal/database"
)

// ClientAPI handles client registration and management endpoints
type ClientAPI struct {
	clientRepo *database.ClientRepository
	statsStore StatsStore
}

// NewClientAPI creates a new ClientAPI
func NewClientAPI(clientRepo *database.ClientRepository) *ClientAPI {
	return &ClientAPI{clientRepo: clientRepo}
}

// ClientRegistrationRequest represents the request body for client registration
type ClientRegistrationRequest struct {
	Name         string                  `json:"name"`
	Version      string                  `json:"version"`
	Capabilities []string                `json:"capabilities,omitempty"`
	Config       *database.ClientConfig  `json:"config,omitempty"`
}

// ClientResponse represents a client in API responses (token hidden for list operations)
type ClientResponse struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Token        string                 `json:"token,omitempty"`
	Capabilities []string               `json:"capabilities"`
	Config       database.ClientConfig  `json:"config"`
	CreatedAt    time.Time              `json:"created_at"`
	LastSeenAt   *time.Time             `json:"last_seen_at,omitempty"`
}

// ClientRegistrationResponse represents the response after successful registration
type ClientRegistrationResponse struct {
	Client ClientResponse `json:"client"`
	Token  string         `json:"token"` // Only returned on registration
}

// ConfigUpdateRequest represents the request body for config updates
type ConfigUpdateRequest struct {
	DefaultModel     *string   `json:"default_model,omitempty"`
	ThinkingModel    *string   `json:"thinking_model,omitempty"`
	MaxOutputTokens  *int      `json:"max_output_tokens,omitempty"`
	TimeoutMs        *int      `json:"timeout_ms,omitempty"`
	ProviderPriority *[]string `json:"provider_priority,omitempty"`
}

// generateToken creates a cryptographically secure random token
func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// generateClientID creates a client ID from name and random suffix
func generateClientID(name string) (string, error) {
	// Sanitize name: lowercase, replace spaces/special chars with hyphens
	sanitized := strings.ToLower(name)
	sanitized = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, sanitized)
	// Remove consecutive hyphens (single pass)
	var result strings.Builder
	result.Grow(len(sanitized))
	prevHyphen := false
	for _, r := range sanitized {
		if r == '-' {
			if !prevHyphen {
				result.WriteRune(r)
				prevHyphen = true
			}
		} else {
			result.WriteRune(r)
			prevHyphen = false
		}
	}
	sanitized = result.String()
	sanitized = strings.Trim(sanitized, "-")
	if sanitized == "" {
		sanitized = "client"
	}

	// Add random suffix
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return "", err
	}
	return sanitized + "-" + hex.EncodeToString(suffix), nil
}

// HandleRegister handles POST /api/clients/register
func (c *ClientAPI) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ClientRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.Version == "" {
		req.Version = "unknown"
	}

	// Generate client ID and token
	clientID, err := generateClientID(req.Name)
	if err != nil {
		http.Error(w, "Failed to generate client ID", http.StatusInternalServerError)
		return
	}

	token, err := generateToken()
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Build client
	now := time.Now()
	client := &database.Client{
		ID:           clientID,
		Name:         req.Name,
		Version:      req.Version,
		Token:        token,
		Capabilities: req.Capabilities,
		CreatedAt:    now,
		LastSeenAt:   &now,
	}
	if client.Capabilities == nil {
		client.Capabilities = []string{}
	}
	if req.Config != nil {
		client.Config = *req.Config
	}

	// Store in database
	if err := c.clientRepo.Create(client); err != nil {
		http.Error(w, "Failed to create client: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return response with token
	resp := ClientRegistrationResponse{
		Client: ClientResponse{
			ID:           client.ID,
			Name:         client.Name,
			Version:      client.Version,
			Token:        client.Token,
			Capabilities: client.Capabilities,
			Config:       client.Config,
			CreatedAt:    client.CreatedAt,
			LastSeenAt:   client.LastSeenAt,
		},
		Token: token,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// HandleGetClient handles GET /api/clients/{id}
func (c *ClientAPI) HandleGetClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract client ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/clients/")
	clientID := strings.TrimSuffix(path, "/config") // Handle /config suffix
	if clientID == "" || clientID == "register" {
		http.Error(w, "Client ID required", http.StatusBadRequest)
		return
	}
	// Validate client ID to prevent path traversal attacks
	if strings.Contains(clientID, "/") || strings.Contains(clientID, "..") || strings.Contains(clientID, "\\") {
		http.Error(w, "Invalid client ID", http.StatusBadRequest)
		return
	}

	client, err := c.clientRepo.Get(clientID)
	if err != nil {
		http.Error(w, "Failed to get client: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if client == nil {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}

	// Update last seen
	_ = c.clientRepo.UpdateLastSeen(clientID)

	resp := ClientResponse{
		ID:           client.ID,
		Name:         client.Name,
		Version:      client.Version,
		Capabilities: client.Capabilities,
		Config:       client.Config,
		CreatedAt:    client.CreatedAt,
		LastSeenAt:   client.LastSeenAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleUpdateConfig handles PATCH /api/clients/{id}/config
func (c *ClientAPI) HandleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract client ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/clients/")
	clientID := strings.TrimSuffix(path, "/config")
	if clientID == "" {
		http.Error(w, "Client ID required", http.StatusBadRequest)
		return
	}

	// Get existing client
	client, err := c.clientRepo.Get(clientID)
	if err != nil {
		http.Error(w, "Failed to get client: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if client == nil {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}

	// Parse update request
	var req ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Apply partial updates
	config := client.Config
	if req.DefaultModel != nil {
		config.DefaultModel = *req.DefaultModel
	}
	if req.ThinkingModel != nil {
		config.ThinkingModel = *req.ThinkingModel
	}
	if req.MaxOutputTokens != nil {
		config.MaxOutputTokens = *req.MaxOutputTokens
	}
	if req.TimeoutMs != nil {
		config.TimeoutMs = *req.TimeoutMs
	}
	if req.ProviderPriority != nil {
		config.ProviderPriority = *req.ProviderPriority
	}

	// Update config
	if err := c.clientRepo.UpdateConfig(clientID, config); err != nil {
		http.Error(w, "Failed to update config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return updated config
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// HandleDeleteClient handles DELETE /api/clients/{id}
func (c *ClientAPI) HandleDeleteClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract client ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/clients/")
	clientID := path
	if clientID == "" {
		http.Error(w, "Client ID required", http.StatusBadRequest)
		return
	}
	// Validate client ID to prevent path traversal attacks
	if strings.Contains(clientID, "/") || strings.Contains(clientID, "..") || strings.Contains(clientID, "\\") {
		http.Error(w, "Invalid client ID", http.StatusBadRequest)
		return
	}

	// Check if client exists
	exists, err := c.clientRepo.Exists(clientID)
	if err != nil {
		http.Error(w, "Failed to check client: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}

	// Delete client
	if err := c.clientRepo.Delete(clientID); err != nil {
		http.Error(w, "Failed to delete client: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleListClients handles GET /api/clients
func (c *ClientAPI) HandleListClients(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clients, err := c.clientRepo.List()
	if err != nil {
		http.Error(w, "Failed to list clients: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to response format (without tokens for security)
	var resp []ClientResponse
	for _, client := range clients {
		resp = append(resp, ClientResponse{
			ID:           client.ID,
			Name:         client.Name,
			Version:      client.Version,
			Capabilities: client.Capabilities,
			Config:       client.Config,
			CreatedAt:    client.CreatedAt,
			LastSeenAt:   client.LastSeenAt,
		})
	}
	if resp == nil {
		resp = []ClientResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"clients": resp,
		"count":   len(resp),
	})
}

// ClientStatsResponse represents statistics for a client
type ClientStatsResponse struct {
	ClientID         string           `json:"client_id"`
	TotalRequests    int              `json:"total_requests"`
	TotalTokens      int64            `json:"total_tokens"`
	RequestTokens    int64            `json:"request_tokens"`
	ResponseTokens   int64            `json:"response_tokens"`
	AvgLatencyMs     float64          `json:"avg_latency_ms"`
	SuccessCount     int              `json:"success_count"`
	ErrorCount       int              `json:"error_count"`
	SuccessRate      float64          `json:"success_rate"`
	TokensByProvider map[string]int64 `json:"tokens_by_provider"`
	RequestsByModel  map[string]int   `json:"requests_by_model"`
}

// StatsStore defines the interface for retrieving client stats
type StatsStore interface {
	GetClientStats(clientID string) (*database.RequestLogStats, error)
}

// SetStatsStore sets the stats store for retrieving client statistics
func (c *ClientAPI) SetStatsStore(store StatsStore) {
	c.statsStore = store
}

// HandleGetStats handles GET /api/clients/{id}/stats
func (c *ClientAPI) HandleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract client ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/clients/")
	clientID := strings.TrimSuffix(path, "/stats")
	if clientID == "" {
		http.Error(w, "Client ID required", http.StatusBadRequest)
		return
	}

	// Check if client exists
	client, err := c.clientRepo.Get(clientID)
	if err != nil {
		http.Error(w, "Failed to get client: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if client == nil {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}

	// Check if stats store is configured
	if c.statsStore == nil {
		http.Error(w, "Stats not available", http.StatusServiceUnavailable)
		return
	}

	// Get stats for client
	stats, err := c.statsStore.GetClientStats(clientID)
	if err != nil {
		http.Error(w, "Failed to get stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to response format
	resp := ClientStatsResponse{
		ClientID:         clientID,
		TotalRequests:    stats.TotalRequests,
		TotalTokens:      stats.TotalTokens,
		RequestTokens:    stats.RequestTokens,
		ResponseTokens:   stats.ResponseTokens,
		AvgLatencyMs:     stats.AvgLatencyMs,
		SuccessCount:     stats.SuccessCount,
		ErrorCount:       stats.ErrorCount,
		SuccessRate:      stats.SuccessRate,
		TokensByProvider: stats.TokensByProvider,
		RequestsByModel:  stats.RequestsByModel,
	}

	// Ensure maps are not nil
	if resp.TokensByProvider == nil {
		resp.TokensByProvider = make(map[string]int64)
	}
	if resp.RequestsByModel == nil {
		resp.RequestsByModel = make(map[string]int)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
