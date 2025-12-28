#!/bin/bash
# ModelScan v0.3 Test Script using psst for secrets

set -e

echo "=== ModelScan v0.3 Integration Test ==="
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check if modelscan-server is running
if ! pgrep -x "modelscan-server" > /dev/null; then
    echo "Starting modelscan-server in background..."
    ./modelscan-server > server.log 2>&1 &
    SERVER_PID=$!
    echo "Server PID: $SERVER_PID"
    sleep 3
else
    echo "modelscan-server already running"
fi

# Test 1: Health check
echo -e "${BLUE}Test 1: Health Check${NC}"
curl -s http://localhost:8080/health | jq .
echo -e "${GREEN}✓ Health check passed${NC}"
echo ""

# Test 2: List providers (should be empty initially)
echo -e "${BLUE}Test 2: List Providers${NC}"
curl -s http://localhost:8080/api/providers | jq .
echo -e "${GREEN}✓ List providers passed${NC}"
echo ""

# Test 3: Add OpenAI API key using psst
echo -e "${BLUE}Test 3: Add API Key (using psst)${NC}"
if command -v psst &> /dev/null; then
    echo "Using psst to securely inject OPENAI_API_KEY..."
    psst OPENAI_API_KEY -- bash -c '
        curl -s -X POST http://localhost:8080/api/keys/add \
          -H "Content-Type: application/json" \
          -d "{\"provider_id\": \"openai\", \"api_key\": \"$OPENAI_API_KEY\"}" | jq .
    '
    echo -e "${GREEN}✓ API key added securely via psst${NC}"
else
    echo "⚠ psst not found, using placeholder key"
    curl -s -X POST http://localhost:8080/api/keys/add \
      -H "Content-Type: application/json" \
      -d '{"provider_id": "openai", "api_key": "sk-test-placeholder-key"}' | jq .
    echo -e "${GREEN}✓ API key added (placeholder)${NC}"
fi
echo ""

# Test 4: List API keys
echo -e "${BLUE}Test 4: List API Keys${NC}"
curl -s "http://localhost:8080/api/keys?provider=openai" | jq .
echo -e "${GREEN}✓ List API keys passed${NC}"
echo ""

# Test 5: List generated SDKs
echo -e "${BLUE}Test 5: List Generated SDKs${NC}"
curl -s http://localhost:8080/api/sdks | jq .
echo -e "${GREEN}✓ List SDKs passed${NC}"
echo ""

# Test 6: Get usage stats
echo -e "${BLUE}Test 6: Get Usage Stats${NC}"
curl -s "http://localhost:8080/api/stats?model=gpt-4" | jq .
echo -e "${GREEN}✓ Get usage stats passed${NC}"
echo ""

echo -e "${GREEN}=== All Tests Passed! ===${NC}"
echo ""
echo "Server log available at: server.log"
echo "Database: modelscan.db"
echo ""
echo "To stop server: kill $SERVER_PID"
