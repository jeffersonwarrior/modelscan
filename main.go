package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jeffersonwarrior/modelscan/config"
	"github.com/jeffersonwarrior/modelscan/providers"
	"github.com/jeffersonwarrior/modelscan/storage"
)

var (
	providerName = flag.String("provider", "all", "Provider to validate (mistral, openai, anthropic, all)")
	outputFormat = flag.String("format", "all", "Output format (sqlite, markdown, all)")
	outputPath   = flag.String("output", ".", "Output directory for results")
	configFile   = flag.String("config", "", "Path to config file with API keys")
	verbose      = flag.Bool("verbose", false, "Verbose output")
)

func main() {
	flag.Parse()

	ctx := context.Background()

	// Load configuration from environment and NEXORA setup
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database if SQLite output is requested OR if we need markdown from SQLite
	if *outputFormat == "all" || *outputFormat == "sqlite" || *outputFormat == "markdown" {
		dbPath := filepath.Join(*outputPath, "providers.db")
		if err := storage.InitDB(dbPath); err != nil {
			log.Printf("Warning: Failed to initialize database: %v", err)
		}
	}

	if *providerName == "all" {
		// Validate all configured providers
		for _, name := range cfg.ListProviders() {
			if err := validateProvider(ctx, name, cfg); err != nil {
				log.Printf("Error validating provider %s: %v", name, err)
			}
		}
	} else {
		// Validate specific provider
		if !cfg.HasProvider(*providerName) {
			log.Fatalf("Provider %s is not configured or missing API key", *providerName)
		}

		if err := validateProvider(ctx, *providerName, cfg); err != nil {
			log.Fatalf("Error validating provider %s: %v", *providerName, err)
		}
	}

	// Export results in requested formats
	if *outputFormat == "all" || *outputFormat == "sqlite" {
		dbPath := filepath.Join(*outputPath, "providers.db")
		if err := storage.ExportToSQLite(dbPath); err != nil {
			log.Printf("Error exporting to SQLite: %v", err)
		} else {
			fmt.Printf("✓ Saved SQLite database to %s\n", dbPath)
		}
	}

	if *outputFormat == "all" || *outputFormat == "markdown" {
		mdPath := filepath.Join(*outputPath, "PROVIDERS.md")
		// Ensure parent directory exists, not create the file as a directory
		if err := os.MkdirAll(*outputPath, 0o755); err != nil {
			log.Printf("Error creating output directory: %v", err)
		}
		if err := storage.ExportToMarkdown(mdPath); err != nil {
			log.Printf("Error exporting to Markdown: %v", err)
		} else {
			fmt.Printf("✓ Saved Markdown report to %s\n", mdPath)
		}
	}

	fmt.Println("\nValidation complete!")
}

func validateProvider(ctx context.Context, name string, cfg *config.Config) error {
	fmt.Printf("\n=== Validating %s Provider ===\n", name)

	// Get provider factory
	factory, exists := providers.GetProviderFactory(name)
	if !exists {
		return fmt.Errorf("unknown provider: %s", name)
	}

	// Get API key from config
	apiKey, err := cfg.GetAPIKey(name)
	if err != nil {
		return err
	}

	// Create provider instance with API key
	provider := factory(apiKey)

	// Validate endpoints
	if err := provider.ValidateEndpoints(ctx, *verbose); err != nil {
		return fmt.Errorf("endpoint validation failed: %w", err)
	}

	// List available models
	models, err := provider.ListModels(ctx, *verbose)
	if err != nil {
		return fmt.Errorf("model listing failed: %w", err)
	}

	fmt.Printf("Found %d models for %s\n", len(models), name)

	// Store results
	if err := storage.StoreProviderInfo(name, models, provider.GetCapabilities()); err != nil {
		return fmt.Errorf("failed to store provider info: %w", err)
	}

	// Also store endpoint results
	endpoints := provider.GetEndpoints()
	if err := storage.StoreEndpointResults(name, endpoints); err != nil {
		return fmt.Errorf("failed to store endpoint info: %w", err)
	}

	return nil
}
