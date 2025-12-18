#!/bin/bash
# fix-all-sdks.sh - Auto-fix formatting and common issues

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SDK_DIR="${SCRIPT_DIR}/sdk"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Auto-fixing All 21 Go SDKs${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

SDKS=(
    "anthropic" "openai" "google" "mistral" "minimax" "kimi" 
    "zai" "synthetic" "xai" "vibe" "nanogpt" "openrouter"
    "together" "fireworks" "groq" "deepseek" "replicate" 
    "perplexity" "cohere" "deepinfra" "hyperbolic"
)

for sdk in "${SDKS[@]}"; do
    sdk_path="${SDK_DIR}/${sdk}"
    
    if [ ! -d "$sdk_path" ]; then
        continue
    fi
    
    echo "Fixing: $sdk"
    cd "$sdk_path"
    
    # Format code
    gofmt -w .
    
    # Tidy dependencies
    go mod tidy
    
    echo -e "${GREEN}  ✓ Fixed${NC}"
done

echo ""
echo -e "${GREEN}✅ All SDKs formatted and dependencies tidied!${NC}"
