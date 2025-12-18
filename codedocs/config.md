# Config Package Documentation

## Package Overview

**Package**: `config`  
**Purpose**: Configuration management with environment variables and file support  
**Stability**: Stable  
**Test Coverage**: ~50%

---

## Core Types

### Config

Central configuration struct for the entire system.

```go
type Config struct {
    DBPath           string
    WALMode          bool
    MaxFileSize      int64
    LogLevel         string
    ProviderDefaults map[string]ProviderConfig
    APIKeys          map[string]string
    ShutdownTimeout  time.Duration
}
```

**Key Methods**:
- `Load() (*Config, error)` - Load from environment and files
- `Default() *Config` - Zero-config defaults
- `Validate() error` - Validate required fields

**Load Order** (high to low precedence):
1. Environment variables (`MODELSCAN_DB_PATH`)
2. Config file (`~/.modelscan/config.yaml`)
3. Defaults

**Defaults**:
```go
DBPath:          ~/.modelscan/modelscan.db
WALMode:         true
MaxFileSize:     10MB
LogLevel:        \"info\"
ShutdownTimeout: 30s
```

---

## ProviderConfig

Provider-specific settings.

```go
type ProviderConfig struct {
    APIKey     string
    BaseURL    string
    Model      string
    Temperature float64
    MaxTokens  int
    Timeout    time.Duration
}
```

---

## Usage

```go
cfg, err := config.Load()
if err != nil {
    log.Fatal(err)
}

// Override specific settings
cfg.DBPath = \"/custom/path.db\"

db, err := storage.NewDatabase(cfg.DBPath)
```

---

## Validation

**Current**: Basic non-nil checks  
**Recommended**: Full validation with `validator.v10`

---

**Last Updated**: December 18, 2025