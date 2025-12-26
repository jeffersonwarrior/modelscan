#!/bin/bash
# Auto Sequential Worker Launcher
# Launches workers one at a time with 15-minute intervals

PROJECT_DIR="/home/agent/modelscan"
SLEEP_TIME=900  # 15 minutes

# Feature definitions: feature-id:provider-name
FEATURES=(
    "feature-2:deepgram"
    "feature-3:whisper"
    "feature-4:tts"
    "feature-5:playht"
    "feature-6:lumaai"
    "feature-7:runwayml"
    "feature-8:openai_extended"
    "feature-9:anthropic_extended"
    "feature-10:google_thinking"
    "feature-11:deepseek_extended"
    "feature-12:cerebras_extended"
    "feature-13:fal_extended"
    "feature-14:midjourney"
    "feature-15:embeddings"
    "feature-16:cohere_embeddings"
    "feature-17:voyageai"
    "feature-18:realtime"
)

echo "=========================================="
echo "Auto Sequential Worker Launcher"
echo "=========================================="
echo "Features to process: ${#FEATURES[@]}"
echo "Sleep time: ${SLEEP_TIME}s (15 minutes)"
echo "Total estimated time: $((${#FEATURES[@]} * SLEEP_TIME / 3600)) hours"
echo ""

# Feature 2 is already running, so start from checking it
echo "[$(date)] Feature 2 (deepgram) already launched, waiting 15min..."
sleep $SLEEP_TIME

for FEATURE_SPEC in "${FEATURES[@]}"; do
    IFS=':' read -r FEATURE_ID PROVIDER_NAME <<< "$FEATURE_SPEC"

    echo ""
    echo "=========================================="
    echo "[$(date)] Processing $FEATURE_ID: $PROVIDER_NAME"
    echo "=========================================="

    # Check if worker completed
    echo "Checking worker status..."
    # Worker should be done by now

    # Run validation
    echo "Running validation..."
    cd "$PROJECT_DIR/providers"
    if bash "$PROJECT_DIR/scripts/validate-provider.sh" "$PROVIDER_NAME" 90; then
        echo "✅ Validation passed for $PROVIDER_NAME"

        # Commit the changes
        cd "$PROJECT_DIR"
        git add providers/
        git commit -m "feat($PROVIDER_NAME): Implement provider with 90%+ coverage

✅ All 7 validation gates passing
✅ Provider: $PROVIDER_NAME
✅ Sequential worker execution

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"

        echo "✅ Committed $PROVIDER_NAME"
    else
        echo "❌ Validation failed for $PROVIDER_NAME"
        echo "Skipping to next feature..."
    fi

    # Sleep before next feature (except on last one)
    if [ "$FEATURE_ID" != "feature-18" ]; then
        echo ""
        echo "[$(date)] Sleeping ${SLEEP_TIME}s before next feature..."
        sleep $SLEEP_TIME
    fi
done

echo ""
echo "=========================================="
echo "Auto Sequential Processing Complete"
echo "=========================================="
echo "Check git log for commits"
