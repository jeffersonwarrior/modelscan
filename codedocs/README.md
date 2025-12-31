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

## Design Documents

- [Tool Calling Standards](./TOOLING-STANDARDS.md) - Tool calling standardization (design phase)
- [SDK Autogen Design](./SDK_AUTOGEN_DESIGN.md) - SDK generation architecture
- [Model Autoconfig](./MODEL_AUTOCONFIG.md) - Model auto-configuration
- [Architecture Recommendations](./ARCHITECTURE_RECOMMENDATIONS.md) - Future improvements
- [Cleanup Report](./CLEANUP_REPORT.md) - Codebase cleanup analysis

## Status

**Version**: 0.3.1
**Last Updated**: December 31, 2025
**Build**: ✓ All packages passing
**Coverage**: 81% average across tested packages