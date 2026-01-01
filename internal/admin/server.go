package admin

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

// ShutdownFunc is a callback function for initiating server shutdown
type ShutdownFunc func() error

// ServerAPI handles server info and control endpoints
type ServerAPI struct {
	startTime        time.Time
	version          string
	port             int
	requestsServed   *int64
	clientCounter    ClientCounter
	providerCounter  ProviderCounter
	keyCounter       KeyCounter
	shutdownFunc     ShutdownFunc
}

// ClientCounter interface for counting clients
type ClientCounter interface {
	Count() (int, error)
}

// ProviderCounter interface for counting providers
type ProviderCounter interface {
	ListProviders() ([]*Provider, error)
}

// KeyCounter interface for counting API keys
type KeyCounter interface {
	ListKeys(providerID string) ([]*APIKey, error)
	CountKeys() (int, error)
}

// ServerInfoResponse represents the response for GET /api/server/info
type ServerInfoResponse struct {
	Version            string `json:"version"`
	UptimeSeconds      int64  `json:"uptime_seconds"`
	PID                int    `json:"pid"`
	Port               int    `json:"port"`
	ClientsConnected   int    `json:"clients_connected"`
	RequestsServed     int64  `json:"requests_served"`
	ProvidersAvailable int    `json:"providers_available"`
	KeysConfigured     int    `json:"keys_configured"`
}

// NewServerAPI creates a new ServerAPI instance
func NewServerAPI(version string, port int) *ServerAPI {
	var counter int64
	return &ServerAPI{
		startTime:      time.Now(),
		version:        version,
		port:           port,
		requestsServed: &counter,
	}
}

// SetClientCounter sets the client counter for the server API
func (s *ServerAPI) SetClientCounter(counter ClientCounter) {
	s.clientCounter = counter
}

// SetProviderCounter sets the provider counter for the server API
func (s *ServerAPI) SetProviderCounter(counter ProviderCounter) {
	s.providerCounter = counter
}

// SetKeyCounter sets the key counter for the server API
func (s *ServerAPI) SetKeyCounter(counter KeyCounter) {
	s.keyCounter = counter
}

// SetShutdownFunc sets the shutdown callback function
func (s *ServerAPI) SetShutdownFunc(fn ShutdownFunc) {
	s.shutdownFunc = fn
}

// IncrementRequests increments the requests served counter
func (s *ServerAPI) IncrementRequests() {
	atomic.AddInt64(s.requestsServed, 1)
}

// HandleServerInfo handles GET /api/server/info
func (s *ServerAPI) HandleServerInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Calculate uptime
	uptime := time.Since(s.startTime)

	// Get client count
	clientCount := 0
	if s.clientCounter != nil {
		count, err := s.clientCounter.Count()
		if err == nil {
			clientCount = count
		}
	}

	// Get provider count
	providerCount := 0
	if s.providerCounter != nil {
		providers, err := s.providerCounter.ListProviders()
		if err == nil {
			providerCount = len(providers)
		}
	}

	// Get key count (optimized single query)
	keyCount := 0
	if s.keyCounter != nil {
		count, err := s.keyCounter.CountKeys()
		if err == nil {
			keyCount = count
		}
	}

	resp := ServerInfoResponse{
		Version:            s.version,
		UptimeSeconds:      int64(uptime.Seconds()),
		PID:                os.Getpid(),
		Port:               s.port,
		ClientsConnected:   clientCount,
		RequestsServed:     atomic.LoadInt64(s.requestsServed),
		ProvidersAvailable: providerCount,
		KeysConfigured:     keyCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ShutdownResponse represents the response for POST /api/server/shutdown
type ShutdownResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// HandleShutdown handles POST /api/server/shutdown
func (s *ServerAPI) HandleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.shutdownFunc == nil {
		http.Error(w, "Shutdown not configured", http.StatusServiceUnavailable)
		return
	}

	// Send response before initiating shutdown
	resp := ShutdownResponse{
		Status:  "shutting_down",
		Message: "Server shutdown initiated",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

	// Flush the response before initiating shutdown
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Initiate shutdown in a goroutine so the response can be sent
	go func() {
		// Small delay to ensure response is sent
		time.Sleep(100 * time.Millisecond)
		if err := s.shutdownFunc(); err != nil {
			// Log error but can't return to client at this point
			log.Printf("Error during shutdown: %v", err)
		}
	}()
}
