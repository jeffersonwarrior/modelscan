package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/jeffersonwarrior/modelscan/internal/service"
)

const (
	// DefaultLogFileName is the log file name for daemon mode
	DefaultLogFileName = "modelscan.log"
)

// DaemonConfig holds daemon mode configuration
type DaemonConfig struct {
	LogPath string // Path to log file (default: ~/.modelscan/modelscan.log)
	PIDPath string // Path to PID file (managed by service.portfile)
}

// GetDefaultLogPath returns the default log file path
func GetDefaultLogPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".modelscan", DefaultLogFileName), nil
}

// RunDaemon starts the server in daemon mode
// This function daemonizes the process and returns control to the caller
func RunDaemon(configPath string) error {
	// Check if already running
	running, pidFile, err := service.IsServerRunning()
	if err != nil {
		return fmt.Errorf("failed to check for existing server: %w", err)
	}
	if running && pidFile != nil {
		return fmt.Errorf("server already running (PID: %d, port: %d)", pidFile.PID, pidFile.Port)
	}

	// Fork and exec to create daemon
	return daemonize(configPath)
}

// daemonize forks the current process as a background daemon
func daemonize(configPath string) error {
	// Get log path
	logPath, err := GetDefaultLogPath()
	if err != nil {
		return fmt.Errorf("failed to get log path: %w", err)
	}

	// Ensure log directory exists
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open/create log file
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Build command to re-exec ourselves without --daemon flag
	cmd := exec.Command(os.Args[0], "--config", configPath, "--daemon-child")

	// Detach from terminal
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session, detach from controlling terminal
	}

	// Redirect stdout/stderr to log file
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil

	// Start the daemon process
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	fmt.Printf("modelscan daemon started (PID: %d)\n", cmd.Process.Pid)
	fmt.Printf("Log file: %s\n", logPath)

	// Parent exits, daemon continues
	// Child process inherited the open file descriptor, so parent should close its copy
	logFile.Close()
	return nil
}

// SetupDaemonLogging configures logging for daemon mode
// Returns a cleanup function to close the log file
func SetupDaemonLogging() (func(), error) {
	logPath, err := GetDefaultLogPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get log path: %w", err)
	}

	// Ensure log directory exists
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open/create log file
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Configure log to write to both file and console (in case of --daemon-child)
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cleanup := func() {
		logFile.Close()
	}

	return cleanup, nil
}

// SetupSignalHandlers sets up signal handlers for the daemon
// Returns a channel that receives signals
func SetupSignalHandlers() chan os.Signal {
	sigChan := make(chan os.Signal, 2)

	// SIGTERM, SIGINT for graceful shutdown
	// SIGHUP for config reload
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	return sigChan
}

// HandleDaemonSignals processes signals for the daemon
// Returns true if shutdown is requested, false if config reload
func HandleDaemonSignals(sig os.Signal, reloadFunc func() error) bool {
	switch sig {
	case syscall.SIGHUP:
		log.Println("Received SIGHUP, reloading configuration...")
		if reloadFunc != nil {
			if err := reloadFunc(); err != nil {
				log.Printf("Config reload failed: %v", err)
			} else {
				log.Println("Configuration reloaded successfully")
			}
		}
		return false // Continue running
	case syscall.SIGTERM, syscall.SIGINT:
		log.Printf("Received %v, initiating shutdown...", sig)
		return true // Shutdown requested
	default:
		log.Printf("Received unexpected signal: %v", sig)
		return false
	}
}

// StopDaemon sends a stop signal to a running daemon
func StopDaemon() error {
	running, pidFile, err := service.IsServerRunning()
	if err != nil {
		return fmt.Errorf("failed to check server status: %w", err)
	}

	if !running || pidFile == nil {
		return fmt.Errorf("no running server found")
	}

	process, err := os.FindProcess(pidFile.PID)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pidFile.PID, err)
	}

	// Send SIGTERM for graceful shutdown
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to process %d: %w", pidFile.PID, err)
	}

	fmt.Printf("Stop signal sent to modelscan daemon (PID: %d)\n", pidFile.PID)
	return nil
}

// ReloadDaemon sends a SIGHUP to reload configuration
func ReloadDaemon() error {
	running, pidFile, err := service.IsServerRunning()
	if err != nil {
		return fmt.Errorf("failed to check server status: %w", err)
	}

	if !running || pidFile == nil {
		return fmt.Errorf("no running server found")
	}

	process, err := os.FindProcess(pidFile.PID)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pidFile.PID, err)
	}

	// Send SIGHUP for config reload
	if err := process.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("failed to send SIGHUP to process %d: %w", pidFile.PID, err)
	}

	fmt.Printf("Reload signal sent to modelscan daemon (PID: %d)\n", pidFile.PID)
	return nil
}

// DaemonStatus returns the status of any running daemon
func DaemonStatus() (*service.PIDFile, error) {
	running, pidFile, err := service.IsServerRunning()
	if err != nil {
		return nil, fmt.Errorf("failed to check server status: %w", err)
	}

	if !running || pidFile == nil {
		return nil, nil
	}

	return pidFile, nil
}
