#!/bin/bash
set -e
cd '/home/agent/modelscan'
PROMPT=$(cat '/home/agent/modelscan/.claude/orchestrator/workers/feature-8.prompt')
# Worker: allowed tools and MCP servers configured via env vars
claude --model claude-sonnet-4-5 --allowedTools "Bash,Read,Write,Edit,Glob,Grep,Task,TodoWrite" --permission-mode default --mcp-config '{"mcpServers":{}}' -p "$PROMPT" 2>&1 | tee '/home/agent/modelscan/.claude/orchestrator/workers/feature-8.log'
echo 'WORKER_EXITED' >> '/home/agent/modelscan/.claude/orchestrator/workers/feature-8.log'
