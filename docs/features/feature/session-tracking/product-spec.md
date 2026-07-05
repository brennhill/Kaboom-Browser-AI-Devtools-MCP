---
doc_type: product-spec
feature_id: feature-session-tracking
status: proposed
owners: []
last_reviewed: 2026-07-05
links:
  index: ./index.md
  tech: ./tech-spec.md
---

# Session Tracking Product Spec

## TL;DR

- Problem: AI agents re-read files they already read, lose track of what they changed, and lack awareness of their own session history. This wastes tokens and causes inconsistencies.
- User value: Eliminates redundant file reads, gives the AI a working memory of its session, and enables other hooks to make context-aware decisions.
- Binary: `kaboom-hooks session-track` (PostToolUse hook)

## Requirements

### SESSION_TRACK_001: Record all file interactions

Every Read, Edit, Write, and Bash tool use must be appended to a session-scoped log file. Each entry records:
- `tool_name`: which tool was called
- `file_path`: which file was touched (empty for Bash)
- `action`: read, edit, write, or bash
- `timestamp`: ISO 8601
- `summary`: for edits, a one-line description of what changed (first 100 chars of new_string); for Bash, the command

The log must be append-only JSONL for concurrent safety (multiple hooks may run in parallel).

### SESSION_TRACK_002: Detect and suppress redundant reads

When a Read fires on a file that was already read this session:
- If the file has NOT been edited since the last read, inject: "You already read this file N minutes ago. No changes since then."
- If the file HAS been edited since the last read, inject: "You already read this file, and you edited it N minutes ago. Your changes: [summary]"

This gives the AI context to skip re-reading or to understand what it previously changed.

### SESSION_TRACK_003: Inject session summary on edit

When an Edit or Write fires, inject a brief session summary:
- "Session: N files read, M files edited, K commands run"
- If a recent Bash command failed (exit code != 0), append: "Last test/build failed: [command]"

This keeps the AI oriented without flooding context.

### SESSION_TRACK_004: Session identity

Sessions are identified by `(ppid, cwd)` — the parent process ID (Claude Code) and working directory. This means:
- Multiple Claude Code instances in different directories get separate sessions
- Restarting Claude Code in the same directory starts a fresh session (new PID)
- Sessions auto-expire after 4 hours of inactivity (stale file cleanup)

### SESSION_TRACK_005: Graceful degradation

If the session directory cannot be created or written to, the hook must exit silently (exit 0, no output). Session tracking is best-effort — it must never block or break the AI's workflow.

### SESSION_TRACK_006: Performance budget

- Append to JSONL: < 2ms
- Read session state + format context: < 10ms
- Total hook execution: < 20ms

### SESSION_TRACK_007: Token efficiency

Context injection must be minimal:
- Redundant read notice: ~30 tokens
- Session summary: ~40 tokens
- Only inject when there IS useful context (not on every call)

## Non-Goals

- Full file content caching (too large, stale quickly)
- Cross-session persistence (sessions are ephemeral by design)
- Undo/rollback functionality
- Tracking AI reasoning or decisions (that's decision-guard's job)
