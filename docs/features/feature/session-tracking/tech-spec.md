---
doc_type: tech-spec
feature_id: feature-session-tracking
status: proposed
owners: []
last_reviewed: 2026-08-05
links:
  index: ./index.md
  product: ./product-spec.md
code_paths:
  - internal/hook/session.go
  - cmd/hooks/main.go
test_paths:
  - internal/hook/session_test.go
  - cmd/hooks/main_test.go
---

# Session Tracking Tech Spec

## TL;DR

- Design: Append-only JSONL session log in `~/.kaboom/sessions/<session-id>/`, read by all hooks through the canonical `session.go` owner
- Key constraints: < 20ms per invocation, no locks (append-only), concurrent-safe
- Rollout risk: Low — purely additive, no changes to existing hooks

## Requirement Mapping

- SESSION_TRACK_001 -> `internal/hook/session.go:runSessionTrack()` and `AppendTouch()` — append the classified JSONL entry
- SESSION_TRACK_002 -> `internal/hook/session.go:checkRedundantRead()` — scans log for prior reads
- SESSION_TRACK_003 -> `internal/hook/session.go:SessionSummary()` — counts reads/edits/commands
- SESSION_TRACK_004 -> `internal/hook/session.go:SessionID()` — derives `(ppid, cwd)` hash
- SESSION_TRACK_005 -> all functions return `("", nil)` on I/O errors
- SESSION_TRACK_006 -> benchmarked and regression-tested in `session_test.go`

## Session Store (shared by all hooks)

```
~/.kaboom/sessions/<session-id>/
  ├── touches.jsonl     # append-only log of tool uses
  └── meta.json         # session metadata (start time, cwd, ppid)
```

### Session ID derivation

Prefers agent-provided session IDs when available (Gemini CLI sets `GEMINI_SESSION_ID`), falls back to `(ppid, cwd)` hash for Claude Code. See [Multi-Agent Hooks](../multi-agent-hooks/tech-spec.md) for details.

```go
func SessionID() string {
    // Prefer agent-provided session ID.
    if id := os.Getenv("GEMINI_SESSION_ID"); id != "" {
        return id[:16]
    }
    // Claude Code fallback: derive from (ppid, cwd).
    ppid := os.Getppid()
    cwd, _ := os.Getwd()
    h := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", ppid, cwd)))
    return hex.EncodeToString(h[:8]) // 16-char hex
}
```

### JSONL entry format

```json
{"t":"2026-03-07T14:30:00Z","tool":"Edit","file":"/project/foo.go","action":"edit","summary":"func hello() {}"}
```

Single-letter keys minimize disk I/O. Fields:
- `t`: timestamp (RFC 3339)
- `tool`: tool_name from hook input
- `file`: file_path from tool_input (empty for Bash)
- `action`: read|edit|write|bash
- `summary`: first 100 chars of new_string (edit), command (bash), or empty (read/write)

### Concurrency model

No locks. Each hook process appends one line atomically (Go `os.OpenFile` with `O_APPEND`). Reads scan the full file — at ~150 bytes/line and ~500 tool uses/session, the file is ~75KB max, readable in < 1ms.

### Stale session cleanup

On startup, `SessionID()` checks `~/.kaboom/sessions/` for directories with `meta.json` older than 4 hours. Removes them in a background goroutine (non-blocking).

## Hook Logic

```
kaboom-hooks session-track:
  1. Parse hook input (tool_name, tool_input)
  2. Derive session ID
  3. Append entry to touches.jsonl
  4. If Read: check for redundant read -> inject notice
  5. If Edit/Write: inject session summary
  6. If Bash: record command + exit status
  7. Write additionalContext to stdout (or nothing)
```

## Cross-Hook API

Other hooks import `internal/hook/session.go`:

```go
// ReadTouches returns all session entries, newest first.
func ReadTouches(sessionDir string) ([]TouchEntry, error)

// FilesEdited returns file paths edited this session.
func FilesEdited(sessionDir string) []string

// LastBashResult returns the most recent Bash command and its exit status.
func LastBashResult(sessionDir string) (command string, exitCode int, found bool)

// WasFileRead returns true if the file was already read this session.
func WasFileRead(sessionDir string, filePath string) (bool, time.Time)
```

These are read-only queries against `touches.jsonl`. Any hook can call them without importing session-track logic.

## Performance

| Operation | Budget | Method |
|-----------|--------|--------|
| Append entry | < 2ms | `O_APPEND` write, no fsync |
| Read all touches | < 5ms | Scan JSONL, ~75KB max |
| Session ID | < 1ms | SHA-256 of ppid+cwd |
| Stale cleanup | async | Background goroutine |

## What gets injected (examples)

**Redundant read (no changes):**
```
[Session] You read this file 3 min ago. No edits since.
```

**Redundant read (with edits):**
```
[Session] You read this file 8 min ago. You edited it 2 min ago: "refactored validateInput to use guard pattern"
```

**Session summary on edit:**
```
[Session] 12 files read, 4 edited, 6 commands. Last test: PASS (go test ./...)
```

**Session summary with failure:**
```
[Session] 8 files read, 3 edited, 4 commands. Last test: FAIL (go test ./internal/hook/)
```
