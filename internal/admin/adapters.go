package admin

import (
	"context"
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
		KeyPrefix:     key.KeyPrefix,
		RequestsCount: key.RequestsCount,
		TokensCount:   key.TokensCount,
		Active:        key.Active,
		Degraded:      key.Degraded,
	}, nil
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
}

// NewKeyManagerAdapter creates a key manager adapter
func NewKeyManagerAdapter(km *keymanager.KeyManager) *KeyManagerAdapter {
	return &KeyManagerAdapter{km: km}
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
