#!/bin/bash
# Provider Validation Gate
# Usage: ./scripts/validate-provider.sh <provider-name> [threshold]
#
# Example: ./scripts/validate-provider.sh elevenlabs 90

set -e

if [ -z "$1" ]; then
    echo "Usage: $0 <provider-name> [coverage-threshold]"
    echo "Example: $0 elevenlabs 90"
    exit 1
fi

PROVIDER=$1
THRESHOLD=${2:-90}

echo "=== Provider Validation Gate: $PROVIDER ==="
echo "Coverage threshold: ${THRESHOLD}%"
echo

# Change to providers directory
cd /home/agent/modelscan/providers

# 1. Build check
echo "1. Build check..."
if [ -f "${PROVIDER}.go" ]; then
    go build . || { echo "❌ FAIL: Build failed"; exit 1; }
    echo "✅ PASS: Build succeeds"
else
    echo "❌ FAIL: ${PROVIDER}.go not found"
    exit 1
fi
echo

# 2. Test check
echo "2. Test check..."
if [ -f "${PROVIDER}_test.go" ]; then
    # Extract test pattern from first test function name
    # e.g., "func TestNewElevenLabsProvider" or "func TestElevenLabsProvider_ListModels" → "ElevenLabs"
    TEST_PATTERN=$(grep "^func Test" ${PROVIDER}_test.go | head -1 | sed -E 's/.*Test(New)?([A-Z][a-zA-Z0-9]*)(Provider)?.*/\2/')
    if [ -z "$TEST_PATTERN" ]; then
        # Fallback: just capitalize first letter
        TEST_PATTERN=$(echo "${PROVIDER}" | sed 's/./\U&/')
    fi
    go test -v -run "$TEST_PATTERN" . || { echo "❌ FAIL: Tests failed"; exit 1; }
    echo "✅ PASS: All tests pass"
else
    echo "❌ FAIL: ${PROVIDER}_test.go not found"
    exit 1
fi
echo

# 3. Coverage check (exact threshold enforcement)
echo "3. Coverage check (${THRESHOLD}% threshold)..."
go test -coverprofile=c.out -run "$TEST_PATTERN" . >/dev/null 2>&1
# Calculate coverage for just the provider file
COVERAGE=$(go tool cover -func=c.out | grep "${PROVIDER}.go:" | awk '{sum+=$3; count++} END {if(count>0) printf "%.1f", sum/count; else print "0"}')
if [ -z "$COVERAGE" ] || [ "$COVERAGE" = "0" ]; then
    # Fallback to total coverage if provider-specific coverage fails
    COVERAGE=$(go tool cover -func=c.out | grep total | awk '{print $3}' | sed 's/%//')
fi

# Use bc for floating point comparison
if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
    echo "❌ FAIL: Coverage ${COVERAGE}% < ${THRESHOLD}%"
    echo "   Spec requires EXACTLY ${THRESHOLD}%+ coverage"
    echo "   No exceptions. Fix code or add tests."
    exit 1
fi
echo "✅ PASS: Coverage ${COVERAGE}% >= ${THRESHOLD}%"
echo

# 4. Race detector
echo "4. Race detector check..."
go test -race -run "$TEST_PATTERN" . >/dev/null 2>&1 || { echo "❌ FAIL: Race conditions detected"; exit 1; }
echo "✅ PASS: No race conditions"
echo

# 5. Go vet
echo "5. Static analysis (go vet)..."
go vet . || { echo "❌ FAIL: go vet found issues"; exit 1; }
echo "✅ PASS: go vet clean"
echo

# 6. Formatting
echo "6. Formatting check..."
UNFORMATTED=$(gofmt -l ${PROVIDER}.go ${PROVIDER}_test.go)
if [ -n "$UNFORMATTED" ]; then
    echo "❌ FAIL: Files need formatting:"
    echo "$UNFORMATTED"
    exit 1
fi
echo "✅ PASS: All files formatted"
echo

# 7. Interface implementation check
echo "7. Provider interface check..."
# Check that provider implements Provider interface
if ! grep -q "func.*ValidateEndpoints" ${PROVIDER}.go; then
    echo "❌ FAIL: ValidateEndpoints method not found"
    exit 1
fi
if ! grep -q "func.*ListModels" ${PROVIDER}.go; then
    echo "❌ FAIL: ListModels method not found"
    exit 1
fi
if ! grep -q "func.*GetCapabilities" ${PROVIDER}.go; then
    echo "❌ FAIL: GetCapabilities method not found"
    exit 1
fi
echo "✅ PASS: Provider interface implemented"
echo

echo "==================================="
echo "✅ ALL GATES PASSED"
echo "==================================="
echo "Provider: $PROVIDER"
echo "Coverage: ${COVERAGE}%"
echo "Status: PRODUCTION-READY"
