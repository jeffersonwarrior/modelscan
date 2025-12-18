# Commands Package Documentation

## Package Overview

**Package**: `cmd/*`  
**Purpose**: Executable entry points  
**Stability**: Stable  
**Test Coverage**: N/A (binaries)

---

## Binaries

### modelscan (CLI Orchestrator)

**File**: `cmd/modelscan/main.go`

Main CLI entry point.

```bash
go build -o modelscan-cli cmd/modelscan/main.go
./modelscan-cli create-agent test llm text-gen
```

### demo

**File**: `cmd/demo/main.go`

Quickstart demo.

### seed-db

**File**: `cmd/seed-db/main.go`

Database seeding utility.

---

**Last Updated**: December 18, 2025