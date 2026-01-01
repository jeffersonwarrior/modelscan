package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

const (
	// PortRangeStart is the beginning of the dynamic port range
	PortRangeStart = 10000
	// PortRangeEnd is the end of the dynamic port range
	PortRangeEnd = 10500
	// DefaultPIDFileName is the default name for the PID file
	DefaultPIDFileName = "modelscan.pid"
)

// PIDFile represents the contents of the PID file
type PIDFile struct {
	PID       int    `json:"pid"`
	Port      int    `json:"port"`
	Host      string `json:"host"`
	StartedAt string `json:"started_at"`
	Version   string `json:"version"`
}

// GetPIDFilePath returns the path to the PID file
// Default: ~/.modelscan/modelscan.pid
func GetPIDFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".modelscan", DefaultPIDFileName), nil
}

// FindOpenPort finds an available port in the configured range
func FindOpenPort() (int, error) {
	for port := PortRangeStart; port <= PortRangeEnd; port++ {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			// Port is in use, try next
			continue
		}
		listener.Close()
		return port, nil
	}
	return 0, fmt.Errorf("no available port found in range %d-%d", PortRangeStart, PortRangeEnd)
}

// WritePIDFile writes the PID file with the given port
func WritePIDFile(port int, version string) error {
	pidPath, err := GetPIDFilePath()
	if err != nil {
		return err
	}

	// Ensure directory exists with restricted permissions (0700)
	dir := filepath.Dir(pidPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	pidFile := PIDFile{
		PID:       os.Getpid(),
		Port:      port,
		Host:      "127.0.0.1",
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Version:   version,
	}

	data, err := json.MarshalIndent(pidFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal PID file: %w", err)
	}

	if err := os.WriteFile(pidPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	return nil
}

// ReadPIDFile reads and parses the PID file
func ReadPIDFile() (*PIDFile, error) {
	pidPath, err := GetPIDFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(pidPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No PID file exists
		}
		return nil, fmt.Errorf("failed to read PID file: %w", err)
	}

	var pidFile PIDFile
	if err := json.Unmarshal(data, &pidFile); err != nil {
		return nil, fmt.Errorf("failed to parse PID file: %w", err)
	}

	return &pidFile, nil
}

// RemovePIDFile removes the PID file
func RemovePIDFile() error {
	pidPath, err := GetPIDFilePath()
	if err != nil {
		return err
	}

	if err := os.Remove(pidPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove PID file: %w", err)
	}

	return nil
}

// IsServerRunning checks if the server from the PID file is still running
// Returns:
//   - bool: true if a server process is running
//   - *PIDFile: the PID file contents if it exists and process is running
//   - error: any error encountered (nil if PID file doesn't exist or is stale)
func IsServerRunning() (bool, *PIDFile, error) {
	pidFile, err := ReadPIDFile()
	if err != nil {
		return false, nil, err
	}

	// No PID file exists
	if pidFile == nil {
		return false, nil, nil
	}

	// Check if process exists by sending signal 0
	running, err := isProcessRunning(pidFile.PID)
	if err != nil {
		return false, nil, fmt.Errorf("failed to check process status: %w", err)
	}

	if !running {
		// Process is not running, PID file is stale
		// Remove the stale PID file gracefully
		if err := RemovePIDFile(); err != nil {
			log.Printf("Warning: failed to remove stale PID file: %v", err)
		}
		return false, nil, nil
	}

	// Verify the port is actually being listened on by this process
	// This handles cases where a different process reused the PID
	if !isPortInUse(pidFile.Host, pidFile.Port) {
		// Port is not in use, likely a different process with same PID
		if err := RemovePIDFile(); err != nil {
			log.Printf("Warning: failed to remove invalid PID file: %v", err)
		}
		return false, nil, nil
	}

	return true, pidFile, nil
}

// isProcessRunning checks if a process with the given PID is running
// Uses signal 0 which doesn't send a signal but checks if the process exists
func isProcessRunning(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		// On Unix, FindProcess always succeeds, so this shouldn't happen
		return false, nil
	}

	// Send signal 0 to check if process exists
	// This doesn't actually send a signal, just checks permissions/existence
	err = process.Signal(syscall.Signal(0))
	if err == nil {
		// Process exists and we have permission to signal it
		return true, nil
	}

	// Check if the error indicates the process doesn't exist
	if err == os.ErrProcessDone || err == syscall.ESRCH {
		return false, nil
	}

	// EPERM means the process exists but we don't have permission to signal it
	// This still means the process is running
	if err == syscall.EPERM {
		return true, nil
	}

	// Other errors - assume process not running
	return false, nil
}

// isPortInUse checks if a port is currently being listened on
func isPortInUse(host string, port int) bool {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
