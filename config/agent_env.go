package config

import (
	"os"
	"strings"
)

// LoadFromAgentEnv attempts to extract API keys from the agent.env file
func LoadFromAgentEnv(config *Config) error {
	envPath := "/home/agent/.env"
	data, err := os.ReadFile(envPath)
	if err != nil {
		return nil // It's okay if we can't find the file
	}

	// Parse the tab-delimited format
	lines := strings.Split(string(data), "\n")
	// Skip header line
	for i, line := range lines {
		if i == 0 || line == "" {
			continue
		}

		// Split by tab
		parts := strings.Split(line, "\t")
		if len(parts) < 5 {
			continue
		}

		keyName := parts[0]
		category := parts[1]
		apiKey := strings.TrimSpace(parts[2])

		// Only process LLM/Search providers for ModelScan
		if category != "LLM" && category != "LLM Router" && category != "Search" {
			continue
		}

		// Skip keys that are empty or just descriptive
		if apiKey == "" || strings.Contains(apiKey, "→") || strings.Contains(apiKey, " ") {
			continue
		}

		// Map to provider names
		providerName := getProviderNameFromKeyName(keyName)
		if providerName == "" {
			continue
		}

		// Create provider config
		providerConfig := ProviderConfig{
			APIKey:      apiKey,
			Description: "From agent.env",
		}

		// Set proper endpoints
		switch providerName {
		case "mistral":
			providerConfig.Endpoint = "https://api.mistral.ai/v1"
		case "openai":
			providerConfig.Endpoint = "https://api.openai.com/v1"
		case "anthropic":
			providerConfig.Endpoint = "https://api.anthropic.com/v1"
		case "xai":
			providerConfig.Endpoint = "https://api.x.ai/v1"
		case "cerebras":
			providerConfig.Endpoint = "https://api.cerebras.ai/v1"
		case "akashiverse":
			providerConfig.Endpoint = "https://api.akashiverse.ai/v1"
		case "perplexity":
			providerConfig.Endpoint = "https://api.perplexity.ai/v1"
		case "openrouter":
			providerConfig.Endpoint = "https://openrouter.ai/api/v1"
		case "gemini":
			providerConfig.Endpoint = "https://generativelanguage.googleapis.com/v1"
		case "firecrawl":
			providerConfig.Endpoint = "https://api.firecrawl.dev/v1"
		}

		// Only save if we don't already have a key from NEXORA
		if _, exists := config.Providers[providerName]; !exists {
			config.Providers[providerName] = providerConfig
		}
	}

	return nil
}

// getProviderNameFromKeyName maps key names to provider identifiers
func getProviderNameFromKeyName(keyName string) string {
	keyMap := map[string]string{
		"Mistral API Key":                     "mistral",
		"OpenAI service account key":          "openai",
		"OpenAI":                              "openai",
		"xAI API Key":                         "xai",
		"xAI API Key 3":                       "xai",
		"xAI API Key 4":                       "xai",
		"Anthropic API Key":                   "anthropic",
		"Anthropic OAuth Token — Claude Code": "anthropic",
		"Cere":                                "cerebras",
		"Cere max":                            "cerebras",
		"Akashiverse":                         "akashiverse",
		"Perplexity API Key":                  "perplexity",
		"Perplexity API Key 1":                "perplexity",
		"Perplexity API Key 2":                "perplexity",
		"OpenRouter API Key":                  "openrouter",
		"Firecrawl":                           "firecrawl",
		"Exa API Key 1":                       "exa",
		"Exa API Key 2":                       "exa",
		"Google":                              "gemini",
		"Vibe API Key":                        "vibe",
	}

	// Handle numbered keys e.g., "xAI API Key 3"
	keyBase := strings.TrimSuffix(keyName, "1")
	keyBase = strings.TrimSuffix(keyBase, "2")
	keyBase = strings.TrimSuffix(keyBase, "3")
	keyBase = strings.TrimSuffix(keyBase, "4")

	// Remove double spaces
	keyBase = strings.ReplaceAll(keyBase, "  ", " ")

	if provider, ok := keyMap[keyBase]; ok {
		return provider
	}

	// Try exact match
	if provider, ok := keyMap[keyName]; ok {
		return provider
	}

	// Extract from known patterns
	if strings.Contains(strings.ToLower(keyName), "gemini") {
		return "gemini"
	}
	if strings.Contains(strings.ToLower(keyName), "anthropic") ||
		strings.Contains(strings.ToLower(keyName), "claude") {
		return "anthropic"
	}
	if strings.Contains(strings.ToLower(keyName), "cere") {
		return "cerebras"
	}

	return ""
}

// extractFromGamma extracts Gamma API key from file
func extractFromGamma(config *Config) {
	// Look for gamma key in the agent env
	envPath := "/home/agent/.env"
	data, err := os.ReadFile(envPath)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, "sk-gamma-") {
			// Extract the key
			parts := strings.Split(line, "\t")
			if len(parts) >= 3 {
				gammaKey := strings.TrimSpace(parts[2])
				config.Providers["gamma"] = ProviderConfig{
					APIKey:      gammaKey,
					Description: "Gamma from agent.env",
					Endpoint:    "https://api.gamma.ai/v1",
				}
			}
		}
	}
}

// extractFromManus extracts Manus API key
func extractFromManus(config *Config) {
	envPath := "/home/agent/.env"
	data, err := os.ReadFile(envPath)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Manus") && strings.Contains(line, "sk-") {
			// Extract Manus key
			// Manus uses OpenAI-compatible API
			parts := strings.Split(line, "\t")
			if len(parts) >= 3 {
				manusKey := strings.TrimSpace(parts[2])
				config.Providers["manus"] = ProviderConfig{
					APIKey:      manusKey,
					Description: "Manus from agent.env",
					Endpoint:    "https://api.manus.ai/v1",
				}
			}
		}
	}
}

// extractFromLlamaIndex extracts LlamaIndex API key
func extractFromLlamaIndex(config *Config) {
	envPath := "/home/agent/.env"
	data, err := os.ReadFile(envPath)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Llamaindex") && strings.Contains(line, "llx-") {
			// Extract key
			parts := strings.Split(line, "\t")
			if len(parts) >= 3 {
				key := strings.TrimSpace(parts[2])
				config.Providers["llamaindex"] = ProviderConfig{
					APIKey:      key,
					Description: "Llamaindex from agent.env",
					Endpoint:    "https://api.llamaindex.ai/v1",
				}
			}
		}
	}
}

// extractFromNanoGPT extracts NanoGPT API key
func extractFromNanoGPT(config *Config) {
	envPath := "/home/agent/.env"
	data, err := os.ReadFile(envPath)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Nano GPT") {
			// Extract key
			parts := strings.Split(line, "\t")
			if len(parts) >= 3 {
				key := strings.TrimSpace(parts[2])
				config.Providers["nanogpt"] = ProviderConfig{
					APIKey:      key,
					Description: "NanoGPT from agent.env",
					Endpoint:    "https://api.nanogpt.ai/v1",
				}
			}
		}
	}
}

// extractYouCom extracts You.com API key
func extractYouCom(config *Config) {
	envPath := "/home/agent/.env"
	data, err := os.ReadFile(envPath)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, "You.com") && strings.Contains(line, "ydc-sk-") {
			// Extract key
			parts := strings.Split(line, "\t")
			if len(parts) >= 3 {
				key := strings.TrimSpace(parts[2])
				config.Providers["you"] = ProviderConfig{
					APIKey:      key,
					Description: "You.com from agent.env",
					Endpoint:    "https://api.you.com/v1",
				}
			}
		}
	}
}

// extractMinimax extracts Minimax API key (JWT type)
func extractMinimax(config *Config) {
	envPath := "/home/agent/.env"
	data, err := os.ReadFile(envPath)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Minimax API Key") && strings.Contains(line, "eyJ") {
			// Extract key (it's a JWT)
			parts := strings.Split(line, "\t")
			if len(parts) >= 3 {
				key := strings.TrimSpace(parts[2])
				config.Providers["minimax"] = ProviderConfig{
					APIKey:      key,
					Description: "Minimax from agent.env",
					Endpoint:    "https://api.minimax.chat/v1",
				}
			}
		}
	}
}

// extractKimiForCoding extracts Kimi coding API key
func extractKimiForCoding(config *Config) {
	envPath := "/home/agent/.env"
	data, err := os.ReadFile(envPath)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Kimi for Coding") {
			// Extract key
			parts := strings.Split(line, "\t")
			if len(parts) >= 3 {
				key := strings.TrimSpace(parts[2])
				config.Providers["kimi"] = ProviderConfig{
					APIKey:      key,
					Description: "Kimi for Coding from agent.env",
					Endpoint:    "https://api.moonshot.cn/v1",
				}
			}
		}
	}
}

// extractFromVibe extracts Vibe API key
func extractFromVibe(config *Config) {
	envPath := "/home/agent/.env"
	data, err := os.ReadFile(envPath)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Vibe API Key") {
			// Extract key
			parts := strings.Split(line, "\t")
			if len(parts) >= 3 {
				key := strings.TrimSpace(parts[2])
				config.Providers["vibe"] = ProviderConfig{
					APIKey:      key,
					Description: "Vibe from agent.env",
					Endpoint:    "https://api.vibe-llm.online/v1",
				}
			}
		}
	}
}
