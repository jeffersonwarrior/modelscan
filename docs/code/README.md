# Code Documentation

This directory contains auto-generated documentation for the ModelScan multi-agent framework.

## Generated Documentation

- [Agent SDK](./agent_sdk.md) - Agent management and coordination
- [Storage Layer](./storage_layer.md) - Database persistence layer
- [CLI Orchestration](./cli_orchestration.md) - Command-line interface and orchestration
- [Router System](./router_system.md) - Message routing and flow control
- [Rate Limiting](./rate_limiting.md) - Token bucket rate limiter

## Architecture Overview

```
┌─────────────────────────────────────────┐
│              CLI Layer                  │
│  ┌─────────────┐  ┌─────────────────┐   │
│  │   Commands  │  │   Orchestrator  │   │
│  └─────────────┘  └─────────────────┘   │
└─────────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────┐
│            Agent SDK Layer              │
│  ┌─────────────┐  ┌─────────────────┐   │
│  │   Agent     │  │      Team       │   │
│  ├───┬───┬───┬──┤  ├───┬───┬───┬───┤   │
│  │Mem │Msg │Tool│  │Mem │Msg │Work │   │
│  └───┴───┴───┴──┘  └───┴───┴───┴───┘   │
└────────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────┐
│           Storage Layer                 │
│  ┌─────────────┐  ┌─────────────────┐   │
│  │ Repository  │  │    Database     │   │
│  │   Pattern   │  │   (SQLite)      │   │
│  └─────────────┘  └─────────────────┘   │
└─────────────────────────────────────────┘
```

## Generation

Documentation is generated using:
```bash
# Generate all documentation
make docs

# Generate specific documentation
go run -tags "docgen" cmd/docgen/main.go
```

## Viewing Documentation

- View locally in your editor
- Check in to repository for review
- Deploy to documentation site as needed