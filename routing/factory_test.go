package routing

import (
	"testing"
)

func TestNewRouter(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "direct mode",
			config: &Config{
				Mode: ModeDirect,
				Direct: &DirectConfig{
					DefaultProvider: "openai",
				},
			},
			wantErr: false,
		},
		{
			name: "proxy mode",
			config: &Config{
				Mode: ModeProxy,
				Proxy: &ProxyConfig{
					BaseURL: "http://localhost:12000",
				},
			},
			wantErr: false,
		},
		{
			name: "unsupported mode",
			config: &Config{
				Mode: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, err := NewRouter(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRouter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if router == nil {
					t.Error("NewRouter() returned nil router")
				} else {
					router.Close()
				}
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if config.Mode != ModeDirect {
		t.Errorf("Mode = %v, want %v", config.Mode, ModeDirect)
	}

	if config.Direct == nil {
		t.Fatal("Direct config is nil")
	}

	if config.Direct.DefaultProvider != "openai" {
		t.Errorf("DefaultProvider = %v, want openai", config.Direct.DefaultProvider)
	}
}

func TestNewProxyConfigFromURL(t *testing.T) {
	baseURL := "http://localhost:12000"
	config := NewProxyConfigFromURL(baseURL)

	if config == nil {
		t.Fatal("NewProxyConfigFromURL() returned nil")
	}

	if config.Mode != ModeProxy {
		t.Errorf("Mode = %v, want %v", config.Mode, ModeProxy)
	}

	if config.Proxy == nil {
		t.Fatal("Proxy config is nil")
	}

	if config.Proxy.BaseURL != baseURL {
		t.Errorf("BaseURL = %v, want %v", config.Proxy.BaseURL, baseURL)
	}

	if config.Proxy.Timeout != 30 {
		t.Errorf("Timeout = %v, want 30", config.Proxy.Timeout)
	}

	if !config.Fallback {
		t.Error("Fallback should be true")
	}
}

func TestNewEmbeddedConfigFromFile(t *testing.T) {
	configPath := "./plano_config.yaml"
	config := NewEmbeddedConfigFromFile(configPath)

	if config == nil {
		t.Fatal("NewEmbeddedConfigFromFile() returned nil")
	}

	if config.Mode != ModeEmbedded {
		t.Errorf("Mode = %v, want %v", config.Mode, ModeEmbedded)
	}

	if config.Embedded == nil {
		t.Fatal("Embedded config is nil")
	}

	if config.Embedded.ConfigPath != configPath {
		t.Errorf("ConfigPath = %v, want %v", config.Embedded.ConfigPath, configPath)
	}

	if config.Embedded.Image != "katanemo/plano:0.4.0" {
		t.Errorf("Image = %v, want katanemo/plano:0.4.0", config.Embedded.Image)
	}

	if config.Embedded.Ports["ingress"] != 10000 {
		t.Errorf("Ingress port = %v, want 10000", config.Embedded.Ports["ingress"])
	}

	if config.Embedded.Ports["egress"] != 12000 {
		t.Errorf("Egress port = %v, want 12000", config.Embedded.Ports["egress"])
	}

	if !config.Fallback {
		t.Error("Fallback should be true")
	}
}

func TestNewRouter_DirectMode(t *testing.T) {
	config := DefaultConfig()
	router, err := NewRouter(config)

	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	defer router.Close()

	if _, ok := router.(*DirectRouter); !ok {
		t.Error("NewRouter() did not return DirectRouter")
	}
}

func TestNewRouter_ProxyMode(t *testing.T) {
	config := NewProxyConfigFromURL("http://localhost:12000")
	router, err := NewRouter(config)

	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	defer router.Close()

	if _, ok := router.(*PlanoProxyRouter); !ok {
		t.Error("NewRouter() did not return PlanoProxyRouter")
	}
}
