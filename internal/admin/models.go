package admin

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
)

// ModelInfo represents a model in the hierarchy response
type ModelInfo struct {
	ID            string `json:"id"`
	Version       string `json:"version,omitempty"`
	ContextWindow int    `json:"context,omitempty"`
	MaxTokens     int    `json:"max_tokens,omitempty"`
	CostIn        string `json:"cost_in,omitempty"`
	CostOut       string `json:"cost_out,omitempty"`
}

// ModelFamily represents a family of models (e.g., Opus, Sonnet, Haiku)
type ModelFamily struct {
	Name   string      `json:"name"`
	Models []ModelInfo `json:"models"`
}

// ProviderModels represents a provider with its model hierarchy
type ProviderModels struct {
	Name     string        `json:"name"`
	HasKey   bool          `json:"has_key"`
	Families []ModelFamily `json:"families"`
}

// HierarchyResponse is the response format for hierarchical model listing
type HierarchyResponse struct {
	Providers []ProviderModels `json:"providers"`
}

// ModelService interface for listing models
type ModelService interface {
	ListModelsWithProvider() ([]ModelWithProviderInfo, error)
	HasKeyForProvider(providerID string) bool
}

// ModelWithProviderInfo represents a model with provider metadata
type ModelWithProviderInfo struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Provider      string  `json:"provider"`
	ContextWindow int     `json:"context_window"`
	MaxTokens     int     `json:"max_tokens,omitempty"`
	CostPer1MIn   float64 `json:"cost_per_1m_in"`
	CostPer1MOut  float64 `json:"cost_per_1m_out"`
}

// handleModels handles GET /api/models with optional format parameter
func (a *API) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	format := r.URL.Query().Get("format")

	// Check if we have a model service configured
	if a.modelService == nil {
		http.Error(w, "Model service not configured", http.StatusServiceUnavailable)
		return
	}

	models, err := a.modelService.ListModelsWithProvider()
	if err != nil {
		http.Error(w, "Failed to list models: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if format == "hierarchy" {
		hierarchy := buildModelHierarchy(models, a.modelService)
		json.NewEncoder(w).Encode(hierarchy)
		return
	}

	// Default flat list
	json.NewEncoder(w).Encode(map[string]interface{}{
		"models": models,
		"count":  len(models),
	})
}

// buildModelHierarchy groups models by provider and family
func buildModelHierarchy(models []ModelWithProviderInfo, svc ModelService) HierarchyResponse {
	// Group models by provider
	providerMap := make(map[string][]ModelWithProviderInfo)
	for _, m := range models {
		providerMap[m.Provider] = append(providerMap[m.Provider], m)
	}

	var providers []ProviderModels
	for providerName, providerModels := range providerMap {
		// Group models by family within provider
		familyMap := make(map[string][]ModelInfo)
		for _, m := range providerModels {
			family := extractFamily(m.ID, m.Name, providerName)
			mi := ModelInfo{
				ID:            m.ID,
				Version:       extractVersion(m.ID, m.Name),
				ContextWindow: m.ContextWindow,
				MaxTokens:     m.MaxTokens,
				CostIn:        formatCost(m.CostPer1MIn),
				CostOut:       formatCost(m.CostPer1MOut),
			}
			familyMap[family] = append(familyMap[family], mi)
		}

		// Convert family map to sorted slice
		var families []ModelFamily
		for familyName, familyModels := range familyMap {
			// Sort models within family by ID
			sort.Slice(familyModels, func(i, j int) bool {
				return familyModels[i].ID < familyModels[j].ID
			})
			families = append(families, ModelFamily{
				Name:   familyName,
				Models: familyModels,
			})
		}

		// Sort families by name
		sort.Slice(families, func(i, j int) bool {
			return families[i].Name < families[j].Name
		})

		hasKey := false
		if svc != nil {
			hasKey = svc.HasKeyForProvider(providerName)
		}

		providers = append(providers, ProviderModels{
			Name:     providerName,
			HasKey:   hasKey,
			Families: families,
		})
	}

	// Sort providers by name
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Name < providers[j].Name
	})

	return HierarchyResponse{Providers: providers}
}

// extractFamily extracts the model family from ID/name
func extractFamily(id, name, provider string) string {
	lower := strings.ToLower(id)

	// Anthropic families
	if strings.Contains(lower, "opus") {
		return "Opus"
	}
	if strings.Contains(lower, "sonnet") {
		return "Sonnet"
	}
	if strings.Contains(lower, "haiku") {
		return "Haiku"
	}

	// OpenAI families
	if strings.Contains(lower, "gpt-4o") {
		return "GPT-4o"
	}
	if strings.Contains(lower, "gpt-4-turbo") || strings.Contains(lower, "gpt-4-1106") || strings.Contains(lower, "gpt-4-0125") {
		return "GPT-4-Turbo"
	}
	if strings.Contains(lower, "gpt-4") {
		return "GPT-4"
	}
	if strings.Contains(lower, "gpt-3.5") || strings.Contains(lower, "gpt-35") {
		return "GPT-3.5"
	}
	if strings.Contains(lower, "o1") || strings.Contains(lower, "o3") {
		return "Reasoning"
	}

	// Google families
	if strings.Contains(lower, "gemini-2") {
		return "Gemini-2"
	}
	if strings.Contains(lower, "gemini-1.5") {
		return "Gemini-1.5"
	}
	if strings.Contains(lower, "gemini-pro") {
		return "Gemini-Pro"
	}
	if strings.Contains(lower, "gemini") {
		return "Gemini"
	}

	// Mistral families
	if strings.Contains(lower, "mixtral") {
		return "Mixtral"
	}
	if strings.Contains(lower, "mistral-large") {
		return "Mistral-Large"
	}
	if strings.Contains(lower, "mistral-small") {
		return "Mistral-Small"
	}
	if strings.Contains(lower, "mistral") {
		return "Mistral"
	}

	// DeepSeek families
	if strings.Contains(lower, "deepseek-coder") {
		return "DeepSeek-Coder"
	}
	if strings.Contains(lower, "deepseek-chat") || strings.Contains(lower, "deepseek-v") {
		return "DeepSeek-Chat"
	}

	// Llama families
	if strings.Contains(lower, "llama-3.3") {
		return "Llama-3.3"
	}
	if strings.Contains(lower, "llama-3.2") {
		return "Llama-3.2"
	}
	if strings.Contains(lower, "llama-3.1") {
		return "Llama-3.1"
	}
	if strings.Contains(lower, "llama-3") {
		return "Llama-3"
	}
	if strings.Contains(lower, "llama") {
		return "Llama"
	}

	// Cohere families
	if strings.Contains(lower, "command-r") {
		return "Command-R"
	}
	if strings.Contains(lower, "command") {
		return "Command"
	}

	// Default: use provider-based grouping or model prefix
	parts := strings.Split(id, "-")
	if len(parts) > 1 {
		// Title case first part (e.g., "gpt" -> "Gpt", "claude" -> "Claude")
		first := parts[0]
		if len(first) > 0 {
			return strings.ToUpper(first[:1]) + strings.ToLower(first[1:])
		}
		return first
	}

	return "Other"
}

// extractVersion extracts version info from model ID/name
func extractVersion(id, name string) string {
	// Common version patterns

	// Anthropic: claude-opus-4-5-20250929 -> 4.5
	if strings.Contains(id, "-4-5-") || strings.Contains(id, "-4.5") {
		return "4.5"
	}
	if strings.Contains(id, "-3-5-") || strings.Contains(id, "-3.5") {
		return "3.5"
	}
	if strings.Contains(id, "-3-") {
		return "3"
	}

	// OpenAI: gpt-4o-2024-05-13 -> extract date
	parts := strings.Split(id, "-")
	for i, p := range parts {
		// Check for date-like pattern (YYYYMMDD or YYYY-MM-DD)
		if len(p) == 8 && isNumeric(p) {
			return p[:4] + "-" + p[4:6] + "-" + p[6:]
		}
		// Check for version like "1106", "0125"
		if len(p) == 4 && isNumeric(p) && i > 0 {
			return p
		}
	}

	// Gemini: gemini-1.5-pro -> 1.5
	if strings.Contains(id, "-1.5-") {
		return "1.5"
	}
	if strings.Contains(id, "-2.0-") || strings.Contains(id, "-2-") {
		return "2.0"
	}

	return ""
}

// formatCost formats cost as string with $ prefix if non-zero
func formatCost(cost float64) string {
	if cost == 0 {
		return ""
	}
	return "$" + strings.TrimRight(strings.TrimRight(
		strings.Replace(
			strings.Replace(
				formatFloat(cost),
				".000000", "", 1),
			".00", "", 1),
		"0"), ".")
}

// formatFloat formats a float with up to 6 decimal places
func formatFloat(f float64) string {
	s := strings.TrimRight(strings.TrimRight(
		strings.Replace(
			strings.Replace(
				strings.Replace(
					strings.Replace(
						formatFloatRaw(f),
						".000000", ".0", 1),
					"00000", "", 1),
				"0000", "", 1),
			"000", "", 1),
		"0"), ".")
	if s == "" {
		return "0"
	}
	return s
}

// formatFloatRaw formats float to string
func formatFloatRaw(f float64) string {
	// Simple formatting without fmt package
	if f == 0 {
		return "0"
	}
	// Use a string builder approach
	var result []byte
	if f < 0 {
		result = append(result, '-')
		f = -f
	}
	// Integer part
	intPart := int64(f)
	fracPart := f - float64(intPart)

	// Convert integer part
	if intPart == 0 {
		result = append(result, '0')
	} else {
		digits := make([]byte, 0, 20)
		for intPart > 0 {
			digits = append(digits, byte('0'+intPart%10))
			intPart /= 10
		}
		for i := len(digits) - 1; i >= 0; i-- {
			result = append(result, digits[i])
		}
	}

	// Fractional part
	if fracPart > 0.000001 {
		result = append(result, '.')
		for i := 0; i < 6 && fracPart > 0.000001; i++ {
			fracPart *= 10
			digit := int(fracPart)
			result = append(result, byte('0'+digit))
			fracPart -= float64(digit)
		}
	}

	return string(result)
}

// isNumeric checks if string contains only digits
func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}
