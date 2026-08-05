---
doc_type: feature_index
feature_id: feature-decision-guard
status: implemented
feature_type: feature
owners: []
last_reviewed: 2026-08-05
code_paths:
  - internal/hook/hook_policy.go
  - cmd/hooks/main.go
test_paths:
  - internal/hook/hook_policy_test.go
  - internal/hook/eval/testdata/decision-guard/
  - cmd/hooks/main_test.go
---

# Decision Guard

| Field         | Value                                   |
|---------------|-----------------------------------------|
| **Status**    | implemented                             |
| **Binary**    | kaboom-hooks                          |
| **Command**   | `kaboom-hooks decision-guard`         |
| **Hook**      | PostToolUse on Edit, Write              |
| **Parent**    | [Quality Gates](../quality-gates/index.md) |

## Specs

- [Product Spec](./product-spec.md)
- [Tech Spec](./tech-spec.md)

## Summary

Decision guard enforces locked architectural decisions. Teams and AI agents accumulate decisions during a project — "use validateAndRespond() for all validation", "error messages follow the format X", "never import package Y directly". Without enforcement, these decisions drift as the AI forgets or re-derives them.

Decision guard reads `.kaboom/decisions.json` from the project root and injects relevant decisions when an edit touches matching code patterns. Decisions can be added by hand, by the AI writing to the file, or via `kaboom-hooks lock-decision`.

## Hook Configuration

```json
{
  "matcher": "Edit|Write",
  "hooks": [{"type": "command", "command": "kaboom-hooks decision-guard", "timeout": 10}]
}
```
