---
status: proposed
scope: feature/in-browser-agent-panel/runtime
ai-priority: high
tags: [runtime, golang, transport, redaction, approvals, reliability]
relates-to: [product-spec.md, tech-spec.md, ../../../architecture/ADR-003-agent-panel-redaction-first.md]
last-verified: 2026-03-05
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Runtime Spec: Browser UI + Go Orchestrator

This document defines the detailed runtime architecture for a browser-native agent UI backed by the existing Kaboom Go binary.

The browser is UI only. All orchestration, tool execution, policy enforcement, and model egress are implemented in Go.

## 1) Goals

1. Keep one deterministic execution core shared by all clients.
2. Support browser-native UX without degrading MCP compatibility.
3. Enforce mandatory redaction and approval safety gates.
4. Preserve "startup should never fail" behavior.
5. Minimize long-term maintenance by avoiding duplicate runtimes.

## 2) Non-Goals

1. Re-implement a full VS Code extension host in the browser.
2. Make terminal-stream parsing the primary control protocol.
3. Couple Kaboom behavior to any single model vendor UX.
4. Introduce breaking changes to current MCP tools/actions.

## 3) Architecture (Normative)

```mermaid
flowchart LR
    subgraph Browser
      UI[Agent Panel UI]
      EXT[Extension Background Broker]
    end

    subgraph Local Host
      WRAP[Go Wrapper / Launcher]
      CORE[Go Runtime Core]
      QUEUE[Async Queue + Correlation IDs]
      TOOLS[MCP Tool Dispatcher]
      REDACT[Redaction Gateway]
      APPROVE[Approval Engine]
      MODEL[Provider Adapter]
      STORE[(State + Buffers + Sessions)]
    end

    UI --> EXT
    EXT -->|streamable HTTP (localhost)| CORE
    WRAP --> CORE
    CORE --> QUEUE
    QUEUE --> TOOLS
    TOOLS --> REDACT
    TOOLS --> APPROVE
    REDACT --> STORE
    APPROVE --> STORE
    TOOLS --> MODEL
    MODEL --> REDACT
    CORE --> STORE

    CORE -.->|stdio MCP| External[Codex/Claude/Gemini CLI]
```

## 4) Core Invariants (MUST)

1. `stdio` MCP transport remains pristine JSON-RPC only.
2. Browser transport is additive and does not alter stdio behavior.
3. All tool execution flows through the same dispatcher path.
4. Any model-bound payload is redacted before egress.
5. Mutating tool actions require approval by default.
6. Panel or provider failures do not crash daemon core.
7. Wrapper returns quickly after ensuring daemon is reachable.

## 5) Transport Design

## 5.1 Stdio Transport

1. Keep current MCP stdio entrypoint behavior.
2. No non-protocol stdout writes at any stage.
3. Diagnostic logging routes to file or stderr only.
4. Unknown notifications return spec-compliant JSON-RPC error without process exit.

## 5.2 Browser Transport

1. Add streamable HTTP endpoint on loopback (`127.0.0.1`).
2. Preferred base path: `/mcp` (single endpoint for JSON-RPC over HTTP).
3. Long-running operations return correlation ID and are polled via existing `observe(what="command_result")`.
4. Optional server events endpoint may be added for UI push (typed event stream), but queue/poll remains canonical control path.

## 5.3 Session & Reconnect

1. Browser client acquires a local `session_id` on first successful initialize.
2. Session includes:
   - transport type
   - created timestamp
   - last activity timestamp
   - tab/workspace context
3. Reconnect uses `session_id` and last cursor to resume command history.
4. Lost session falls back to clean reinitialize with explicit UI notice.

## 6) Runtime Modules (Go)

## 6.1 Session Manager

Responsibilities:
1. Create/track sessions by transport and client metadata.
2. Enforce TTL and cleanup on inactivity.
3. Maintain per-session pending command map.

Data model:
```json
{
  "session_id": "sess_01H...",
  "transport": "stdio|http",
  "client_name": "kaboom-panel",
  "created_at": "2026-02-16T12:00:00Z",
  "last_seen_at": "2026-02-16T12:00:05Z",
  "state": "ready|degraded|closed"
}
```

## 6.2 Tool Dispatcher

Responsibilities:
1. Validate tool name and input schema.
2. Route to existing tool handlers unchanged.
3. Classify actions as read-only/mutating/destructive for approval pipeline.

Mutation classification baseline:
1. Read-only: `observe.*`, `analyze.*`, `interact.get_*`, `interact.list_*`.
2. Mutating: `interact.click|type|navigate|upload|execute_js|refresh|new_tab`.
3. Sensitive mutating: file writes, git push, shell commands with network effects (future code-agent extensions).

## 6.3 Approval Engine

Responsibilities:
1. Intercept mutating actions lacking valid approval token.
2. Emit structured approval request object for UI.
3. Validate scope, action, expiry, and one-shot usage rules.

Approval token schema:
```json
{
  "approval_id": "appr_01H...",
  "session_id": "sess_01H...",
  "action": "interact.click",
  "scope": "tab:7",
  "expires_at": "2026-02-16T12:01:00Z",
  "single_use": true
}
```

Default policy:
1. Read-only actions auto-approved.
2. Mutating actions require explicit user approval.
3. Auto-watch cannot execute mutating actions directly.

## 6.4 Redaction Gateway

Responsibilities:
1. Apply deterministic secret masking before persistence, response, and model egress.
2. Maintain counters and diagnostics without leaking raw values.
3. Enforce fail-closed behavior on policy load/parse/runtime failure.

Redaction stages:
1. `source_redact`: extension-origin payload normalization.
2. `server_redact`: pre-store and pre-response sanitization.
3. `egress_redact`: final sanitizer before provider adapter.

Required classes:
1. Auth tokens and API keys.
2. Cookies, session IDs, CSRF values.
3. Credentials/password-like fields.
4. Payment/account identifiers.
5. User-configured patterns.

## 6.5 Model Adapter

Responsibilities:
1. Provide vendor-agnostic prompt execution interface.
2. Accept only redacted context envelopes.
3. Support local/no-provider mode for offline triage.

Interface:
```go
type ModelAdapter interface {
    Name() string
    Health(ctx context.Context) error
    Generate(ctx context.Context, req RedactedPromptRequest) (RedactedPromptResponse, error)
}
```

Failure behavior:
1. Adapter failure returns safe error to UI.
2. No daemon panic; no session loss.
3. Retry policy bounded and idempotent.

## 7) Browser UI Contract

UI state machine:
1. `idle`
2. `connecting`
3. `ready`
4. `needs_approval`
5. `degraded` (extension disconnected / daemon unavailable / provider unavailable)
6. `error`

Required views:
1. Conversation timeline.
2. Evidence cards (errors/network/actions/tool results).
3. Approval prompts with precise action/scope.
4. Redaction indicators (`policy_version`, `redacted_count`, `blocked_count`).
5. Connection health and session metadata.

## 8) Data Contracts

## 8.1 Agent Watch Config (proposed)

```json
{
  "action": "agent_watch",
  "operation": "enable",
  "events": ["errors", "network_errors", "security"],
  "rate_limit_per_min": 20,
  "dedupe_window_seconds": 15
}
```

## 8.2 Agent Context Envelope (proposed internal)

```json
{
  "context_id": "ctx_01H...",
  "session_id": "sess_01H...",
  "tab_id": 7,
  "url": "https://app.example.test/checkout",
  "trigger": "errors",
  "summary": "TypeError in checkout submit after POST /api/payments",
  "evidence": {
    "errors": [],
    "network": [],
    "actions": [],
    "command_results": []
  },
  "redaction": {
    "policy_version": "v2",
    "redacted_count": 6,
    "blocked": false
  },
  "created_at": "2026-02-16T12:00:00Z"
}
```

## 8.3 Structured Approval Request Event

```json
{
  "type": "approval_required",
  "session_id": "sess_01H...",
  "request_id": "req_01H...",
  "action": "interact.click",
  "scope": "tab:7",
  "reason": "Mutating action requires user confirmation",
  "expires_at": "2026-02-16T12:01:00Z"
}
```

## 9) Security Requirements

1. Bind browser transport to loopback only by default.
2. Use startup-scoped bearer token for local HTTP transport.
3. Enforce origin allowlist for browser requests.
4. Deny model egress on redaction failure.
5. Never log raw secrets or unredacted prompt payload.
6. Keep provider API keys out of page context and extension content scripts.

## 10) Reliability Requirements

1. Wrapper spawn behavior:
   - check existing daemon health
   - recycle stale/incorrect version daemon
   - ensure reachable health endpoint
   - return success to caller regardless of pre-existing stale processes
2. PID cleanup:
   - remove legacy and current PID names
   - handle platform differences (Windows/macOS/Linux)
   - tolerate already-dead process and permission errors without hard fail
3. Startup diagnostics:
   - structured startup log to file
   - explicit startup stage markers
   - no stdout contamination

## 11) Performance Targets

1. `initialize` p95 < 120ms (warm daemon).
2. `tools/list` p95 < 80ms.
3. Context build + redact p95 < 180ms for 100KB merged payload.
4. Approval roundtrip p95 < 250ms (excluding user think time).
5. Memory overhead for panel runtime features < 40MB steady-state.

## 12) Testing Strategy (Required)

## 12.1 Contract Tests

1. Golden tests for initialize/tools/list across stdio transport.
2. Unknown notification handling is non-fatal.
3. Browser transport and stdio expose identical tool schema and semantics.

## 12.2 Reliability Tests

1. Startup under stale PID/process conditions.
2. Version-mismatch recycle behavior.
3. Restart/reconnect with pending command resolution.
4. Transport closed mid-request returns structured error and recovers.

## 12.3 Security/Redaction Tests

1. Synthetic secret corpus in errors/network/actions/logs.
2. Assert no secret in:
   - MCP responses
   - diagnostics files
   - exported session artifacts
   - model-bound request payload
3. Fail-closed path when redaction policy unavailable.

## 12.4 Approval Tests

1. Mutating action without token -> `approval_required`.
2. Expired token rejected.
3. Token scope mismatch rejected.
4. Single-use token cannot be replayed.

## 12.5 Cross-Platform Tests

1. Cleanup scripts pass on Linux/macOS/Windows.
2. Path handling for state/run directories is normalized.
3. Process termination behavior includes platform fallback commands.

## 13) Rollout Plan

## Phase 1: Browser Read-Only Agent UI
1. Add browser transport + session manager.
2. Enable chat/evidence rendering with read-only tool calls.
3. No mutating tool execution from panel.

Exit criteria:
1. Existing MCP clients unchanged.
2. Browser reconnect/resume stable.
3. Redaction counters visible in UI.

## Phase 2: Approval-Gated Mutations
1. Add approval engine and token flow.
2. Enable gated mutating interact actions.
3. Add watch mode with read-only auto-context only.

Exit criteria:
1. No ungated mutating execution path.
2. Approval flows fully covered by tests.

## Phase 3: Provider Adapter & Export
1. Add provider adapters (OpenAI/Anthropic/local).
2. Add redacted session export and replay metadata.
3. Add operational dashboards/alerts.

Exit criteria:
1. Provider failures isolated from daemon health.
2. Redaction and export tests green.

## 14) Operational Runbook Notes

1. Health endpoint must include:
   - `name: kaboom-browser-devtools`
   - `version`
   - `transport_status` (stdio/http)
2. Include diagnostic command for startup path:
   - wrapper stage
   - daemon pid/version
   - health handshake result
3. Ship one-command local verification script that validates:
   - stdio initialize
   - tools/list
   - browser transport initialize
   - redaction policy loaded

## 15) Open Decisions

1. Whether browser transport should be strict MCP-over-HTTP only vs MCP + typed event endpoint.
2. Whether approval tokens are strictly one-shot (recommended) or short-lived reusable.
3. Canonical redaction policy authority when extension and daemon disagree (recommend daemon-authoritative).
