# Provider Implementation Task: {{PROVIDER_NAME}}

## Your Mission

Implement the {{PROVIDER_NAME}} provider with **90%+ test coverage** and full compliance with validation gates.

## Non-Negotiable Requirements

**You CANNOT mark this task complete unless ALL validation gates pass.**

### Validation Gates (Must Pass)

Before completion, you MUST run:

```bash
bash /home/agent/modelscan/scripts/validate-provider.sh {{PROVIDER_NAME}} 90
```

**If ANY gate fails:**
- ❌ DO NOT mark complete
- ❌ DO NOT ask overseer to accept lower threshold
- ✅ FIX the issue
- ✅ Re-run validation
- ✅ Repeat until ALL gates pass

### The 7 Gates

1. **Build**: `go build ./providers/{{PROVIDER_NAME}}.go` must succeed
2. **Tests**: All unit tests must pass
3. **Coverage**: EXACTLY 90.0%+ (not 89.9%, not 89.5%)
4. **Race Detector**: `go test -race` must be clean
5. **Static Analysis**: `go vet` must be clean
6. **Formatting**: `gofmt` must be clean
7. **Interface**: Must implement all Provider interface methods

## How to Mark Complete

**ONLY** use this command (enforces validation):

```bash
bash /home/agent/modelscan/scripts/swarm-mark-complete.sh {{FEATURE_ID}} {{PROVIDER_NAME}}
```

This script will:
1. Run all 7 validation gates
2. If ANY fail → REJECT completion
3. If ALL pass → Mark complete

**There is no manual override. There is no "close enough."**

## Implementation Spec

### Files to Create

1. `providers/{{PROVIDER_NAME}}.go` - Provider implementation
2. `providers/{{PROVIDER_NAME}}_test.go` - Unit tests (90%+ coverage)
3. `providers/{{PROVIDER_NAME}}_integration_test.go` - Integration tests (optional)

### Provider Interface

```go
type Provider interface {
    ValidateEndpoints(ctx context.Context, verbose bool) error
    ListModels(ctx context.Context, verbose bool) ([]Model, error)
    GetCapabilities() ProviderCapabilities
    GetEndpoints() []Endpoint
    TestModel(ctx context.Context, modelID string, verbose bool) error
}
```

### Use Shared HTTP Client

```go
import "github.com/jeffersonwarrior/modelscan/internal/http"

// In your provider
client := http.NewClient(http.Config{
    BaseURL: "https://api.{{PROVIDER_NAME}}.com",
    APIKey:  apiKey,
    Timeout: 30 * time.Second,
    Retry: http.RetryConfig{
        MaxAttempts: 3,
        BaseDelay:   1 * time.Second,
    },
})
```

## Test Requirements

**Minimum 15 tests covering:**

- Provider instantiation (valid key, empty key)
- ListModels (success, error, timeout, cancellation)
- ValidateEndpoints (all working, some failed, network error)
- TestModel (success, invalid model, rate limit, auth failure)
- GetCapabilities (correct values)
- Rate limiting (RPM enforced, TPM enforced, burst allowed)
- Cost tracking (input tokens, output tokens, total)
- Streaming support (if applicable)
- Provider-specific features

## When Blocked

1. **Read provider API docs** (search for official docs)
2. **Web search** for error messages
3. **Check similar providers** (e.g., OpenAI, Anthropic)
4. **Only after 30min** → Ask overseer via A2A message

## Git Workflow

```bash
# All work on feature branch
git checkout -b feat/{{PROVIDER_NAME}}-implementation

# Commit frequently (atomic commits)
git add providers/{{PROVIDER_NAME}}.go
git commit -m "feat({{PROVIDER_NAME}}): initial implementation"

git add providers/{{PROVIDER_NAME}}_test.go
git commit -m "test({{PROVIDER_NAME}}): add unit tests"

# Final commit after validation passes
git add .
git commit -m "feat({{PROVIDER_NAME}}): complete with 90%+ coverage

✅ All validation gates passing
✅ Coverage: {{ACTUAL_COVERAGE}}%
✅ Tests: {{TEST_COUNT}} passing
✅ Race detector clean
✅ Interface fully implemented

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"
```

## Philosophy

> "90% means 90%. Not 89.9%. Exact compliance."

This is **math and science**, not expressionism. The validation script is the source of truth.

- If validation passes → you succeeded
- If validation fails → you didn't succeed yet
- No human judgment involved
- No negotiation
- No exceptions

## Questions?

Ask NOW before starting work. Once you begin, the validation gates are your only metric.

---

**Ready to start? Read the API docs, ask questions, then implement.**
