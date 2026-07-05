---
doc_type: product-spec
feature_id: feature-decision-guard
status: proposed
owners: []
last_reviewed: 2026-07-05
links:
  index: ./index.md
  tech: ./tech-spec.md
---

# Decision Guard Product Spec

## TL;DR

- Problem: AI agents forget or re-debate architectural decisions made earlier in a session or across sessions. A team decides "always use the shared HTTP client", but two sessions later the AI creates a new `http.Client{}` because it doesn't remember the decision.
- User value: Lock decisions once, enforce them automatically on every edit. The AI sees the relevant rule at the moment it's about to violate it — before the code is committed.
- Binary: `kaboom-hooks decision-guard` (PostToolUse hook) + `kaboom-hooks lock-decision` (CLI utility)

## Requirements

### DECISION_001: Decision file format

Decisions are stored in `.kaboom/decisions.json` in the project root (same directory as `.kaboom.json`). Format:

```json
[
  {
    "id": "use-shared-http-client",
    "pattern": "http.Client{",
    "rule": "Use the shared HTTP client from internal/util/http.go. Do not create new http.Client instances.",
    "scope": "*.go",
    "locked_at": "2026-03-07T14:30:00Z",
    "locked_by": "team"
  }
]
```

Fields:
- `id`: unique identifier (kebab-case)
- `pattern`: literal string or regex (prefixed with `re:`) to match against new code
- `rule`: the instruction to inject when the pattern matches
- `scope`: glob pattern for file types this applies to (e.g., `*.go`, `*.ts`, `*`)
- `locked_at`: ISO 8601 timestamp
- `locked_by`: who locked it — "team", "ai", or a person's name

### DECISION_002: Pattern matching on edits

When an Edit or Write fires, check the `new_string` (or `content` for Write) against all decision patterns:
- Literal patterns: `strings.Contains(newCode, pattern)`
- Regex patterns (prefixed with `re:`): regex match against newCode
- Scope filtering: only check decisions whose `scope` glob matches the edited file's extension

Inject ALL matching decisions, not just the first.

### DECISION_003: Injection format

When one or more decisions match:

```
[Decision Guard] 1 locked decision applies to this edit:

  use-shared-http-client: Use the shared HTTP client from internal/util/http.go. Do not create new http.Client instances.

Comply with these decisions or update .kaboom/decisions.json if they are outdated.
```

When multiple match:

```
[Decision Guard] 2 locked decisions apply to this edit:

  use-shared-http-client: Use the shared HTTP client from internal/util/http.go.
  error-format: Error messages must follow: "{OPERATION}: {ROOT_CAUSE}. {RECOVERY_ACTION}"

Comply with these decisions or update .kaboom/decisions.json if they are outdated.
```

### DECISION_004: CLI for locking decisions

`kaboom-hooks lock-decision` provides a quick way to add decisions:

```bash
kaboom-hooks lock-decision \
  --id "use-shared-http-client" \
  --pattern "http.Client{" \
  --rule "Use the shared HTTP client from internal/util/http.go" \
  --scope "*.go"
```

This appends to `.kaboom/decisions.json`, creating the file if it doesn't exist. The file lives in the project root (same as `.kaboom.json`).

### DECISION_005: AI-writable decisions

The AI can lock decisions by writing directly to `.kaboom/decisions.json`. The hook doesn't care who wrote the file — it just reads it. This means:
- A human can add decisions by hand
- The AI can add decisions when asked ("remember this for future sessions")
- CI can validate decisions are respected

### DECISION_006: Decision expiry (optional)

Decisions can include an optional `expires_at` field. If set, the decision is ignored after that date. This is useful for temporary rules ("don't modify the auth module until the security audit is complete").

### DECISION_007: Performance budget

- Read and parse decisions.json: < 5ms
- Pattern matching against new code: < 5ms
- Total hook execution: < 15ms

## Non-Goals

- Blocking the edit (hooks inject context, they don't reject changes)
- Versioning or history of decision changes
- Conflict resolution between contradictory decisions
- Automatic decision discovery (the AI or team must explicitly lock decisions)
