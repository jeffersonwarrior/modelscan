# CLI Package Documentation

## Package Overview

**Package**: `sdk/cli`  
**Purpose**: CLI orchestrator and command management for ModelScan  
**Stability**: Beta  
**Test Coverage**: 100%

---

## Architecture

The CLI package implements a layered command pattern with clear separation of concerns:

```
┌─────────────────────────────────────────────┐
│           User Input (Cobra)                │
├─────────────────────────────────────────────┤
│         CLI Root Command                    │
├─────────────────────────────────────────────┤
│         Orchestrator (Lifecycle)            │
├─────────────────────────────────────────────┤
│         Command Implementations             │
├─────────────────────────────────────────────┤
│         Storage Layer (Repositories)        │
└─────────────────────────────────────────────┘
```

---

## Core Types

### 1. CLI

**File**: `cli.go`

The root command container that manages all CLI operations.

```go
type CLI struct {
    root *cobra.Command
    orchestrator *Orchestrator
    config *config.Config
}
```

**Key Methods**:
- `NewCLI(cfg *config.Config) *CLI` - Factory function
- `Execute() error` - Run the CLI
- `registerCommands()` - Register all subcommands

**Responsibilities**:
- Command registration and organization
- Configuration management
- Orchestrator lifecycle coordination

---

### 2. Orchestrator

**File**: `orchestrator.go`

Manages the lifecycle of the ModelScan system including storage, signal handling, and graceful shutdown.

```go
type Orchestrator struct {
    db     *storage.Database
    config *config.Config
    stopCh chan struct{}
    wg     sync.WaitGroup
}
```

**Key Methods**:
- `NewOrchestrator(cfg *config.Config) (*Orchestrator, error)` - Create and initialize
- `Start(ctx context.Context) error` - Start the system
- `Stop() error` - Graceful shutdown
- `Storage() *storage.Database` - Access storage layer

**Lifecycle Management**:
1. Initialize storage connection
2. Run database migrations
3. Setup signal handlers (SIGTERM, SIGINT)
4. Block until shutdown signal
5. Release resources in order

**Current Limitations**:
- No context cancellation in long-running operations
- HTTP server not implemented yet
- Resource cleanup order not enforced

---

### 3. Command Interface

All CLI commands implement this interface for consistency.

```go
type Command interface {
    Name() string
    Short() string
    Long() string
    RunE(cmd *cobra.Command, args []string) error
}
```

---

## Command Implementations

### Agent Commands

#### `CreateAgentCommand`
```bash
$ modelscan create-agent <name> <mode> <capability>
$ modelscan create-agent my-agent llm text-generation
```

Creates a new agent with specified configuration.

**Fields**:
- `orchestrator *Orchestrator` - System access

**Validation**:
- Name must be non-empty
- Mode must be valid (llm, tool, workflow)
- At least one capability required

---

#### `ListAgentsCommand`
```bash
$ modelscan list-agents
```

Lists all agents in the system.

**Output Format**:
```
ID       Name       Mode      Capabilities  Status
123e456  my-agent   llm       text-gen      active
```

---

#### `GetAgentCommand`
```bash
$ modelscan get-agent <id>
```

Displays detailed information about a specific agent.

---

#### `DeleteAgentCommand`
```bash
$ modelscan delete-agent <id>
```

Removes an agent from the system.

**Confirmation**: Prompts user for confirmation before deletion

---

### Team Commands

#### `CreateTeamCommand`
```bash
$ modelscan create-team <name> [description]
```

Creates a new team for agent collaboration.

---

#### `ListTeamsCommand`
```bash
$ modelscan list-teams
```

Lists all teams with member counts.

---

### Utility Commands

#### `StatusCommand`
```bash
$ modelscan status
```

Shows system status including:
- Database connection state
- Number of agents/teams/tasks
- Uptime
- Version information

---

#### `HelpCommand`
```bash
$ modelscan help
$ modelscan help <command>
```

Displays help information. Commands are sorted alphabetically in the output.

**Special Features**:
- Automatic command discovery
- Grouping by command type
- ANSI color support (when terminal supports it)

---

## Configuration

The CLI uses Viper for configuration management.

**Supported Sources** (in order of precedence):
1. Command line flags
2. Environment variables (prefixed with `MODELSCAN_`)
3. Config file (`~/.modelscan/config.yaml`)
4. Defaults

**Key Configuration Options**:
```yaml
db_path: ~/.modelscan/modelscan.db
log_level: info
max_file_size: 10485760
shutdown_timeout: 30s
```

---

## Error Handling

**Current State**:
- Commands return errors via `RunE`
- Some error context is lost in translation
- No structured error types

**Recommended Improvements**:
1. Define `CLIError` type with context
2. Add error wrapping with `fmt.Errorf("...: %w", err)`
3. Log errors at appropriate levels
4. Exit with proper codes (1 for error, 2 for usage error)

---

## Testing

**Test Files**:
- `cli_test.go` - Integration tests
- `commands_test.go` - Unit tests for individual commands

**Coverage**: 100% for CLI package

**Key Test Scenarios**:
- Command creation and registration
- Help output formatting
- Error message clarity
- Storage integration
- Signal handling

**Test Utilities**:
- `newTestOrchestrator()` - Creates in-memory DB for testing
- `captureOutput()` - Captures stdout/stderr for verification

---

## Usage Examples

### Basic Usage

```bash
# Start with default config
$ modelscan-cli

# Create an agent
$ modelscan-cli create-agent my-agent llm text-generation

# List all agents
$ modelscan-cli list-agents

# Get detailed status
$ modelscan-cli status
```

### Advanced Configuration

```bash
# Custom database location
$ modelscan-cli --db-path=/tmp/modelscan.db status

# Verbose logging
$ MODELSCAN_LOG_LEVEL=debug modelscan-cli create-agent test llm text-generation

# Environment-based config
$ export MODELSCAN_DB_PATH=/data/modelscan.db
$ modelscan-cli list-agents
```

---

## Future Enhancements

1. **Plugin System**: Support dynamic command loading
2. **Interactive Mode**: REPL-style interface
3. **Watch Mode**: Auto-reload on config changes
4. **Remote API**: Client-server mode over HTTP
5. **Completion Scripts**: Bash/Zsh/Fish auto-completion

---

## Known Issues

1. **Context Cancellation**: Long-running operations cannot be cancelled
2. **Error Context**: Some errors lack sufficient debugging information
3. **Resource Cleanup**: Shutdown sequence needs improvement
4. **No Version Command**: Missing `version` command

---

## Development Notes

### Adding a New Command

1. Create new file in `sdk/cli/` (e.g., `new_command.go`)
2. Implement `Command` interface
3. Register in `cli.go:registerCommands()`
4. Add test file `new_command_test.go`
5. Update help documentation

### Command Template

```go
type NewCommand struct {
    orchestrator *Orchestrator
}

func (c *NewCommand) Name() string { return "new" }
func (c *NewCommand) Short() string { return "Brief description" }
func (c *NewCommand) Long() string { return "Long description with usage examples" }

func (c *NewCommand) RunE(cmd *cobra.Command, args []string) error {
    // Implementation here
    return nil
}
```

---

**Last Updated**: December 18, 2025  
**Package Version**: v0.2.0  
**Maintainer**: ModelScan Team