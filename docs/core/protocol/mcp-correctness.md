---
status: active
scope: architecture/mcp
ai-priority: high
tags: [mcp, correctness, constraints, reference]
relates-to: [../../.claude/refs/architecture.md, ../../.claude/refs/async-command-architecture.md]
last-verified: 2026-02-11
---

# MCP Correctness

**See Also:** [.claude/refs/architecture.md](../../../.claude/refs/architecture.md) (canonical system design)

Kaboom MCP implements [Model Context Protocol](https://modelcontextprotocol.io/specification/2025-11-25) JSON-RPC semantics with a stdio client boundary and a local HTTP bridge.

**Protocol Version:** `2024-11-05` (with version negotiation for `2025-11-25`)

## Compliance Status

**All MUST constraints:** ✅ PASS
**Intentional deviations:** 3 (documented below)
**Transport:** stdio JSON-RPC for MCP clients, local `/mcp` HTTP bridge for shared daemon

## Violations Summary

**Current:** None (all violations fixed)

## Intentional Deviations

| ID | Deviation | Justification |
|----|-----------|---------------|
| L-6, J-6 | Respond to `initialized` notification with `{}` | Some MCP clients (including Claude Code) expect a response. Removing it could break compatibility. Track spec evolution. |
| — | `_meta` field on tools with `data_counts` | Non-standard but uses `_` prefix convention. Provides AI with buffer state without extra tool call. No spec conflict. |
| — | `X-Kaboom MCP-Client` header for multi-client | Not part of MCP spec. Internal transport-layer addition for our `/mcp` HTTP bridge. |
| — | Only `type: "text"` content blocks | MCP allows image/resource content types. All our data is textual. No spec violation — we just don't use all content types. |

## Key Implementation Details

**Capabilities declared:**
- `tools` — 5 tools: observe, generate, configure, interact, analyze
- `resources` — 2 resources: `kaboom://guide`, `kaboom://quickstart`
- `resourceTemplates` — 1 template: `kaboom://demo/{name}`

**Error handling:**
- Three-tier model: transport (HTTP status) → protocol (JSON-RPC error) → application (`isError: true`)
- Tool execution failures use `isError: true` (not JSON-RPC errors)
- Rate limiting: 100 tool calls per minute per client

**Security:**
- Server binds `127.0.0.1` only
- Origin validation with strict localhost/extension allowlist
- Sensitive data redaction in all tool outputs
- Sensitive fields redacted in bridge diagnostics and debug logs

**Transport:**
- **Primary (MCP clients):** stdio JSON-RPC (`npx kaboom-agentic-browser` process per MCP client)
  - Silent stdio transport contract (no non-protocol stdout/stderr noise)
  - JSON-RPC 2.0 request/response semantics
  - Notification handling per MCP/JSON-RPC constraints
- **Bridge transport (internal):** `POST /mcp` on localhost daemon
  - Stdio wrapper proxies MCP calls to shared HTTP daemon
  - Enables one persistent server shared by multiple MCP clients
  - Browser extension also uses local HTTP endpoints for telemetry and async command exchange

## Test Coverage

80+ MCP constraints covered by:
- `mcp_protocol_test.go` — JSON-RPC protocol correctness and invariants
- `connection_lifecycle_test.go` — startup/retry/recovery lifecycle invariants
- `multi_client_test.go` — concurrent client isolation behavior
- `handler_unit_test.go` — HTTP bridge handler behavior and redaction paths

## References

- [MCP 2024-11-05 Specification](https://spec.modelcontextprotocol.io/specification/2024-11-05/basic/transports/)
- [MCP Transport Concepts](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports)
