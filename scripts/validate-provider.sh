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
    go build ./${PROVIDER}.go || { echo "❌ FAIL: Build failed"; exit 1; }
    echo "✅ PASS: Build succeeds"
else
    echo "❌ FAIL: ${PROVIDER}.go not found"
    exit 1
fi
echo

# 2. Test check
echo "2. Test check..."
if [ -f "${PROVIDER}_test.go" ]; then
    go test -v ./${PROVIDER}_test.go || { echo "❌ FAIL: Tests failed"; exit 1; }
    echo "✅ PASS: All tests pass"
else
    echo "❌ FAIL: ${PROVIDER}_test.go not found"
    exit 1
fi
echo

# 3. Coverage check (exact threshold enforcement)
echo "3. Coverage check (${THRESHOLD}% threshold)..."
go test -coverprofile=c.out ./${PROVIDER}.go ./${PROVIDER}_test.go >/dev/null 2>&1
COVERAGE=$(go tool cover -func=c.out | grep total | awk '{print $3}' | sed 's/%//')

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
go test -race ./${PROVIDER}_test.go >/dev/null 2>&1 || { echo "❌ FAIL: Race conditions detected"; exit 1; }
echo "✅ PASS: No race conditions"
echo

# 5. Go vet
echo "5. Static analysis (go vet)..."
go vet ./${PROVIDER}.go || { echo "❌ FAIL: go vet found issues"; exit 1; }
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
