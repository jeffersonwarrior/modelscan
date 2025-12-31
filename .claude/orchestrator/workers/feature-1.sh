#!/bin/bash
set -e
cd '/home/agent/modelscan'
PROMPT=$(cat '/home/agent/modelscan/.claude/orchestrator/workers/feature-1.prompt')
claude -p "$PROMPT" --allowedTools Bash,Read,Write,Edit,Glob,Grep 2>&1 | tee '/home/agent/modelscan/.claude/orchestrator/workers/feature-1.log'
echo 'WORKER_EXITED' >> '/home/agent/modelscan/.claude/orchestrator/workers/feature-1.log'
