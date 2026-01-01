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

const version = "0.5.5"

func main() {
	// Command-line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	daemonMode := flag.Bool("daemon", false, "Run as a background daemon")
	daemonChild := flag.Bool("daemon-child", false, "Internal flag for daemon child process")
	stopDaemon := flag.Bool("stop", false, "Stop a running daemon")
	reloadDaemon := flag.Bool("reload", false, "Reload daemon configuration (SIGHUP)")
	statusDaemon := flag.Bool("status", false, "Show daemon status")
	flag.Parse()

	if *showVersion {
		fmt.Printf("modelscan-server version %s\n", version)
		fmt.Println("Auto-discovering SDK service with intelligent provider onboarding")
		os.Exit(0)
	}

	// Handle daemon control commands
	if *stopDaemon {
		if err := StopDaemon(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *reloadDaemon {
		if err := ReloadDaemon(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *statusDaemon {
		pidFile, err := DaemonStatus()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if pidFile == nil {
			fmt.Println("modelscan daemon is not running")
		} else {
			fmt.Printf("modelscan daemon is running\n")
			fmt.Printf("  PID:     %d\n", pidFile.PID)
			fmt.Printf("  Port:    %d\n", pidFile.Port)
			fmt.Printf("  Host:    %s\n", pidFile.Host)
			fmt.Printf("  Started: %s\n", pidFile.StartedAt)
			fmt.Printf("  Version: %s\n", pidFile.Version)
		}
		os.Exit(0)
	}

	// Daemon mode: fork and exit parent
	if *daemonMode {
		if err := RunDaemon(*configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting daemon: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Daemon child: setup logging to file
	if *daemonChild {
		cleanup, err := SetupDaemonLogging()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error setting up daemon logging: %v\n", err)
			os.Exit(1)
		}
		defer cleanup()
	}

	log.Printf("Starting modelscan v%s", version)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Printf("Warning: failed to load config from %s: %v", *configPath, err)
		log.Println("Using default configuration (database: ./modelscan.db, server: localhost:8080)")
		log.Println("To create a config file, see: config.example.yaml")
		cfg = config.DefaultConfig()
	} else {
		log.Printf("Configuration loaded from: %s", *configPath)
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

	// Write PID file for daemon discovery
	if err := service.WritePIDFile(cfg.Server.Port, version); err != nil {
		log.Printf("Warning: failed to write PID file: %v", err)
	} else {
		pidPath, _ := service.GetPIDFilePath()
		log.Printf("PID file written: %s", pidPath)
	}
	// Ensure PID file is removed on exit
	defer func() {
		if err := service.RemovePIDFile(); err != nil {
			log.Printf("Warning: failed to remove PID file: %v", err)
		}
	}()

	if *daemonChild {
		log.Println("Running as daemon (use --stop to shutdown)")
	} else {
		log.Println("Press Ctrl+C to shutdown (press twice to force)")
	}

	// Wait for signals with SIGHUP support
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	// Config reload function for SIGHUP
	reloadConfig := func() error {
		newCfg, err := config.Load(*configPath)
		if err != nil {
			return fmt.Errorf("failed to reload config: %w", err)
		}
		log.Printf("Configuration reloaded from: %s", *configPath)
		log.Printf("Note: Some settings require restart to take effect")
		// Update logging of new config values
		log.Printf("  Database: %s", newCfg.Database.Path)
		log.Printf("  Server: %s:%d", newCfg.Server.Host, newCfg.Server.Port)
		log.Printf("  Agent Model: %s", newCfg.Discovery.AgentModel)
		return nil
	}

	// Signal handling loop
	for {
		sig := <-sigChan

		if sig == syscall.SIGHUP {
			log.Println("Received SIGHUP, reloading configuration...")
			if err := reloadConfig(); err != nil {
				log.Printf("Config reload failed: %v", err)
			}
			continue // Keep running after config reload
		}

		// SIGTERM or SIGINT - shutdown
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
				log.Printf("Service shutdown error: %v", err)
				os.Exit(1)
			}
			log.Println("✓ Shutdown complete")
			return
		case <-sigChan:
			log.Println("Force shutdown requested")
			os.Exit(1)
		}
	}
}
