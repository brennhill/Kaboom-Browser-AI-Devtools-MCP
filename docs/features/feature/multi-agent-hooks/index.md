---
doc_type: feature_index
feature_id: feature-multi-agent-hooks
status: implemented
feature_type: feature
owners: []
last_reviewed: 2026-09-05
code_paths:
  - internal/hooks/hook_policy.go
  - internal/hooks/session.go
  - internal/hooks/eval/eval.go
  - cmd/hooks/main.go
  - internal/tools/configure/handler/qualitygates/handler.go
  - cmd/browser-agent/internal/toolruntime/tools_configure.go
  - internal/tools/configure/handler/dispatcher.go
test_paths:
  - internal/tools/configure/handler/dispatcher_test.go
  - internal/hooks/hook_policy_test.go
  - internal/hooks/session_test.go
  - internal/hooks/eval/eval_test.go
  - internal/hooks/eval/testdata/quality-gate/standards/002-file-size-warning.json
  - internal/hooks/eval/testdata/u02-single-responsibility/enforce-001-file-over-limit.json
  - cmd/hooks/main_test.go
  - internal/tools/configure/handler/qualitygates/handler_test.go
---

# Multi-Agent Hook Protocol

| Field         | Value                                   |
|---------------|-----------------------------------------|
| **Status**    | implemented                             |
| **Binary**    | kaboom-hooks                            |
| **Agents**    | Claude Code, Gemini CLI, Codex (future) |
| **Parent**    | [Quality Gates](../quality-gates/index.md) |

## Specs

- [Product Spec](./product-spec.md)
- [Tech Spec](./tech-spec.md)

## Summary

The `kaboom-hooks` binary auto-detects which AI coding agent is calling it and adapts its output protocol accordingly. All hooks (quality-gate, compress-output, session-track, blast-radius, decision-guard) work across agents without separate binaries or configuration. The hook logic is agent-agnostic; only the thin I/O protocol layer adapts.

File-size evals materialize their own oversized temporary source under the
repository root. The temporary file remains syntactically valid Go while
concurrent repository-wide checks can observe it, and eval cleanup removes it
afterward. Evals do not depend on keeping an unhealthy production or test file
oversized.

## Supported Agents

| Agent | Hook Event | Config File | Output Format | Session ID |
|-------|-----------|-------------|---------------|------------|
| Claude Code | PostToolUse | `.claude/settings.json` | `{"additionalContext": "..."}` | Derived from `(ppid, cwd)` |
| Gemini CLI | AfterTool | `.gemini/settings.json` | `{"hookSpecificOutput": {"additionalContext": "..."}}` | `GEMINI_SESSION_ID` env var |
| Codex | post_exec (future) | `codex.toml` | TBD | TBD |
