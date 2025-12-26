#!/bin/bash
# HTTP Foundation Validation Gate
# Must pass before Feature 0 is considered complete

set -e

cd internal/http

echo "=== HTTP Foundation Validation Gate ==="
echo

# 1. Build check
echo "1. Build check..."
go build . || { echo "❌ FAIL: Build failed"; exit 1; }
echo "✅ PASS: Build succeeds"
echo

# 2. Test check
echo "2. Test check..."
go test -v . || { echo "❌ FAIL: Tests failed"; exit 1; }
echo "✅ PASS: All tests pass"
echo

# 3. Coverage check (93% threshold)
echo "3. Coverage check (93% threshold)..."
go test -coverprofile=c.out . >/dev/null 2>&1
COVERAGE=$(go tool cover -func=c.out | grep total | awk '{print $3}' | sed 's/%//')

# Use bc for floating point comparison
if (( $(echo "$COVERAGE < 93.0" | bc -l) )); then
    echo "❌ FAIL: Coverage ${COVERAGE}% < 93.0%"
    echo "   Spec requires exactly 93%+ coverage"
    exit 1
fi
echo "✅ PASS: Coverage ${COVERAGE}% >= 93.0%"
echo

# 4. Race detector
echo "4. Race detector check..."
go test -race . >/dev/null 2>&1 || { echo "❌ FAIL: Race conditions detected"; exit 1; }
echo "✅ PASS: No race conditions"
echo

# 5. Go vet
echo "5. Static analysis (go vet)..."
go vet . || { echo "❌ FAIL: go vet found issues"; exit 1; }
echo "✅ PASS: go vet clean"
echo

# 6. Formatting
echo "6. Formatting check..."
UNFORMATTED=$(gofmt -l *.go)
if [ -n "$UNFORMATTED" ]; then
    echo "❌ FAIL: Files need formatting:"
    echo "$UNFORMATTED"
    exit 1
fi
echo "✅ PASS: All files formatted"
echo

echo "==================================="
echo "✅ ALL GATES PASSED"
echo "==================================="
echo "Coverage: ${COVERAGE}%"
echo "HTTP Foundation is production-ready"
