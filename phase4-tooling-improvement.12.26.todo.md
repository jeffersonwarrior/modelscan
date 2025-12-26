# Phase 4: Tooling Improvement Plan
**Date**: December 26, 2025
**Status**: Planning
**Goal**: Streamline core toolset from 20+47 aliases to 25 tools with ~30 intuitive aliases

---

## Current State Analysis

**Existing Toolset**: 20 core tools with 47 aliases (2.35 aliases/tool)

### Issues Identified

1. **Naming Inconsistency**
   - Mixed conventions: `web_fetch` vs `web-search`
   - Unclear prefixes: `multiedit` vs `smart_edit`

2. **Fetch Tool Confusion**
   - Three variants with overlapping purposes
   - `fetch`, `web_fetch`, `agentic_fetch` - unclear when to use each

3. **Edit Tool Redundancy**
   - `edit`, `multiedit`, `smart_edit` - should consolidate

4. **Missing Core Operations**
   - No `mv/rename` - file moving
   - No `rm/delete` - file deletion
   - No `cp/copy` - file copying
   - No `mkdir` - directory creation
   - No `diff` - file comparison
   - No `tree` - directory visualization

5. **Alias Bloat**
   - `fetch` has 8 aliases alone
   - Many redundant variations

6. **Git Operations Missing**
   - Currently require `bash git ...`
   - Should have native git wrapper

7. **Process Management Inconsistency**
   - `job_kill` and `job_output` aliased to `bash`
   - Should be first-class tools

---

## Target Toolset (25 Core Tools)

### File Operations (9 tools)

- [ ] `read` - Read file contents
  - Aliases: `cat`, `view`

- [ ] `write` - Create/overwrite files
  - Aliases: `create`

- [ ] `edit` - AI-assisted file editing (consolidate smart_edit here)
  - Aliases: `modify`, `update`

- [ ] `multiedit` - Batch file modifications
  - Aliases: none needed

- [ ] `mv` - Move/rename files
  - Aliases: `rename`, `move`

- [ ] `rm` - Delete files
  - Aliases: `delete`, `remove`

- [ ] `cp` - Copy files
  - Aliases: `copy`

- [ ] `mkdir` - Create directories
  - Aliases: none needed

- [ ] `diff` - Compare files
  - Aliases: `compare`

### Search & Discovery (5 tools)

- [ ] `ls` - List directory contents
  - Aliases: `dir`, `list`

- [ ] `glob` - Pattern-based file matching
  - Aliases: none needed

- [ ] `find` - Search for files
  - Aliases: `locate`

- [ ] `grep` - Search file contents
  - Aliases: `search`, `rg`

- [ ] `tree` - Directory tree visualization
  - Aliases: none needed

### Web & Network (3 tools)

- [ ] `fetch` - HTTP requests with smart routing
  - Aliases: `curl`, `wget`, `http`
  - Modes: `simple` (default), `smart` (routing), `research` (agent-based)

- [ ] `web_search` - Web search engine
  - Aliases: `search_web`, `websearch`

- [ ] `download` - File downloads
  - Aliases: `dl`, `get`

### Code Intelligence (3 tools)

- [ ] `sourcegraph` - Code search
  - Aliases: `code_search`, `sg`

- [ ] `lsp_diagnostics` - LSP diagnostics
  - Aliases: `diagnostics`, `lsp_diag`

- [ ] `lsp_references` - LSP references
  - Aliases: `references`, `lsp_refs`

### Execution & Process (3 tools)

- [ ] `bash` - Shell command execution
  - Aliases: `shell`, `exec`, `run`

- [ ] `kill_job` - Terminate background processes
  - Aliases: `kill`, `stop_job`

- [ ] `get_job_output` - Retrieve job output
  - Aliases: `job_output`, `get_output`

### Version Control (2 tools)

- [ ] `git` - Git operations wrapper
  - Sub-commands: `commit`, `status`, `diff`, `log`, `push`, `pull`
  - Aliases: none needed

- [ ] `git_interactive` - Interactive git operations
  - Aliases: `git_i`

---

## Implementation Plan

### Phase 4.1: Core File Operations
**Priority**: High
**Estimated Effort**: 2-3 days

- [ ] Implement `mv` tool
  - Support single file and batch moves
  - Handle rename vs move distinction
  - Add path validation

- [ ] Implement `rm` tool
  - Add safety confirmation for destructive ops
  - Support recursive deletion
  - Trash/permanent delete options

- [ ] Implement `cp` tool
  - Support single and batch copy
  - Preserve metadata options
  - Handle symlinks

- [ ] Implement `mkdir` tool
  - Support `-p` (create parents)
  - Handle permissions

- [ ] Implement `diff` tool
  - Support unified/context diff formats
  - Integrate with git diff when applicable

- [ ] Implement `tree` tool
  - Respect .gitignore
  - Depth limiting
  - Size/date information

### Phase 4.2: Fetch Consolidation
**Priority**: Medium
**Estimated Effort**: 1-2 days

- [ ] Refactor `fetch` tool
  - Add `--mode` parameter: `simple`, `smart`, `research`
  - Default to `simple` for backward compatibility
  - Move smart routing logic to `smart` mode
  - Move agent-based research to `research` mode

- [ ] Remove `web_fetch` and `agentic_fetch` tools
  - Update all references to use `fetch --mode=...`
  - Update documentation

- [ ] Update aliases
  - Keep: `curl`, `wget`, `http`
  - Remove redundant aliases

### Phase 4.3: Edit Consolidation
**Priority**: Medium
**Estimated Effort**: 1 day

- [ ] Merge `smart_edit` capabilities into `edit`
  - Make AI-assistance default behavior
  - Add `--simple` flag for non-AI edits

- [ ] Keep `multiedit` as separate tool
  - Batch operations use case is distinct

- [ ] Update aliases
  - Keep: `modify`, `update`

### Phase 4.4: Git Integration
**Priority**: Medium
**Estimated Effort**: 2 days

- [ ] Implement `git` wrapper tool
  - Sub-command routing: `git commit`, `git status`, etc.
  - Safety hooks for destructive operations
  - Integrate with existing git hooks

- [ ] Implement `git_interactive` for complex workflows
  - Interactive staging
  - Interactive rebase support

### Phase 4.5: Process Management
**Priority**: Low
**Estimated Effort**: 1 day

- [ ] Convert `job_kill` from alias to first-class tool
  - Better error handling
  - Process tree killing option

- [ ] Convert `job_output` to `get_job_output` tool
  - Streaming output support
  - Follow mode (like tail -f)

### Phase 4.6: Naming Standardization
**Priority**: Low
**Estimated Effort**: 1 day

- [ ] Standardize all tool names to underscore convention
  - `web_search`, `code_search`, `lsp_diagnostics`
  - Update all references

- [ ] Rationalize aliases to 1-2 most intuitive per tool
  - Document alias choices
  - Remove redundant aliases

### Phase 4.7: Documentation & Migration
**Priority**: High
**Estimated Effort**: 1 day

- [ ] Update tool documentation
  - Clear purpose for each tool
  - When to use which variant
  - Migration guide from old names

- [ ] Create deprecation warnings for removed tools
  - Suggest replacement in warning message
  - Grace period before removal

- [ ] Update tests
  - Test all new tools
  - Update tests using deprecated tools

---

## Success Criteria

- [ ] **Reduced Complexity**: 25 core tools (from 20)
- [ ] **Fewer Aliases**: ~30 total aliases (from 47)
- [ ] **Better Coverage**: All core file operations available
- [ ] **Clear Naming**: Consistent naming convention throughout
- [ ] **No Confusion**: Each tool has clear, distinct purpose
- [ ] **Git Support**: Native git operations without bash wrapper
- [ ] **Backward Compatible**: Deprecation warnings, not breakage

---

## Risks & Mitigations

### Risk 1: Breaking Existing Workflows
**Mitigation**:
- Deprecation warnings before removal
- Alias support during transition period
- Clear migration documentation

### Risk 2: Increased Tool Count
**Mitigation**:
- Each new tool solves specific gap
- Net complexity reduction via consolidation
- Better discoverability through clear naming

### Risk 3: Implementation Effort
**Mitigation**:
- Phased rollout (4.1-4.7)
- Focus on high-priority items first
- Reuse existing code patterns

---

## Timeline Estimate

| Phase | Priority | Effort | Dependencies |
|-------|----------|--------|--------------|
| 4.1   | High     | 2-3d   | None         |
| 4.2   | Medium   | 1-2d   | None         |
| 4.3   | Medium   | 1d     | None         |
| 4.4   | Medium   | 2d     | 4.1          |
| 4.5   | Low      | 1d     | None         |
| 4.6   | Low      | 1d     | 4.1-4.5      |
| 4.7   | High     | 1d     | 4.1-4.6      |

**Total**: 9-12 days of focused work

---

## Open Questions

1. Should `edit` use AI assistance by default, or require opt-in?
2. What should the deprecation timeline be for removed tools?
3. Should `fetch --mode=research` require explicit user confirmation?
4. Do we need `git_interactive` or can we handle this with `bash git` for now?
5. Should `rm` default to trash or permanent delete?

---

## Next Steps

1. Review this plan with team
2. Get consensus on naming conventions
3. Prioritize phases based on user pain points
4. Start implementation with Phase 4.1 (Core File Operations)
5. Create tracking issues for each tool implementation

---

**Notes**: This plan focuses on improving developer experience while maintaining backward compatibility during transition. Emphasis on clear, consistent naming and eliminating tool confusion.
