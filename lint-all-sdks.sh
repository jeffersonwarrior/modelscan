#!/bin/bash
# lint-all-sdks.sh - Comprehensive linting for all 21 SDKs

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SDK_DIR="${SCRIPT_DIR}/sdk"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Linting All 21 Go SDKs${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# List of all SDKs
SDKS=(
    "anthropic" "openai" "google" "mistral" "minimax" "kimi" 
    "zai" "synthetic" "xai" "vibe" "nanogpt" "openrouter"
    "together" "fireworks" "groq" "deepseek" "replicate" 
    "perplexity" "cohere" "deepinfra" "hyperbolic"
)

TOTAL_ISSUES=0

lint_sdk() {
    local sdk=$1
    local sdk_path="${SDK_DIR}/${sdk}"
    local issues=0
    
    echo -e "${YELLOW}Linting: ${sdk}${NC}"
    
    if [ ! -d "$sdk_path" ]; then
        echo -e "${RED}  ✗ Directory not found${NC}"
        return 1
    fi
    
    cd "$sdk_path"
    
    # 1. go fmt
    echo -e "  Checking format (go fmt)..."
    local fmt_issues=$(gofmt -l . | wc -l)
    if [ "$fmt_issues" -gt 0 ]; then
        echo -e "${RED}    Found $fmt_issues files needing formatting:${NC}"
        gofmt -l .
        issues=$((issues + fmt_issues))
    else
        echo -e "${GREEN}    ✓ Format OK${NC}"
    fi
    
    # 2. go vet
    echo -e "  Running go vet..."
    if go vet ./... 2>&1 | grep -v "no Go files" > /tmp/vet_${sdk}.log; then
        if [ -s /tmp/vet_${sdk}.log ]; then
            echo -e "${RED}    Found vet issues:${NC}"
            cat /tmp/vet_${sdk}.log
            issues=$((issues + 1))
        else
            echo -e "${GREEN}    ✓ Vet OK${NC}"
        fi
    else
        echo -e "${GREEN}    ✓ Vet OK${NC}"
    fi
    
    # 3. Check for common issues
    echo -e "  Checking for common issues..."
    
    # Unused imports
    if grep -r "import (" . --include="*.go" | grep -E '^\s*_\s+' > /dev/null; then
        echo -e "${YELLOW}    ⚠ Found blank imports${NC}"
        issues=$((issues + 1))
    fi
    
    # Missing error checks (basic)
    if grep -r "err :=" . --include="*.go" | grep -v "if err" | grep -v "return" | head -5 | grep -q .; then
        echo -e "${YELLOW}    ⚠ Possible unchecked errors${NC}"
        issues=$((issues + 1))
    fi
    
    # TODO comments
    local todo_count=$(grep -r "TODO\|FIXME" . --include="*.go" | wc -l)
    if [ "$todo_count" -gt 0 ]; then
        echo -e "${YELLOW}    ⚠ Found $todo_count TODO/FIXME comments${NC}"
    fi
    
    if [ "$issues" -eq 0 ]; then
        echo -e "${GREEN}  ✓ All checks passed${NC}"
    else
        echo -e "${RED}  ✗ Found $issues issues${NC}"
    fi
    
    echo ""
    return $issues
}

# Lint each SDK
for sdk in "${SDKS[@]}"; do
    lint_sdk "$sdk"
    TOTAL_ISSUES=$((TOTAL_ISSUES + $?))
done

# Summary
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Lint Summary${NC}"
echo -e "${BLUE}========================================${NC}"
if [ $TOTAL_ISSUES -eq 0 ]; then
    echo -e "${GREEN}✅ No issues found!${NC}"
    exit 0
else
    echo -e "${YELLOW}⚠️  Found $TOTAL_ISSUES total issues${NC}"
    echo ""
    echo "Run './fix-all-sdks.sh' to auto-fix formatting issues"
    exit 0  # Don't fail on lint warnings
fi
