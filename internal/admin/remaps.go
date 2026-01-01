package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RemapStore interface for remap rule data operations
type RemapStore interface {
	Create(rule *RemapRule) error
	Get(id int) (*RemapRule, error)
	List(clientID *string) ([]*RemapRule, error)
	Update(rule *RemapRule) error
	Delete(id int) error
	SetEnabled(id int, enabled bool) error
}

// RemapRule represents a model remapping rule
type RemapRule struct {
	ID         int       `json:"id"`
	ClientID   string    `json:"client_id"`
	FromModel  string    `json:"from_model"`
	ToModel    string    `json:"to_model"`
	ToProvider string    `json:"to_provider"`
	Priority   int       `json:"priority"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
}

// RemapAPI handles remap rule management endpoints
type RemapAPI struct {
	store RemapStore
}

// NewRemapAPI creates a new RemapAPI
func NewRemapAPI(store RemapStore) *RemapAPI {
	return &RemapAPI{store: store}
}

// RemapCreateRequest represents the request body for creating a remap rule
type RemapCreateRequest struct {
	ClientID   string `json:"client_id"`
	FromModel  string `json:"from_model"`
	ToModel    string `json:"to_model"`
	ToProvider string `json:"to_provider"`
	Priority   int    `json:"priority"`
	Enabled    *bool  `json:"enabled,omitempty"`
}

// RemapUpdateRequest represents the request body for updating a remap rule
type RemapUpdateRequest struct {
	ClientID   *string `json:"client_id,omitempty"`
	FromModel  *string `json:"from_model,omitempty"`
	ToModel    *string `json:"to_model,omitempty"`
	ToProvider *string `json:"to_provider,omitempty"`
	Priority   *int    `json:"priority,omitempty"`
	Enabled    *bool   `json:"enabled,omitempty"`
}

// HandleRemaps handles GET /api/rules/remap (list) and POST /api/rules/remap (create)
func (a *RemapAPI) HandleRemaps(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleListRemaps(w, r)
	case http.MethodPost:
		a.handleCreateRemap(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleListRemaps handles GET /api/rules/remap
func (a *RemapAPI) handleListRemaps(w http.ResponseWriter, r *http.Request) {
	// Check for client_id filter
	clientIDStr := r.URL.Query().Get("client_id")
	var clientID *string
	if clientIDStr != "" {
		clientID = &clientIDStr
	}

	rules, err := a.store.List(clientID)
	if err != nil {
		http.Error(w, "Failed to list remap rules: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if rules == nil {
		rules = []*RemapRule{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"rules": rules,
		"count": len(rules),
	})
}

// handleCreateRemap handles POST /api/rules/remap
func (a *RemapAPI) handleCreateRemap(w http.ResponseWriter, r *http.Request) {
	var req RemapCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.ClientID == "" {
		http.Error(w, "client_id is required", http.StatusBadRequest)
		return
	}
	if req.FromModel == "" {
		http.Error(w, "from_model is required", http.StatusBadRequest)
		return
	}
	if req.ToModel == "" {
		http.Error(w, "to_model is required", http.StatusBadRequest)
		return
	}
	if req.ToProvider == "" {
		http.Error(w, "to_provider is required", http.StatusBadRequest)
		return
	}

	// Default enabled to true if not specified
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	// Create rule
	rule := &RemapRule{
		ClientID:   req.ClientID,
		FromModel:  req.FromModel,
		ToModel:    req.ToModel,
		ToProvider: req.ToProvider,
		Priority:   req.Priority,
		Enabled:    enabled,
		CreatedAt:  time.Now(),
	}

	if err := a.store.Create(rule); err != nil {
		http.Error(w, "Failed to create remap rule: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rule)
}

// HandleRemapByID handles GET/PATCH/DELETE /api/rules/remap/{id}
func (a *RemapAPI) HandleRemapByID(w http.ResponseWriter, r *http.Request) {
	// Extract rule ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/rules/remap/")
	if path == "" {
		http.Error(w, "Rule ID required", http.StatusBadRequest)
		return
	}

	ruleID, err := strconv.Atoi(path)
	if err != nil {
		http.Error(w, "Invalid rule ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		a.handleGetRemap(w, r, ruleID)
	case http.MethodPatch:
		a.handleUpdateRemap(w, r, ruleID)
	case http.MethodDelete:
		a.handleDeleteRemap(w, r, ruleID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetRemap handles GET /api/rules/remap/{id}
func (a *RemapAPI) handleGetRemap(w http.ResponseWriter, r *http.Request, ruleID int) {
	rule, err := a.store.Get(ruleID)
	if err != nil {
		http.Error(w, "Failed to get remap rule: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if rule == nil {
		http.Error(w, "Remap rule not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rule)
}

// handleUpdateRemap handles PATCH /api/rules/remap/{id}
func (a *RemapAPI) handleUpdateRemap(w http.ResponseWriter, r *http.Request, ruleID int) {
	// Get existing rule
	rule, err := a.store.Get(ruleID)
	if err != nil {
		http.Error(w, "Failed to get remap rule: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if rule == nil {
		http.Error(w, "Remap rule not found", http.StatusNotFound)
		return
	}

	var req RemapUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Apply updates
	if req.ClientID != nil {
		rule.ClientID = *req.ClientID
	}
	if req.FromModel != nil {
		rule.FromModel = *req.FromModel
	}
	if req.ToModel != nil {
		rule.ToModel = *req.ToModel
	}
	if req.ToProvider != nil {
		rule.ToProvider = *req.ToProvider
	}
	if req.Priority != nil {
		rule.Priority = *req.Priority
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}

	// Update rule
	if err := a.store.Update(rule); err != nil {
		http.Error(w, "Failed to update remap rule: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rule)
}

// handleDeleteRemap handles DELETE /api/rules/remap/{id}
func (a *RemapAPI) handleDeleteRemap(w http.ResponseWriter, r *http.Request, ruleID int) {
	// Check if rule exists
	rule, err := a.store.Get(ruleID)
	if err != nil {
		http.Error(w, "Failed to check remap rule: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if rule == nil {
		http.Error(w, "Remap rule not found", http.StatusNotFound)
		return
	}

	// Delete rule
	if err := a.store.Delete(ruleID); err != nil {
		http.Error(w, "Failed to delete remap rule: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
