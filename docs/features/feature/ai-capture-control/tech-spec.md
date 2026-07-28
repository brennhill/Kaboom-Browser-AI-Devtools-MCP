---
status: proposed
scope: feature/ai-capture-control/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-ai-capture-control
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-ai-capture-control.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [AI Capture Control Review](ai-capture-control-review.md).

# Technical Spec: AI Capture Control

## Purpose

The extension has capture settings (log level, WebSocket mode, network body capture, etc.) that determine what data flows to the server. Today, these are configured manually through the extension's options page. When the AI needs richer data — "I'm debugging a WebSocket issue, I need to see message payloads" — it has to ask the human to toggle a setting. That breaks the feedback loop.

AI capture control lets the AI adjust its own observation capabilities via the existing `configure` tool. The AI calls `configure(what: "capture", settings: { ws_mode: "messages" })` and the extension picks up the new setting on its next heartbeat. The AI is borrowing elevated access for the session, not permanently changing preferences.

All changes are session-scoped (reset on server restart), transparent (emit an alert), and auditable (written to a structured log file).

---

## How It Works

### Setting Changes via MCP

The AI uses the existing composite tool interface:

```
configure(what: "capture", settings: { ws_mode: "messages", log_level: "all" })
```

The server stores the override in memory. On the extension's next settings poll (every 5 seconds via the existing `/settings` endpoint), the override is included in the response. The extension applies it immediately.

### Controllable Settings

| Setting | Values | Default | What it controls |
|---------|--------|---------|-----------------|
| `log_level` | `error`, `warn`, `all` | `error` | Which console methods are captured |
| `ws_mode` | `off`, `lifecycle`, `messages` | `lifecycle` | Whether WS message payloads are captured |
| `network_bodies` | `true`, `false` | `true` | Whether HTTP response bodies are stored |
| `screenshot_on_error` | `true`, `false` | `false` | Whether errors trigger a screenshot |
| `action_replay` | `true`, `false` | `true` | Whether user actions are recorded |

### Rate Limiting

Setting changes are rate-limited to 1 change per second. If the AI calls `configure(what: "capture")` more frequently, the server returns an error: "Rate limited: capture settings can be changed at most once per second." This prevents settings oscillation (intentional or accidental) from flooding the audit log or causing unnecessary extension reconfiguration. The 5-second extension poll already provides a natural throttle on actual behavior change, but the rate limit protects the server side.

### Session Scoping

AI-set overrides live only in server memory. When the server restarts, all overrides are cleared and the extension reverts to its configured defaults (from `chrome.storage.sync`).

The AI can also explicitly reset:
```
configure(what: "capture", settings: "reset")
```

This clears all overrides immediately without restarting.

### Extension Integration

The extension already polls `GET /settings` every 5 seconds to check connection status. The response currently returns `{ connected: true }`. With this feature, it also returns any active capture overrides:

```json
{
  "connected": true,
  "capture_overrides": {
    "ws_mode": "messages",
    "log_level": "all"
  }
}
```

The extension applies overrides by merging them on top of the user's base settings (from `chrome.storage.sync`). When overrides are present, the popup shows a small indicator ("AI-controlled") so the user knows settings have been adjusted.

### Alert on Change

When the AI changes a capture setting, the server emits an info-level alert through the existing alert piggyback system:

```json
{
  "level": "info",
  "type": "capture_override",
  "message": "AI changed ws_mode: lifecycle → messages",
  "setting": "ws_mode",
  "from": "lifecycle",
  "to": "messages",
  "timestamp": "2026-01-24T15:32:00Z"
}
```

This alert appears in the next `observe` response, so the user sees it in the natural flow of AI output.

### Page Info Integration

`observe(what: "page")` includes the current override state:

```json
{
  "url": "http://localhost:3000/dashboard",
  "title": "Dashboard",
  "capture_overrides": {
    "ws_mode": { "value": "messages", "default": "lifecycle", "changed_at": "2026-01-24T15:32:00Z" },
    "log_level": { "value": "all", "default": "error", "changed_at": "2026-01-24T15:32:01Z" }
  }
}
```

---

## Audit Log

### Purpose

A structured log file records all AI-initiated setting changes. Solo developers can ignore it. Enterprise teams can point log collectors at it for operational monitoring or change auditing.

### File Location

`~/.kaboom/audit.jsonl` by default. Configurable via `--audit-log` flag or `KABOOM_AUDIT_LOG` environment variable.

### Format

One JSON object per line (JSONL). Each line is a self-contained event:

```
{"ts":"2026-01-24T15:32:00Z","event":"capture_override","setting":"ws_mode","from":"lifecycle","to":"messages","source":"ai","agent":"claude-code"}
{"ts":"2026-01-24T15:32:01Z","event":"capture_override","setting":"log_level","from":"error","to":"all","source":"ai","agent":"claude-code"}
{"ts":"2026-01-24T15:45:00Z","event":"capture_reset","reason":"explicit","source":"ai","agent":"claude-code"}
{"ts":"2026-01-24T16:00:00Z","event":"capture_reset","reason":"session_end","source":"server","agent":""}
```

The `agent` field identifies which MCP client made the change. It's extracted from the MCP client metadata (the `clientInfo.name` field from the MCP `initialize` request). If no client info is available, it defaults to "unknown". This allows enterprise teams to distinguish changes from different AI agents in multi-agent environments.

### Rotation

The log file rotates when it exceeds 10MB. Rotation renames the current file to `audit.jsonl.1` (and shifts `.1` to `.2`, `.2` to `.3`). Maximum 3 rotated files retained (30MB total worst case). Rotation is checked before each write.

### Implementation

The audit logger is a simple struct with a mutex-protected file handle:
- `Write(event)`: Marshal to JSON, append newline, write to file, check size for rotation
- `Close()`: Flush and close the file handle
- Created at server startup, closed at shutdown
- If the log file can't be opened (permissions, disk full), the server logs a warning and continues without auditing. Capture control still works — auditing is best-effort.

---

## Data Model

### Capture Override (server memory)

The server maintains a map of setting name to override value, plus metadata:
- Setting name (string)
- Override value (string or bool)
- Previous value (for alert message)
- Timestamp of change

### Settings Response (extension)

The `/settings` endpoint response adds a `capture_overrides` field. The extension merges these on top of its local settings. When the field is absent or empty, the extension uses its local settings unchanged.

---

## Edge Cases

- **Extension not connected**: Overrides are stored on the server. When the extension connects (or reconnects), it picks them up on the next poll. No data is lost.
- **User changes settings in popup while AI override is active**: The user's change takes effect locally but will be overwritten on the next 5-second poll. To truly override the AI, the user should disconnect the extension or restart the server. The popup indicator ("AI-controlled") alerts them to this.
- **Multiple AI clients**: If two AI agents set conflicting overrides, the last writer wins. The audit log records both changes.
- **Invalid setting name**: The `configure` handler returns an error: "Unknown capture setting: foo. Valid: log_level, ws_mode, network_bodies, screenshot_on_error, action_replay."
- **Invalid setting value**: The handler returns an error with valid values: "Invalid value 'verbose' for log_level. Valid: error, warn, all."
- **Server restart during session**: All overrides cleared. Extension reverts to user settings on next poll. Audit log records `session_end` event.
- **Audit log directory doesn't exist**: Created automatically (`os.MkdirAll`).
- **Rapid setting changes**: Rate-limited to 1/second. Returns error if exceeded. The last accepted change is what takes effect.
- **Multiple settings in one call**: A single `configure` call with multiple settings (e.g., `{ ws_mode: "messages", log_level: "all" }`) counts as one change for rate limiting purposes. Each setting generates its own audit entry.

---

## Performance Constraints

- Setting change: under 0.1ms (map write + alert emit + audit log append)
- Settings poll response generation: under 0.05ms (serialize small map)
- Audit log write: under 1ms (json.Marshal + file write, not fsync'd per write)
- Audit log rotation: under 10ms (rename files)
- Memory for overrides: under 1KB (5 settings × small strings)

---

## Test Scenarios

1. `configure(what: "capture", settings: { log_level: "all" })` → override stored
2. `/settings` response includes active overrides
3. Multiple settings changed in one call
4. Reset clears all overrides
5. Invalid setting name → error response
6. Invalid setting value → error response with valid options
7. Alert emitted on change with correct from/to values
8. `observe(what: "page")` shows active overrides
9. Server restart clears overrides (session-scoped)
10. Audit log file created on first write
11. Audit log entry has correct JSON structure
12. Audit log rotation at 10MB threshold
13. Audit log rotation keeps max 3 files
14. Audit log failure doesn't block capture control
15. Extension merges overrides on top of local settings
16. Override survives extension reconnection
17. Rate limit: second change within 1s → error response
18. Rate limit: change after 1s → succeeds
19. Audit log includes agent identity from MCP client info
20. Multiple agents: audit log distinguishes changes by agent name

---

## File Locations

Server implementation: `cmd/browser-agent/capture_control.go` (override storage, settings endpoint enhancement, audit logger).

Extension integration: `extension/background.js` (settings poll response handling, override application).

Tests: `cmd/browser-agent/capture_control_test.go`, `extension-tests/capture-control.test.js`.
