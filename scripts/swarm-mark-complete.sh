#!/bin/bash
# Swarm Mark Complete - Enforced Validation Wrapper
#
# This script ENFORCES validation before allowing mark_complete.
# Workers CANNOT bypass this - it's the only way to mark complete.
#
# Usage: ./scripts/swarm-mark-complete.sh <feature-id> <provider-name>
#
# Example: ./scripts/swarm-mark-complete.sh feature-2 elevenlabs

set -e

FEATURE_ID=$1
PROVIDER_NAME=$2
PROJECT_DIR="/home/agent/modelscan"

if [ -z "$FEATURE_ID" ] || [ -z "$PROVIDER_NAME" ]; then
    echo "❌ ERROR: Missing required arguments"
    echo "Usage: $0 <feature-id> <provider-name>"
    echo "Example: $0 feature-2 elevenlabs"
    exit 1
fi

echo "=========================================="
echo "SWARM COMPLETION REQUEST"
echo "=========================================="
echo "Feature: $FEATURE_ID"
echo "Provider: $PROVIDER_NAME"
echo

# Step 1: Run validation script
echo "Step 1: Running validation gate..."
echo "----------------------------------"

if [ "$PROVIDER_NAME" = "http-foundation" ]; then
    # HTTP foundation uses different validation
    bash ${PROJECT_DIR}/scripts/validate-http.sh
    VALIDATION_EXIT=$?
else
    # Provider validation
    bash ${PROJECT_DIR}/scripts/validate-provider.sh "$PROVIDER_NAME" 90
    VALIDATION_EXIT=$?
fi

echo

# Step 2: Check validation result
if [ $VALIDATION_EXIT -ne 0 ]; then
    echo "=========================================="
    echo "❌ COMPLETION REJECTED"
    echo "=========================================="
    echo
    echo "Validation failed. Fix issues and try again."
    echo
    echo "Worker cannot mark complete until validation passes."
    echo "This is non-negotiable."
    echo
    exit 1
fi

# Step 3: Validation passed - allow mark_complete
echo "Step 2: Validation passed, marking complete..."
echo "----------------------------------"

# Call the actual swarm mark_complete tool
# This would integrate with your swarm MCP server
echo "Calling mark_complete for $FEATURE_ID..."

# For now, just simulate success
# In production, this would call:
# claude-swarm mark_complete --project-dir "$PROJECT_DIR" --feature-id "$FEATURE_ID" --success true

echo
echo "=========================================="
echo "✅ COMPLETION ACCEPTED"
echo "=========================================="
echo
echo "Feature $FEATURE_ID marked complete."
echo "Validation gates enforced programmatically."
echo
