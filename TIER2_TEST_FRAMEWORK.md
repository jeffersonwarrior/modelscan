# Tier 2 Test Framework - Master Quality Control

## Objective
Implement 18 production-ready AI providers with zero external dependencies (except stdlib, sqlite3, google/uuid). All providers must pass TDD requirements with 90%+ test coverage.

## The 18 Providers

### Audio (5)
1. **ElevenLabs** - TTS with voice cloning
2. **Deepgram** - Live streaming STT
3. **OpenAI Whisper** - Audio transcription (extend existing)
4. **OpenAI TTS** - Text-to-speech (extend existing)
5. **PlayHT** - Ultra-realistic TTS

### Video (2)
6. **Luma AI** - Dream Machine video generation
7. **Runway ML** - Gen-2 video generation

### LLM (5)
8. **OpenAI** - o1, GPT-4o (extend existing, remove external SDK)
9. **Anthropic** - Claude 3.5 with extended thinking (extend existing, remove external SDK)
10. **Google Gemini** - Gemini 2.5/3.0 with thinking modes (extend existing, remove external SDK)
11. **DeepSeek** - DeepSeek R1 reasoning (extend existing)
12. **Cerebras** - Ultra-fast inference (extend existing)

### Image (2)
13. **FAL** - FLUX.1 models (extend existing)
14. **Midjourney** - V6 text-to-image (unofficial API workaround)

### Embeddings (3)
15. **OpenAI Embeddings** - text-embedding-3 (extend existing)
16. **Cohere Embeddings** - embed-multilingual-v3.0
17. **Voyage AI** - voyage-2, voyage-code-2

### Real-Time (1)
18. **OpenAI Realtime API** - WebSocket voice streaming (extend existing)

---

## Master Acceptance Criteria

### For Each Provider

#### ✅ Code Quality
- [ ] Zero external dependencies (except mattn/go-sqlite3, google/uuid)
- [ ] Implements `providers.Provider` interface completely
- [ ] All code in `providers/<name>.go` pattern
- [ ] Follows existing code style (gofmt, go vet clean)
- [ ] No hardcoded values (use constants/config)
- [ ] Proper error handling with wrapped errors
- [ ] Context cancellation support
- [ ] No race conditions (verified with -race flag)

#### ✅ Test Coverage (90%+ Required)
- [ ] Unit tests: `providers/<name>_test.go`
- [ ] Integration tests with mocks
- [ ] E2E tests with real API (if keys available)
- [ ] Coverage report shows 90%+ for provider code
- [ ] All edge cases tested (rate limits, errors, timeouts)

#### ✅ Provider Interface Implementation
```go
type Provider interface {
    ValidateEndpoints(ctx context.Context, verbose bool) error
    ListModels(ctx context.Context, verbose bool) ([]Model, error)
    GetCapabilities() ProviderCapabilities
    GetEndpoints() []Endpoint
    TestModel(ctx context.Context, modelID string, verbose bool) error
}
```

#### ✅ HTTP Client Requirements (Feature 0 Foundation)
- [ ] Uses shared `internal/http/client.go` (NOT external SDKs)
- [ ] Automatic retry with exponential backoff
- [ ] Rate limiting integration
- [ ] Request/response logging hooks
- [ ] Timeout management
- [ ] Context propagation
- [ ] Connection pooling

#### ✅ Rate Limiting Integration
- [ ] Queries SQLite for provider rate limits
- [ ] Respects RPM, TPM, RPD limits
- [ ] Handles burst allowance
- [ ] Returns rate limit errors properly
- [ ] Tests verify limit enforcement

#### ✅ Cost Tracking
- [ ] Calculates per-request costs
- [ ] Returns cost breakdown (input/output)
- [ ] Uses pricing from SQLite database
- [ ] Tests verify cost accuracy

#### ✅ Streaming Support (if applicable)
- [ ] Uses `sdk/stream/stream.go` interface
- [ ] SSE/WebSocket/HTTP chunked as needed
- [ ] Handles mid-stream errors
- [ ] Context cancellation works
- [ ] Tests verify streaming behavior

#### ✅ Extended Thinking (LLM only)
- [ ] **Anthropic**: `budget_tokens`, thinking block parsing
- [ ] **Gemini 2.5**: `budgetTokens` parameter
- [ ] **Gemini 3.0**: `effort` parameter (adaptive/medium/high)
- [ ] Tests verify thinking responses

#### ✅ JSON Schema Validation (if using structured output)
- [ ] Schema validation for requests
- [ ] Type coercion where needed
- [ ] Required field enforcement
- [ ] Structured error messages

---

## Test Structure Per Provider

### 1. Unit Tests (providers/<name>_test.go)

**Required Tests:**
```go
// Provider instantiation
func TestNew<Provider>_ValidKey(t *testing.T)
func TestNew<Provider>_EmptyKey(t *testing.T)

// ListModels
func TestListModels_Success(t *testing.T)
func TestListModels_APIError(t *testing.T)
func TestListModels_Timeout(t *testing.T)
func TestListModels_ContextCancellation(t *testing.T)

// ValidateEndpoints
func TestValidateEndpoints_AllWorking(t *testing.T)
func TestValidateEndpoints_SomeFailed(t *testing.T)
func TestValidateEndpoints_NetworkError(t *testing.T)

// TestModel
func TestModel_Success(t *testing.T)
func TestModel_InvalidModel(t *testing.T)
func TestModel_RateLimit(t *testing.T)
func TestModel_AuthFailure(t *testing.T)

// Capabilities
func TestGetCapabilities_ReturnsCorrectValues(t *testing.T)

// Rate limiting
func TestRateLimit_RPM_Enforced(t *testing.T)
func TestRateLimit_TPM_Enforced(t *testing.T)
func TestRateLimit_BurstAllowed(t *testing.T)

// Cost tracking
func TestCostTracking_InputTokens(t *testing.T)
func TestCostTracking_OutputTokens(t *testing.T)
func TestCostTracking_Total(t *testing.T)

// Streaming (if applicable)
func TestStreaming_Success(t *testing.T)
func TestStreaming_Error(t *testing.T)
func TestStreaming_Cancellation(t *testing.T)

// Provider-specific features
// e.g., for Anthropic:
func TestExtendedThinking_BudgetTokens(t *testing.T)
func TestExtendedThinking_ThinkingBlocks(t *testing.T)
```

**Coverage Target:** 90%+ line coverage

### 2. Integration Tests (providers/<name>_integration_test.go)

**Build tag:** `// +build integration`

**Required Tests:**
```go
// Full workflow with mocks
func TestIntegration_ListAndTest(t *testing.T)
func TestIntegration_RateLimitRecovery(t *testing.T)
func TestIntegration_CostAccumulation(t *testing.T)
func TestIntegration_MultipleRequests(t *testing.T)
func TestIntegration_HealthTracking(t *testing.T)
```

### 3. E2E Tests (providers/<name>_e2e_test.go)

**Build tag:** `// +build e2e`

**Required Tests:**
```go
// Real API calls (requires valid API key)
func TestE2E_RealAPICall(t *testing.T) {
    apiKey := os.Getenv("<PROVIDER>_API_KEY")
    if apiKey == "" {
        t.Skip("API key not set")
    }

    // Actual API call
    // Verify response
    // Log cost
    // Maximum budget: $0.01 per test
}

func TestE2E_Streaming(t *testing.T)
func TestE2E_ExtendedThinking(t *testing.T) // if applicable
func TestE2E_RateLimitHeaders(t *testing.T)
```

---

## How Workers Prove Completion

### ✅ Files Created/Modified
1. `providers/<name>.go` - Provider implementation
2. `providers/<name>_test.go` - Unit tests
3. `providers/<name>_integration_test.go` - Integration tests
4. `providers/<name>_e2e_test.go` - E2E tests
5. Git commits to `tier2/core-providers` branch

### ✅ Tests Pass
```bash
# Unit tests
go test -v ./providers -run Test<Provider>

# Coverage check
go test -coverprofile=coverage.out ./providers/<name>.go ./providers/<name>_test.go
go tool cover -func=coverage.out | grep total
# Must show: 90.0%+ coverage

# Integration tests
go test -tags=integration -v ./providers -run TestIntegration<Provider>

# E2E tests (if key available)
export <PROVIDER>_API_KEY=<key>
go test -tags=e2e -v ./providers -run TestE2E<Provider>

# Race detection
go test -race ./providers/<name>_test.go

# Vet check
go vet ./providers/<name>.go
```

### ✅ Build Success
```bash
go build ./...
# Must complete without errors
```

### ✅ Meets Acceptance Criteria
Worker must document in commit message:
```
Provider: <Name> - Implementation Complete

✓ Implements Provider interface
✓ Zero external dependencies
✓ 90%+ test coverage (actual: XX.X%)
✓ Unit tests: XX passing
✓ Integration tests: XX passing
✓ E2E tests: XX passing (or skipped if no key)
✓ Rate limiting integrated
✓ Cost tracking working
✓ Streaming support (if applicable)
✓ Extended thinking (if applicable)
✓ All tests pass
✓ go vet clean
✓ gofmt clean
```

---

## Feature 0: HTTP Foundation

Before launching provider workers, Feature 0 must complete:

### Deliverables
1. `internal/http/client.go` - Shared HTTP client
2. `internal/http/client_test.go` - Full test suite
3. `internal/http/retry.go` - Retry logic with backoff
4. `internal/http/retry_test.go` - Retry tests
5. Documentation for provider workers

### Acceptance Criteria
- [ ] Connection pooling working
- [ ] Retry with exponential backoff (3 attempts default)
- [ ] Rate limit header parsing
- [ ] Context cancellation support
- [ ] Timeout management (default 30s, configurable)
- [ ] Request/response logging hooks
- [ ] 95%+ test coverage
- [ ] No external dependencies
- [ ] Thread-safe
- [ ] Comprehensive error handling

---

## Master Validation (Final Phase)

After all 18 providers complete:

### ✅ Integration Test Suite
```bash
# Test all providers together
go test -v ./providers -run TestAll

# Coverage across all providers
go test -coverprofile=coverage.out ./providers/...
go tool cover -html=coverage.out -o coverage.html

# Verify coverage meets 90%+ average
```

### ✅ Build Validation
```bash
# Build entire project
go build ./...

# Verify no external SDKs
go mod graph | grep -v "mattn/go-sqlite3" | grep -v "google/uuid" | grep -v "spf13/cobra"
# Should only show stdlib dependencies
```

### ✅ P1 Issue Resolution
- [ ] Anthropic extended thinking works
- [ ] Gemini 2.5 token budget works
- [ ] Gemini 3.0 effort settings work

### ✅ P2 Issue Resolution
- [ ] JSON Schema validation framework in place
- [ ] Schema validation tests pass

---

## Success Criteria Summary

**DONE = ALL of the following:**

1. ✅ Feature 0 (HTTP foundation) complete with 95%+ coverage
2. ✅ All 18 providers implemented
3. ✅ Each provider: 90%+ test coverage
4. ✅ All unit tests pass (18 x ~15 tests = ~270 tests)
5. ✅ All integration tests pass
6. ✅ E2E tests pass (or skipped if no key)
7. ✅ Zero external dependencies (except allowed)
8. ✅ All code committed to `tier2/core-providers` branch
9. ✅ `go build ./...` succeeds
10. ✅ `go vet ./...` clean
11. ✅ `gofmt -l .` clean
12. ✅ P1 issues resolved
13. ✅ P2 issues resolved
14. ✅ Integration test suite passes
15. ✅ Documentation updated

**Total Estimated Tests:** ~350-400 tests across all providers

---

## Worker Autonomy Guidelines

### When Blocked
1. Read provider API documentation
2. Web search for error messages/solutions
3. Check similar provider implementations
4. If still stuck after 30 minutes → A2A Overseer

### Quality Standards
- Scientific rigor: tests prove correctness
- No guesswork: verify every claim with tests
- No "done" until all acceptance criteria met
- No shortcuts: implement fully or mark as blocked

### Git Workflow
1. Create feature branch from `tier2/core-providers`
2. Commit frequently (atomic commits)
3. Push to remote
4. Final commit message includes completion checklist

---

This framework ensures every provider is production-ready, scientifically validated, and measurable.
