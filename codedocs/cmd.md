# Commands Package Documentation

**Package**: `cmd/*`
**Purpose**: Executable entry points
**Stability**: Stable
**Version**: 0.3.1

---

## Binaries

### modelscan (Main CLI)

**File**: `cmd/modelscan/main.go` (145 lines)

Primary CLI for provider validation, model listing, and database initialization.

**Usage:**
```bash
# Initialize database
modelscan --init

# Show version
modelscan --version

# Start service (requires initialized database)
modelscan --config config.yaml
```

**Flags:**
- `--config` - Path to configuration file (default: config.yaml)
- `--version` - Show version information
- `--init` - Initialize database and exit

**Features:**
- Database initialization and migration
- Configuration validation
- Service orchestration
- Graceful shutdown on SIGINT/SIGTERM

---

### modelscan-server (HTTP Service)

**File**: `cmd/modelscan-server/main.go` (100 lines)

Long-running HTTP service with auto-discovering SDK capabilities.

**Usage:**
```bash
# Start with config
modelscan-server --config config.yaml

# Start with defaults
modelscan-server
```

**Defaults:**
- Database: `./modelscan.db`
- Server: `localhost:8080`
- Agent Model: `claude-sonnet-4-5`

**Features:**
- Graceful config fallback
- Service lifecycle management
- Signal handling (SIGINT, SIGTERM)
- HTTP API on port 8080

---

### demo (Quickstart Tool)

**File**: `cmd/demo/main.go` (230 lines)

Demonstration tool with extended usage examples.

**Purpose:**
- Provider validation examples
- Model listing demonstrations
- SDK usage patterns

---

### seed-db (Database Utility)

**File**: `cmd/seed-db/main.go` (44 lines)

Database seeding utility for development and testing.

**Purpose:**
- Populate initial provider data
- Create test fixtures
- Development environment setup

---

## Build

```bash
# Build all binaries
make all

# Build specific binary
go build -o modelscan cmd/modelscan/main.go
go build -o modelscan-server cmd/modelscan-server/main.go
```

---

**Last Updated**: December 31, 2025
