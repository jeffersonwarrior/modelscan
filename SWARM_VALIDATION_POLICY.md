# Swarm Validation Policy

## Mandatory Validation Gates

All workers MUST pass validation gates before marking features complete.

### Feature 0: HTTP Foundation Layer

**Validation Script**: `scripts/validate-http.sh`

**Requirements**:
- ✅ Build succeeds: `go build ./internal/http`
- ✅ All tests pass: `go test -v ./internal/http`
- ✅ **93.0%+ coverage** (70 comprehensive tests, all critical paths covered)
- ✅ Race detector clean: `go test -race ./internal/http`
- ✅ Static analysis clean: `go vet ./internal/http`
- ✅ Formatting clean: `gofmt -l ./internal/http`

**Rationale**: 93% threshold reflects production-grade quality with comprehensive edge case testing. Remaining 7% consists of hard-to-trigger error paths in network code. Provider implementations use 90% threshold.

**Enforcement**: Workers cannot use `mark_complete` until validation script returns exit code 0.

---

### Provider Features (2-19)

**Validation Script**: `scripts/validate-provider.sh <provider-name>`

**Requirements** (per TIER2_TEST_FRAMEWORK.md):
- ✅ Build succeeds: `go build ./providers/<name>.go`
- ✅ All tests pass: `go test -v ./providers/<name>_test.go`
- ✅ **90.0%+ coverage** (exact threshold)
- ✅ Integration tests pass: `go test -tags=integration -v ./providers`
- ✅ E2E tests pass or skip: `go test -tags=e2e -v ./providers`
- ✅ Race detector clean
- ✅ Static analysis clean
- ✅ Formatting clean
- ✅ Implements `providers.Provider` interface completely

**Enforcement**: Same as Feature 0 - validation must pass before completion.

---

## Worker Protocol

### Before Starting Work

1. Read feature specification
2. Read validation requirements (this document)
3. Review validation script
4. Ask questions about acceptance criteria

### During Work

1. Run validation frequently (TDD approach)
2. Fix issues immediately
3. Don't accumulate technical debt

### Before Marking Complete

1. **MANDATORY**: Run validation script
2. If exit code != 0: FIX ISSUES, do not proceed
3. Only after validation passes: use `mark_complete`
4. Include validation output in completion message

---

## Overseer Protocol

### Before Accepting Completion

1. Verify worker ran validation (check output)
2. Re-run validation independently
3. If validation fails: reject completion, send back to worker
4. Only mark success=true if validation passes

---

## Philosophy

> "Coverage thresholds are exact minimums based on code criticality and test quality."

**HTTP Foundation (93%)**: Infrastructure code with comprehensive edge case coverage across 70 tests.
**Providers (90%)**: Application code with standard test requirements.

Validation gates enforce scientific rigor. Thresholds are set deliberately, not arbitrarily.

- Coverage thresholds are **exact minimums**
- All quality checks must pass
- No "good enough" - only "meets spec"
- This is math and science, not expressionism

---

## Validation Script Locations

```bash
# HTTP Foundation
./scripts/validate-http.sh

# Individual providers
./scripts/validate-provider.sh elevenlabs
./scripts/validate-provider.sh deepgram
# ... (for all 18 providers)
```

---

## Failure Handling

If validation fails:
1. **DO NOT** mark complete
2. **DO NOT** adjust thresholds to match current state
3. **DO** fix the code/tests to meet the threshold
4. **DO** re-run validation until it passes

This is non-negotiable.
