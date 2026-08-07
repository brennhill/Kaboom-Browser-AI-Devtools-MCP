---
status: active
scope: architecture/mcp
ai-priority: high
tags: [mcp, correctness, constraints, reference]
relates-to: [../../.claude/refs/architecture.md, ../../.claude/refs/async-command-architecture.md]
last-verified: 2026-08-07
---

# MCP Correctness

**See Also:** [.claude/refs/architecture.md](../../../.claude/refs/architecture.md) (canonical system design)

Kaboom MCP implements Model Context Protocol JSON-RPC semantics with a stdio
client boundary and a local HTTP bridge.

**Protocol Versions:** `2024-11-05` and `2025-06-18`, negotiated from the
client initialize request

## Compliance Status

**All MUST constraints:** ✅ PASS
**Intentional deviations:** None
**Transport:** stdio JSON-RPC for MCP clients, local `/mcp` HTTP bridge for shared daemon

## Violations Summary

**Current:** None (all violations fixed)

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

MCP constraints are owned by the boundary they protect:

- `cmd/browser-agent/internal/mcprouter/router_test.go` — JSON-RPC envelope,
  method, ID, version-negotiation, and notification semantics
- `cmd/browser-agent/internal/mcpprotocol/responses_test.go` — initialize,
  discovery, and resource response contracts
- `cmd/browser-agent/internal/mcphttp/handler_test.go` — HTTP request parsing,
  error codes, notification status, and newline framing
- `cmd/browser-agent/internal/bridge/bridge_unit_test.go` — line and
  content-length stdio framing, including invalid-payload recovery
- `scripts/uat/protocol/test-mcp-spec-compliance.sh` — installed-artifact
  protocol verification through the complete bridge and daemon stack

## References

- [MCP 2024-11-05 Specification](https://spec.modelcontextprotocol.io/specification/2024-11-05/basic/transports/)
- [MCP Transport Concepts](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports)
