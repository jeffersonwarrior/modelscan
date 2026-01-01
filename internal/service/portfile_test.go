package service

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestFindOpenPort(t *testing.T) {
	port, err := FindOpenPort()
	if err != nil {
		t.Fatalf("FindOpenPort failed: %v", err)
	}

	if port < PortRangeStart || port > PortRangeEnd {
		t.Errorf("Port %d is outside expected range %d-%d", port, PortRangeStart, PortRangeEnd)
	}
}

func TestFindOpenPort_SkipsUsedPorts(t *testing.T) {
	// Occupy the first port in range
	ln, err := net.Listen("tcp", "127.0.0.1:10000")
	if err != nil {
		t.Skipf("Cannot occupy port 10000: %v", err)
	}
	defer ln.Close()

	port, err := FindOpenPort()
	if err != nil {
		t.Fatalf("FindOpenPort failed: %v", err)
	}

	if port == 10000 {
		t.Error("FindOpenPort returned occupied port 10000")
	}
}

func TestPortRangeConstants(t *testing.T) {
	if PortRangeStart != 10000 {
		t.Errorf("PortRangeStart = %d; want 10000", PortRangeStart)
	}
	if PortRangeEnd != 10500 {
		t.Errorf("PortRangeEnd = %d; want 10500", PortRangeEnd)
	}
	if PortRangeEnd <= PortRangeStart {
		t.Error("PortRangeEnd must be greater than PortRangeStart")
	}
}

func TestGetPIDFilePath(t *testing.T) {
	path, err := GetPIDFilePath()
	if err != nil {
		t.Fatalf("GetPIDFilePath failed: %v", err)
	}

	dir := filepath.Dir(path)
	base := filepath.Base(path)

	if base != DefaultPIDFileName {
		t.Errorf("PID file name = %q; want %q", base, DefaultPIDFileName)
	}

	if filepath.Base(dir) != ".modelscan" {
		t.Errorf("Parent directory = %q; want .modelscan", filepath.Base(dir))
	}
}

func TestIsProcessRunning_CurrentProcess(t *testing.T) {
	// Current process should be running
	running, err := isProcessRunning(os.Getpid())
	if err != nil {
		t.Fatalf("isProcessRunning error: %v", err)
	}
	if !running {
		t.Error("isProcessRunning returned false for current process")
	}
}

func TestIsProcessRunning_NonExistent(t *testing.T) {
	// Non-existent PID should not be running
	running, err := isProcessRunning(99999999)
	if err != nil {
		t.Fatalf("isProcessRunning error: %v", err)
	}
	if running {
		t.Error("isProcessRunning returned true for non-existent PID")
	}
}

func TestIsProcessRunning_InvalidPID(t *testing.T) {
	running, err := isProcessRunning(-1)
	if err != nil {
		t.Fatalf("isProcessRunning error: %v", err)
	}
	if running {
		t.Error("isProcessRunning returned true for invalid PID -1")
	}

	running, err = isProcessRunning(0)
	if err != nil {
		t.Fatalf("isProcessRunning error: %v", err)
	}
	if running {
		t.Error("isProcessRunning returned true for PID 0")
	}
}

func TestIsPortInUse(t *testing.T) {
	// Start a listener on a port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer ln.Close()

	// Get the port we're listening on
	addr := ln.Addr().(*net.TCPAddr)

	if !isPortInUse("127.0.0.1", addr.Port) {
		t.Error("isPortInUse returned false for port with active listener")
	}
}

func TestIsPortInUse_NotInUse(t *testing.T) {
	// Find an available port
	port, err := FindOpenPort()
	if err != nil {
		t.Fatalf("FindOpenPort failed: %v", err)
	}

	if isPortInUse("127.0.0.1", port) {
		t.Errorf("isPortInUse returned true for unused port %d", port)
	}
}

func TestPIDFileWriteReadRemove(t *testing.T) {
	// Use temp directory for testing
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Ensure clean state
	_ = RemovePIDFile()

	// Test ReadPIDFile returns nil when no file exists
	pf, err := ReadPIDFile()
	if err != nil {
		t.Fatalf("ReadPIDFile failed on non-existent file: %v", err)
	}
	if pf != nil {
		t.Error("Expected nil PIDFile for non-existent file")
	}

	// Test WritePIDFile
	testPort := 10123
	testVersion := "0.5.5-test"
	if err := WritePIDFile(testPort, testVersion); err != nil {
		t.Fatalf("WritePIDFile failed: %v", err)
	}

	// Verify directory was created
	dirPath := filepath.Join(tmpDir, ".modelscan")
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("modelscan directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected .modelscan to be a directory")
	}

	// Test ReadPIDFile
	pf, err = ReadPIDFile()
	if err != nil {
		t.Fatalf("ReadPIDFile failed: %v", err)
	}
	if pf == nil {
		t.Fatal("Expected non-nil PIDFile")
	}

	if pf.Port != testPort {
		t.Errorf("Port = %d; want %d", pf.Port, testPort)
	}
	if pf.PID != os.Getpid() {
		t.Errorf("PID = %d; want %d", pf.PID, os.Getpid())
	}
	if pf.Host != "127.0.0.1" {
		t.Errorf("Host = %s; want 127.0.0.1", pf.Host)
	}
	if pf.Version != testVersion {
		t.Errorf("Version = %s; want %s", pf.Version, testVersion)
	}
	if pf.StartedAt == "" {
		t.Error("StartedAt should not be empty")
	}

	// Test RemovePIDFile
	if err := RemovePIDFile(); err != nil {
		t.Fatalf("RemovePIDFile failed: %v", err)
	}

	// Verify file was removed
	pf, err = ReadPIDFile()
	if err != nil {
		t.Fatalf("ReadPIDFile after remove failed: %v", err)
	}
	if pf != nil {
		t.Error("Expected nil PIDFile after removal")
	}

	// Test RemovePIDFile on non-existent file (should not error)
	if err := RemovePIDFile(); err != nil {
		t.Errorf("RemovePIDFile on non-existent file failed: %v", err)
	}
}

func TestIsServerRunning_NoPIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	running, pf, err := IsServerRunning()
	if err != nil {
		t.Fatalf("IsServerRunning failed: %v", err)
	}
	if running {
		t.Error("Expected server not running with no PID file")
	}
	if pf != nil {
		t.Error("Expected nil PIDFile with no PID file")
	}
}

func TestIsServerRunning_StaleFile(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create PID file with current process but no server on that port
	if err := WritePIDFile(19999, "test"); err != nil {
		t.Fatalf("WritePIDFile failed: %v", err)
	}

	// Process exists but port not responding - should clean up stale file
	running, pf, err := IsServerRunning()
	if err != nil {
		t.Fatalf("IsServerRunning failed: %v", err)
	}
	if running {
		t.Error("Expected server not running (port not responding)")
	}
	if pf != nil {
		t.Error("Expected nil PIDFile when stale")
	}

	// File should have been removed
	existingPF, _ := ReadPIDFile()
	if existingPF != nil {
		t.Error("Expected stale PID file to be removed")
	}
}

func TestIsServerRunning_ActiveServer(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Start a temporary server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	// Write PID file pointing to our test server
	if err := WritePIDFile(port, "test"); err != nil {
		t.Fatalf("WritePIDFile failed: %v", err)
	}

	// Now IsServerRunning should return true
	running, pf, err := IsServerRunning()
	if err != nil {
		t.Fatalf("IsServerRunning failed: %v", err)
	}
	if !running {
		t.Error("Expected server to be running")
	}
	if pf == nil {
		t.Fatal("Expected non-nil PIDFile")
	}
	if pf.Port != port {
		t.Errorf("Port = %d; want %d", pf.Port, port)
	}
}
