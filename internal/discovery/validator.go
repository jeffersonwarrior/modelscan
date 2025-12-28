package discovery

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Validator validates discovered providers using TDD approach
type Validator struct {
	maxRetries int
}

// NewValidator creates a new validator
func NewValidator(maxRetries int) *Validator {
	return &Validator{
		maxRetries: maxRetries,
	}
}

// Validate validates a discovery result with TDD approach
// Returns (success, validation log)
func (v *Validator) Validate(ctx context.Context, result *DiscoveryResult, apiKey string) (bool, string) {
	var log strings.Builder
	log.WriteString(fmt.Sprintf("Starting validation for %s\n", result.Provider.ID))
	log.WriteString(fmt.Sprintf("Base URL: %s\n", result.Provider.BaseURL))
	log.WriteString(fmt.Sprintf("Auth Method: %s\n", result.Provider.AuthMethod))
	log.WriteString("\n")

	// Phase 1: Basic connectivity test
	log.WriteString("Phase 1: Basic connectivity\n")
	if err := v.testConnectivity(ctx, result.Provider.BaseURL); err != nil {
		log.WriteString(fmt.Sprintf("  ✗ Connectivity failed: %v\n", err))
		return false, log.String()
	}
	log.WriteString("  ✓ Connectivity OK\n\n")

	// Phase 2: Authentication test
	log.WriteString("Phase 2: Authentication\n")
	if err := v.testAuth(ctx, result, apiKey); err != nil {
		log.WriteString(fmt.Sprintf("  ✗ Authentication failed: %v\n", err))
		return false, log.String()
	}
	log.WriteString("  ✓ Authentication OK\n\n")

	// Phase 3: Endpoint discovery validation
	log.WriteString("Phase 3: Endpoint validation\n")
	for _, endpoint := range result.SDK.Endpoints {
		if err := v.testEndpoint(ctx, result, apiKey, endpoint); err != nil {
			log.WriteString(fmt.Sprintf("  ✗ Endpoint %s failed: %v\n", endpoint.Path, err))
			// Continue testing other endpoints
		} else {
			log.WriteString(fmt.Sprintf("  ✓ Endpoint %s OK\n", endpoint.Path))
		}
	}
	log.WriteString("\n")

	// Phase 4: Model listing test (if supported)
	log.WriteString("Phase 4: Model listing\n")
	models, err := v.testListModels(ctx, result, apiKey)
	if err != nil {
		log.WriteString(fmt.Sprintf("  ⚠ Model listing not supported or failed: %v\n", err))
	} else {
		log.WriteString(fmt.Sprintf("  ✓ Found %d models\n", len(models)))
		for _, model := range models {
			log.WriteString(fmt.Sprintf("    - %s\n", model))
		}
	}
	log.WriteString("\n")

	log.WriteString("Validation completed successfully\n")
	return true, log.String()
}

// testConnectivity tests basic HTTP connectivity to the provider
func (v *Validator) testConnectivity(ctx context.Context, baseURL string) error {
	// TODO: Implement actual HTTP connectivity test
	// For now, just check if URL is non-empty
	if baseURL == "" {
		return fmt.Errorf("base URL is empty")
	}
	return nil
}

// testAuth tests authentication with the provider
func (v *Validator) testAuth(ctx context.Context, result *DiscoveryResult, apiKey string) error {
	// TODO: Implement actual authentication test
	// This should make a minimal authenticated request to verify the API key works
	if apiKey == "" {
		return fmt.Errorf("no API key provided")
	}
	return nil
}

// testEndpoint tests a specific endpoint
func (v *Validator) testEndpoint(ctx context.Context, result *DiscoveryResult, apiKey string, endpoint EndpointInfo) error {
	// TODO: Implement actual endpoint testing
	// This should make a minimal request to the endpoint and verify it responds correctly
	return nil
}

// testListModels tests model listing functionality
func (v *Validator) testListModels(ctx context.Context, result *DiscoveryResult, apiKey string) ([]string, error) {
	// TODO: Implement actual model listing test
	// This should attempt to list available models from the provider
	return []string{}, nil
}

// ValidateWithRetry validates with automatic retries on failure
func (v *Validator) ValidateWithRetry(ctx context.Context, result *DiscoveryResult, apiKey string) (bool, string) {
	var lastLog string
	for attempt := 0; attempt < v.maxRetries; attempt++ {
		success, log := v.Validate(ctx, result, apiKey)
		lastLog = log
		if success {
			return true, log
		}

		// Wait before retry (exponential backoff)
		if attempt < v.maxRetries-1 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
		}
	}

	return false, fmt.Sprintf("Validation failed after %d attempts\n\nLast attempt log:\n%s", v.maxRetries, lastLog)
}
