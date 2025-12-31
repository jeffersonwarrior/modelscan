package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/jeffersonwarrior/modelscan/internal/config"
	"github.com/jeffersonwarrior/modelscan/internal/database"
	"github.com/jeffersonwarrior/modelscan/internal/service"
)

const version = "0.3.0"

func main() {
	// Command-line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	initDB := flag.Bool("init", false, "Initialize database and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("modelscan version %s\n", version)
		fmt.Println("Auto-discovering SDK service with intelligent provider onboarding")
		os.Exit(0)
	}

	log.Printf("Starting modelscan v%s", version)
	log.Printf("Configuration: %s", *configPath)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Database: %s", cfg.Database.Path)
	log.Printf("Server: %s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Agent Model: %s", cfg.Discovery.AgentModel)

	// Initialize database if requested
	if *initDB {
		log.Println("Initializing database...")
		if err := initializeDatabase(cfg.Database.Path); err != nil {
			log.Fatalf("Database initialization failed: %v", err)
		}
		log.Println("Database initialized successfully")
		os.Exit(0)
	}

	// Check if database exists
	if _, err := os.Stat(cfg.Database.Path); os.IsNotExist(err) {
		log.Printf("Database not found at %s", cfg.Database.Path)
		log.Println("Run with --init flag to initialize database")
		log.Println("Example: modelscan --init")
		os.Exit(1)
	}

	// Create service
	svc := service.NewService(&service.Config{
		DatabasePath:  cfg.Database.Path,
		ServerHost:    cfg.Server.Host,
		ServerPort:    cfg.Server.Port,
		AgentModel:    cfg.Discovery.AgentModel,
		ParallelBatch: cfg.Discovery.ParallelBatch,
		CacheDays:     cfg.Discovery.CacheDays,
		OutputDir:     cfg.Discovery.OutputDir,
		RoutingMode:   cfg.Discovery.RoutingMode,
	})

	// Initialize service
	if err := svc.Initialize(); err != nil {
		log.Fatalf("Service initialization failed: %v", err)
	}

	// Bootstrap from existing data
	if err := svc.Bootstrap(); err != nil {
		log.Printf("Warning: bootstrap failed: %v", err)
	}

	// Start HTTP server
	if err := svc.Start(); err != nil {
		log.Fatalf("Service start failed: %v", err)
	}

	log.Println("✓ Service started successfully")
	log.Println("Press Ctrl+C to shutdown (press twice to force)")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigChan

	log.Printf("Received signal: %v", sig)
	log.Println("Initiating graceful shutdown...")

	// Graceful shutdown with timeout
	done := make(chan error, 1)
	go func() {
		done <- svc.Stop()
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Printf("Shutdown error: %v", err)
			os.Exit(1)
		}
		log.Println("✓ Shutdown complete")
	case <-sigChan:
		log.Println("Force shutdown requested")
		os.Exit(1)
	}
}

// initializeDatabase creates a new database with schema
func initializeDatabase(dbPath string) error {
	// Create directory if needed
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Initialize database with schema (Open will run migrations)
	db, err := database.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	log.Printf("Database created at: %s", dbPath)

	// Test the database
	_, err = db.ListProviders()
	if err != nil {
		return fmt.Errorf("failed to verify database: %w", err)
	}

	return nil
}
