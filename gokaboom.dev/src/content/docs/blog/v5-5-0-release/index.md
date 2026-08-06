---
title: "KaBOOM v5.5.0: Rock-Solid MCP Protocol Compliance"
description: "v5.5.0 delivers 100% MCP protocol compliance with critical fixes for stdio transport, notification handling, and response framing. Claude Desktop and Cursor now connect flawlessly."
date: 2026-02-04T21:09:00Z
authors:
  - brenn
tags:
  - releases
  - mcp
  - stability
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['--releases', 'releases', 'mcp', 'stability', 'blog', 'v5', 'release']
---

Kaboom v5.5.0 is a stability release focused on MCP protocol compliance. If you experienced "Unexpected end of JSON input" errors or connection issues with Claude Desktop or Cursor, this release fixes them all.

## The Problem: Intermittent Connection Failures

Users reported sporadic errors when connecting to KaBOOM via Claude Desktop:

```
[error] Unexpected end of JSON input
```

The MCP server appeared to be working — valid JSON responses were logged — but immediately after each response, a parse error occurred.

## Root Cause: Three Protocol Violations

Our investigation uncovered three distinct MCP protocol violations:

### 1. Double Newlines in stdio Output

Go's `json.Encoder.Encode()` adds a trailing newline to JSON output. Our stdio bridge then called `fmt.Println()`, adding a *second* newline. The empty line between messages was parsed as an empty JSON message, causing the parse error.

**Fix:** Changed `fmt.Println(string(body))` to `fmt.Print(string(body))` in the bridge — the HTTP response already includes the trailing newline.

### 2. Notification Responses

JSON-RPC 2.0 notifications (requests without an `id` field) must not receive responses. We were responding to `notifications/initialized` with an empty response, violating the spec.

**Fix:** Notifications now return `nil` from the handler and receive no response. HTTP transport returns 204 No Content.

### 3. Exit Race Condition

The stdio bridge could exit before the final response was written to stdout, truncating the last message.

**Fix:** Implemented an exit gate pattern — the process waits for any pending responses to flush before exiting.

## Comprehensive Protocol Tests

v5.5.0 adds 10 new Go tests that verify MCP protocol compliance:

- `TestMCPProtocol_ResponseNewlines` — exactly one trailing newline per response
- `TestMCPProtocol_NotificationNoResponse` — notifications receive no response
- `TestMCPProtocol_JSONRPCStructure` — valid JSON-RPC 2.0 structure
- `TestMCPProtocol_IDNeverNull` — response ID is never null (Cursor requirement)
- `TestMCPProtocol_ErrorCodes` — standard JSON-RPC error codes
- `TestMCPProtocol_InitializeResponse` — MCP initialize handshake
- `TestMCPProtocol_ToolsListStructure` — tools/list response format
- `TestMCPProtocol_HandlerUnit` — handler method dispatch
- `TestMCPProtocol_HTTPHandler` — HTTP transport compliance
- `TestMCPProtocol_BridgeCodeVerification` — static analysis of bridge code

These tests are designed to be **unalterable** — they verify the MCP spec, not implementation details. Any future change that breaks MCP compliance will fail these tests.

## Silenced Version Check Errors

The GitHub API version check now fails silently. Previously, rate limit errors (403) would log warnings even though version checking is non-critical.

## Prior Versions Deprecated

All npm packages prior to v5.5.0 have been deprecated. Users installing old versions will see a warning directing them to upgrade.

## Upgrade

```bash
npx kaboom-agentic-browser@5.5.0
```

Or update your MCP configuration:

```json
{
  "mcpServers": {
    "kaboom": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "kaboom-agentic-browser@5.5.0", "--port", "7890", "--persist"]
    }
  }
}
```

## Full Changelog

[GitHub Release](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases/tag/v5.5.0)
