package admin

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// API provides HTTP endpoints for admin operations
type API struct {
	mux          *http.ServeMux
	db           Database
	discovery    DiscoveryAgent
	generator    Generator
	keyManager   KeyManager
	clientAPI    *ClientAPI
	aliasAPI     *AliasAPI
	remapAPI     *RemapAPI
	rateLimitAPI *RateLimitAPI
	serverAPI    *ServerAPI
	modelService ModelService
}

// Database interface for data operations
type Database interface {
	CreateProvider(p *Provider) error
	GetProvider(id string) (*Provider, error)
	ListProviders() ([]*Provider, error)
	CreateAPIKey(providerID, apiKey string) (*APIKey, error)
	GetAPIKey(id int) (*APIKey, error)
	DeleteAPIKey(id int) error
	ListActiveAPIKeys(providerID string) ([]*APIKey, error)
	GetUsageStats(modelID string, since time.Time) (map[string]interface{}, error)
	GetKeyStats(keyID int, since time.Time) (*KeyStats, error)
}

// DiscoveryAgent interface for provider discovery
type DiscoveryAgent interface {
	Discover(providerID string, apiKey string) (*DiscoveryResult, error)
}

// Generator interface for SDK generation
type Generator interface {
	Generate(req GenerateRequest) (*GenerateResult, error)
	List() ([]string, error)
	Delete(providerID string) error
}

// KeyManager interface for key management
type KeyManager interface {
	GetKey(providerID string) (*APIKey, error)
	ListKeys(providerID string) ([]*APIKey, error)
	RegisterActualKey(keyHash, actualKey string)
	TestKey(keyID int) (*KeyTestResult, error)
}

// KeyTestResult represents the result of testing an API key
type KeyTestResult struct {
	Valid              bool     `json:"valid"`
	RateLimitRemaining int      `json:"rate_limit_remaining,omitempty"`
	ModelsAccessible   []string `json:"models_accessible,omitempty"`
	Error              string   `json:"error,omitempty"`
}

// KeyStats represents usage statistics for an API key
type KeyStats struct {
	RequestsToday    int     `json:"requests_today"`
	TokensToday      int     `json:"tokens_today"`
	RateLimitPercent float64 `json:"rate_limit_percent"`
	DegradationCount int     `json:"degradation_count"`
}

// Provider represents a provider
type Provider struct {
	ID           string
	Name         string
	BaseURL      string
	AuthMethod   string
	PricingModel string
	Status       string
}

// APIKey represents an API key
type APIKey struct {
	ID            int
	ProviderID    string
	KeyHash       string // SHA256 hash of the key (not exposed in JSON)
	KeyPrefix     *string
	RequestsCount int
	TokensCount   int
	Active        bool
	Degraded      bool
}

// DiscoveryResult represents discovery results
type DiscoveryResult struct {
	ProviderID    string
	ProviderName  string
	BaseURL       string
	AuthMethod    string
	AuthHeader    string
	PricingModel  string
	Documentation string
	SDKType       string
	Success       bool
	Message       string
}

// GenerateRequest represents SDK generation request
type GenerateRequest struct {
	ProviderID   string
	ProviderName string
	BaseURL      string
	SDKType      string
}

// GenerateResult represents SDK generation result
type GenerateResult struct {
	FilePath string
	Success  bool
	Error    error
}

// Config holds API configuration
type Config struct {
	Host string
	Port int
}

// NewAPI creates a new admin API
func NewAPI(cfg Config, db Database, discovery DiscoveryAgent, generator Generator, keyManager KeyManager) *API {
	api := &API{
		mux:        http.NewServeMux(),
		db:         db,
		discovery:  discovery,
		generator:  generator,
		keyManager: keyManager,
	}

	api.setupRoutes()
	return api
}

// SetClientAPI sets the client API handler
func (a *API) SetClientAPI(clientAPI *ClientAPI) {
	a.clientAPI = clientAPI
}

// SetAliasAPI sets the alias API handler
func (a *API) SetAliasAPI(aliasAPI *AliasAPI) {
	a.aliasAPI = aliasAPI
}

// SetModelService sets the model service for hierarchical model listing
func (a *API) SetModelService(svc ModelService) {
	a.modelService = svc
}

// SetRemapAPI sets the remap API handler
func (a *API) SetRemapAPI(remapAPI *RemapAPI) {
	a.remapAPI = remapAPI
}

// SetServerAPI sets the server API handler
func (a *API) SetServerAPI(serverAPI *ServerAPI) {
	a.serverAPI = serverAPI
}

// SetRateLimitAPI sets the rate limit API handler
func (a *API) SetRateLimitAPI(rateLimitAPI *RateLimitAPI) {
	a.rateLimitAPI = rateLimitAPI
}

// setupRoutes configures HTTP routes
func (a *API) setupRoutes() {
	// Provider management
	a.mux.HandleFunc("/api/providers", a.handleProviders)
	a.mux.HandleFunc("/api/providers/add", a.handleAddProvider)

	// API key management
	a.mux.HandleFunc("/api/keys", a.handleKeys)
	a.mux.HandleFunc("/api/keys/add", a.handleAddKey)
	a.mux.HandleFunc("/api/keys/", a.handleKeyByID)

	// Discovery
	a.mux.HandleFunc("/api/discover", a.handleDiscover)

	// SDK management
	a.mux.HandleFunc("/api/sdks", a.handleSDKs)
	a.mux.HandleFunc("/api/sdks/generate", a.handleGenerateSDK)

	// Usage stats
	a.mux.HandleFunc("/api/stats", a.handleStats)

	// Models (hierarchical)
	a.mux.HandleFunc("/api/models", a.handleModels)

	// Client management
	a.mux.HandleFunc("/api/clients/register", a.handleClientsRegister)
	a.mux.HandleFunc("/api/clients", a.handleClients)
	a.mux.HandleFunc("/api/clients/", a.handleClientByID)

	// Alias management
	a.mux.HandleFunc("/api/aliases", a.handleAliases)
	a.mux.HandleFunc("/api/aliases/", a.handleAliasByName)

	// Remap rules management
	a.mux.HandleFunc("/api/rules/remap", a.handleRemaps)
	a.mux.HandleFunc("/api/rules/remap/", a.handleRemapByID)

	// Rate limit management
	a.mux.HandleFunc("/api/ratelimits", a.handleRateLimits)
	a.mux.HandleFunc("/api/ratelimits/", a.handleRateLimitByClientID)

	// Server info and control
	a.mux.HandleFunc("/api/server/info", a.handleServerInfo)
	a.mux.HandleFunc("/api/server/shutdown", a.handleServerShutdown)

	// Health check
	a.mux.HandleFunc("/health", a.handleHealth)
}

// handleClientsRegister handles POST /api/clients/register
func (a *API) handleClientsRegister(w http.ResponseWriter, r *http.Request) {
	if a.clientAPI == nil {
		http.Error(w, "Client API not configured", http.StatusServiceUnavailable)
		return
	}
	a.clientAPI.HandleRegister(w, r)
}

// handleClients handles GET /api/clients (list all clients)
func (a *API) handleClients(w http.ResponseWriter, r *http.Request) {
	if a.clientAPI == nil {
		http.Error(w, "Client API not configured", http.StatusServiceUnavailable)
		return
	}
	a.clientAPI.HandleListClients(w, r)
}

// handleClientByID routes requests for /api/clients/{id}, /api/clients/{id}/config, and /api/clients/{id}/stats
func (a *API) handleClientByID(w http.ResponseWriter, r *http.Request) {
	if a.clientAPI == nil {
		http.Error(w, "Client API not configured", http.StatusServiceUnavailable)
		return
	}

	// Route based on path and method
	path := r.URL.Path
	switch {
	case r.Method == http.MethodGet && isClientStatsPath(path):
		a.clientAPI.HandleGetStats(w, r)
	case r.Method == http.MethodGet && !isConfigPath(path):
		a.clientAPI.HandleGetClient(w, r)
	case r.Method == http.MethodPatch && isConfigPath(path):
		a.clientAPI.HandleUpdateConfig(w, r)
	case r.Method == http.MethodDelete && !isConfigPath(path) && !isClientStatsPath(path):
		a.clientAPI.HandleDeleteClient(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// isConfigPath checks if the path ends with /config
func isConfigPath(path string) bool {
	return len(path) > 7 && path[len(path)-7:] == "/config"
}

// isClientStatsPath checks if the path ends with /stats
func isClientStatsPath(path string) bool {
	return len(path) > 6 && path[len(path)-6:] == "/stats"
}

// handleAliases handles GET/POST /api/aliases
func (a *API) handleAliases(w http.ResponseWriter, r *http.Request) {
	if a.aliasAPI == nil {
		http.Error(w, "Alias API not configured", http.StatusServiceUnavailable)
		return
	}
	a.aliasAPI.HandleAliases(w, r)
}

// handleAliasByName handles GET/PUT/DELETE /api/aliases/{name}
func (a *API) handleAliasByName(w http.ResponseWriter, r *http.Request) {
	if a.aliasAPI == nil {
		http.Error(w, "Alias API not configured", http.StatusServiceUnavailable)
		return
	}
	a.aliasAPI.HandleAliasByName(w, r)
}

// handleRemaps handles GET/POST /api/rules/remap
func (a *API) handleRemaps(w http.ResponseWriter, r *http.Request) {
	if a.remapAPI == nil {
		http.Error(w, "Remap API not configured", http.StatusServiceUnavailable)
		return
	}
	a.remapAPI.HandleRemaps(w, r)
}

// handleRemapByID handles GET/PATCH/DELETE /api/rules/remap/{id}
func (a *API) handleRemapByID(w http.ResponseWriter, r *http.Request) {
	if a.remapAPI == nil {
		http.Error(w, "Remap API not configured", http.StatusServiceUnavailable)
		return
	}
	a.remapAPI.HandleRemapByID(w, r)
}

// handleProviders lists all providers
func (a *API) handleProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	providers, err := a.db.ListProviders()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"providers": providers,
		"count":     len(providers),
	})
}

// handleAddProvider adds a new provider
func (a *API) handleAddProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Identifier string `json:"identifier"` // model ID or URL
		APIKey     string `json:"api_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Trigger discovery
	result, err := a.discovery.Discover(req.Identifier, req.APIKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Save provider if discovery was successful
	if result.Success {
		provider := &Provider{
			ID:           result.ProviderID,
			Name:         result.ProviderName,
			BaseURL:      result.BaseURL,
			AuthMethod:   result.AuthMethod,
			PricingModel: result.PricingModel,
			Status:       "online",
		}

		if err := a.db.CreateProvider(provider); err != nil {
			// Provider might already exist - log but continue
			log.Printf("Warning: failed to create provider %s: %v (may already exist)", provider.ID, err)
		}

		// Add the API key if provided
		if req.APIKey != "" {
			_, err := a.db.CreateAPIKey(result.ProviderID, req.APIKey)
			if err != nil {
				// Key might already exist - log but continue
				log.Printf("Warning: failed to create API key for %s: %v (may already exist)", result.ProviderID, err)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleKeys lists API keys for a provider
func (a *API) handleKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	providerID := r.URL.Query().Get("provider")
	if providerID == "" {
		http.Error(w, "provider parameter required", http.StatusBadRequest)
		return
	}

	keys, err := a.keyManager.ListKeys(providerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"keys":  keys,
		"count": len(keys),
	})
}

// handleAddKey adds a new API key
func (a *API) handleAddKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ProviderID string `json:"provider_id"`
		APIKey     string `json:"api_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	key, err := a.db.CreateAPIKey(req.ProviderID, req.APIKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Register the actual key value in memory for proxy functionality
	// SECURITY NOTE: Stores plaintext key in memory - necessary for proxy but creates security risk
	a.keyManager.RegisterActualKey(key.KeyHash, req.APIKey)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(key)
}

// handleDiscover triggers discovery for a provider
func (a *API) handleDiscover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Identifier string `json:"identifier"`
		APIKey     string `json:"api_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result, err := a.discovery.Discover(req.Identifier, req.APIKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleSDKs lists generated SDKs
func (a *API) handleSDKs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sdks, err := a.generator.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sdks":  sdks,
		"count": len(sdks),
	})
}

// handleGenerateSDK generates an SDK
func (a *API) handleGenerateSDK(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result, err := a.generator.Generate(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleStats retrieves usage statistics
func (a *API) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	modelID := r.URL.Query().Get("model")
	if modelID == "" {
		http.Error(w, "model parameter required", http.StatusBadRequest)
		return
	}

	since := time.Now().AddDate(0, 0, -7) // Last 7 days
	stats, err := a.db.GetUsageStats(modelID, since)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleKeyByID routes requests for /api/keys/{id}, /api/keys/{id}/test, and /api/keys/{id}/stats
func (a *API) handleKeyByID(w http.ResponseWriter, r *http.Request) {
	// Parse path: /api/keys/{id} or /api/keys/{id}/test or /api/keys/{id}/stats
	path := strings.TrimPrefix(r.URL.Path, "/api/keys/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "key id required", http.StatusBadRequest)
		return
	}

	keyID, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "invalid key id", http.StatusBadRequest)
		return
	}

	// Route based on path suffix
	if len(parts) >= 2 {
		switch parts[1] {
		case "test":
			a.handleKeyTest(w, r, keyID)
			return
		case "stats":
			a.handleKeyStats(w, r, keyID)
			return
		}
	}

	// Route based on HTTP method
	switch r.Method {
	case http.MethodGet:
		a.handleGetKey(w, r, keyID)
	case http.MethodDelete:
		a.handleDeleteKey(w, r, keyID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetKey handles GET /api/keys/{id}
func (a *API) handleGetKey(w http.ResponseWriter, r *http.Request, keyID int) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key, err := a.db.GetAPIKey(keyID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if key == nil {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(key)
}

// handleDeleteKey handles DELETE /api/keys/{id}
func (a *API) handleDeleteKey(w http.ResponseWriter, r *http.Request, keyID int) {
	// First check if key exists
	key, err := a.db.GetAPIKey(keyID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if key == nil {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	// Delete the key
	if err := a.db.DeleteAPIKey(keyID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleKeyTest handles POST /api/keys/{id}/test
func (a *API) handleKeyTest(w http.ResponseWriter, r *http.Request, keyID int) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	result, err := a.keyManager.TestKey(keyID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleKeyStats handles GET /api/keys/{id}/stats
func (a *API) handleKeyStats(w http.ResponseWriter, r *http.Request, keyID int) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// First verify the key exists
	key, err := a.db.GetAPIKey(keyID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if key == nil {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	// Get stats for today
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	stats, err := a.db.GetKeyStats(keyID, startOfDay)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleRateLimits handles GET/POST /api/ratelimits
func (a *API) handleRateLimits(w http.ResponseWriter, r *http.Request) {
	if a.rateLimitAPI == nil {
		http.Error(w, "Rate Limit API not configured", http.StatusServiceUnavailable)
		return
	}
	a.rateLimitAPI.HandleRateLimits(w, r)
}

// handleRateLimitByClientID handles GET/PATCH/DELETE /api/ratelimits/{client_id}
func (a *API) handleRateLimitByClientID(w http.ResponseWriter, r *http.Request) {
	if a.rateLimitAPI == nil {
		http.Error(w, "Rate Limit API not configured", http.StatusServiceUnavailable)
		return
	}
	a.rateLimitAPI.HandleRateLimitByClientID(w, r)
}

// handleServerInfo handles GET /api/server/info
func (a *API) handleServerInfo(w http.ResponseWriter, r *http.Request) {
	if a.serverAPI == nil {
		http.Error(w, "Server API not configured", http.StatusServiceUnavailable)
		return
	}
	a.serverAPI.HandleServerInfo(w, r)
}

// handleServerShutdown handles POST /api/server/shutdown
func (a *API) handleServerShutdown(w http.ResponseWriter, r *http.Request) {
	if a.serverAPI == nil {
		http.Error(w, "Server API not configured", http.StatusServiceUnavailable)
		return
	}
	a.serverAPI.HandleShutdown(w, r)
}

// handleHealth returns health status
func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"time":   time.Now(),
	})
}

// ServeHTTP implements http.Handler
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

// Start starts the API server
func (a *API) Start(addr string) error {
	return http.ListenAndServe(addr, a)
}
