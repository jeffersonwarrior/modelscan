package admin

import (
	"encoding/json"
	"net/http"
	"time"
)

// API provides HTTP endpoints for admin operations
type API struct {
	mux        *http.ServeMux
	db         Database
	discovery  DiscoveryAgent
	generator  Generator
	keyManager KeyManager
}

// Database interface for data operations
type Database interface {
	CreateProvider(p *Provider) error
	GetProvider(id string) (*Provider, error)
	ListProviders() ([]*Provider, error)
	CreateAPIKey(providerID, apiKey string) (*APIKey, error)
	ListActiveAPIKeys(providerID string) ([]*APIKey, error)
	GetUsageStats(modelID string, since time.Time) (map[string]interface{}, error)
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

// setupRoutes configures HTTP routes
func (a *API) setupRoutes() {
	// Provider management
	a.mux.HandleFunc("/api/providers", a.handleProviders)
	a.mux.HandleFunc("/api/providers/add", a.handleAddProvider)

	// API key management
	a.mux.HandleFunc("/api/keys", a.handleKeys)
	a.mux.HandleFunc("/api/keys/add", a.handleAddKey)

	// Discovery
	a.mux.HandleFunc("/api/discover", a.handleDiscover)

	// SDK management
	a.mux.HandleFunc("/api/sdks", a.handleSDKs)
	a.mux.HandleFunc("/api/sdks/generate", a.handleGenerateSDK)

	// Usage stats
	a.mux.HandleFunc("/api/stats", a.handleStats)

	// Health check
	a.mux.HandleFunc("/health", a.handleHealth)
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
			// Provider might already exist - that's OK
			// Log error but continue
			_ = err
		}

		// Add the API key if provided
		if req.APIKey != "" {
			_, err := a.db.CreateAPIKey(result.ProviderID, req.APIKey)
			if err != nil {
				// Key might already exist - that's OK
				_ = err
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
