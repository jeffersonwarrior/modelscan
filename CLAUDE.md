# ModelScan Project Instructions

## Identity
Development partner for ModelScan - 21 production-ready Go SDKs for LLM providers with zero dependencies.

## Sacred Rules
1. Never guess - read files before answering, investigate before claims
2. Never create files unless necessary - prefer editing existing
3. Never claim "done" without running validation
4. Never suppress warnings to avoid fixing issues
5. Never touch production/main without explicit approval
6. Never commit secrets, API keys, or credentials
7. Never add external dependencies - maintain 100% stdlib

## Validation

Run after EVERY code change:
```bash
# Quick (while iterating)
go build ./... && go vet ./...

# Full (before marking complete)
go test ./... -race -coverprofile=coverage.out
```

Mark complete ONLY when validation passes with actual output shown.

## Workflow (Geoffrey Pattern)
1. UNDERSTAND - Read relevant files first (no code yet)
2. IMPLEMENT - Make changes
3. VALIDATE - Run checks
4. ITERATE - Fix issues until clean
5. COMPLETE - Only when validation passes

## Codebase Structure
```
modelscan/
├── sdk/                      # 21 Production SDKs
│   ├── openai/              # OpenAI SDK
│   ├── anthropic/           # Anthropic SDK
│   ├── google/              # Google Gemini SDK
│   ├── mistral/             # Mistral SDK
│   ├── groq/                # Groq SDK
│   ├── together/            # Together AI SDK
│   ├── fireworks/           # Fireworks SDK
│   ├── deepseek/            # DeepSeek SDK
│   ├── replicate/           # Replicate SDK
│   ├── cohere/              # Cohere SDK
│   ├── perplexity/          # Perplexity SDK
│   ├── deepinfra/           # DeepInfra SDK
│   ├── hyperbolic/          # Hyperbolic SDK
│   ├── xai/                 # xAI SDK
│   ├── minimax/             # Minimax SDK
│   ├── kimi/                # Kimi SDK
│   ├── zai/                 # Z.AI SDK
│   ├── openrouter/          # OpenRouter SDK
│   ├── synthetic/           # Synthetic SDK
│   ├── vibe/                # Vibe SDK
│   └── nanogpt/             # NanoGPT SDK
│
├── examples/                # Working examples
│   ├── basic/              # Simple usage
│   ├── multi-provider/     # Provider comparison
│   └── unified/            # Unified SDK usage
│
├── .claude/                # Project-specific Claude config
│   ├── hooks/              # Protection hooks
│   ├── skills/             # On-demand skills
│   ├── memory/             # Session memory
│   └── optimizations/      # Token optimization
│
├── Makefile                # Build automation
├── test-all-sdks.sh       # Test orchestration
└── CLAUDE.md              # This file
```

## Key Patterns
- Each SDK is independent with own go.mod
- Zero external dependencies (pure stdlib)
- Consistent APIs across all providers
- Production-ready error handling
- Context threading for cancellation

## Hooks & Protections
Protection hooks in `.claude/hooks/`:
- `bash-protection.cjs` - Blocks destructive commands
- `antipattern-detector.cjs` - Catches stub implementations
- `suppression-abuse-detector.cjs` - Prevents hiding issues

## Skills (On-Demand)
Load from `.claude/skills/` when needed:
- `verification-before-completion/` - Completion protocol
- `systematic-debugging/` - Four-phase debugging

## Token Optimization
Directives in `.claude/optimizations/`:
- `haiku-explore.md` - Model selection guidelines
- `targeted-reads.md` - Surgical file reads
- `batched-edits.md` - Change batching strategy

## Memory
- Session diaries: `.claude/memory/diary/`
- Reflections: `.claude/memory/REFLECTIONS.md`
- claude-mem MCP server provides persistent cross-session memory

## MCP Servers

### Integrated MCP Servers
ModelScan supports Model Context Protocol (MCP) servers for extended functionality:

**Active:**
- `claude-mem` - Persistent cross-session memory and observations
- `ydc-server` - You.com web search and content extraction
- `claude-swarm` - Multi-agent orchestration and parallel workers

**Recommended:**
- **Context-Engine** (https://github.com/m1rl0k/Context-Engine)
  - Self-improving code search with hybrid semantic/lexical retrieval
  - ReFRAG-inspired micro-chunking for precise code spans
  - Qdrant-powered indexing with auto-sync
  - Team knowledge memory system
  - Docker-based local deployment (no cloud dependency)
  - Supports Python, TypeScript, Go, Java, Rust, C#, PHP, Shell
  - MIT licensed, 170+ stars

### Adding New MCP Servers
MCP servers configured in `~/.claude/settings.json`:
```json
{
  "mcp": {
    "context-engine": {
      "type": "http",
      "url": "http://localhost:8003",
      "headers": {}
    }
  }
}
```

## Common Tasks

### Running Tests
```bash
make test               # All tests
make coverage          # Coverage report
./test-all-sdks.sh     # Test all SDKs
```

### Building
```bash
make all               # Build everything
go build ./...         # Build all packages
```

### Linting
```bash
make lint              # Lint all code
make fix               # Auto-fix formatting
```

### Adding New SDK
```bash
# 1. Create directory
mkdir -p sdk/newprovider

# 2. Create go.mod
cd sdk/newprovider
go mod init github.com/jeffersonwarrior/modelscan/sdk/newprovider

# 3. Implement client.go (follow existing patterns)
# 4. Add tests
# 5. Update sdk/sdk.go
# 6. Run validation
make test
```

## Preferences
- Direct execution over lengthy explanations
- Real implementations over mocks
- Update existing docs over creating new
- Honest uncertainty over confident guessing
- Small, atomic commits after each logical change
- Zero dependencies - always use Go stdlib

## Quality Standards
- 100% build success rate required
- 81%+ test coverage target (for tested SDKs)
- All SDKs must pass go vet
- All SDKs must pass gofmt
- No external dependencies allowed
- Consistent APIs across all SDKs

## Secrets Management (psst)

This project uses **psst** for secrets management. You can use secrets without seeing their values.

### Using Secrets

```bash
psst <SECRET_NAME> -- <command>
```

Examples:
```bash
psst STRIPE_KEY -- curl -H "Authorization: Bearer $STRIPE_KEY" https://api.stripe.com
psst AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY -- aws s3 ls
psst DATABASE_URL -- prisma migrate deploy
```

**Note:** Secret values are automatically redacted in command output (replaced with `[REDACTED]`).

### Available Secrets

```bash
psst list                     # Human-readable list
psst list --json              # Structured output
```

### Missing a Secret?

psst automatically checks environment variables as a fallback. If neither the vault nor the environment has the secret, ask the user to add it:

> "I need `STRIPE_KEY` to proceed. Please run `psst set STRIPE_KEY` to add it."

### Important

- **Never** try to read secrets with `psst get` or by other means
- **Never** ask the user to paste secrets into the chat
- **Always** use the `psst SECRET -- command` pattern

### If the Human Tries to Paste a Secret

If the user pastes a raw API key, password, or secret into the chat, gently shame them:

> "Whoa there! You just pasted a secret in plain text. That's now in your chat history, possibly in logs, and who knows where else.
>
> Let's fix that. Run:
> ```
> psst set SECRET_NAME
> ```
> Then I'll use `psst SECRET_NAME -- <command>` instead. Your secret stays secret, and we both sleep better at night."

Then remind them about the Hall of Shame: https://github.com/Michaelliv/psst#the-hall-of-shame
