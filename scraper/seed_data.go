package scraper

import (
	"database/sql"
	"log"
	"time"

	"github.com/jeffersonwarrior/modelscan/storage"
)

// SeedInitialRateLimits populates the database with known rate limits for core providers
func SeedInitialRateLimits() error {
	log.Println("Seeding initial rate limits for 15 core providers...")

	// OpenAI Rate Limits (5 tiers)
	openaiLimits := []storage.RateLimit{
		// Tier 1 (Free)
		{ProviderName: "openai", PlanType: "tier-1", LimitType: "rpm", LimitValue: 500, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://platform.openai.com/docs/guides/rate-limits", LastVerified: time.Now()},
		{ProviderName: "openai", PlanType: "tier-1", LimitType: "tpm", LimitValue: 200000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://platform.openai.com/docs/guides/rate-limits", LastVerified: time.Now()},
		{ProviderName: "openai", PlanType: "tier-1", LimitType: "rpd", LimitValue: 10000, ResetWindowSeconds: 86400, AppliesTo: "account", SourceURL: "https://platform.openai.com/docs/guides/rate-limits", LastVerified: time.Now()},

		// Tier 2
		{ProviderName: "openai", PlanType: "tier-2", LimitType: "rpm", LimitValue: 3500, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://platform.openai.com/docs/guides/rate-limits", LastVerified: time.Now()},
		{ProviderName: "openai", PlanType: "tier-2", LimitType: "tpm", LimitValue: 450000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://platform.openai.com/docs/guides/rate-limits", LastVerified: time.Now()},

		// Tier 3
		{ProviderName: "openai", PlanType: "tier-3", LimitType: "rpm", LimitValue: 5000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://platform.openai.com/docs/guides/rate-limits", LastVerified: time.Now()},
		{ProviderName: "openai", PlanType: "tier-3", LimitType: "tpm", LimitValue: 1000000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://platform.openai.com/docs/guides/rate-limits", LastVerified: time.Now()},

		// Tier 4
		{ProviderName: "openai", PlanType: "tier-4", LimitType: "rpm", LimitValue: 10000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://platform.openai.com/docs/guides/rate-limits", LastVerified: time.Now()},
		{ProviderName: "openai", PlanType: "tier-4", LimitType: "tpm", LimitValue: 10000000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://platform.openai.com/docs/guides/rate-limits", LastVerified: time.Now()},

		// Tier 5
		{ProviderName: "openai", PlanType: "tier-5", LimitType: "rpm", LimitValue: 30000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://platform.openai.com/docs/guides/rate-limits", LastVerified: time.Now()},
		{ProviderName: "openai", PlanType: "tier-5", LimitType: "tpm", LimitValue: 100000000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://platform.openai.com/docs/guides/rate-limits", LastVerified: time.Now()},
	}

	// Anthropic Rate Limits (4 tiers)
	anthropicLimits := []storage.RateLimit{
		// Tier 1
		{ProviderName: "anthropic", PlanType: "tier-1", LimitType: "rpm", LimitValue: 50, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://docs.anthropic.com/en/api/rate-limits", LastVerified: time.Now()},
		{ProviderName: "anthropic", PlanType: "tier-1", LimitType: "tpm", LimitValue: 100000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://docs.anthropic.com/en/api/rate-limits", LastVerified: time.Now()},
		{ProviderName: "anthropic", PlanType: "tier-1", LimitType: "rpd", LimitValue: 1000, ResetWindowSeconds: 86400, AppliesTo: "account", SourceURL: "https://docs.anthropic.com/en/api/rate-limits", LastVerified: time.Now()},

		// Tier 2
		{ProviderName: "anthropic", PlanType: "tier-2", LimitType: "rpm", LimitValue: 1000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://docs.anthropic.com/en/api/rate-limits", LastVerified: time.Now()},
		{ProviderName: "anthropic", PlanType: "tier-2", LimitType: "tpm", LimitValue: 300000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://docs.anthropic.com/en/api/rate-limits", LastVerified: time.Now()},

		// Tier 3
		{ProviderName: "anthropic", PlanType: "tier-3", LimitType: "rpm", LimitValue: 2000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://docs.anthropic.com/en/api/rate-limits", LastVerified: time.Now()},
		{ProviderName: "anthropic", PlanType: "tier-3", LimitType: "tpm", LimitValue: 1000000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://docs.anthropic.com/en/api/rate-limits", LastVerified: time.Now()},

		// Tier 4
		{ProviderName: "anthropic", PlanType: "tier-4", LimitType: "rpm", LimitValue: 4000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://docs.anthropic.com/en/api/rate-limits", LastVerified: time.Now()},
		{ProviderName: "anthropic", PlanType: "tier-4", LimitType: "tpm", LimitValue: 4000000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://docs.anthropic.com/en/api/rate-limits", LastVerified: time.Now()},
	}

	// DeepSeek Rate Limits
	deepseekLimits := []storage.RateLimit{
		{ProviderName: "deepseek", PlanType: "pay_per_go", LimitType: "rpm", LimitValue: 60, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://platform.deepseek.com/api-docs/pricing/", LastVerified: time.Now()},
		{ProviderName: "deepseek", PlanType: "pay_per_go", LimitType: "rph", LimitValue: 3600, ResetWindowSeconds: 3600, AppliesTo: "account", SourceURL: "https://platform.deepseek.com/api-docs/pricing/", LastVerified: time.Now()},
	}

	// Cerebras Rate Limits
	cerebrasLimits := []storage.RateLimit{
		{ProviderName: "cerebras", PlanType: "free", LimitType: "rpm", LimitValue: 30, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://inference-docs.cerebras.ai/api-reference/rate-limits", LastVerified: time.Now()},
		{ProviderName: "cerebras", PlanType: "free", LimitType: "tpm", LimitValue: 1000000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://inference-docs.cerebras.ai/api-reference/rate-limits", LastVerified: time.Now()},
		{ProviderName: "cerebras", PlanType: "pay_per_go", LimitType: "rpm", LimitValue: 900, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://inference-docs.cerebras.ai/api-reference/rate-limits", LastVerified: time.Now()},
	}

	// Google Gemini Rate Limits
	geminiLimits := []storage.RateLimit{
		{ProviderName: "google-gemini", PlanType: "free", LimitType: "rpm", LimitValue: 15, ResetWindowSeconds: 60, AppliesTo: "model", ModelID: sql.NullString{String: "gemini-2.0-flash", Valid: true}, SourceURL: "https://ai.google.dev/pricing", LastVerified: time.Now()},
		{ProviderName: "google-gemini", PlanType: "free", LimitType: "tpm", LimitValue: 1000000, ResetWindowSeconds: 60, AppliesTo: "model", ModelID: sql.NullString{String: "gemini-2.0-flash", Valid: true}, SourceURL: "https://ai.google.dev/pricing", LastVerified: time.Now()},
		{ProviderName: "google-gemini", PlanType: "pay_per_go", LimitType: "rpm", LimitValue: 1000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://ai.google.dev/pricing", LastVerified: time.Now()},
		{ProviderName: "google-gemini", PlanType: "pay_per_go", LimitType: "tpm", LimitValue: 4000000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://ai.google.dev/pricing", LastVerified: time.Now()},
	}

	// ElevenLabs Rate Limits (audio)
	elevenLabsLimits := []storage.RateLimit{
		{ProviderName: "elevenlabs", PlanType: "free", LimitType: "rpm", LimitValue: 2, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://elevenlabs.io/docs/api-reference/rate-limits", LastVerified: time.Now()},
		{ProviderName: "elevenlabs", PlanType: "starter", LimitType: "rpm", LimitValue: 20, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://elevenlabs.io/docs/api-reference/rate-limits", LastVerified: time.Now()},
		{ProviderName: "elevenlabs", PlanType: "pro", LimitType: "rpm", LimitValue: 50, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://elevenlabs.io/docs/api-reference/rate-limits", LastVerified: time.Now()},
	}

	// Deepgram Rate Limits (audio)
	deepgramLimits := []storage.RateLimit{
		{ProviderName: "deepgram", PlanType: "pay_per_go", LimitType: "concurrent", LimitValue: 250, ResetWindowSeconds: 0, AppliesTo: "account", SourceURL: "https://developers.deepgram.com/docs/rate-limits", LastVerified: time.Now()},
	}

	// Groq Rate Limits
	groqLimits := []storage.RateLimit{
		{ProviderName: "groq", PlanType: "free", LimitType: "rpm", LimitValue: 30, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://console.groq.com/docs/rate-limits", LastVerified: time.Now()},
		{ProviderName: "groq", PlanType: "free", LimitType: "rpd", LimitValue: 14400, ResetWindowSeconds: 86400, AppliesTo: "account", SourceURL: "https://console.groq.com/docs/rate-limits", LastVerified: time.Now()},
		{ProviderName: "groq", PlanType: "pay_per_go", LimitType: "rpm", LimitValue: 7000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://console.groq.com/docs/rate-limits", LastVerified: time.Now()},
	}

	// Replicate Rate Limits
	replicateLimits := []storage.RateLimit{
		{ProviderName: "replicate", PlanType: "free", LimitType: "concurrent", LimitValue: 1, ResetWindowSeconds: 0, AppliesTo: "account", SourceURL: "https://replicate.com/docs/reference/http", LastVerified: time.Now()},
		{ProviderName: "replicate", PlanType: "pay_per_go", LimitType: "concurrent", LimitValue: 100, ResetWindowSeconds: 0, AppliesTo: "account", SourceURL: "https://replicate.com/docs/reference/http", LastVerified: time.Now()},
	}

	// Together.ai Rate Limits
	togetherLimits := []storage.RateLimit{
		{ProviderName: "together", PlanType: "free", LimitType: "rpm", LimitValue: 60, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://docs.together.ai/docs/rate-limits", LastVerified: time.Now()},
		{ProviderName: "together", PlanType: "pay_per_go", LimitType: "rpm", LimitValue: 600, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://docs.together.ai/docs/rate-limits", LastVerified: time.Now()},
	}

	// Cohere Rate Limits
	cohereLimits := []storage.RateLimit{
		{ProviderName: "cohere", PlanType: "free", LimitType: "rpm", LimitValue: 100, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://docs.cohere.com/docs/rate-limits", LastVerified: time.Now()},
		{ProviderName: "cohere", PlanType: "pay_per_go", LimitType: "rpm", LimitValue: 10000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://docs.cohere.com/docs/rate-limits", LastVerified: time.Now()},
	}

	// Mistral Rate Limits
	mistralLimits := []storage.RateLimit{
		{ProviderName: "mistral", PlanType: "free", LimitType: "rpm", LimitValue: 1, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://docs.mistral.ai/api/#rate-limits", LastVerified: time.Now()},
		{ProviderName: "mistral", PlanType: "pay_per_go", LimitType: "rpm", LimitValue: 5, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://docs.mistral.ai/api/#rate-limits", LastVerified: time.Now()},
	}

	// Perplexity Rate Limits
	perplexityLimits := []storage.RateLimit{
		{ProviderName: "perplexity", PlanType: "free", LimitType: "rpm", LimitValue: 20, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://docs.perplexity.ai/docs/rate-limits", LastVerified: time.Now()},
		{ProviderName: "perplexity", PlanType: "pro", LimitType: "rpm", LimitValue: 50, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://docs.perplexity.ai/docs/rate-limits", LastVerified: time.Now()},
	}

	// xAI (Grok) Rate Limits
	xaiLimits := []storage.RateLimit{
		{ProviderName: "xai", PlanType: "pay_per_go", LimitType: "rpm", LimitValue: 60, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://docs.x.ai/api/rate-limits", LastVerified: time.Now()},
		{ProviderName: "xai", PlanType: "pay_per_go", LimitType: "tpm", LimitValue: 1000000, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://docs.x.ai/api/rate-limits", LastVerified: time.Now()},
	}

	// AI21 Rate Limits
	ai21Limits := []storage.RateLimit{
		{ProviderName: "ai21", PlanType: "free", LimitType: "rpm", LimitValue: 10, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://docs.ai21.com/docs/rate-limits", LastVerified: time.Now()},
		{ProviderName: "ai21", PlanType: "pay_per_go", LimitType: "rpm", LimitValue: 300, ResetWindowSeconds: 60, AppliesTo: "account", SourceURL: "https://docs.ai21.com/docs/rate-limits", LastVerified: time.Now()},
	}

	// Combine all limits
	allLimits := append(openaiLimits, anthropicLimits...)
	allLimits = append(allLimits, deepseekLimits...)
	allLimits = append(allLimits, cerebrasLimits...)
	allLimits = append(allLimits, geminiLimits...)
	allLimits = append(allLimits, elevenLabsLimits...)
	allLimits = append(allLimits, deepgramLimits...)
	allLimits = append(allLimits, groqLimits...)
	allLimits = append(allLimits, replicateLimits...)
	allLimits = append(allLimits, togetherLimits...)
	allLimits = append(allLimits, cohereLimits...)
	allLimits = append(allLimits, mistralLimits...)
	allLimits = append(allLimits, perplexityLimits...)
	allLimits = append(allLimits, xaiLimits...)
	allLimits = append(allLimits, ai21Limits...)

	// Insert all rate limits
	for _, limit := range allLimits {
		if err := storage.InsertRateLimit(limit); err != nil {
			return err
		}
	}

	log.Printf("✅ Inserted %d rate limits for 15 providers", len(allLimits))
	return nil
}

// SeedInitialPricing populates the database with known pricing for core providers
func SeedInitialPricing() error {
	log.Println("Seeding initial pricing for 15 core providers...")

	pricing := []storage.ProviderPricing{
		// OpenAI
		{ProviderName: "openai", ModelID: "gpt-4o", PlanType: "tier-1", InputCost: 2.50, OutputCost: 10.00, UnitType: "1M tokens", Currency: "USD"},
		{ProviderName: "openai", ModelID: "gpt-4o-mini", PlanType: "tier-1", InputCost: 0.15, OutputCost: 0.60, UnitType: "1M tokens", Currency: "USD"},
		{ProviderName: "openai", ModelID: "o1", PlanType: "tier-1", InputCost: 15.00, OutputCost: 60.00, UnitType: "1M tokens", Currency: "USD"},

		// Anthropic
		{ProviderName: "anthropic", ModelID: "claude-3.5-sonnet", PlanType: "tier-1", InputCost: 3.00, OutputCost: 15.00, UnitType: "1M tokens", Currency: "USD"},
		{ProviderName: "anthropic", ModelID: "claude-3.5-haiku", PlanType: "tier-1", InputCost: 0.80, OutputCost: 4.00, UnitType: "1M tokens", Currency: "USD"},

		// DeepSeek
		{ProviderName: "deepseek", ModelID: "deepseek-chat", PlanType: "pay_per_go", InputCost: 0.14, OutputCost: 0.28, UnitType: "1M tokens", Currency: "USD"},
		{ProviderName: "deepseek", ModelID: "deepseek-reasoner", PlanType: "pay_per_go", InputCost: 0.55, OutputCost: 2.19, UnitType: "1M tokens", Currency: "USD"},

		// Cerebras (FREE!)
		{ProviderName: "cerebras", ModelID: "llama3.1-8b", PlanType: "free", InputCost: 0.00, OutputCost: 0.00, UnitType: "1M tokens", Currency: "USD", IncludedUnits: sql.NullInt64{Int64: 1000000, Valid: true}},
		{ProviderName: "cerebras", ModelID: "llama3.1-70b", PlanType: "pay_per_go", InputCost: 0.60, OutputCost: 0.60, UnitType: "1M tokens", Currency: "USD"},

		// Google Gemini
		{ProviderName: "google-gemini", ModelID: "gemini-2.0-flash", PlanType: "free", InputCost: 0.00, OutputCost: 0.00, UnitType: "1M tokens", Currency: "USD", IncludedUnits: sql.NullInt64{Int64: 1500, Valid: true}},
		{ProviderName: "google-gemini", ModelID: "gemini-1.5-pro", PlanType: "pay_per_go", InputCost: 1.25, OutputCost: 5.00, UnitType: "1M tokens", Currency: "USD"},

		// ElevenLabs
		{ProviderName: "elevenlabs", ModelID: "eleven_turbo_v2_5", PlanType: "free", InputCost: 0.00, OutputCost: 0.00, UnitType: "per character", Currency: "USD", IncludedUnits: sql.NullInt64{Int64: 10000, Valid: true}},
		{ProviderName: "elevenlabs", ModelID: "eleven_turbo_v2_5", PlanType: "starter", InputCost: 0.00003, OutputCost: 0.00003, UnitType: "per character", Currency: "USD"},

		// Deepgram
		{ProviderName: "deepgram", ModelID: "nova-2", PlanType: "pay_per_go", InputCost: 0.0043, OutputCost: 0.0043, UnitType: "per second", Currency: "USD"},

		// Groq
		{ProviderName: "groq", ModelID: "llama-3.3-70b", PlanType: "pay_per_go", InputCost: 0.59, OutputCost: 0.79, UnitType: "1M tokens", Currency: "USD"},
		{ProviderName: "groq", ModelID: "mixtral-8x7b", PlanType: "pay_per_go", InputCost: 0.24, OutputCost: 0.24, UnitType: "1M tokens", Currency: "USD"},

		// Together.ai
		{ProviderName: "together", ModelID: "meta-llama/Meta-Llama-3.1-70B", PlanType: "pay_per_go", InputCost: 0.60, OutputCost: 0.60, UnitType: "1M tokens", Currency: "USD"},

		// Cohere
		{ProviderName: "cohere", ModelID: "command-r-plus", PlanType: "pay_per_go", InputCost: 2.50, OutputCost: 10.00, UnitType: "1M tokens", Currency: "USD"},

		// Mistral
		{ProviderName: "mistral", ModelID: "mistral-large", PlanType: "pay_per_go", InputCost: 2.00, OutputCost: 6.00, UnitType: "1M tokens", Currency: "USD"},
	}

	for _, p := range pricing {
		if err := storage.InsertProviderPricing(p); err != nil {
			return err
		}
	}

	log.Printf("✅ Inserted %d pricing entries", len(pricing))
	return nil
}
