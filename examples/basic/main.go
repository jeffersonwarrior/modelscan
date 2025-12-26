package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jeffersonwarrior/modelscan/providers"
)

func main() {
	// Get API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable not set")
	}

	// Create OpenAI provider
	provider := providers.NewOpenAIProvider(apiKey)

	// List available models
	ctx := context.Background()
	models, err := provider.ListModels(ctx, false)
	if err != nil {
		log.Fatalf("Error listing models: %v", err)
	}

	fmt.Printf("Found %d OpenAI models\n", len(models))

	if len(models) > 0 {
		fmt.Printf("Example model: %s (%s)\n", models[0].ID, models[0].Name)
		fmt.Printf("  Context: %d tokens\n", models[0].ContextWindow)
		fmt.Printf("  Supports vision: %v\n", models[0].SupportsImages)
		fmt.Printf("  Supports tools: %v\n", models[0].SupportsTools)
	}
}
