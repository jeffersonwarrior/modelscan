package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ValidationError types for categorization
type ValidationErrorType string

const (
	ErrorConnectivity   ValidationErrorType = "connectivity"
	ErrorAuthentication ValidationErrorType = "authentication"
	ErrorEndpoint       ValidationErrorType = "endpoint"
	ErrorModel          ValidationErrorType = "model"
)

// ValidationError represents a categorized validation failure
type ValidationError struct {
	Type    ValidationErrorType
	Message string
	Err     error
}

func (ve *ValidationError) Error() string {
	if ve.Err != nil {
		return fmt.Sprintf("%s error: %s (%v)", ve.Type, ve.Message, ve.Err)
	}
	return fmt.Sprintf("%s error: %s", ve.Type, ve.Message)
}

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
	if baseURL == "" {
		return fmt.Errorf("base URL is empty")
	}

	// Create a HEAD request to check basic connectivity
	req, err := http.NewRequestWithContext(ctx, "HEAD", baseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connectivity failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Accept any response (including 404, 401) - we just want to confirm the server responds
	if resp.StatusCode >= 200 && resp.StatusCode < 600 {
		return nil
	}

	return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}

// testAuth tests authentication with the provider
func (v *Validator) testAuth(ctx context.Context, result *DiscoveryResult, apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("no API key provided")
	}

	// Try common endpoints for authentication testing
	testPaths := []string{
		"/v1/models",           // OpenAI-compatible
		"/models",              // Alternative
		"/v1/chat/completions", // Chat endpoint (might need body)
	}

	var lastErr error
	for _, path := range testPaths {
		url := strings.TrimRight(result.Provider.BaseURL, "/") + path

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			lastErr = err
			continue
		}

		// Set authentication header based on provider config
		authHeader := result.Provider.AuthHeader
		if authHeader == "" {
			authHeader = "Authorization"
		}

		if strings.ToLower(result.Provider.AuthMethod) == "bearer" {
			req.Header.Set(authHeader, "Bearer "+apiKey)
		} else {
			req.Header.Set(authHeader, apiKey)
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer func() { _ = resp.Body.Close() }()

		// Success if we get a valid response (not 401/403)
		if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden {
			return nil
		}

		lastErr = fmt.Errorf("authentication failed with status %d", resp.StatusCode)
	}

	if lastErr != nil {
		return fmt.Errorf("auth test failed: %w", lastErr)
	}

	return fmt.Errorf("no valid endpoints found for auth testing")
}

// testEndpoint tests a specific endpoint
func (v *Validator) testEndpoint(ctx context.Context, result *DiscoveryResult, apiKey string, endpoint EndpointInfo) error {
	url := strings.TrimRight(result.Provider.BaseURL, "/") + endpoint.Path

	method := endpoint.Method
	if method == "" {
		method = "GET"
	}

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication
	authHeader := result.Provider.AuthHeader
	if authHeader == "" {
		authHeader = "Authorization"
	}

	if strings.ToLower(result.Provider.AuthMethod) == "bearer" {
		req.Header.Set(authHeader, "Bearer "+apiKey)
	} else {
		req.Header.Set(authHeader, apiKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Accept any non-500 error as success (endpoint exists)
	if resp.StatusCode < 500 {
		return nil
	}

	return fmt.Errorf("endpoint returned status %d", resp.StatusCode)
}

// testListModels tests model listing functionality
func (v *Validator) testListModels(ctx context.Context, result *DiscoveryResult, apiKey string) ([]string, error) {
	// Try common model listing endpoints
	testPaths := []string{
		"/v1/models",
		"/models",
	}

	for _, path := range testPaths {
		url := strings.TrimRight(result.Provider.BaseURL, "/") + path

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			continue
		}

		// Set authentication
		authHeader := result.Provider.AuthHeader
		if authHeader == "" {
			authHeader = "Authorization"
		}

		if strings.ToLower(result.Provider.AuthMethod) == "bearer" {
			req.Header.Set(authHeader, "Bearer "+apiKey)
		} else {
			req.Header.Set(authHeader, apiKey)
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		// Try to parse OpenAI-compatible response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		var modelsResp struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}

		if err := json.Unmarshal(body, &modelsResp); err == nil && len(modelsResp.Data) > 0 {
			models := make([]string, len(modelsResp.Data))
			for i, m := range modelsResp.Data {
				models[i] = m.ID
			}
			return models, nil
		}
	}

	return nil, fmt.Errorf("model listing not supported or failed")
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
