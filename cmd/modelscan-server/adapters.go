package main

import (
	"time"

	"github.com/jeffersonwarrior/modelscan/internal/database"
	"github.com/jeffersonwarrior/modelscan/internal/keymanager"
)

// dbAdapter adapts database.DB to keymanager.Database interface
type dbAdapter struct {
	db *database.DB
}

func (d *dbAdapter) ListActiveAPIKeys(providerID string) ([]*keymanager.APIKey, error) {
	keys, err := d.db.ListActiveAPIKeys(providerID)
	if err != nil {
		return nil, err
	}

	result := make([]*keymanager.APIKey, len(keys))
	for i, k := range keys {
		result[i] = &keymanager.APIKey{
			ID:             k.ID,
			ProviderID:     k.ProviderID,
			KeyHash:        k.KeyHash,
			KeyPrefix:      k.KeyPrefix,
			Tier:           k.Tier,
			RPMLimit:       k.RPMLimit,
			TPMLimit:       k.TPMLimit,
			DailyLimit:     k.DailyLimit,
			ResetInterval:  k.ResetInterval,
			LastReset:      k.LastReset,
			RequestsCount:  k.RequestsCount,
			TokensCount:    k.TokensCount,
			Active:         k.Active,
			Degraded:       k.Degraded,
			DegradedUntil:  k.DegradedUntil,
			CreatedAt:      k.CreatedAt,
		}
	}
	return result, nil
}

func (d *dbAdapter) IncrementKeyUsage(keyID int, tokens int) error {
	return d.db.IncrementKeyUsage(keyID, tokens)
}

func (d *dbAdapter) MarkKeyDegraded(keyID int, until time.Time) error {
	return d.db.MarkKeyDegraded(keyID, until)
}

func (d *dbAdapter) ResetKeyLimits(keyID int) error {
	return d.db.ResetKeyLimits(keyID)
}

func (d *dbAdapter) GetAPIKey(id int) (*keymanager.APIKey, error) {
	k, err := d.db.GetAPIKey(id)
	if err != nil || k == nil {
		return nil, err
	}

	return &keymanager.APIKey{
		ID:             k.ID,
		ProviderID:     k.ProviderID,
		KeyHash:        k.KeyHash,
		KeyPrefix:      k.KeyPrefix,
		Tier:           k.Tier,
		RPMLimit:       k.RPMLimit,
		TPMLimit:       k.TPMLimit,
		DailyLimit:     k.DailyLimit,
		ResetInterval:  k.ResetInterval,
		LastReset:      k.LastReset,
		RequestsCount:  k.RequestsCount,
		TokensCount:    k.TokensCount,
		Active:         k.Active,
		Degraded:       k.Degraded,
		DegradedUntil:  k.DegradedUntil,
		CreatedAt:      k.CreatedAt,
	}, nil
}
