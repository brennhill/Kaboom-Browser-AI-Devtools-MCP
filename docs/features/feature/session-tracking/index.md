---
doc_type: feature_index
feature_id: feature-session-tracking
status: implemented
feature_type: feature
owners: []
last_reviewed: 2026-08-05
code_paths:
  - internal/hook/session.go
  - cmd/hooks/main.go
test_paths:
  - internal/hook/session_test.go
  - internal/hook/eval/testdata/session-track/
  - cmd/hooks/main_test.go
---

# Session Tracking

| Field         | Value                                   |
|---------------|-----------------------------------------|
| **Status**    | implemented                             |
| **Binary**    | kaboom-hooks                          |
| **Command**   | `kaboom-hooks session-track`          |
| **Hook**      | PostToolUse on Read, Edit, Write, Bash  |
| **Parent**    | [Quality Gates](../quality-gates/index.md) |

## Specs

- [Product Spec](./product-spec.md)
- [Tech Spec](./tech-spec.md)

## Summary

Session tracking records every file read, edit, and command execution during an AI coding session. On subsequent tool uses, it injects session context — which files were already read, what was changed, what tests passed or failed — so the AI avoids redundant work and maintains awareness of its own actions.

This is the foundation hook that other hooks (`blast-radius`, `decision-guard`) read from to make smarter decisions.

Session metadata is written atomically with private permissions and validated
before reuse. Touches remain a single-write append-only JSONL hot path, followed
by file sync and checked close. Deterministic write, sync, quota, partial-write,
cancellation, restart, read, and corruption failures collapse to stable
value-free codes. A failed preflight or append is returned as valid hook-protocol
context and is never described as recorded; corrupt history is never extended.
Persistence and tracking decisions now live in one change-coupled `session.go`
owner, with their durability, recovery, and classification tests together in
`session_test.go`.

## Install

```bash
# Already included in kaboom-hooks
kaboom-hooks session-track
```

## Hook Configuration

```json
{
  "matcher": "Read|Edit|Write|Bash",
  "hooks": [{"type": "command", "command": "kaboom-hooks session-track", "timeout": 10}]
}
```
