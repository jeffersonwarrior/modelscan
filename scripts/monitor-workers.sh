#!/bin/bash
# Autonomous Worker Monitor
#
# Monitors all active workers and enforces validation gates.
# Runs independently of swarm's built-in monitoring.
#
# Usage: ./scripts/monitor-workers.sh

PROJECT_DIR="/home/agent/modelscan"
CHECK_INTERVAL=60  # seconds
FREEZE_TIMEOUT=300 # 5 minutes of no git activity = frozen

# Track last git commit time for each worker
declare -A LAST_COMMIT_TIME

monitor_worker() {
    local feature_id=$1
    local provider_name=$2

    echo "[$feature_id] Checking $provider_name..."

    # Check if provider files exist
    if [ ! -f "${PROJECT_DIR}/providers/${provider_name}.go" ]; then
        echo "  ⏳ No implementation file yet"
        return
    fi

    # Check last git commit time for this provider
    LAST_COMMIT=$(git log -1 --format=%ct --all -- "providers/${provider_name}*" 2>/dev/null || echo 0)
    CURRENT_TIME=$(date +%s)

    # Initialize tracking if first time
    if [ -z "${LAST_COMMIT_TIME[$feature_id]}" ]; then
        LAST_COMMIT_TIME[$feature_id]=$LAST_COMMIT
    fi

    # Check for freeze (no commits in FREEZE_TIMEOUT seconds)
    TIME_SINCE_COMMIT=$((CURRENT_TIME - LAST_COMMIT))

    if [ $TIME_SINCE_COMMIT -gt $FREEZE_TIMEOUT ] && [ $LAST_COMMIT -gt 0 ]; then
        echo "  ⚠️  FROZEN: No activity for ${TIME_SINCE_COMMIT}s"
        echo "  💡 Consider prompting worker or restarting"
    elif [ $LAST_COMMIT -gt ${LAST_COMMIT_TIME[$feature_id]} ]; then
        echo "  ✅ Active: New commit detected"
        LAST_COMMIT_TIME[$feature_id]=$LAST_COMMIT
    else
        echo "  🔄 Working: Last activity ${TIME_SINCE_COMMIT}s ago"
    fi

    # Check if tests exist and pass
    if [ -f "${PROJECT_DIR}/providers/${provider_name}_test.go" ]; then
        cd "${PROJECT_DIR}/providers"

        # Quick test check (don't run full validation yet)
        if go test -run "Test" ./${provider_name}_test.go >/dev/null 2>&1; then
            TEST_COUNT=$(go test -v ./${provider_name}_test.go 2>&1 | grep -c "PASS:")
            echo "  🧪 Tests: $TEST_COUNT passing"

            # Quick coverage check
            COVERAGE=$(go test -coverprofile=c.out ./${provider_name}.go ./${provider_name}_test.go 2>/dev/null && \
                       go tool cover -func=c.out 2>/dev/null | grep total | awk '{print $3}' || echo "0%")
            echo "  📊 Coverage: $COVERAGE"

            # If coverage >= 90%, suggest validation
            COVERAGE_NUM=$(echo $COVERAGE | sed 's/%//')
            if (( $(echo "$COVERAGE_NUM >= 90.0" | bc -l 2>/dev/null || echo 0) )); then
                echo "  🎯 READY FOR VALIDATION!"
                echo "     Worker should run: bash scripts/validate-provider.sh $provider_name 90"
            fi
        else
            echo "  ❌ Tests: Some failing"
        fi

        cd - >/dev/null
    fi

    echo
}

main_loop() {
    echo "=========================================="
    echo "Worker Monitor - Autonomous Validation"
    echo "=========================================="
    echo "Project: $PROJECT_DIR"
    echo "Check interval: ${CHECK_INTERVAL}s"
    echo "Freeze timeout: ${FREEZE_TIMEOUT}s"
    echo

    while true; do
        echo "=== $(date '+%Y-%m-%d %H:%M:%S') ==="
        echo

        # Monitor each active worker
        # Read from orchestrator state if available
        if [ -f "${PROJECT_DIR}/.claude/orchestrator/state.json" ]; then
            # Parse feature list from state.json
            # For now, hardcode known workers
            # In production, parse JSON programmatically

            monitor_worker "feature-2" "elevenlabs"
            monitor_worker "feature-3" "deepgram"
            monitor_worker "feature-4" "whisper"
            # ... add all 18 workers
        else
            echo "No orchestrator state found."
            echo "Looking for provider files directly..."
            echo

            # Scan providers directory for any *_test.go files
            for test_file in ${PROJECT_DIR}/providers/*_test.go; do
                if [ -f "$test_file" ]; then
                    provider=$(basename "$test_file" _test.go)
                    monitor_worker "auto-detect" "$provider"
                fi
            done
        fi

        echo "Next check in ${CHECK_INTERVAL}s..."
        echo "Press Ctrl+C to stop monitoring"
        echo

        sleep $CHECK_INTERVAL
    done
}

# Handle Ctrl+C gracefully
trap 'echo ""; echo "Monitoring stopped."; exit 0' INT

# Start monitoring
main_loop
