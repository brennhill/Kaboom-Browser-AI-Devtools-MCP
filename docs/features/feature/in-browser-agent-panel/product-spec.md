---
feature: in-browser-agent-panel
status: proposed
tool: observe, analyze, interact, configure
mode: agent_panel, agent_watch
version: null
authors: [brenn]
created: 2026-02-16
updated: 2026-02-16
doc_type: product-spec
feature_id: feature-in-browser-agent-panel
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: In-Browser Agent Panel (Redaction-First)

User-facing requirements and boundaries for turning Kaboom into an in-browser agent experience without weakening core reliability or data safety.

- See also: [Tech Spec](tech-spec.md)
- See also: [Runtime Spec (Go Orchestrator)](runtime-spec-go-orchestrator.md)
- See also: [Core Product Spec](../../../core/product/product-spec.md)

## Problem

Current usage requires context-switching between IDE chat and browser behavior. This slows debugging loops, reduces situational awareness, and makes it harder to maintain a consistent investigation thread.

At the same time, automatically piping browser context to an agent increases risk of exposing secrets unless redaction is enforced at all output surfaces.

## Solution

Add an in-browser Agent Panel that:

1. Uses existing Kaboom capabilities (`observe`, `analyze`, `interact`, `generate`) as the execution core.
2. Adds optional event-driven context piping for common debugging triggers.
3. Enforces mandatory redaction before any data is shown, stored, or sent to an agent model.

This keeps Kaboom's deterministic core unchanged while improving day-to-day agent workflow.

## User Stories

- As a developer, I want to open a chat inside the browser so I can debug in context without switching windows.
- As an LLM agent, I want structured auto-context (errors, network failures, recent actions) so I can propose fixes faster.
- As a security-conscious user, I want redaction to be mandatory so secrets never appear in logs, diagnostics, or chat payloads.
- As a maintainer, I want existing MCP clients (Codex/Claude/Gemini) to keep working unchanged.

## MCP Interface

The panel is a client over the existing MCP tool surface.

### Required Existing Calls

```json
{
  "tool": "observe",
  "arguments": { "what": "error_bundles" }
}
```

```json
{
  "tool": "interact",
  "arguments": { "action": "click", "selector": "#submit" }
}
```

### Planned Additions (Proposed)

```json
{
  "tool": "configure",
  "arguments": {
    "action": "agent_watch",
    "operation": "enable",
    "events": ["errors", "network_errors", "security"],
    "rate_limit_per_min": 20
  }
}
```

```json
{
  "tool": "observe",
  "arguments": {
    "what": "agent_context",
    "window_seconds": 30
  }
}
```

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | Agent Panel runs as an optional browser-side UI and does not replace existing MCP clients. | must |
| R2 | Auto-piped context is disabled by default and requires explicit user opt-in. | must |
| R3 | Mandatory redaction applies to all panel-visible and model-bound payloads. | must |
| R4 | Any mutating agent action (click/type/navigate/upload/execute_js) requires user confirmation by default. | must |
| R5 | Panel can attach relevant context bundles automatically for error/debug workflows. | should |
| R6 | Panel sessions can be exported as a reproducible artifact (redacted). | should |
| R7 | Users can tune auto-pipe event classes and rate limits. | should |

## Non-Goals

- This feature does NOT make Kaboom autonomous by default.
- This feature does NOT send data to a cloud model unless the user configures a provider.
- This feature does NOT bypass extension/browser permission boundaries.
- This feature does NOT remove existing external MCP workflows.

## Performance SLOs

| Metric | Target |
|--------|--------|
| Panel initial render after open | < 500ms |
| Auto-context packet build (typical) | < 150ms |
| Redaction overhead per 50KB payload | < 15ms |
| Context event drop rate under normal load | 0% |

## Security Considerations

- Redaction is mandatory, defense-in-depth:
  1. Extension/source redaction.
  2. Server-side redaction before persistence and before response.
  3. Agent egress redaction before model-bound payload assembly.
- Fail-closed behavior: if redaction policy cannot load or execute, payload is dropped and surfaced as a safe error.
- Stable placeholders (for example `[REDACTED_TOKEN]`) preserve debugging utility without exposing values.
- Mutating commands require explicit approval unless user intentionally enables a relaxed mode.

## Edge Cases

- Extension disconnected: panel shows degraded mode, no silent fallback to stale context.
- Redaction engine unavailable: block outbound payload and show actionable diagnostics.
- High event volume: rate-limited auto-pipe with dedup and summary rollups.
- CSP-restricted pages: panel reports execution limits and recommends supported alternatives.
- Multiple tabs/windows: context is tab-scoped and visibly labeled.

## Dependencies

- Depends on: existing async queue + correlation ID architecture, extension connectivity, redaction engine.
- Depended on by: demo flows, guided debugging workflows, future agentic CI handoff exports.

## Assumptions

- A1: Existing MCP contracts remain stable and are the primary execution interface.
- A2: Redaction patterns and policies can be versioned and loaded at startup.
- A3: Users may run without a configured model provider (panel still useful for local triage).

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Should agent model execution be in-extension, local daemon, or pluggable provider adapter only? | open | Recommend pluggable adapter with local-first default. |
| OI-2 | Should read-only actions be auto-approved while mutating actions remain gated? | open | Likely yes, but needs explicit UX copy. |
| OI-3 | Should `agent_watch` be a new tool/action or a `configure` extension action? | open | Prefer `configure` for contract stability. |
| OI-4 | What minimum redaction policy version blocks panel startup? | open | Proposed: hard fail below policy schema v2. |
