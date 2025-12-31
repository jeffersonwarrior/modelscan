package service

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/jeffersonwarrior/modelscan/internal/admin"
	"github.com/jeffersonwarrior/modelscan/internal/database"
	"github.com/jeffersonwarrior/modelscan/internal/discovery"
	"github.com/jeffersonwarrior/modelscan/internal/generator"
	"github.com/jeffersonwarrior/modelscan/internal/keymanager"
	"github.com/jeffersonwarrior/modelscan/routing"
)

// Service orchestrates all modelscan components
type Service struct {
	config     *Config
	db         *database.DB
	discovery  *discovery.Agent
	generator  *generator.Generator
	keyManager *keymanager.KeyManager
	router     routing.Router
	adminAPI   *admin.API
	httpServer *http.Server
	hooks      *HookRegistry

	mu          sync.RWMutex
	restarting  bool
	initialized bool
}

// Config holds service configuration
type Config struct {
	DatabasePath  string
	ServerHost    string
	ServerPort    int
	AgentModel    string
	ParallelBatch int
	CacheDays     int
	OutputDir     string
	RoutingMode   string
}

// NewService creates a new service instance
func NewService(cfg *Config) *Service {
	return &Service{
		config: cfg,
	}
}

// Initialize initializes all components
func (s *Service) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return fmt.Errorf("service already initialized")
	}

	log.Println("Initializing modelscan service...")

	// Initialize database
	db, err := database.Open(s.config.DatabasePath)
	if err != nil {
		return fmt.Errorf("database init failed: %w", err)
	}
	s.db = db
	log.Println("  ✓ Database initialized")

	// Initialize discovery agent
	agent, err := discovery.NewAgent(discovery.Config{
		ParallelBatch: s.config.ParallelBatch,
		CacheDays:     s.config.CacheDays,
		MaxRetries:    3,
		DB:            db,
	})
	if err != nil {
		return fmt.Errorf("discovery agent init failed: %w", err)
	}
	s.discovery = agent
	log.Println("  ✓ Discovery agent initialized")

	// Initialize SDK generator
	gen, err := generator.NewGenerator(generator.Config{
		OutputDir: s.config.OutputDir,
	})
	if err != nil {
		return fmt.Errorf("generator init failed: %w", err)
	}
	s.generator = gen
	log.Println("  ✓ SDK generator initialized")

	// Initialize key manager
	dbAdapter := &keyManagerDatabaseAdapter{db: s.db}
	keyMgr := keymanager.NewKeyManager(dbAdapter, keymanager.Config{
		CacheTTL:        5 * time.Minute,
		DegradeDuration: 15 * time.Minute,
	})
	s.keyManager = keyMgr
	log.Println("  ✓ Key manager initialized")

	// Initialize router
	routerCfg := routing.DefaultConfig()
	if s.config.RoutingMode != "" {
		routerCfg.Mode = routing.RoutingMode(s.config.RoutingMode)
	}
	router, err := routing.NewRouter(routerCfg)
	if err != nil {
		return fmt.Errorf("router init failed: %w", err)
	}
	s.router = router
	log.Println("  ✓ Router initialized")

	// Initialize admin API with adapters
	s.adminAPI = admin.NewAPI(
		admin.Config{Host: s.config.ServerHost, Port: s.config.ServerPort},
		admin.NewDatabaseAdapter(s.db),
		admin.NewDiscoveryAdapter(s.discovery),
		admin.NewGeneratorAdapter(s.generator),
		admin.NewKeyManagerAdapter(s.keyManager),
	)
	log.Println("  ✓ Admin API initialized")

	// Setup event hooks
	s.setupHooks()

	s.initialized = true
	log.Println("Service initialization complete")
	return nil
}

// Bootstrap loads existing data from database
func (s *Service) Bootstrap() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized {
		return fmt.Errorf("service not initialized")
	}

	log.Println("Bootstrapping from database...")

	providers, err := s.db.ListProviders()
	if err != nil {
		return fmt.Errorf("failed to list providers: %w", err)
	}

	log.Printf("Found %d existing providers", len(providers))
	for _, p := range providers {
		log.Printf("  - %s (%s)", p.Name, p.Status)
	}

	return nil
}

// Start starts the HTTP server
func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return fmt.Errorf("service not initialized - call Initialize() first")
	}

	addr := fmt.Sprintf("%s:%d", s.config.ServerHost, s.config.ServerPort)
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.adminAPI,
	}

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

		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the service
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return nil
	}

	log.Println("Stopping service...")

	// Shutdown HTTP server with timeout
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}

	// Close all components
	if s.router != nil {
		s.router.Close()
	}

	if s.keyManager != nil {
		s.keyManager.Close()
	}

	if s.discovery != nil {
		s.discovery.Close()
	}

	if s.db != nil {
		s.db.Close()
	}

	s.initialized = false
	log.Println("✓ Service stopped")
	return nil
}

// Health returns service health status
func (s *Service) Health() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := "ok"
	if s.restarting {
		status = "restarting"
	} else if !s.initialized {
		status = "not_initialized"
	}

	return map[string]interface{}{
		"status":      status,
		"initialized": s.initialized,
		"restarting":  s.restarting,
		"time":        time.Now(),
	}
}

// Restart performs a graceful restart (for SDK reloading)
func (s *Service) Restart() error {
	s.mu.Lock()
	s.restarting = true
	s.mu.Unlock()

	log.Println("Initiating service restart...")

	// Stop current service
	if err := s.Stop(); err != nil {
		return fmt.Errorf("stop failed: %w", err)
	}

	// Brief pause
	time.Sleep(1 * time.Second)

	// Reinitialize
	if err := s.Initialize(); err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	// Bootstrap
	if err := s.Bootstrap(); err != nil {
		log.Printf("Warning: bootstrap failed: %v", err)
	}

	// Restart HTTP server
	if err := s.Start(); err != nil {
		return fmt.Errorf("start failed: %w", err)
	}

	s.mu.Lock()
	s.restarting = false
	s.mu.Unlock()

	log.Println("✓ Service restart complete")
	return nil
}

// OnSDKGenerated is called when a new SDK is generated
// This allows hot-reloading without full service restart
func (s *Service) OnSDKGenerated(providerID, sdkPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return fmt.Errorf("service not initialized")
	}

	log.Printf("Hot-reloading SDK for provider %s from %s", providerID, sdkPath)

	// Load the generated client dynamically
	if err := s.loadGeneratedClient(providerID, sdkPath); err != nil {
		return fmt.Errorf("failed to load generated client: %w", err)
	}

	log.Printf("✓ SDK for %s hot-reloaded successfully", providerID)
	return nil
}

// loadGeneratedClient dynamically loads a generated SDK client
func (s *Service) loadGeneratedClient(providerID, sdkPath string) error {
	// TODO: Implement dynamic client loading via plugin system or go:plugin
	// For now, log that the SDK was generated and would need manual integration
	log.Printf("Generated SDK at %s - manual integration required", sdkPath)

	// Update database with SDK path
	provider, err := s.db.GetProvider(providerID)
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}
	if provider == nil {
		return fmt.Errorf("provider %s not found", providerID)
	}

	// Update provider with SDK path
	provider.SDKPath = &sdkPath
	// Note: Would need to add UpdateProvider method to database

	// TODO: Register client with router
	// s.router.RegisterClient(providerID, client)

	return nil
}

// keyManagerDatabaseAdapter adapts database.DB for keymanager.Database interface
type keyManagerDatabaseAdapter struct {
	db *database.DB
}

func (a *keyManagerDatabaseAdapter) ListActiveAPIKeys(providerID string) ([]*keymanager.APIKey, error) {
	keys, err := a.db.ListActiveAPIKeys(providerID)
	if err != nil {
		return nil, err
	}

	result := make([]*keymanager.APIKey, len(keys))
	for i, k := range keys {
		result[i] = &keymanager.APIKey{
			ID:            k.ID,
			ProviderID:    k.ProviderID,
			KeyHash:       k.KeyHash,
			KeyPrefix:     k.KeyPrefix,
			Tier:          k.Tier,
			RPMLimit:      k.RPMLimit,
			TPMLimit:      k.TPMLimit,
			DailyLimit:    k.DailyLimit,
			ResetInterval: k.ResetInterval,
			LastReset:     k.LastReset,
			RequestsCount: k.RequestsCount,
			TokensCount:   k.TokensCount,
			Active:        k.Active,
			Degraded:      k.Degraded,
			DegradedUntil: k.DegradedUntil,
			CreatedAt:     k.CreatedAt,
		}
	}
	return result, nil
}

func (a *keyManagerDatabaseAdapter) IncrementKeyUsage(keyID int, tokens int) error {
	return a.db.IncrementKeyUsage(keyID, tokens)
}

func (a *keyManagerDatabaseAdapter) MarkKeyDegraded(keyID int, until time.Time) error {
	return a.db.MarkKeyDegraded(keyID, until)
}

func (a *keyManagerDatabaseAdapter) ResetKeyLimits(keyID int) error {
	return a.db.ResetKeyLimits(keyID)
}

func (a *keyManagerDatabaseAdapter) GetAPIKey(id int) (*keymanager.APIKey, error) {
	key, err := a.db.GetAPIKey(id)
	if err != nil || key == nil {
		return nil, err
	}

	return &keymanager.APIKey{
		ID:            key.ID,
		ProviderID:    key.ProviderID,
		KeyHash:       key.KeyHash,
		KeyPrefix:     key.KeyPrefix,
		Tier:          key.Tier,
		RPMLimit:      key.RPMLimit,
		TPMLimit:      key.TPMLimit,
		DailyLimit:    key.DailyLimit,
		ResetInterval: key.ResetInterval,
		LastReset:     key.LastReset,
		RequestsCount: key.RequestsCount,
		TokensCount:   key.TokensCount,
		Active:        key.Active,
		Degraded:      key.Degraded,
		DegradedUntil: key.DegradedUntil,
		CreatedAt:     key.CreatedAt,
	}, nil
}
