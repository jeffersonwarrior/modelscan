package service

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jeffersonwarrior/modelscan/config"
	"github.com/jeffersonwarrior/modelscan/internal/admin"
	"github.com/jeffersonwarrior/modelscan/internal/database"
	"github.com/jeffersonwarrior/modelscan/internal/discovery"
	"github.com/jeffersonwarrior/modelscan/internal/generator"
	"github.com/jeffersonwarrior/modelscan/internal/keymanager"
	"github.com/jeffersonwarrior/modelscan/providers"
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
	restarting  atomic.Bool
	initialized bool

	// Model cache with TTL
	modelCache      []ModelWithProvider
	modelCacheTime  time.Time
	modelCacheTTL   time.Duration
	modelCacheMu    sync.RWMutex
}

// ModelWithProvider extends providers.Model with the source provider
type ModelWithProvider struct {
	providers.Model
	Provider string `json:"provider"`
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
		config:        cfg,
		modelCacheTTL: 5 * time.Minute, // Default cache TTL
	}
}

// Initialize initializes all components
func (s *Service) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return fmt.Errorf("service already initialized")
	}

	// Validate configuration
	if s.config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	if s.config.DatabasePath == "" {
		return fmt.Errorf("database path is required")
	}
	if s.config.ServerHost == "" {
		s.config.ServerHost = "127.0.0.1" // Default
	}
	if s.config.ServerPort == 0 {
		s.config.ServerPort = 8080 // Default
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
		admin.NewKeyManagerAdapter(s.keyManager, s.db),
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

	restarting := s.restarting.Load()
	status := "ok"
	if restarting {
		status = "restarting"
	} else if !s.initialized {
		status = "not_initialized"
	}

	return map[string]interface{}{
		"status":      status,
		"initialized": s.initialized,
		"restarting":  restarting,
		"time":        time.Now(),
	}
}

// Restart performs a graceful restart (for SDK reloading)
func (s *Service) Restart() error {
	s.restarting.Store(true)

	log.Println("Initiating service restart...")

	// Stop current service
	if err := s.Stop(); err != nil {
		return fmt.Errorf("stop failed: %w", err)
	}

	// Brief pause
	time.Sleep(1 * time.Second)

	// Reinitialize
	if err := s.Initialize(); err != nil {
		s.restarting.Store(false)
		return fmt.Errorf("initialization failed: %w", err)
	}

	// Bootstrap
	if err := s.Bootstrap(); err != nil {
		s.restarting.Store(false)
		return fmt.Errorf("bootstrap failed: %w", err)
	}

	// Restart HTTP server
	if err := s.Start(); err != nil {
		s.restarting.Store(false)
		return fmt.Errorf("start failed: %w", err)
	}

	s.restarting.Store(false)

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

	// TODO: Once dynamic loading is implemented:
	// 1. Load the dynamic client from sdkPath
	// 2. Register it with the router:
	//    client := loadDynamicClient(sdkPath)
	//    if directRouter, ok := s.router.(*routing.DirectRouter); ok {
	//        directRouter.RegisterClientWithFullMiddleware(providerID, client, s.keyManager)
	//    }
	// 3. Update database with SDK path (requires UpdateProvider method)

	return nil
}

// RegisterClient registers a client with the router using full middleware stack
func (s *Service) RegisterClient(providerID string, client routing.Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return fmt.Errorf("service not initialized")
	}

	// Check if router is DirectRouter (supports client registration)
	directRouter, ok := s.router.(*routing.DirectRouter)
	if !ok {
		return fmt.Errorf("current router mode does not support client registration")
	}

	// Register with full middleware (key management + tooling)
	if err := directRouter.RegisterClientWithFullMiddleware(providerID, client, s.keyManager); err != nil {
		return fmt.Errorf("failed to register client: %w", err)
	}

	log.Printf("✓ Registered client for %s with full middleware stack", providerID)
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

// GetKey returns the actual API key string for a provider.
// Uses the keymanager's round-robin selection to pick the best key.
func (s *Service) GetKey(ctx context.Context, providerID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized {
		return "", fmt.Errorf("service not initialized")
	}

	return s.keyManager.GetActualKey(ctx, providerID)
}

// GetProxyURL returns the full proxy URL string (http://host:port)
func (s *Service) GetProxyURL() string {
	return fmt.Sprintf("http://%s:%d", s.config.ServerHost, s.config.ServerPort)
}

// ListAllModels aggregates models from all providers with caching
func (s *Service) ListAllModels(ctx context.Context) ([]ModelWithProvider, error) {
	// Check cache first
	s.modelCacheMu.RLock()
	if len(s.modelCache) > 0 && time.Since(s.modelCacheTime) < s.modelCacheTTL {
		cached := make([]ModelWithProvider, len(s.modelCache))
		copy(cached, s.modelCache)
		s.modelCacheMu.RUnlock()
		return cached, nil
	}
	s.modelCacheMu.RUnlock()

	// Load API keys from config
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Get all registered provider names
	registeredProviders := providers.ListProviders()

	// Use channel to collect results from goroutines (avoids mutex contention)
	type result struct {
		models []ModelWithProvider
		err    error
	}
	resultsChan := make(chan result, len(registeredProviders))

	var wg sync.WaitGroup

	for _, providerName := range registeredProviders {
		// Check if we have an API key for this provider
		apiKey, err := cfg.GetAPIKey(providerName)
		if err != nil || apiKey == "" {
			// Skip providers without keys
			continue
		}

		// Get the factory for this provider
		factory, exists := providers.GetProviderFactory(providerName)
		if !exists {
			continue
		}

		wg.Add(1)
		go func(name string, key string, factory providers.ProviderFactory) {
			defer wg.Done()

			// Create provider instance
			provider := factory(key)

			// List models with timeout
			modelCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			models, err := provider.ListModels(modelCtx, false)
			if err != nil {
				resultsChan <- result{err: fmt.Errorf("%s: %w", name, err)}
				return
			}

			// Add provider field to each model
			providerModels := make([]ModelWithProvider, len(models))
			for i, m := range models {
				providerModels[i] = ModelWithProvider{
					Model:    m,
					Provider: name,
				}
			}
			resultsChan <- result{models: providerModels}
		}(providerName, apiKey, factory)
	}

	// Close channel when all goroutines finish
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results from channel
	var allModels []ModelWithProvider
	var errs []error
	for res := range resultsChan {
		if res.err != nil {
			errs = append(errs, res.err)
		} else {
			allModels = append(allModels, res.models...)
		}
	}

	// Update cache
	s.modelCacheMu.Lock()
	s.modelCache = make([]ModelWithProvider, len(allModels))
	copy(s.modelCache, allModels)
	s.modelCacheTime = time.Now()
	s.modelCacheMu.Unlock()

	// Return models even if some providers failed
	if len(allModels) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("all provider requests failed: %v", errs)
	}

	return allModels, nil
}

// InvalidateModelCache clears the model cache
func (s *Service) InvalidateModelCache() {
	s.modelCacheMu.Lock()
	s.modelCache = nil
	s.modelCacheTime = time.Time{}
	s.modelCacheMu.Unlock()
}

// SetModelCacheTTL sets the cache TTL for ListAllModels
func (s *Service) SetModelCacheTTL(ttl time.Duration) {
	s.modelCacheMu.Lock()
	s.modelCacheTTL = ttl
	s.modelCacheMu.Unlock()
}
