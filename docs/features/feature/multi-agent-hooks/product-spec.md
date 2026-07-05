---
doc_type: product-spec
feature_id: feature-multi-agent-hooks
status: proposed
owners: []
last_reviewed: 2026-07-05
links:
  index: ./index.md
  tech: ./tech-spec.md
---

# Multi-Agent Hook Protocol Product Spec

## TL;DR

- Problem: Each AI coding agent (Claude Code, Gemini CLI, Codex) has a different hook protocol. Building separate hook binaries per agent doesn't scale.
- User value: One binary, one install, works with any supported agent. Users install `kaboom-hooks` once and configure it in whichever agent they use.
- Binary: `kaboom-hooks` (same binary, auto-detects calling agent)

## Requirements

### MULTI_AGENT_001: Auto-detect calling agent

The hook binary must detect which agent is calling it, without flags or configuration. Detection order:
1. `GEMINI_SESSION_ID` env var set -> Gemini CLI
2. `CODEX_SESSION_ID` env var set -> Codex (future)
3. Default -> Claude Code

### MULTI_AGENT_002: Adapt output format

Each agent expects a different JSON output structure for context injection:

**Claude Code** (PostToolUse):
```json
{"additionalContext": "context string here"}
```

**Gemini CLI** (AfterTool):
```json
{"hookSpecificOutput": {"additionalContext": "context string here"}}
```

The `WriteOutput()` function must emit the correct format based on the detected agent.

### MULTI_AGENT_003: Shared input format

All supported agents use the same input fields:
- `tool_name`: which tool was called
- `tool_input`: the tool's arguments (file_path, command, etc.)
- `tool_response`: the tool's result

No input adaptation needed — `ReadInput()` works unchanged.

### MULTI_AGENT_004: Session ID from environment

When the agent provides a session ID via environment variable, use it directly instead of deriving from `(ppid, cwd)`:
- Gemini CLI: `GEMINI_SESSION_ID`
- Claude Code: derive from `(ppid, cwd)` (no env var provided)

This gives more reliable session identity when the agent supports it.

### MULTI_AGENT_005: Setup command per agent

`configure(what="setup_quality_gates")` must detect installed agents and write hooks to the correct config file:
- Claude Code: `.claude/settings.json` with `PostToolUse` matchers
- Gemini CLI: `.gemini/settings.json` with `AfterTool` matchers

### MULTI_AGENT_006: Gemini-specific capabilities

Gemini CLI provides hooks that Claude Code doesn't:
- `BeforeTool`: can block edits with `{"decision": "deny"}` — decision-guard could enforce hard blocks
- `SessionStart` / `SessionEnd`: session-track could initialize/cleanup without heuristics
- `BeforeToolSelection`: could filter tools dynamically

These are opt-in enhancements, not required for basic functionality. All hooks must work without them.

### MULTI_AGENT_007: Single install path

The bash installer and npm packages ship one `kaboom-hooks` binary. No per-agent builds. The binary self-adapts at runtime.

## Non-Goals

- Supporting agents without hook protocols (e.g., ChatGPT, Copilot)
- Agent-specific hook logic (the logic layer is agent-agnostic)
- Automatic agent detection at install time (hooks are configured per-agent)
