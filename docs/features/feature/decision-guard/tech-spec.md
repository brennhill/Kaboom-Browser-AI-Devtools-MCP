---
doc_type: tech-spec
feature_id: feature-decision-guard
status: proposed
owners: []
last_reviewed: 2026-07-05
links:
  index: ./index.md
  product: ./product-spec.md
code_paths:
  - internal/hook/decision_guard.go
  - cmd/hooks/main.go
test_paths:
  - internal/hook/decision_guard_test.go
  - cmd/hooks/main_test.go
---

# Decision Guard Tech Spec

## TL;DR

- Design: Read `.kaboom/decisions.json`, match patterns against new code, inject matching rules
- Key constraints: < 15ms, no network, file-based decisions committed to repo
- Rollout risk: Low — purely additive, decisions file is opt-in

## Requirement Mapping

- DECISION_001 -> `internal/hook/decision_guard.go:Decision` struct + `LoadDecisions()`
- DECISION_002 -> `internal/hook/decision_guard.go:MatchDecisions()` — pattern + scope matching
- DECISION_003 -> `internal/hook/decision_guard.go:FormatDecisions()` — output formatting
- DECISION_004 -> `cmd/hooks/main.go:runLockDecision()` — CLI subcommand
- DECISION_005 -> no special code needed — file is plain JSON
- DECISION_006 -> `MatchDecisions()` skips expired decisions
- DECISION_007 -> benchmarked in tests

## Data Model

```go
type Decision struct {
    ID        string     `json:"id"`
    Pattern   string     `json:"pattern"`           // literal or "re:..." regex
    Rule      string     `json:"rule"`
    Scope     string     `json:"scope"`             // glob for file extensions
    LockedAt  time.Time  `json:"locked_at"`
    LockedBy  string     `json:"locked_by"`
    ExpiresAt *time.Time `json:"expires_at,omitempty"`
}
```

## Decision File Location

The hook walks up from the edited file looking for `.kaboom/decisions.json`, same algorithm as `findProjectRoot()` in `quality_gate.go` (looks for `.kaboom.json`). This means the decisions file is project-scoped, not global.

```
project/
  .kaboom.json              # quality gates config
  .kaboom/
    decisions.json            # locked decisions
  kaboom-code-standards.md  # standards doc
```

## Hook Logic

```
kaboom-hooks decision-guard:
  1. Parse hook input (tool_name, tool_input)
  2. Skip if not Edit or Write
  3. Extract file_path and new_string/content
  4. Find project root (walk up for .kaboom.json)
  5. Load .kaboom/decisions.json (silently skip if missing)
  6. For each decision:
     a. Skip if expired
     b. Skip if scope doesn't match file extension
     c. Check pattern against new code:
        - Literal: strings.Contains(newCode, pattern)
        - Regex (re: prefix): regexp.MatchString(newCode)
  7. Format matching decisions
  8. Write additionalContext to stdout
```

## Pattern Matching

```go
func (d Decision) Matches(newCode string, filePath string) bool {
    // Scope check
    if d.Scope != "" && d.Scope != "*" {
        matched, _ := filepath.Match(d.Scope, filepath.Base(filePath))
        if !matched {
            return false
        }
    }
    // Expiry check
    if d.ExpiresAt != nil && time.Now().After(*d.ExpiresAt) {
        return false
    }
    // Pattern check
    if strings.HasPrefix(d.Pattern, "re:") {
        re, err := regexp.Compile(d.Pattern[3:])
        if err != nil {
            return false
        }
        return re.MatchString(newCode)
    }
    return strings.Contains(newCode, d.Pattern)
}
```

Regex compilation: for performance, compile once and cache in a `sync.Once` per decision. But since decision lists are small (< 50) and the hook is short-lived (process-per-invocation), inline compilation is fine.

## CLI: lock-decision

New subcommand in `cmd/hooks/main.go`:

```go
var subcommands = map[string]func() int{
    "compress-output": runCompressOutput,
    "quality-gate":    runQualityGate,
    "session-track":   runSessionTrack,
    "blast-radius":    runBlastRadius,
    "decision-guard":  runDecisionGuard,
    "lock-decision":   runLockDecision,  // utility, not a hook
}
```

`lock-decision` reads flags, finds the project root, appends to `.kaboom/decisions.json`:

```go
func runLockDecision() int {
    // Parse flags: --id, --pattern, --rule, --scope
    // Find project root (cwd, walk up for .kaboom.json)
    // Read existing decisions (or create empty array)
    // Append new decision with locked_at=now, locked_by="cli"
    // Write back to .kaboom/decisions.json
    // Print confirmation to stderr
    return 0
}
```

## Output Examples

**Single decision match:**
```
[Decision Guard] 1 locked decision applies to this edit:

  use-shared-http-client: Use the shared HTTP client from internal/util/http.go. Do not create new http.Client instances.

Comply with these decisions or update .kaboom/decisions.json if they are outdated.
```

**No matches:**
No output (exit 0, empty stdout).

## Performance

| Operation | Budget | Method |
|-----------|--------|--------|
| Find project root | < 2ms | Walk up for .kaboom.json |
| Read decisions.json | < 2ms | os.ReadFile + json.Unmarshal |
| Pattern matching | < 5ms | string contains / regex per decision |
| Total | < 15ms | |

## Relationship to quality-gate

Decision guard and quality-gate are complementary:
- **quality-gate** injects the full standards doc on every edit — broad, static rules
- **decision-guard** injects specific, targeted rules only when the AI is about to violate them — narrow, dynamic

Both can be installed independently. When both are installed, the AI sees standards + decisions on each edit.
