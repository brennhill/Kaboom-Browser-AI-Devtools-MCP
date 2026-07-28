---
feature: read-only-mode
status: proposed
tool: configure
mode: security
version: 0.7.12
doc_type: product-spec
feature_id: feature-read-only-mode
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Read-Only Mode

## Problem Statement

In production environments, AI agents should ONLY observe and analyze, never mutate state. Allowing mutation tools (interact actions like execute_js, navigate, click) in production is dangerous — agents could accidentally trigger transactions, delete data, or disrupt live systems. Organizations need a security mode that disables all mutation capabilities while preserving observation.

## Solution

Add read-only mode as a server-wide configuration flag. When enabled, all mutation-capable tools and actions are disabled. Only non-mutating operations remain available: observe (all modes), analyze (including `what:"dom"`), generate (analysis artifacts), and safe configure actions like `health`. Attempts to call mutation tools return clear errors. Mode is enforced at server level, cannot be bypassed by clients.

## Requirements

- Disable all mutating interact tool actions (execute_js, navigate, click/type/select/check, etc.)
- Allow observe tool (all modes: logs, network, websocket, vitals, api, etc.)
- Allow generate tool (reproduction, test, sarif, har, csp) — artifacts don't mutate browser state
- Allow analyze tool (`what:"dom"` in read-only) and safe configure actions (for example `health`) — no state changes
- Clear error messages when mutation attempted: "Read-only mode enabled, mutation tools disabled"
- Server-level enforcement (not client-side) — cannot be bypassed
- Configuration via CLI flag: --read-only or environment variable KABOOM_READ_ONLY=true
- Status visible via configure({what: "health"})

## Out of Scope

- Granular per-tool permissions (covered by tool-allowlisting feature)
- Time-based read-only windows
- Read-only mode for specific tabs only (server-wide enforcement)

## Success Criteria

- Agent can observe production system (logs, network, errors) in read-only mode
- Agent CANNOT execute JS, navigate, or fill forms when read-only enabled
- Mutation attempts fail with clear, actionable error messages
- Read-only mode cannot be disabled without server restart (immutable at runtime)

## User Workflow

1. SRE starts Kaboom in read-only mode: `kaboom --read-only`
2. Agent connects, attempts to analyze production issue
3. Agent uses `observe({what: "errors"})` — succeeds
4. Agent tries `interact({what: "execute_js"})` — fails with "Read-only mode enabled"
5. Agent generates analysis: `generate({what: "reproduction"})` — succeeds
6. Agent provides findings to human, no production state mutated

## Examples

### Server start in read-only mode:
```bash
kaboom --read-only --port 7890
# or
KABOOM_READ_ONLY=true kaboom
```

## Check read-only status:
```json
configure({what: "health"})
// Returns:
{
  "read_only_mode": true,
  "port": 7890,
  "persist": true
}
```

## Mutation attempt fails:
```json
interact({what: "execute_js", code: "alert('test')"})
// Returns:
{
  "error": "read_only_mode_enabled",
  "message": "Interactive features are disabled in read-only mode. Only observation and analysis tools are available."
}
```

## Allowed operations:
```json
// Observation: allowed
observe({what: "errors"})
observe({what: "network_waterfall"})

// Analysis generation: allowed
generate({what: "reproduction"})
generate({what: "sarif"})

// DOM query (read-only): allowed
analyze({what: "dom", selector: ".error-message"})

// Mutation: blocked
interact({what: "navigate"}) // ERROR
interact({what: "click", selector: "button.submit"}) // ERROR
configure({what: "clear"}) // ERROR
```

---

## Notes

- Read-only mode is immutable after server start (requires restart to toggle)
- Designed for production environments where mutation is prohibited
- Complementary to tool-allowlisting (read-only is broader, allowlist is granular)
