package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/jeffersonwarrior/modelscan/sdk/storage"
	_ "github.com/mattn/go-sqlite3"
)

// Orchestrator manages agents and tasks from the CLI
type Orchestrator struct {
	storage      *storage.Storage
	config       *Config
	agents       map[string]*Agent
	teams        map[string]*Team
	tasks        map[string]*Task
	handlers     map[string]CommandHandler
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	shutdownChan chan os.Signal
	running      bool
}

// Config holds CLI configuration
type Config struct {
	DatabasePath   string        `json:"database_path"`
	LogLevel       string        `json:"log_level"`
	DataRetention  time.Duration `json:"data_retention"`
	MaxConcurrency int           `json:"max_concurrency"`
	StartupAction  string        `json:"startup_action"`
}

// Agent represents a managed agent in the CLI
type Agent struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	Capabilities []string               `json:"capabilities"`
	Config       map[string]interface{} `json:"config"`
	Status       string                 `json:"status"`
	LastSeen     time.Time              `json:"last_seen"`
	ProcessID    int                    `json:"process_id,omitempty"`
}

// Team represents a team of agents
type Team struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Agents      []string               `json:"agents"`
	Config      map[string]interface{} `json:"config"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// Task represents a managed task
type Task struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Status      string                 `json:"status"`
	AgentID     string                 `json:"agent_id"`
	TeamID      string                 `json:"team_id,omitempty"`
	Input       string                 `json:"input"`
	Output      string                 `json:"output"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   time.Time              `json:"started_at,omitempty"`
	CompletedAt time.Time              `json:"completed_at,omitempty"`
}

// CommandHandler handles CLI commands
type CommandHandler interface {
	Handle(ctx context.Context, args []string) error
	Name() string
	Description() string
}

// NewOrchestrator creates a new CLI orchestrator
func NewOrchestrator(config *Config) (*Orchestrator, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Initialize AgentDB which includes schema creation
	agentDB, err := storage.NewAgentDB(config.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize storage
	storage := storage.NewStorage(agentDB.GetDB(), config.DataRetention)

	ctx, cancel := context.WithCancel(context.Background())
	
	orchestrator := &Orchestrator{
		storage:      storage,
		config:       config,
		agents:       make(map[string]*Agent),
		teams:        make(map[string]*Team),
		tasks:        make(map[string]*Task),
		handlers:     make(map[string]CommandHandler),
		ctx:          ctx,
		cancel:       cancel,
		shutdownChan: make(chan os.Signal, 1),
	}

	// Register signal handlers
	signal.Notify(orchestrator.shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize default handlers
	orchestrator.registerDefaultHandlers()

	return orchestrator, nil
}

// DefaultConfig returns default CLI configuration
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	return &Config{
		DatabasePath:   filepath.Join(homeDir, ".modelscan", "modelscan.db"),
		LogLevel:       "info",
		DataRetention:  24 * time.Hour * 7, // 7 days
		MaxConcurrency: 10,
		StartupAction:  "zero-state",
	}
}

// Start starts the orchestrator
func (o *Orchestrator) Start() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.running {
		return fmt.Errorf("orchestrator already running")
	}

	log.Printf("Starting ModelScan CLI Orchestrator")
	log.Printf("Database: %s", o.config.DatabasePath)
	log.Printf("Log Level: %s", o.config.LogLevel)
	log.Printf("Data Retention: %v", o.config.DataRetention)

	// Perform health check
	if err := o.storage.PerformHealthCheck(o.ctx); err != nil {
		return fmt.Errorf("storage health check failed: %w", err)
	}

	// Initialize zero state
	if err := o.zeroStateStartup(); err != nil {
		return fmt.Errorf("zero state initialization failed: %w", err)
	}

	// Load existing agents
	if err := o.loadAgents(); err != nil {
		return fmt.Errorf("failed to load agents: %w", err)
	}

	// Load existing teams
	if err := o.loadTeams(); err != nil {
		return fmt.Errorf("failed to load teams: %w", err)
	}

	// Load existing tasks
	if err := o.loadTasks(); err != nil {
		return fmt.Errorf("failed to load tasks: %w", err)
	}

	// Start cleanup routine
	go o.cleanupRoutine()

	o.running = true
	log.Printf("ModelScan CLI Orchestrator started successfully")

	return nil
}

// Stop stops the orchestrator
func (o *Orchestrator) Stop() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if !o.running {
		return nil
	}

	log.Printf("Stopping ModelScan CLI Orchestrator")

	// Cancel context
	o.cancel()

	// Cancel all pending tasks
	if err := o.storage.CancelAllPendingTasks(o.ctx); err != nil {
		log.Printf("Warning: failed to cancel pending tasks: %v", err)
	}

	// Close storage
	if err := o.storage.Close(); err != nil {
		log.Printf("Warning: failed to close storage: %v", err)
	}

	o.running = false
	log.Printf("ModelScan CLI Orchestrator stopped")

	return nil
}

// zeroStateStartup performs zero-state initialization
func (o *Orchestrator) zeroStateStartup() error {
	log.Printf("Performing zero-state initialization")

	// Initialize database zero state
	if err := o.storage.InitializeZeroState(o.ctx); err != nil {
		return fmt.Errorf("failed to initialize zero state: %w", err)
	}

	// Clear in-memory agents
	o.agents = make(map[string]*Agent)
	o.teams = make(map[string]*Team)
	o.tasks = make(map[string]*Task)

	log.Printf("Zero-state initialization completed")

	return nil
}

// loadAgents loads agents from storage
func (o *Orchestrator) loadAgents() error {
	dbAgents, err := o.storage.Agents.ListActive(o.ctx, 1000, 0)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	for _, dbAgent := range dbAgents {
		agent := &Agent{
			ID:           dbAgent.ID,
			Name:         dbAgent.Name,
			Type:         "", // Will be derived from config
			Capabilities: dbAgent.Capabilities,
			Config:       make(map[string]interface{}),
			Status:       dbAgent.Status,
			LastSeen:     dbAgent.UpdatedAt,
		}

		// Parse config JSON
		if len(dbAgent.Config) > 0 {
			if err := json.Unmarshal([]byte(dbAgent.Config), &agent.Config); err != nil {
				log.Printf("Warning: failed to parse agent config for %s: %v", dbAgent.ID, err)
			} else {
				// Extract agent type from config
				if agentType, ok := agent.Config["type"].(string); ok {
					agent.Type = agentType
				}
			}
		}

		o.agents[agent.ID] = agent
	}

	log.Printf("Loaded %d agents from storage", len(o.agents))
	return nil
}

// loadTeams loads teams from storage
func (o *Orchestrator) loadTeams() error {
	dbTeams, err := o.storage.Teams.List(o.ctx, 1000, 0)
	if err != nil {
		return fmt.Errorf("failed to list teams: %w", err)
	}

	for _, dbTeam := range dbTeams {
		team := &Team{
			ID:          dbTeam.ID,
			Name:        dbTeam.Name,
			Description: dbTeam.Description,
			Agents:      []string{},
			Config:      make(map[string]interface{}),
			Metadata:    dbTeam.Metadata,
		}

		// Parse config JSON
		if len(dbTeam.Config) > 0 {
			if err := json.Unmarshal([]byte(dbTeam.Config), &team.Config); err != nil {
				log.Printf("Warning: failed to parse team config for %s: %v", dbTeam.ID, err)
			}
		}

		// Load team members
		members, err := o.storage.Teams.GetMembers(o.ctx, dbTeam.ID)
		if err != nil {
			log.Printf("Warning: failed to load team members for %s: %v", dbTeam.ID, err)
		} else {
			for _, member := range members {
				team.Agents = append(team.Agents, member.AgentID)
			}
		}

		o.teams[team.ID] = team
	}

	log.Printf("Loaded %d teams from storage", len(o.teams))
	return nil
}

// loadTasks loads tasks from storage
func (o *Orchestrator) loadTasks() error {
	// Load only active tasks
	dbTasks, err := o.storage.Tasks.ListByStatus(o.ctx, "pending", 1000, 0)
	if err != nil {
		return fmt.Errorf("failed to list pending tasks: %w", err)
	}

	runningTasks, err := o.storage.Tasks.ListByStatus(o.ctx, "running", 1000, 0)
	if err != nil {
		return fmt.Errorf("failed to list running tasks: %w", err)
	}

	allTasks := append(dbTasks, runningTasks...)

	for _, dbTask := range allTasks {
		task := &Task{
			ID:        dbTask.ID,
			Type:      dbTask.Type,
			Status:    dbTask.Status,
			AgentID:   dbTask.AgentID,
			Input:     dbTask.Input,
			Output:    dbTask.Output,
			Metadata:  dbTask.Metadata,
			CreatedAt: dbTask.CreatedAt,
		}

		if dbTask.TeamID != nil {
			task.TeamID = *dbTask.TeamID
		}

		if dbTask.StartedAt != nil {
			task.StartedAt = *dbTask.StartedAt
		}

		if dbTask.CompletedAt != nil {
			task.CompletedAt = *dbTask.CompletedAt
		}

		o.tasks[task.ID] = task
	}

	log.Printf("Loaded %d active tasks from storage", len(o.tasks))
	return nil
}

// cleanupRoutine performs periodic cleanup
func (o *Orchestrator) cleanupRoutine() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	select {
	case <-o.ctx.Done():
		return
	case <-ticker.C:
		if err := o.storage.CleanupOldData(o.ctx); err != nil {
			log.Printf("Warning: cleanup failed: %v", err)
		}
	}
}

// WaitForShutdown waits for shutdown signal
func (o *Orchestrator) WaitForShutdown() {
	<-o.shutdownChan
	log.Printf("Received shutdown signal")
	o.Stop()
}

// IsRunning returns true if the orchestrator is running
func (o *Orchestrator) IsRunning() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.running
}

// GetStorage returns the storage instance
func (o *Orchestrator) GetStorage() *storage.Storage {
	return o.storage
}

// GetConfig returns the configuration
func (o *Orchestrator) GetConfig() *Config {
	return o.config
}

// registerDefaultHandlers registers default command handlers
func (o *Orchestrator) registerDefaultHandlers() {
	// Built-in handlers will be registered by the CLI commands
}