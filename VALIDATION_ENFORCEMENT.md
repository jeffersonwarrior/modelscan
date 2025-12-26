# Validation Enforcement - How It Works

## Problem We're Solving

**Before**: Acceptance criteria documented but not enforced
- Workers self-report completion
- Overseer manually checks (can be forgotten/skipped)
- "Close enough" gets accepted (93% instead of 95%)

**After**: Programmatic enforcement - no human judgment
- Validation script is the ONLY way to mark complete
- Script exits with code 1 if ANY gate fails
- No manual override, no exceptions

## The Three-Layer Enforcement

### Layer 1: Validation Scripts (The Gates)

**HTTP Foundation**: `scripts/validate-http.sh`
- Threshold: 93%
- 6 gates (build, test, coverage, race, vet, fmt)
- Exit code 0 = pass, 1 = fail

**Providers**: `scripts/validate-provider.sh <name> <threshold>`
- Threshold: 90%
- 7 gates (build, test, coverage, race, vet, fmt, interface)
- Exit code 0 = pass, 1 = fail

### Layer 2: Completion Wrapper (The Enforcer)

**Script**: `scripts/swarm-mark-complete.sh <feature-id> <provider-name>`

```bash
# Step 1: Run validation
bash scripts/validate-provider.sh $PROVIDER 90

# Step 2: Check exit code
if [ $? -ne 0 ]; then
    echo "❌ COMPLETION REJECTED"
    exit 1
fi

# Step 3: Only if validation passed
mark_complete --feature-id $FEATURE_ID --success true
```

**Workers use THIS script instead of direct mark_complete calls.**

### Layer 3: Autonomous Monitoring (The Watchdog)

**Script**: `scripts/monitor-workers.sh`

Runs in background, every 60 seconds:
1. Check git commit activity (detect frozen workers)
2. Run quick test checks
3. Calculate coverage
4. Suggest validation when ready (>=90%)
5. Alert if worker frozen (>5min no activity)

## Worker Workflow (Enforced)

### What Workers Do

```bash
# 1. Implement provider
vim providers/elevenlabs.go

# 2. Write tests
vim providers/elevenlabs_test.go

# 3. Run tests during development (optional)
go test -v ./providers/elevenlabs_test.go

# 4. When ready, attempt completion
bash scripts/swarm-mark-complete.sh feature-2 elevenlabs
```

### What Happens Behind the Scenes

```bash
# swarm-mark-complete.sh runs:
bash scripts/validate-provider.sh elevenlabs 90

# validate-provider.sh checks:
# ✅ Build succeeds
# ✅ Tests pass (all of them)
# ✅ Coverage >= 90.0% (not 89.9%)
# ✅ Race detector clean
# ✅ go vet clean
# ✅ gofmt clean
# ✅ Interface implemented

# If ALL pass:
#   → mark_complete success=true
#   → Feature marked done

# If ANY fail:
#   → exit 1
#   → Worker sees "❌ COMPLETION REJECTED"
#   → Worker must fix and retry
```

## Overseer Workflow (Minimal Manual Work)

### Before Launching Workers

```bash
# 1. Start autonomous monitor
bash scripts/monitor-workers.sh &

# 2. Launch workers (with enforced prompts)
# Each worker gets templates/PROVIDER_WORKER_PROMPT.md
# which tells them to use swarm-mark-complete.sh

# 3. Monitor output
# Watch for "READY FOR VALIDATION!" messages
```

### During Execution

**No manual validation needed!**

The monitor script shows:
- ✅ Active workers (making commits)
- ⚠️ Frozen workers (no activity >5min)
- 🧪 Test status
- 📊 Coverage percentages
- 🎯 Ready for validation alerts

### Responding to Failures

If a worker tries to complete but validation fails:

```
Worker: bash scripts/swarm-mark-complete.sh feature-2 elevenlabs

Output:
========================================
❌ COMPLETION REJECTED
========================================

Coverage 89.5% < 90%

Worker cannot mark complete until validation passes.
This is non-negotiable.
```

**Overseer does**: Nothing. The system enforced the rule.

**Worker must do**: Add more tests, fix coverage, retry.

## How This Enforces "95% means 95%"

### The Math

```bash
# In validate-provider.sh:
THRESHOLD=90
COVERAGE=89.9

if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
    exit 1  # REJECT
fi
```

**Result**:
- 89.9% < 90.0% → FAIL ❌
- 90.0% = 90.0% → PASS ✅
- 90.1% > 90.0% → PASS ✅

**No rounding. No "close enough."**

### The Exit Codes

Scripts use exit codes (POSIX standard):
- `exit 0` = success
- `exit 1` = failure

**Shell won't proceed** if exit code is non-zero.

```bash
# This BLOCKS:
bash validate.sh || exit 1
mark_complete  # <-- Never runs if validation failed
```

### The No-Override Policy

```bash
# Worker CANNOT do this:
mark_complete --success true  # Missing validation!

# Worker MUST do this:
bash swarm-mark-complete.sh feature-2 elevenlabs
# ^ This runs validation first, fails if not 90%
```

**There is no manual override.** The only way to pass is to meet the threshold.

## Testing the Enforcement

### Simulate a failing worker

```bash
# Create a provider with 80% coverage (intentionally low)
cat > providers/test_provider.go << 'EOF'
package providers

func Add(a, b int) int {
    if a < 0 { return 0 }  // Untested branch
    if b < 0 { return 0 }  // Untested branch
    return a + b
}
EOF

cat > providers/test_provider_test.go << 'EOF'
package providers

import "testing"

func TestAdd(t *testing.T) {
    result := Add(5, 3)
    if result != 8 {
        t.Errorf("Add(5,3) = %d, want 8", result)
    }
}
EOF

# Try to validate (will fail)
bash scripts/validate-provider.sh test_provider 90

# Output:
# ❌ FAIL: Coverage 66.7% < 90%
# (Exit code 1)
```

### Simulate a passing worker

```bash
# Add tests to cover the branches
cat > providers/test_provider_test.go << 'EOF'
package providers

import "testing"

func TestAdd(t *testing.T) {
    tests := []struct {
        a, b, want int
    }{
        {5, 3, 8},
        {-1, 3, 0},  // Cover a < 0 branch
        {5, -1, 0},  // Cover b < 0 branch
    }

    for _, tt := range tests {
        result := Add(tt.a, tt.b)
        if result != tt.want {
            t.Errorf("Add(%d,%d) = %d, want %d", tt.a, tt.b, result, tt.want)
        }
    }
}
EOF

# Validate again
bash scripts/validate-provider.sh test_provider 90

# Output:
# ✅ PASS: Coverage 100% >= 90%
# (Exit code 0)
```

## Why This Works

1. **Scripts are deterministic**: Same input = same output
2. **Exit codes are boolean**: Pass (0) or Fail (1), no gray area
3. **Shell enforces dependencies**: Can't call B if A failed
4. **No human judgment**: Computer checks the math
5. **Workers can't bypass**: Only path to completion goes through validation

## Summary

**Before**: "Worker says 93%, overseer accepts it"
**After**: "Script says <95%, completion blocked automatically"

The philosophy shift:
- From: "Did the worker do enough?"
- To: "Does the validation script exit 0?"

**Math doesn't negotiate. Neither does the script.**
