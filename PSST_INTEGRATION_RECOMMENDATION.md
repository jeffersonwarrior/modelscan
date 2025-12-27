# psst Integration Recommendation

**Date:** 2025-12-27  
**Context:** After removing API keys from git history  
**Tool:** [psst](https://github.com/Michaelliv/psst) - AI-native secrets manager

---

## The Problem We Just Fixed

We accidentally committed `.env.tier2` containing 5 real API keys:
- OpenAI (sk-proj-*)
- Anthropic (sk-ant-api03-*)
- Google (AIzaSy*)
- Cerebras (csk-*)
- xAI (xai-*)

Required `git-filter-repo` to rewrite entire history (42 commits).

**Root cause:** No secure secrets management workflow for AI agents.

---

## What is psst?

**psst** is an AI-native secrets manager that lets agents **use** secrets without **seeing** them.

### Core Concept
```bash
# Agent writes:
psst OPENAI_API_KEY -- go test ./providers

# What happens:
# 1. psst retrieves encrypted OPENAI_API_KEY from vault
# 2. Injects it into subprocess environment
# 3. Runs: go test ./providers (with $OPENAI_API_KEY set)
# 4. Returns exit code to agent
# 5. Automatically redacts secret from output

# What agent sees:
# ✅ ok github.com/jeffersonwarrior/modelscan/providers 18.587s

# What agent DOESN'T see:
# sk-proj-HPZ_Y3YSG1pS... (the actual key)
```

**The secret never enters the agent's context.**

---

## Security Model

1. **Encrypted at Rest**
   - AES-256-GCM encryption
   - Vault stored in `~/.psst/` or project `.psst/`

2. **OS Keychain for Encryption Key**
   - macOS: Keychain.app
   - Linux: libsecret/gnome-keyring
   - Windows: Credential Manager
   - Unlocks automatically when you log in

3. **Automatic Redaction**
   - Secrets replaced with `[REDACTED]` in command output
   - Use `--no-mask` flag for debugging if needed

4. **Local-First**
   - No cloud sync
   - No accounts
   - Secrets never leave your machine

---

## Integration with modelscan

### Current Workflow (UNSAFE)
```bash
# Developer creates .env.tier2
echo "OPENAI_API_KEY=sk-proj-..." > .env.tier2

# Agent reads it
cat .env.tier2

# Keys now in:
# - Agent context window
# - Terminal history
# - Git history (if committed)
# - Screenshot material
```

### Proposed Workflow with psst (SAFE)
```bash
# One-time setup
npm install -g @pssst/cli
psst init
psst set OPENAI_API_KEY     # Interactive prompt, value hidden
psst set ANTHROPIC_API_KEY
psst set GOOGLE_API_KEY
psst set CEREBRAS_API_KEY
psst set XAI_API_KEY

# Or import existing .env
psst import .env.tier2
rm .env.tier2  # Delete plaintext file

# Onboard agent (adds instructions to CLAUDE.md)
psst onboard

# Agent uses secrets
psst OPENAI_API_KEY -- go test ./providers/openai_test.go
psst ANTHROPIC_API_KEY OPENAI_API_KEY -- go test ./providers
psst CEREBRAS_API_KEY -- ./scripts/validate-provider.sh cerebras
```

---

## Benefits for modelscan

### 1. Zero Git Exposure
- No `.env` files to accidentally commit
- No secrets in git history
- No need for `git-filter-repo` cleanup

### 2. Zero Agent Context Exposure
- Agent orchestrates commands but never sees keys
- Keys not in terminal history
- Keys not in screenshots/logs

### 3. Developer Experience
```bash
# List what's available
psst list

# Easy rotation
psst set OPENAI_API_KEY  # Updates existing

# Project-local vaults
cd /different/project
psst init --local  # Creates .psst/ in project
```

### 4. CI/CD Support
```bash
# In GitHub Actions
export PSST_PASSWORD="vault-password"
psst OPENAI_API_KEY -- go test ./...
```

### 5. Multi-Provider Testing
```bash
# Test all providers with secrets managed by psst
psst OPENAI_API_KEY ANTHROPIC_API_KEY GOOGLE_API_KEY \
     CEREBRAS_API_KEY XAI_API_KEY MISTRAL_API_KEY \
     -- go test ./providers -v
```

---

## Recommended Implementation Steps

### Phase 1: Setup (5 minutes)
```bash
# Install
npm install -g @pssst/cli

# Initialize vault
psst init

# Import existing secrets
psst import .env.tier2

# Verify
psst list

# Delete plaintext file
rm .env.tier2
```

### Phase 2: Agent Onboarding (1 minute)
```bash
# Add psst instructions to CLAUDE.md
psst onboard

# This teaches your agent:
# - How to use psst SECRET -- command
# - To ask you to add missing secrets
# - To shame you for pasting secrets in plaintext 🤫
```

### Phase 3: Update Scripts (10 minutes)
Replace all environment variable reads with psst commands:

**Before:**
```bash
export OPENAI_API_KEY="sk-proj-..."
go test ./providers
```

**After:**
```bash
psst OPENAI_API_KEY -- go test ./providers
```

**Before (test-all-sdks.sh):**
```bash
source .env.tier2
./scripts/validate-provider.sh openai
```

**After:**
```bash
psst OPENAI_API_KEY ANTHROPIC_API_KEY GOOGLE_API_KEY \
  -- ./scripts/validate-provider.sh openai
```

### Phase 4: Update .gitignore (1 minute)
```bash
# Add to .gitignore
.psst/        # Project-local vaults
~/.psst/      # Global vault (shouldn't be in project anyway)
```

### Phase 5: Documentation (5 minutes)
Add to README.md:

```markdown
## Secrets Management

This project uses [psst](https://github.com/Michaelliv/psst) for secure secrets management.

### Setup
\`\`\`bash
npm install -g @pssst/cli
psst init
\`\`\`

### Add API Keys
\`\`\`bash
psst set OPENAI_API_KEY
psst set ANTHROPIC_API_KEY
psst set GOOGLE_API_KEY
psst set CEREBRAS_API_KEY
psst set XAI_API_KEY
\`\`\`

### Run Tests
\`\`\`bash
psst OPENAI_API_KEY -- go test ./providers/openai_test.go
\`\`\`
```

---

## Migration Path

### Option 1: Immediate (Recommended)
1. Install psst
2. Import .env.tier2
3. Delete .env.tier2
4. Update CLAUDE.md with psst onboarding
5. Start using immediately

### Option 2: Gradual
1. Install psst
2. Import .env.tier2
3. Keep .env.tier2 as fallback (but .gitignored)
4. Migrate scripts one at a time
5. Delete .env.tier2 when confident

---

## Comparison: psst vs Alternatives

### vs .env files
| Feature | .env | psst |
|---------|------|------|
| Agent sees secrets | ✅ Yes (bad) | ❌ No (good) |
| Git exposure risk | ⚠️ High | ✅ None |
| Encrypted at rest | ❌ No | ✅ Yes |
| Auto-redaction | ❌ No | ✅ Yes |
| Setup complexity | Low | Low |

### vs Environment Variables
| Feature | export ENV=key | psst |
|---------|----------------|------|
| Shell history | ✅ Exposed | ✅ Hidden |
| Agent context | ✅ Exposed | ✅ Hidden |
| Encrypted | ❌ No | ✅ Yes |
| Rotation | Manual | Easy |

### vs HashiCorp Vault
| Feature | Vault | psst |
|---------|-------|------|
| Target use case | Infrastructure/teams | Local dev/AI agents |
| Setup complexity | High | Low |
| Cloud dependency | Optional | None |
| Agent integration | Manual | Built-in |
| Cost | Free tier limits | Free |

---

## Security Considerations

### What psst Protects Against
✅ Accidental git commits  
✅ Agent context exposure  
✅ Terminal history leaks  
✅ Screenshot/log exposure  
✅ .env file commits  

### What psst Doesn't Protect Against
⚠️ Agent with shell access running `psst get SECRET`  
⚠️ Malicious code running on your machine  
⚠️ Keychain compromise (OS-level attack)  
⚠️ Someone with physical access to unlocked machine  

**Mitigation:** These are fundamental trade-offs of local development. If you trust your agent with shell access, you trust it not to exfiltrate secrets maliciously.

---

## Cost-Benefit Analysis

### Costs
- **Time:** ~20 minutes initial setup
- **Dependency:** Node.js/Bun runtime for psst
- **Learning:** New command syntax for agents

### Benefits
- **Security:** Zero git/context exposure
- **Compliance:** Encrypted secrets at rest
- **DX:** Easy rotation, no manual .env editing
- **Peace of Mind:** No more `git-filter-repo` emergencies

**ROI:** Prevents one accidental key commit = worth it

---

## Recommendation

**✅ ADOPT psst for modelscan**

**Rationale:**
1. We just spent significant effort removing keys from git history
2. psst solves the root cause (no secure agent workflow)
3. Low friction (works like environment variables)
4. AI-native design (built for this exact use case)
5. Active development (v0.1.3 released yesterday)
6. MIT license (compatible with modelscan)

**Next Steps:**
1. Install: `npm install -g @pssst/cli`
2. Setup: `psst init && psst import .env.tier2`
3. Onboard: `psst onboard` (updates CLAUDE.md)
4. Commit: Add psst instructions to CONTRIBUTING.md

---

## Questions for Discussion

1. **Global vs Project-Local Vault?**
   - Global: `~/.psst/` (shared across projects)
   - Local: `.psst/` in project root (per-project secrets)
   - Recommendation: Start with global, move to local if needed

2. **CI/CD Integration?**
   - Use `PSST_PASSWORD` env var in GitHub Actions?
   - Or stick with GitHub Secrets for CI?
   - Recommendation: GitHub Secrets for CI, psst for local dev

3. **Key Rotation Policy?**
   - How often to rotate provider API keys?
   - Recommendation: Quarterly + after any exposure

---

## References

- **GitHub:** https://github.com/Michaelliv/psst
- **Latest Release:** v0.1.3 (Dec 26, 2025)
- **License:** MIT
- **Stars:** 23
- **Language:** TypeScript (Bun)

---

**Conclusion:** psst is the right tool for this problem. It's purpose-built for AI agent workflows and prevents the exact issue we just fixed.
