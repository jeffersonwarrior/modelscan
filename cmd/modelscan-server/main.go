package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jeffersonwarrior/modelscan/internal/config"
	"github.com/jeffersonwarrior/modelscan/internal/service"
)

const version = "0.3.0"

func main() {
	// Command-line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
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

	// Create service
	svc := service.NewService(&service.Config{
		DatabasePath:  cfg.Database.Path,
		ServerHost:    cfg.Server.Host,
		ServerPort:    cfg.Server.Port,
		AgentModel:    cfg.Discovery.AgentModel,
		ParallelBatch: cfg.Discovery.ParallelBatch,
		CacheDays:     cfg.Discovery.CacheDays,
		OutputDir:     "generated",
		RoutingMode:   "direct", // Default to direct routing
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

	log.Println("Press Ctrl+C to shutdown")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigChan

	log.Printf("Received signal: %v", sig)
	log.Println("Initiating graceful shutdown...")

	// Stop service
	if err := svc.Stop(); err != nil {
		log.Printf("Service shutdown error: %v", err)
		os.Exit(1)
	}

	log.Println("✓ Shutdown complete")
}
