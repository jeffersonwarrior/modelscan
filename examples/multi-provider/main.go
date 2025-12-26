package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jeffersonwarrior/modelscan/providers"
)

func main() {
	ctx := context.Background()
	fmt.Println("ModelScan Multi-Provider Comparison")

	// OpenAI
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		openai := providers.NewOpenAIProvider(apiKey)
		models, err := openai.ListModels(ctx, false)
		if err == nil && len(models) > 0 {
			fmt.Printf("OpenAI: %d models available\n", len(models))
			fmt.Printf("  Latest: %s\n", models[0].ID)
		}
	}

	// Anthropic
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		anthropic := providers.NewAnthropicProvider(apiKey)
		models, err := anthropic.ListModels(ctx, false)
		if err == nil && len(models) > 0 {
			fmt.Printf("Anthropic: %d models available\n", len(models))
			fmt.Printf("  Latest: %s\n", models[0].ID)
		}
	}

	// Google
	if apiKey := os.Getenv("GOOGLE_API_KEY"); apiKey != "" {
		google := providers.NewGoogleProvider(apiKey)
		models, err := google.ListModels(ctx, false)
		if err == nil && len(models) > 0 {
			fmt.Printf("Google: %d models available\n", len(models))
			fmt.Printf("  Latest: %s\n", models[0].ID)
		}
	}

	fmt.Println("\nSet API keys as environment variables to enable providers")
}
