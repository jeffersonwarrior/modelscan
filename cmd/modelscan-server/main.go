package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jeffersonwarrior/modelscan/internal/admin"
	"github.com/jeffersonwarrior/modelscan/internal/config"
	"github.com/jeffersonwarrior/modelscan/internal/database"
	"github.com/jeffersonwarrior/modelscan/internal/discovery"
	"github.com/jeffersonwarrior/modelscan/internal/generator"
	"github.com/jeffersonwarrior/modelscan/internal/keymanager"
)

const version = "0.3.0"

func main() {
	// Command-line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	initDB := flag.Bool("init", false, "Initialize database and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("modelscan-server version %s\n", version)
		fmt.Println("Auto-discovering SDK service with intelligent provider onboarding")
		os.Exit(0)
	}

	log.Printf("Starting modelscan v%s", version)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Printf("Warning: failed to load config: %v", err)
		log.Println("Using default configuration")
		cfg = config.DefaultConfig()
	}

	log.Printf("Database: %s", cfg.Database.Path)
	log.Printf("Server: %s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Agent Model: %s", cfg.Discovery.AgentModel)

	// Initialize database
	if *initDB {
		log.Println("Initializing database...")
		db, err := database.Open(cfg.Database.Path)
		if err != nil {
			log.Fatalf("Database initialization failed: %v", err)
		}
		defer db.Close()
		log.Println("✓ Database initialized successfully")
		os.Exit(0)
	}

	// Open database
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	log.Println("✓ Database opened")

	// Initialize discovery agent
	discoveryAgent, err := discovery.NewAgent(discovery.Config{
		Model:         cfg.Discovery.AgentModel,
		ParallelBatch: cfg.Discovery.ParallelBatch,
		CacheDays:     cfg.Discovery.CacheDays,
		MaxRetries:    3,
	})
	if err != nil {
		log.Fatalf("Failed to create discovery agent: %v", err)
	}
	defer discoveryAgent.Close()
	log.Println("✓ Discovery agent initialized")

	// Initialize SDK generator
	gen, err := generator.NewGenerator(generator.Config{
		OutputDir: "generated",
	})
	if err != nil {
		log.Fatalf("Failed to create generator: %v", err)
	}
	log.Println("✓ SDK generator initialized")

	// Initialize key manager with adapter
	dbAdapt := &dbAdapter{db: db}
	keyMgr := keymanager.NewKeyManager(dbAdapt, keymanager.Config{
		CacheTTL:        5 * time.Minute,
		DegradeDuration: 15 * time.Minute,
	})
	defer keyMgr.Close()
	log.Println("✓ Key manager initialized")

	// Create adapter for admin API
	apiAdapter := &adminAPIAdapter{
		db:         db,
		discovery:  discoveryAgent,
		generator:  gen,
		keyManager: keyMgr,
	}

	// Initialize admin API
	adminAPI := admin.NewAPI(
		admin.Config{Host: cfg.Server.Host, Port: cfg.Server.Port},
		apiAdapter,
		apiAdapter,
		apiAdapter,
		apiAdapter,
	)
	log.Println("✓ Admin API initialized")

	// Bootstrap from existing data
	log.Println("Bootstrapping from database...")
	providers, err := db.ListProviders()
	if err != nil {
		log.Printf("Warning: failed to list providers: %v", err)
	} else {
		log.Printf("Found %d existing providers", len(providers))
		for _, p := range providers {
			log.Printf("  - %s (%s)", p.Name, p.Status)
		}
	}

	// Start HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: adminAPI,
	}

	// Start server in background
	go func() {
		log.Printf("✓ HTTP server listening on %s", addr)
		log.Println("")
		log.Println("Admin API endpoints:")
		log.Printf("  - GET  http://%s/health", addr)
		log.Printf("  - GET  http://%s/api/providers", addr)
		log.Printf("  - POST http://%s/api/providers/add", addr)
		log.Printf("  - GET  http://%s/api/keys?provider=<id>", addr)
		log.Printf("  - POST http://%s/api/keys/add", addr)
		log.Printf("  - GET  http://%s/api/sdks", addr)
		log.Printf("  - GET  http://%s/api/stats?model=<id>", addr)
		log.Println("")
		log.Println("Press Ctrl+C to shutdown")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigChan

	log.Printf("Received signal: %v", sig)
	log.Println("Initiating graceful shutdown...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("✓ Shutdown complete")
}

// adminAPIAdapter adapts our components to the admin API interfaces
type adminAPIAdapter struct {
	db         *database.DB
	discovery  *discovery.Agent
	generator  *generator.Generator
	keyManager *keymanager.KeyManager
}

// Database interface implementation
func (a *adminAPIAdapter) CreateProvider(p *admin.Provider) error {
	return a.db.CreateProvider(&database.Provider{
		ID:           p.ID,
		Name:         p.Name,
		BaseURL:      p.BaseURL,
		AuthMethod:   p.AuthMethod,
		PricingModel: p.PricingModel,
		Status:       p.Status,
	})
}

func (a *adminAPIAdapter) GetProvider(id string) (*admin.Provider, error) {
	p, err := a.db.GetProvider(id)
	if err != nil || p == nil {
		return nil, err
	}
	return &admin.Provider{
		ID:           p.ID,
		Name:         p.Name,
		BaseURL:      p.BaseURL,
		AuthMethod:   p.AuthMethod,
		PricingModel: p.PricingModel,
		Status:       p.Status,
	}, nil
}

func (a *adminAPIAdapter) ListProviders() ([]*admin.Provider, error) {
	providers, err := a.db.ListProviders()
	if err != nil {
		return nil, err
	}
	result := make([]*admin.Provider, len(providers))
	for i, p := range providers {
		result[i] = &admin.Provider{
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

func (a *adminAPIAdapter) CreateAPIKey(providerID, apiKey string) (*admin.APIKey, error) {
	key, err := a.db.CreateAPIKey(providerID, apiKey)
	if err != nil {
		return nil, err
	}
	return &admin.APIKey{
		ID:            key.ID,
		ProviderID:    key.ProviderID,
		KeyPrefix:     key.KeyPrefix,
		RequestsCount: key.RequestsCount,
		TokensCount:   key.TokensCount,
		Active:        key.Active,
		Degraded:      key.Degraded,
	}, nil
}

func (a *adminAPIAdapter) ListActiveAPIKeys(providerID string) ([]*admin.APIKey, error) {
	keys, err := a.db.ListActiveAPIKeys(providerID)
	if err != nil {
		return nil, err
	}
	result := make([]*admin.APIKey, len(keys))
	for i, k := range keys {
		result[i] = &admin.APIKey{
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

func (a *adminAPIAdapter) GetUsageStats(modelID string, since time.Time) (map[string]interface{}, error) {
	return a.db.GetUsageStats(modelID, since)
}

// Discovery interface implementation
func (a *adminAPIAdapter) Discover(providerID string, apiKey string) (*admin.DiscoveryResult, error) {
	result, err := a.discovery.Discover(context.Background(), discovery.DiscoveryRequest{
		Identifier: providerID,
		APIKey:     apiKey,
	})
	if err != nil {
		return nil, err
	}

	return &admin.DiscoveryResult{
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

// Generator interface implementation
func (a *adminAPIAdapter) Generate(req admin.GenerateRequest) (*admin.GenerateResult, error) {
	result, err := a.generator.Generate(generator.GenerateRequest{
		ProviderID:   req.ProviderID,
		ProviderName: req.ProviderName,
		BaseURL:      req.BaseURL,
		SDKType:      req.SDKType,
	})
	if err != nil {
		return nil, err
	}

	return &admin.GenerateResult{
		FilePath: result.FilePath,
		Success:  result.Success,
		Error:    result.Error,
	}, nil
}

func (a *adminAPIAdapter) List() ([]string, error) {
	return a.generator.List()
}

func (a *adminAPIAdapter) Delete(providerID string) error {
	return a.generator.Delete(providerID)
}

// KeyManager interface implementation
func (a *adminAPIAdapter) GetKey(providerID string) (*admin.APIKey, error) {
	key, err := a.keyManager.GetKey(context.Background(), providerID)
	if err != nil {
		return nil, err
	}
	return &admin.APIKey{
		ID:         key.ID,
		ProviderID: key.ProviderID,
		KeyPrefix:  key.KeyPrefix,
	}, nil
}

func (a *adminAPIAdapter) ListKeys(providerID string) ([]*admin.APIKey, error) {
	keys, err := a.keyManager.ListKeys(context.Background(), providerID)
	if err != nil {
		return nil, err
	}
	result := make([]*admin.APIKey, len(keys))
	for i, k := range keys {
		result[i] = &admin.APIKey{
			ID:         k.ID,
			ProviderID: k.ProviderID,
			KeyPrefix:  k.KeyPrefix,
		}
	}
	return result, nil
}
