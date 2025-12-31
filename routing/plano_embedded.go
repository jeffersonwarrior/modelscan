package routing

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	healthCheckInterval = 30 * time.Second
	maxRestartAttempts  = 3
	restartBackoff      = 2 * time.Second
)

// PlanoEmbeddedRouter manages an embedded Plano Docker container
type PlanoEmbeddedRouter struct {
	config         *EmbeddedConfig
	containerID    string
	proxyRouter    *PlanoProxyRouter
	fallback       Router
	isRunning      bool
	mu             sync.RWMutex
	stopChan       chan struct{}
	restartCount   int
	lastHealthy    time.Time
	isHealthy      bool
	useFallbackNow bool // Set to true when max restarts exceeded
}

// NewPlanoEmbeddedRouter creates a new embedded Plano router
func NewPlanoEmbeddedRouter(config *EmbeddedConfig) (*PlanoEmbeddedRouter, error) {
	if config == nil {
		return nil, fmt.Errorf("embedded config is required")
	}

	if config.ConfigPath == "" {
		return nil, fmt.Errorf("config path is required")
	}

	// Set defaults
	if config.Image == "" {
		config.Image = "katanemo/plano:0.4.0"
	}

	if config.Ports == nil {
		config.Ports = map[string]int{
			"ingress": 10000,
			"egress":  12000,
		}
	}

	return &PlanoEmbeddedRouter{
		config:      config,
		stopChan:    make(chan struct{}),
		isHealthy:   false,
		lastHealthy: time.Now(),
	}, nil
}

// SetFallback sets a fallback router
func (r *PlanoEmbeddedRouter) SetFallback(fallback Router) {
	r.fallback = fallback
}

// Start starts the embedded Plano container
func (r *PlanoEmbeddedRouter) Start() error {
	// Check if Docker is available
	if err := r.checkDocker(); err != nil {
		return fmt.Errorf("docker not available: %w", err)
	}

	// Check if config file exists
	configPath, err := filepath.Abs(r.config.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}

	if _, err = os.Stat(configPath); err != nil {
		return fmt.Errorf("config file not found: %w", err)
	}

	// Build docker run command
	args := []string{
		"run",
		"-d",
		"--name", r.generateContainerName(),
		"-v", fmt.Sprintf("%s:/app/plano_config.yaml:ro", configPath),
	}

	// Add port mappings
	for _, port := range r.config.Ports {
		args = append(args, "-p", fmt.Sprintf("%d:%d", port, port))
	}

	// Add environment variables
	for key, value := range r.config.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Add image
	args = append(args, r.config.Image)

	// Run container
	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start container: %w (output: %s)", err, string(output))
	}

	r.containerID = strings.TrimSpace(string(output))

	// Wait for container to be healthy
	if err = r.waitForHealthy(); err != nil {
		// Cleanup on failure
		_ = r.Stop()
		return fmt.Errorf("container failed health check: %w", err)
	}

	r.isRunning = true

	// Create proxy router to communicate with the container
	ingressPort := r.config.Ports["ingress"]
	if ingressPort == 0 {
		ingressPort = 10000
	}

	proxyConfig := &ProxyConfig{
		BaseURL: fmt.Sprintf("http://localhost:%d", ingressPort),
		Timeout: 30,
	}

	proxyRouter, err := NewPlanoProxyRouter(proxyConfig)
	if err != nil {
		_ = r.Stop()
		return fmt.Errorf("failed to create proxy router: %w", err)
	}

	r.proxyRouter = proxyRouter

	// Mark as healthy and start health monitoring
	r.mu.Lock()
	r.isHealthy = true
	r.lastHealthy = time.Now()
	r.mu.Unlock()

	// Start health monitoring goroutine
	go r.healthMonitor()

	return nil
}

// Route routes through the embedded Plano container
func (r *PlanoEmbeddedRouter) Route(ctx context.Context, req Request) (*Response, error) {
	r.mu.RLock()
	useFallback := r.useFallbackNow || !r.isRunning || !r.isHealthy
	r.mu.RUnlock()

	// Use fallback if container is unhealthy or max restarts exceeded
	if useFallback {
		if r.fallback != nil {
			return r.fallback.Route(ctx, req)
		}
		return nil, fmt.Errorf("embedded plano is unhealthy and no fallback configured")
	}

	// Try routing through proxy
	if r.proxyRouter == nil {
		if r.fallback != nil {
			return r.fallback.Route(ctx, req)
		}
		return nil, fmt.Errorf("embedded plano proxy not initialized")
	}

	return r.proxyRouter.Route(ctx, req)
}

// Stop stops and removes the embedded Plano container
func (r *PlanoEmbeddedRouter) Stop() error {
	if r.containerID == "" {
		return nil
	}

	// Stop container
	stopCmd := exec.Command("docker", "stop", r.containerID)
	if err := stopCmd.Run(); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Remove container
	rmCmd := exec.Command("docker", "rm", r.containerID)
	if err := rmCmd.Run(); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	r.containerID = ""
	r.isRunning = false

	return nil
}

// Close stops the container and closes the proxy router
func (r *PlanoEmbeddedRouter) Close() error {
	// Stop health monitoring
	close(r.stopChan)

	// Close proxy router
	if r.proxyRouter != nil {
		_ = r.proxyRouter.Close()
	}

	// Stop container
	return r.Stop()
}

// checkDocker verifies Docker is available
func (r *PlanoEmbeddedRouter) checkDocker() error {
	cmd := exec.Command("docker", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker command failed: %w", err)
	}
	return nil
}

// generateContainerName generates a unique container name
func (r *PlanoEmbeddedRouter) generateContainerName() string {
	return fmt.Sprintf("modelscan-plano-%d", time.Now().Unix())
}

// waitForHealthy waits for the container to be healthy
func (r *PlanoEmbeddedRouter) waitForHealthy() error {
	maxRetries := 30
	retryDelay := 1 * time.Second

	for i := 0; i < maxRetries; i++ {
		// Check if container is running
		cmd := exec.Command("docker", "inspect", "--format", "{{.State.Running}}", r.containerID)
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to inspect container: %w", err)
		}

		if strings.TrimSpace(string(output)) != "true" {
			return fmt.Errorf("container is not running")
		}

		// Try to connect to the service
		ingressPort := r.config.Ports["ingress"]
		if ingressPort == 0 {
			ingressPort = 10000
		}

		// Simple health check: try to reach the endpoint
		testReq := Request{
			Model: "none",
			Messages: []Message{
				{Role: "user", Content: "health check"},
			},
		}

		testRouter, err := NewPlanoProxyRouter(&ProxyConfig{
			BaseURL: fmt.Sprintf("http://localhost:%d", ingressPort),
			Timeout: 5,
		})
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err = testRouter.Route(ctx, testReq)
			cancel()
			_ = testRouter.Close()

			if err == nil {
				return nil
			}
		}

		time.Sleep(retryDelay)
	}

	return fmt.Errorf("container did not become healthy within timeout")
}

// IsRunning returns true if the embedded Plano is running
func (r *PlanoEmbeddedRouter) IsRunning() bool {
	return r.isRunning
}

// GetContainerID returns the Docker container ID
func (r *PlanoEmbeddedRouter) GetContainerID() string {
	return r.containerID
}

// healthMonitor continuously monitors container health and triggers restarts
func (r *PlanoEmbeddedRouter) healthMonitor() {
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopChan:
			return
		case <-ticker.C:
			if err := r.performHealthCheck(); err != nil {
				log.Printf("Health check failed: %v", err)
				r.handleUnhealthy()
			}
		}
	}
}

// performHealthCheck checks if the container is healthy
func (r *PlanoEmbeddedRouter) performHealthCheck() error {
	// Check if container is still running
	if r.containerID == "" {
		return fmt.Errorf("no container ID")
	}

	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Running}}", r.containerID)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to inspect container: %w", err)
	}

	if strings.TrimSpace(string(output)) != "true" {
		return fmt.Errorf("container is not running")
	}

	// Try a simple health check through the proxy
	if r.proxyRouter != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		testReq := Request{
			Model: "none",
			Messages: []Message{
				{Role: "user", Content: "health"},
			},
		}

		_, err = r.proxyRouter.Route(ctx, testReq)
		if err != nil {
			return fmt.Errorf("proxy health check failed: %w", err)
		}
	}

	// Update health status
	r.mu.Lock()
	r.isHealthy = true
	r.lastHealthy = time.Now()
	r.mu.Unlock()

	return nil
}

// handleUnhealthy handles container becoming unhealthy
func (r *PlanoEmbeddedRouter) handleUnhealthy() {
	r.mu.Lock()
	r.isHealthy = false
	currentRestarts := r.restartCount
	r.mu.Unlock()

	// Check if max restarts exceeded
	if currentRestarts >= maxRestartAttempts {
		r.mu.Lock()
		r.useFallbackNow = true
		r.mu.Unlock()
		log.Printf("Max restart attempts (%d) exceeded, switching to fallback mode permanently", maxRestartAttempts)
		return
	}

	// Attempt restart
	log.Printf("Container unhealthy, attempting restart %d/%d", currentRestarts+1, maxRestartAttempts)

	r.mu.Lock()
	r.restartCount++
	r.mu.Unlock()

	// Wait before restarting
	time.Sleep(restartBackoff * time.Duration(currentRestarts+1))

	// Stop container
	_ = r.Stop()

	// Try to restart
	if err := r.Start(); err != nil {
		log.Printf("Restart failed: %v", err)
		r.mu.Lock()
		r.isHealthy = false
		r.mu.Unlock()
	} else {
		log.Printf("Container restarted successfully")
		r.mu.Lock()
		r.isHealthy = true
		r.lastHealthy = time.Now()
		r.mu.Unlock()
	}
}

// IsHealthy returns whether the embedded container is healthy
func (r *PlanoEmbeddedRouter) IsHealthy() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isHealthy
}

// GetRestartCount returns the number of times the container has been restarted
func (r *PlanoEmbeddedRouter) GetRestartCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.restartCount
}
