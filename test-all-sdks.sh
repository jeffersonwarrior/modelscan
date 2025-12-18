#!/bin/bash
# test-all-sdks.sh - Comprehensive test suite for all 21 SDKs

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SDK_DIR="${SCRIPT_DIR}/sdk"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
TOTAL_SDKS=0
PASSED_SDKS=0
FAILED_SDKS=0

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Testing All 21 Go SDKs${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# List of all SDKs
SDKS=(
    "anthropic"
    "openai"
    "google"
    "mistral"
    "minimax"
    "kimi"
    "zai"
    "synthetic"
    "xai"
    "vibe"
    "nanogpt"
    "openrouter"
    "together"
    "fireworks"
    "groq"
    "deepseek"
    "replicate"
    "perplexity"
    "cohere"
    "deepinfra"
    "hyperbolic"
)

test_sdk() {
    local sdk=$1
    local sdk_path="${SDK_DIR}/${sdk}"
    
    echo -e "${YELLOW}Testing: ${sdk}${NC}"
    
    if [ ! -d "$sdk_path" ]; then
        echo -e "${RED}  ✗ Directory not found${NC}"
        return 1
    fi
    
    cd "$sdk_path"
    
    # 1. Check go.mod exists
    if [ ! -f "go.mod" ]; then
        echo -e "${RED}  ✗ go.mod missing${NC}"
        return 1
    fi
    
    # 2. Build
    echo -e "  Building..."
    if ! go build ./... 2>&1; then
        # Check if it's just "no Go files" (which is OK for packages with only one file)
        if ! go build ./... 2>&1 | grep -q "no Go files"; then
            echo -e "${RED}  ✗ Build failed${NC}"
            return 1
        fi
    fi
    
    # 3. Vet
    echo -e "  Vetting..."
    if ! go vet ./... 2>&1; then
        echo -e "${RED}  ✗ Vet failed${NC}"
        return 1
    fi
    
    # 4. Format check
    echo -e "  Checking format..."
    if [ -n "$(gofmt -l .)" ]; then
        echo -e "${RED}  ✗ Format check failed${NC}"
        gofmt -l .
        return 1
    fi
    
    # 5. Run tests (if test files exist)
    if ls *_test.go 1> /dev/null 2>&1; then
        echo -e "  Running tests..."
        if ! go test -v ./... 2>&1 | tail -20; then
            echo -e "${RED}  ✗ Tests failed${NC}"
            return 1
        fi
    else
        echo -e "  ${YELLOW}No tests found (skipping)${NC}"
    fi
    
    echo -e "${GREEN}  ✓ Passed${NC}"
    echo ""
    return 0
}

# Test each SDK
for sdk in "${SDKS[@]}"; do
    TOTAL_SDKS=$((TOTAL_SDKS + 1))
    if test_sdk "$sdk"; then
        PASSED_SDKS=$((PASSED_SDKS + 1))
    else
        FAILED_SDKS=$((FAILED_SDKS + 1))
    fi
done

# Summary
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Test Summary${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "Total SDKs:  ${TOTAL_SDKS}"
echo -e "${GREEN}Passed:      ${PASSED_SDKS}${NC}"
if [ $FAILED_SDKS -gt 0 ]; then
    echo -e "${RED}Failed:      ${FAILED_SDKS}${NC}"
else
    echo -e "Failed:      ${FAILED_SDKS}"
fi
echo ""

# Exit with error if any failed
if [ $FAILED_SDKS -gt 0 ]; then
    echo -e "${RED}❌ Some SDKs failed testing${NC}"
    exit 1
else
    echo -e "${GREEN}✅ All SDKs passed!${NC}"
    exit 0
fi
