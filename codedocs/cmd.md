# Commands Package Documentation

**Package**: `cmd/*`
**Purpose**: Executable entry points
**Stability**: Stable
**Version**: 0.5.5

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

**Files**:
- `cmd/modelscan-server/main.go` (207 lines) - Entry point
- `cmd/modelscan-server/daemon.go` (229 lines) - Daemon mode

Long-running HTTP service with daemon mode and hot-reload support.

**Usage:**
```bash
# Foreground mode
modelscan-server --config config.yaml

# Daemon mode (background)
modelscan-server --daemon

# Daemon control
modelscan-server --status    # Show running daemon info
modelscan-server --stop      # Graceful shutdown
modelscan-server --reload    # Reload configuration (SIGHUP)
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--config` | Path to configuration file (default: config.yaml) |
| `--version` | Show version information |
| `--daemon` | Run as background daemon |
| `--stop` | Stop running daemon (sends SIGTERM) |
| `--reload` | Reload configuration (sends SIGHUP) |
| `--status` | Show daemon status (PID, port, version) |

**Defaults:**
- Database: `./modelscan.db`
- Server: `localhost:8080`
- Agent Model: `claude-sonnet-4-5`

**Features:**
- Graceful config fallback
- Service lifecycle management
- Signal handling (SIGINT, SIGTERM, SIGHUP)
- HTTP API with dynamic port selection
- PID file management for daemon discovery
- Hot configuration reload without restart

---

## Daemon Mode

### Architecture

```
modelscan-server --daemon
         │
         ├── Checks for existing daemon (PID file)
         ├── Forks child process (--daemon-child)
         ├── Detaches from terminal (setsid)
         ├── Redirects output to log file
         └── Parent exits, child continues
```

### PID File

**Location**: `~/.modelscan/modelscan.pid`

**Format (JSON):**
```json
{
  "pid": 12345,
  "port": 8080,
  "host": "127.0.0.1",
  "version": "0.5.5",
  "started_at": "2026-01-01T12:00:00Z"
}
```

**Functions** (`internal/service/portfile.go`):
- `WritePIDFile(port, version)` - Write PID file after server starts
- `ReadPIDFile()` - Read existing PID file
- `RemovePIDFile()` - Clean up on shutdown
- `IsServerRunning()` - Check if daemon is running (validates PID)

### Log File

**Location**: `~/.modelscan/modelscan.log`

- Created automatically in daemon mode
- Append-only with timestamps
- Directory created with 0700 permissions

### Signal Handling

| Signal | Action |
|--------|--------|
| `SIGTERM` | Graceful shutdown (30s timeout) |
| `SIGINT` | Graceful shutdown (Ctrl+C) |
| `SIGHUP` | Hot configuration reload |

**Double Ctrl+C**: Force immediate shutdown (foreground mode only)

### Example Workflow

```bash
# Start daemon
$ modelscan-server --daemon
modelscan daemon started (PID: 12345)
Log file: /home/user/.modelscan/modelscan.log

# Check status
$ modelscan-server --status
modelscan daemon is running
  PID:     12345
  Port:    8080
  Host:    127.0.0.1
  Started: 2026-01-01T12:00:00Z
  Version: 0.5.5

# Reload config
$ modelscan-server --reload
Reload signal sent to modelscan daemon (PID: 12345)

# Stop daemon
$ modelscan-server --stop
Stop signal sent to modelscan daemon (PID: 12345)
```

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

**Last Updated**: January 1, 2026
