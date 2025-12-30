# Architecture Recommendations & Cleanup Plan

## Current Issues

### 1. Triple Code Duplication
- `cmd/modelscan-server/main.go` - 172 lines of adapters
- `internal/integration/integration.go` - 269 lines of adapters
- `internal/service/service.go` - 269 lines of adapters
- **Total**: ~700 lines duplicated (13% of internal/)

### 2. Architectural Drift
- V0.3_ARCHITECTURE.md documents `internal/service/` as coordinator
- Reality: `cmd/modelscan-server/main.go` does everything inline
- `internal/integration/` exists but not documented
- `internal/service/` exists but unused

### 3. Module Complexity
- 7 internal packages each with own go.mod
- Cannot run `go test ./internal/...`
- Must cd into each package directory

## Recommendations

### Architecture: Layered Service Pattern

```
┌─────────────────────────────────────────┐
│     cmd/modelscan-server/main.go        │  Entry point (50 lines)
│     - Parse flags                       │
│     - Load config                       │
│     - Create service                    │
│     - Handle signals                    │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│     internal/service/service.go         │  Service coordinator (200 lines)
│     - Initialize components             │
│     - Lifecycle management              │
│     - Component wiring                  │
│     - NO adapters (use direct types)    │
└─────────────────┬───────────────────────┘
                  │
        ┌─────────┼─────────┐
        ▼         ▼         ▼
   ┌────────┬────────┬────────┐
   │Database│Discovery│KeyMgr  │  Core packages
   └────────┴────────┴────────┘
```

**Key principles**:
1. **Single source of truth**: Only `internal/service/` wires components
2. **No adapters needed**: Use types directly from packages
3. **Thin entry point**: `main.go` just starts/stops service
4. **Delete**: `internal/integration/` (redundant)

### Module Structure: Single Module

**Current**: 8 separate modules (main + 7 internal)
**Recommended**: 1 module

```go
module github.com/jeffersonwarrior/modelscan

// No replace directives needed
// No separate internal go.mod files
```

**Benefits**:
- Simpler dependency management
- Standard `go test ./...` works
- Faster builds (no cross-module coordination)
- Easier refactoring

**When to use separate modules**:
- Independent versioning needed
- Different release cycles
- External consumption of internal packages
- **None of these apply here**

## Implementation Plan

### Phase 1: Simplify to Service Pattern
1. Update `internal/service/service.go`:
   - Remove adapter code
   - Use package types directly
   - Implement routing integration
2. Update `cmd/modelscan-server/main.go`:
   - Remove all inline code
   - Just call service.New() and service.Start()
3. Delete `internal/integration/` entirely

### Phase 2: Consolidate Modules
1. Remove all `internal/*/go.mod` files
2. Remove replace directives from root go.mod
3. Run `go mod tidy`
4. Update tests to use standard paths

### Phase 3: Integrate Routing
1. Import routing package in service
2. Create router based on config
3. Wire into RouteRequest method
4. Remove TODO stub

### Phase 4: Polish
1. Update .gitignore
2. Commit remaining go.mod/sum files
3. Update V0.3_ARCHITECTURE.md to match reality
4. Full validation

## Type Unification Strategy

The adapter code exists because of type incompatibilities:
- `database.Provider` vs `admin.Provider`
- `database.APIKey` vs `admin.APIKey`

**Solution**: Use database types as source of truth everywhere.

Admin API should accept database types:
```go
// Before (requires adapters)
func (api *API) CreateProvider(p *admin.Provider) error

// After (no adapters needed)
func (api *API) CreateProvider(p *database.Provider) error
```

This eliminates 700 lines of conversion code.
