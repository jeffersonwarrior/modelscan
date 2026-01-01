package admin

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// AliasStore interface for alias data operations
type AliasStore interface {
	CreateAlias(alias *Alias) error
	GetAlias(name string, clientID *string) (*Alias, error)
	ListAllAliases() ([]*Alias, error)
	ListAliases(clientID *string) ([]*Alias, error)
	DeleteAlias(name string, clientID *string) error
	UpdateAlias(name string, clientID *string, newModelID string) error
}

// Alias represents a model alias
type Alias struct {
	Name      string    `json:"name"`
	ModelID   string    `json:"model_id"`
	ClientID  *string   `json:"client_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// AliasAPI handles alias management endpoints
type AliasAPI struct {
	store AliasStore
}

// NewAliasAPI creates a new AliasAPI
func NewAliasAPI(store AliasStore) *AliasAPI {
	return &AliasAPI{store: store}
}

// AliasCreateRequest represents the request body for creating an alias
type AliasCreateRequest struct {
	Name     string  `json:"name"`
	ModelID  string  `json:"model_id"`
	ClientID *string `json:"client_id,omitempty"`
}

// AliasResponse represents an alias in API responses
type AliasResponse struct {
	Name      string    `json:"name"`
	ModelID   string    `json:"model_id"`
	ClientID  *string   `json:"client_id,omitempty"`
	IsGlobal  bool      `json:"is_global"`
	CreatedAt time.Time `json:"created_at"`
}

// HandleAliases handles GET /api/aliases (list) and POST /api/aliases (create)
func (a *AliasAPI) HandleAliases(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleListAliases(w, r)
	case http.MethodPost:
		a.handleCreateAlias(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleListAliases handles GET /api/aliases
func (a *AliasAPI) handleListAliases(w http.ResponseWriter, r *http.Request) {
	// Check for client_id filter
	clientID := r.URL.Query().Get("client_id")

	var aliases []*Alias
	var err error

	if clientID != "" {
		aliases, err = a.store.ListAliases(&clientID)
	} else {
		// List all aliases (global and client-specific)
		aliases, err = a.store.ListAllAliases()
	}

	if err != nil {
		http.Error(w, "Failed to list aliases: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to response format
	var resp []AliasResponse
	for _, alias := range aliases {
		resp = append(resp, AliasResponse{
			Name:      alias.Name,
			ModelID:   alias.ModelID,
			ClientID:  alias.ClientID,
			IsGlobal:  alias.ClientID == nil,
			CreatedAt: alias.CreatedAt,
		})
	}
	if resp == nil {
		resp = []AliasResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"aliases": resp,
		"count":   len(resp),
	})
}

// handleCreateAlias handles POST /api/aliases
func (a *AliasAPI) handleCreateAlias(w http.ResponseWriter, r *http.Request) {
	var req AliasCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.ModelID == "" {
		http.Error(w, "model_id is required", http.StatusBadRequest)
		return
	}

	// Check if alias already exists
	existing, err := a.store.GetAlias(req.Name, req.ClientID)
	if err != nil {
		http.Error(w, "Failed to check existing alias: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if existing != nil {
		http.Error(w, "Alias already exists", http.StatusConflict)
		return
	}

	// Create alias
	alias := &Alias{
		Name:      req.Name,
		ModelID:   req.ModelID,
		ClientID:  req.ClientID,
		CreatedAt: time.Now(),
	}

	if err := a.store.CreateAlias(alias); err != nil {
		http.Error(w, "Failed to create alias: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := AliasResponse{
		Name:      alias.Name,
		ModelID:   alias.ModelID,
		ClientID:  alias.ClientID,
		IsGlobal:  alias.ClientID == nil,
		CreatedAt: alias.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// HandleAliasByName handles GET/DELETE /api/aliases/{name}
func (a *AliasAPI) HandleAliasByName(w http.ResponseWriter, r *http.Request) {
	// Extract alias name from path
	path := strings.TrimPrefix(r.URL.Path, "/api/aliases/")
	name := path
	if name == "" {
		http.Error(w, "Alias name required", http.StatusBadRequest)
		return
	}

	// Get optional client_id from query params
	clientIDStr := r.URL.Query().Get("client_id")
	var clientID *string
	if clientIDStr != "" {
		clientID = &clientIDStr
	}

	switch r.Method {
	case http.MethodGet:
		a.handleGetAlias(w, r, name, clientID)
	case http.MethodDelete:
		a.handleDeleteAlias(w, r, name, clientID)
	case http.MethodPut:
		a.handleUpdateAlias(w, r, name, clientID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetAlias handles GET /api/aliases/{name}
func (a *AliasAPI) handleGetAlias(w http.ResponseWriter, r *http.Request, name string, clientID *string) {
	alias, err := a.store.GetAlias(name, clientID)
	if err != nil {
		http.Error(w, "Failed to get alias: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if alias == nil {
		// Try global alias if client-specific not found
		if clientID != nil {
			alias, err = a.store.GetAlias(name, nil)
			if err != nil {
				http.Error(w, "Failed to get alias: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if alias == nil {
			http.Error(w, "Alias not found", http.StatusNotFound)
			return
		}
	}

	resp := AliasResponse{
		Name:      alias.Name,
		ModelID:   alias.ModelID,
		ClientID:  alias.ClientID,
		IsGlobal:  alias.ClientID == nil,
		CreatedAt: alias.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleDeleteAlias handles DELETE /api/aliases/{name}
func (a *AliasAPI) handleDeleteAlias(w http.ResponseWriter, r *http.Request, name string, clientID *string) {
	// Check if alias exists
	alias, err := a.store.GetAlias(name, clientID)
	if err != nil {
		http.Error(w, "Failed to check alias: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if alias == nil {
		http.Error(w, "Alias not found", http.StatusNotFound)
		return
	}

	// Delete alias
	if err := a.store.DeleteAlias(name, clientID); err != nil {
		http.Error(w, "Failed to delete alias: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AliasUpdateRequest represents the request body for updating an alias
type AliasUpdateRequest struct {
	ModelID string `json:"model_id"`
}

// handleUpdateAlias handles PUT /api/aliases/{name}
func (a *AliasAPI) handleUpdateAlias(w http.ResponseWriter, r *http.Request, name string, clientID *string) {
	var req AliasUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ModelID == "" {
		http.Error(w, "model_id is required", http.StatusBadRequest)
		return
	}

	// Check if alias exists
	alias, err := a.store.GetAlias(name, clientID)
	if err != nil {
		http.Error(w, "Failed to check alias: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if alias == nil {
		http.Error(w, "Alias not found", http.StatusNotFound)
		return
	}

	// Update alias
	if err := a.store.UpdateAlias(name, clientID, req.ModelID); err != nil {
		http.Error(w, "Failed to update alias: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return updated alias
	alias.ModelID = req.ModelID
	resp := AliasResponse{
		Name:      alias.Name,
		ModelID:   alias.ModelID,
		ClientID:  alias.ClientID,
		IsGlobal:  alias.ClientID == nil,
		CreatedAt: alias.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
