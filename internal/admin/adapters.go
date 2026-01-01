package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/jeffersonwarrior/modelscan/internal/database"
	"github.com/jeffersonwarrior/modelscan/internal/discovery"
	"github.com/jeffersonwarrior/modelscan/internal/generator"
	"github.com/jeffersonwarrior/modelscan/internal/keymanager"
)

// DatabaseAdapter adapts database.DB to admin.Database interface
type DatabaseAdapter struct {
	db *database.DB
}

// NewDatabaseAdapter creates a database adapter
func NewDatabaseAdapter(db *database.DB) *DatabaseAdapter {
	return &DatabaseAdapter{db: db}
}

func (a *DatabaseAdapter) CreateProvider(p *Provider) error {
	return a.db.CreateProvider(&database.Provider{
		ID:           p.ID,
		Name:         p.Name,
		BaseURL:      p.BaseURL,
		AuthMethod:   p.AuthMethod,
		PricingModel: p.PricingModel,
		Status:       p.Status,
	})
}

func (a *DatabaseAdapter) GetProvider(id string) (*Provider, error) {
	p, err := a.db.GetProvider(id)
	if err != nil || p == nil {
		return nil, err
	}
	return &Provider{
		ID:           p.ID,
		Name:         p.Name,
		BaseURL:      p.BaseURL,
		AuthMethod:   p.AuthMethod,
		PricingModel: p.PricingModel,
		Status:       p.Status,
	}, nil
}

func (a *DatabaseAdapter) ListProviders() ([]*Provider, error) {
	providers, err := a.db.ListProviders()
	if err != nil {
		return nil, err
	}
	result := make([]*Provider, len(providers))
	for i, p := range providers {
		result[i] = &Provider{
			ID:           p.ID,
			Name:         p.Name,
			BaseURL:      p.BaseURL,
			AuthMethod:   p.AuthMethod,
			PricingModel: p.PricingModel,
			Status:       p.Status,
		}
	}
	return result, nil
}

func (a *DatabaseAdapter) CreateAPIKey(providerID, apiKey string) (*APIKey, error) {
	key, err := a.db.CreateAPIKey(providerID, apiKey)
	if err != nil {
		return nil, err
	}
	return &APIKey{
		ID:            key.ID,
		ProviderID:    key.ProviderID,
		KeyHash:       key.KeyHash,
		KeyPrefix:     key.KeyPrefix,
		RequestsCount: key.RequestsCount,
		TokensCount:   key.TokensCount,
		Active:        key.Active,
		Degraded:      key.Degraded,
	}, nil
}

func (a *DatabaseAdapter) GetAPIKey(id int) (*APIKey, error) {
	key, err := a.db.GetAPIKey(id)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, nil
	}
	return &APIKey{
		ID:            key.ID,
		ProviderID:    key.ProviderID,
		KeyHash:       key.KeyHash,
		KeyPrefix:     key.KeyPrefix,
		RequestsCount: key.RequestsCount,
		TokensCount:   key.TokensCount,
		Active:        key.Active,
		Degraded:      key.Degraded,
	}, nil
}

func (a *DatabaseAdapter) DeleteAPIKey(id int) error {
	return a.db.DeleteAPIKey(id)
}

func (a *DatabaseAdapter) ListActiveAPIKeys(providerID string) ([]*APIKey, error) {
	keys, err := a.db.ListActiveAPIKeys(providerID)
	if err != nil {
		return nil, err
	}
	result := make([]*APIKey, len(keys))
	for i, k := range keys {
		result[i] = &APIKey{
			ID:            k.ID,
			ProviderID:    k.ProviderID,
			KeyPrefix:     k.KeyPrefix,
			RequestsCount: k.RequestsCount,
			TokensCount:   k.TokensCount,
			Active:        k.Active,
			Degraded:      k.Degraded,
		}
	}
	return result, nil
}

func (a *DatabaseAdapter) GetUsageStats(modelID string, since time.Time) (map[string]interface{}, error) {
	return a.db.GetUsageStats(modelID, since)
}

func (a *DatabaseAdapter) GetKeyStats(keyID int, since time.Time) (*KeyStats, error) {
	stats, err := a.db.GetKeyStats(keyID, since)
	if err != nil {
		return nil, err
	}
	if stats == nil {
		return nil, nil
	}
	return &KeyStats{
		RequestsToday:    stats.RequestsToday,
		TokensToday:      stats.TokensToday,
		RateLimitPercent: stats.RateLimitPercent,
		DegradationCount: stats.DegradationCount,
	}, nil
}

// DiscoveryAdapter adapts discovery.Agent to admin.DiscoveryAgent interface
type DiscoveryAdapter struct {
	agent *discovery.Agent
}

// NewDiscoveryAdapter creates a discovery adapter
func NewDiscoveryAdapter(agent *discovery.Agent) *DiscoveryAdapter {
	return &DiscoveryAdapter{agent: agent}
}

func (a *DiscoveryAdapter) Discover(providerID string, apiKey string) (*DiscoveryResult, error) {
	result, err := a.agent.Discover(context.Background(), discovery.DiscoveryRequest{
		Identifier: providerID,
		APIKey:     apiKey,
	})
	if err != nil {
		return nil, err
	}

	return &DiscoveryResult{
		ProviderID:    result.Provider.ID,
		ProviderName:  result.Provider.Name,
		BaseURL:       result.Provider.BaseURL,
		AuthMethod:    result.Provider.AuthMethod,
		AuthHeader:    result.Provider.AuthHeader,
		PricingModel:  result.Provider.PricingModel,
		Documentation: result.Provider.Documentation,
		SDKType:       result.SDK.Type,
		Success:       result.Validated,
		Message:       result.ValidationLog,
	}, nil
}

// GeneratorAdapter adapts generator.Generator to admin.Generator interface
type GeneratorAdapter struct {
	gen *generator.Generator
}

// NewGeneratorAdapter creates a generator adapter
func NewGeneratorAdapter(gen *generator.Generator) *GeneratorAdapter {
	return &GeneratorAdapter{gen: gen}
}

func (a *GeneratorAdapter) Generate(req GenerateRequest) (*GenerateResult, error) {
	result, err := a.gen.Generate(generator.GenerateRequest{
		ProviderID:   req.ProviderID,
		ProviderName: req.ProviderName,
		BaseURL:      req.BaseURL,
		SDKType:      req.SDKType,
	})
	if err != nil {
		return nil, err
	}

	return &GenerateResult{
		FilePath: result.FilePath,
		Success:  result.Success,
		Error:    result.Error,
	}, nil
}

func (a *GeneratorAdapter) List() ([]string, error) {
	return a.gen.List()
}

func (a *GeneratorAdapter) Delete(providerID string) error {
	return a.gen.Delete(providerID)
}

// KeyManagerAdapter adapts keymanager.KeyManager to admin.KeyManager interface
type KeyManagerAdapter struct {
	km *keymanager.KeyManager
	db *database.DB
}

// NewKeyManagerAdapter creates a key manager adapter
func NewKeyManagerAdapter(km *keymanager.KeyManager, db *database.DB) *KeyManagerAdapter {
	return &KeyManagerAdapter{km: km, db: db}
}

func (a *KeyManagerAdapter) GetKey(providerID string) (*APIKey, error) {
	key, err := a.km.GetKey(context.Background(), providerID)
	if err != nil {
		return nil, err
	}
	return &APIKey{
		ID:         key.ID,
		ProviderID: key.ProviderID,
		KeyPrefix:  key.KeyPrefix,
	}, nil
}

func (a *KeyManagerAdapter) ListKeys(providerID string) ([]*APIKey, error) {
	keys, err := a.km.ListKeys(context.Background(), providerID)
	if err != nil {
		return nil, err
	}
	result := make([]*APIKey, len(keys))
	for i, k := range keys {
		result[i] = &APIKey{
			ID:         k.ID,
			ProviderID: k.ProviderID,
			KeyPrefix:  k.KeyPrefix,
		}
	}
	return result, nil
}

// CountKeys returns the total number of API keys across all providers
func (a *KeyManagerAdapter) CountKeys() (int, error) {
	return a.db.CountAPIKeys()
}

// RegisterActualKey stores the actual API key in memory for later retrieval.
// Must be called after CreateAPIKey to enable GetActualKey functionality.
func (a *KeyManagerAdapter) RegisterActualKey(keyHash, actualKey string) {
	a.km.RegisterActualKey(keyHash, actualKey)
}

// TestKey tests an API key for validity
func (a *KeyManagerAdapter) TestKey(keyID int) (*KeyTestResult, error) {
	// Get key from database
	key, err := a.db.GetAPIKey(keyID)
	if err != nil {
		return &KeyTestResult{
			Valid: false,
			Error: fmt.Sprintf("failed to get key: %v", err),
		}, nil
	}
	if key == nil {
		return &KeyTestResult{
			Valid: false,
			Error: "key not found",
		}, nil
	}

	// Check if key is active and not degraded
	valid := key.Active && !key.Degraded

	// Calculate remaining rate limit
	rateRemaining := 0
	if key.RPMLimit != nil {
		remaining := *key.RPMLimit - key.RequestsCount
		if remaining > 0 {
			rateRemaining = remaining
		}
	}

	return &KeyTestResult{
		Valid:              valid,
		RateLimitRemaining: rateRemaining,
		ModelsAccessible:   []string{}, // Would require provider-specific model list query
	}, nil
}

// DatabaseAliasAdapter adapts database.DB to the AliasStore interface
type DatabaseAliasAdapter struct {
	db *database.DB
}

// NewDatabaseAliasAdapter creates a new adapter
func NewDatabaseAliasAdapter(db *database.DB) *DatabaseAliasAdapter {
	return &DatabaseAliasAdapter{db: db}
}

// CreateAlias creates a new alias
func (a *DatabaseAliasAdapter) CreateAlias(alias *Alias) error {
	dbAlias := &database.Alias{
		Name:      alias.Name,
		ModelID:   alias.ModelID,
		ClientID:  alias.ClientID,
		CreatedAt: alias.CreatedAt,
	}
	return a.db.CreateAlias(dbAlias)
}

// GetAlias retrieves an alias by name and optional client ID
func (a *DatabaseAliasAdapter) GetAlias(name string, clientID *string) (*Alias, error) {
	dbAlias, err := a.db.GetAlias(name, clientID)
	if err != nil {
		return nil, err
	}
	if dbAlias == nil {
		return nil, nil
	}
	return &Alias{
		Name:      dbAlias.Name,
		ModelID:   dbAlias.ModelID,
		ClientID:  dbAlias.ClientID,
		CreatedAt: dbAlias.CreatedAt,
	}, nil
}

// ListAllAliases returns all aliases
func (a *DatabaseAliasAdapter) ListAllAliases() ([]*Alias, error) {
	dbAliases, err := a.db.ListAllAliases()
	if err != nil {
		return nil, err
	}
	aliases := make([]*Alias, len(dbAliases))
	for i, dbAlias := range dbAliases {
		aliases[i] = &Alias{
			Name:      dbAlias.Name,
			ModelID:   dbAlias.ModelID,
			ClientID:  dbAlias.ClientID,
			CreatedAt: dbAlias.CreatedAt,
		}
	}
	return aliases, nil
}

// ListAliases returns aliases for a specific client
func (a *DatabaseAliasAdapter) ListAliases(clientID *string) ([]*Alias, error) {
	dbAliases, err := a.db.ListAliases(clientID)
	if err != nil {
		return nil, err
	}
	aliases := make([]*Alias, len(dbAliases))
	for i, dbAlias := range dbAliases {
		aliases[i] = &Alias{
			Name:      dbAlias.Name,
			ModelID:   dbAlias.ModelID,
			ClientID:  dbAlias.ClientID,
			CreatedAt: dbAlias.CreatedAt,
		}
	}
	return aliases, nil
}

// DeleteAlias deletes an alias
func (a *DatabaseAliasAdapter) DeleteAlias(name string, clientID *string) error {
	return a.db.DeleteAlias(name, clientID)
}

// UpdateAlias updates an alias's model ID
func (a *DatabaseAliasAdapter) UpdateAlias(name string, clientID *string, newModelID string) error {
	return a.db.UpdateAlias(name, clientID, newModelID)
}

// DatabaseRemapAdapter adapts database.RemapRuleRepository to the RemapStore interface
type DatabaseRemapAdapter struct {
	repo *database.RemapRuleRepository
}

// NewDatabaseRemapAdapter creates a new adapter
func NewDatabaseRemapAdapter(repo *database.RemapRuleRepository) *DatabaseRemapAdapter {
	return &DatabaseRemapAdapter{repo: repo}
}

// Create creates a new remap rule
func (a *DatabaseRemapAdapter) Create(rule *RemapRule) error {
	dbRule := &database.RemapRule{
		ClientID:   rule.ClientID,
		FromModel:  rule.FromModel,
		ToModel:    rule.ToModel,
		ToProvider: rule.ToProvider,
		Priority:   rule.Priority,
		Enabled:    rule.Enabled,
		CreatedAt:  rule.CreatedAt,
	}
	if err := a.repo.Create(dbRule); err != nil {
		return err
	}
	rule.ID = dbRule.ID
	return nil
}

// Get retrieves a remap rule by ID
func (a *DatabaseRemapAdapter) Get(id int) (*RemapRule, error) {
	dbRule, err := a.repo.Get(id)
	if err != nil {
		return nil, err
	}
	if dbRule == nil {
		return nil, nil
	}
	return &RemapRule{
		ID:         dbRule.ID,
		ClientID:   dbRule.ClientID,
		FromModel:  dbRule.FromModel,
		ToModel:    dbRule.ToModel,
		ToProvider: dbRule.ToProvider,
		Priority:   dbRule.Priority,
		Enabled:    dbRule.Enabled,
		CreatedAt:  dbRule.CreatedAt,
	}, nil
}

// List retrieves all remap rules, optionally filtered by client ID
func (a *DatabaseRemapAdapter) List(clientID *string) ([]*RemapRule, error) {
	dbRules, err := a.repo.List(clientID)
	if err != nil {
		return nil, err
	}
	rules := make([]*RemapRule, len(dbRules))
	for i, dbRule := range dbRules {
		rules[i] = &RemapRule{
			ID:         dbRule.ID,
			ClientID:   dbRule.ClientID,
			FromModel:  dbRule.FromModel,
			ToModel:    dbRule.ToModel,
			ToProvider: dbRule.ToProvider,
			Priority:   dbRule.Priority,
			Enabled:    dbRule.Enabled,
			CreatedAt:  dbRule.CreatedAt,
		}
	}
	return rules, nil
}

// Update updates an existing remap rule
func (a *DatabaseRemapAdapter) Update(rule *RemapRule) error {
	dbRule := &database.RemapRule{
		ID:         rule.ID,
		ClientID:   rule.ClientID,
		FromModel:  rule.FromModel,
		ToModel:    rule.ToModel,
		ToProvider: rule.ToProvider,
		Priority:   rule.Priority,
		Enabled:    rule.Enabled,
		CreatedAt:  rule.CreatedAt,
	}
	return a.repo.Update(dbRule)
}

// Delete removes a remap rule by ID
func (a *DatabaseRemapAdapter) Delete(id int) error {
	return a.repo.Delete(id)
}

// SetEnabled enables or disables a remap rule
func (a *DatabaseRemapAdapter) SetEnabled(id int, enabled bool) error {
	return a.repo.SetEnabled(id, enabled)
}

// DatabaseRateLimitAdapter adapts database.ClientRateLimitRepository to the RateLimitStore interface
type DatabaseRateLimitAdapter struct {
	repo *database.ClientRateLimitRepository
}

// NewDatabaseRateLimitAdapter creates a new adapter
func NewDatabaseRateLimitAdapter(repo *database.ClientRateLimitRepository) *DatabaseRateLimitAdapter {
	return &DatabaseRateLimitAdapter{repo: repo}
}

// Get retrieves a rate limit by client ID
func (a *DatabaseRateLimitAdapter) Get(clientID string) (*ClientRateLimit, error) {
	dbRL, err := a.repo.Get(clientID)
	if err != nil {
		return nil, err
	}
	if dbRL == nil {
		return nil, nil
	}
	return convertFromDBRateLimit(dbRL), nil
}

// GetOrCreate gets an existing rate limit or creates a new one with defaults
func (a *DatabaseRateLimitAdapter) GetOrCreate(clientID string) (*ClientRateLimit, error) {
	dbRL, err := a.repo.GetOrCreate(clientID)
	if err != nil {
		return nil, err
	}
	return convertFromDBRateLimit(dbRL), nil
}

// Create creates a new rate limit
func (a *DatabaseRateLimitAdapter) Create(rl *ClientRateLimit) error {
	dbRL := convertToDBRateLimit(rl)
	if err := a.repo.Create(dbRL); err != nil {
		return err
	}
	rl.ID = dbRL.ID
	return nil
}

// Update updates a rate limit
func (a *DatabaseRateLimitAdapter) Update(rl *ClientRateLimit) error {
	dbRL := convertToDBRateLimit(rl)
	return a.repo.Update(dbRL)
}

// UpdateLimits updates only the limit values
func (a *DatabaseRateLimitAdapter) UpdateLimits(clientID string, rpmLimit, tpmLimit, dailyLimit *int) error {
	return a.repo.UpdateLimits(clientID, rpmLimit, tpmLimit, dailyLimit)
}

// Delete removes a rate limit by client ID
func (a *DatabaseRateLimitAdapter) Delete(clientID string) error {
	return a.repo.Delete(clientID)
}

// List retrieves all rate limits
func (a *DatabaseRateLimitAdapter) List() ([]*ClientRateLimit, error) {
	dbLimits, err := a.repo.List()
	if err != nil {
		return nil, err
	}
	limits := make([]*ClientRateLimit, len(dbLimits))
	for i, dbRL := range dbLimits {
		limits[i] = convertFromDBRateLimit(dbRL)
	}
	return limits, nil
}

// IncrementUsage increments usage counters
func (a *DatabaseRateLimitAdapter) IncrementUsage(clientID string, requests, tokens int) error {
	return a.repo.IncrementUsage(clientID, requests, tokens)
}

// CheckLimits checks if a client is within their rate limits
func (a *DatabaseRateLimitAdapter) CheckLimits(clientID string) (bool, string, error) {
	return a.repo.CheckLimits(clientID)
}

// ResetMinuteCounters resets RPM and TPM counters
func (a *DatabaseRateLimitAdapter) ResetMinuteCounters() error {
	return a.repo.ResetMinuteCounters()
}

// ResetDailyCounters resets daily counters
func (a *DatabaseRateLimitAdapter) ResetDailyCounters() error {
	return a.repo.ResetDailyCounters()
}

// Exists checks if a rate limit exists for a client
func (a *DatabaseRateLimitAdapter) Exists(clientID string) (bool, error) {
	return a.repo.Exists(clientID)
}

// convertFromDBRateLimit converts a database rate limit to an admin rate limit
func convertFromDBRateLimit(dbRL *database.ClientRateLimit) *ClientRateLimit {
	return &ClientRateLimit{
		ID:           dbRL.ID,
		ClientID:     dbRL.ClientID,
		RPMLimit:     dbRL.RPMLimit,
		TPMLimit:     dbRL.TPMLimit,
		DailyLimit:   dbRL.DailyLimit,
		CurrentRPM:   dbRL.CurrentRPM,
		CurrentTPM:   dbRL.CurrentTPM,
		CurrentDaily: dbRL.CurrentDaily,
		LastReset:    dbRL.LastReset,
	}
}

// convertToDBRateLimit converts an admin rate limit to a database rate limit
func convertToDBRateLimit(rl *ClientRateLimit) *database.ClientRateLimit {
	return &database.ClientRateLimit{
		ID:           rl.ID,
		ClientID:     rl.ClientID,
		RPMLimit:     rl.RPMLimit,
		TPMLimit:     rl.TPMLimit,
		DailyLimit:   rl.DailyLimit,
		CurrentRPM:   rl.CurrentRPM,
		CurrentTPM:   rl.CurrentTPM,
		CurrentDaily: rl.CurrentDaily,
		LastReset:    rl.LastReset,
	}
}
