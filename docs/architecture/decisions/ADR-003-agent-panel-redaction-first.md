---
adr: ADR-003
feature: in-browser-agent-panel
status: proposed
date: 2026-02-16
last_reviewed: 2026-03-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# ADR-003: In-Browser Agent Panel Must Be Redaction-First and Layered

## Context

We want Kaboom to support a browser-native agent workflow (chat + action orchestration) to reduce debugging friction. A naive implementation risks:

1. coupling UI-agent behavior to core daemon stability,
2. bypassing existing MCP contracts,
3. leaking secrets when auto-context is piped to model prompts.

Kaboom has a strict reliability requirement: startup and core protocol behavior must remain dependable for all external clients.

## Decision

We adopt a layered architecture:

1. Keep daemon + MCP tools as deterministic core.
2. Implement the in-browser Agent Panel as an optional client/orchestrator layer.
3. Enforce mandatory redaction at source, server, and model-egress stages.
4. Gate mutating actions behind explicit user approval tokens by default.
5. Treat redaction engine failures as fail-closed for model egress.

The panel may add additive interfaces (for example watch-mode configuration), but must not break existing MCP behavior.

## Consequences

### Positive

- Preserves compatibility with Codex/Claude/Gemini MCP clients.
- Reduces context switching for browser-centric debugging.
- Establishes a clear, auditable secret-safety contract.
- Limits blast radius: panel failures do not destabilize core daemon.

### Negative

- Higher implementation complexity than a single monolithic "agent mode."
- Additional policy/version coordination between extension and daemon.
- More test surface (redaction + approvals + watch-mode throttling).

### Neutral

- Some workflows remain intentionally manual (approval prompts).
- Initial rollout should prioritize read-only panel capability.

## Alternatives Considered

### Alternative A: Monolithic in-browser agent (replace MCP model)

- Description: move logic into extension-side agent runtime and reduce MCP usage.
- Rejected because: increases fragility, reduces testability, and risks protocol drift.

### Alternative B: Keep only external IDE chat clients

- Description: no in-browser panel, continue current multi-window workflow.
- Rejected because: preserves reliability but does not address major UX friction.

### Alternative C: Optional redaction in panel only

- Description: allow users to disable redaction for "full fidelity."
- Rejected because: unacceptable security posture and high leak risk in logs/exports/prompts.

## References

- Feature spec: [In-Browser Agent Panel Product Spec](../../features/feature/in-browser-agent-panel/product-spec.md)
- Technical design: [In-Browser Agent Panel Tech Spec](../../features/feature/in-browser-agent-panel/tech-spec.md)
- Related architecture policy: [ADR-002 Async Queue Immutability](ADR-002-async-queue-immutability.md)
