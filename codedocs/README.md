# ModelScan Code Documentation

Production-ready Go SDKs for 21+ LLM providers with zero dependencies.

## Architecture

- [Architecture Overview](./architecture.md) - System design and data flow
- [V0.3 Architecture](./V0.3_ARCHITECTURE.md) - Auto-discovering SDK service

## Core Packages

| Package | Status | Coverage | Description |
|---------|--------|----------|-------------|
| [cmd](./cmd.md) | Stable | N/A | Binary entry points |
| [providers](./providers.md) | Production | Varied | 21+ provider implementations |
| [routing](./router.md) | Production | 80%+ | Plano routing modes (direct/proxy/embedded) |
| [internal/database](./storage.md) | Production | 75% | SQLite persistence with migrations |
| [internal/config](./V0.3_ARCHITECTURE.md#2-config-system) | Stable | 85% | YAML configuration with env overrides |
| [internal/discovery](./discovery.md) | Beta | 70% | LLM-powered provider discovery |
| [internal/http](./http-client.md) | Production | 85% | HTTP client with retry logic |
| [internal/service](./V0.3_ARCHITECTURE.md#7-service-orchestration) | Production | 80% | Service orchestration |

## v0.5.5 MClaude Integration Packages

| Package | Status | Coverage | Description |
|---------|--------|----------|-------------|
| [internal/proxy](./proxy.md) | Production | 80%+ | LLM API proxy with SSE streaming |
| [internal/admin](./admin-api.md) | Production | 80%+ | REST API for clients, aliases, remaps |
| [internal/keymanager](./keymanager.md) | Production | 85% | Round-robin key selection, rate limiting |
| [internal/tooling](./tooling.md) | Production | 80%+ | Universal tool calling across providers |
| [cmd/modelscan/setup](./setup.md) | Production | 75% | Interactive setup wizard |

## Audits & Quality

- **[Audit Report 2026-01-01](./AUDIT-2026-01-01.md)** - Full codebase audit (123 issues identified)
- **[Audit Fixes 2026-01-01](./AUDIT-FIXES-2026-01-01.md)** - ✓ 37 critical/high issues fixed
- [Cleanup Report](./CLEANUP_REPORT.md) - Codebase cleanup analysis
- [Architecture Recommendations](./ARCHITECTURE_RECOMMENDATIONS.md) - Future improvements

## Design Documents

- [Tool Calling Standards](./TOOLING-STANDARDS.md) - Tool calling standardization (design phase)
- [SDK Autogen Design](./SDK_AUTOGEN_DESIGN.md) - SDK generation architecture
- [Model Autoconfig](./MODEL_AUTOCONFIG.md) - Model auto-configuration

## Agentic Tooling (v0.5.5+)

- [Swarm Integration](./agentic/MODELSCAN-SWARM-INTEGRATION.md) - Multi-agent orchestration
- [Swarm Protocol](./agentic/SWARM-OVERSIGHT-PROTOCOL.md) - Worker oversight protocol
- [v0.5.0 Swarm Tracker](./agentic/V0.5.0-TRACKER.md) - Feature implementation tracking

## Status

**Version**: 0.5.5
**Last Updated**: January 1, 2026
**Build**: ✓ All packages passing
**Tests**: ✓ Race detection enabled
**Coverage**: 81% average across tested packages
**Audit**: 123 issues identified (13 critical, 24 high) - see [Audit Report](./AUDIT-2026-01-01.md)