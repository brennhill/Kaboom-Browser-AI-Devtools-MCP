---
status: proposed
scope: feature/self-testing/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-self-testing
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-self-testing.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Self Testing Review](./index.md).

# Tech Spec: AI Self-Testing Infrastructure

## Overview

This spec defines three features that enable AI agents to autonomously verify Kaboom's behavior during development and UAT. Together, they close the loop between "AI writes code" and "AI tests code" without requiring human browser interaction.

**Problem Statement:** During UAT, the AI agent can only verify server-side behavior via HTTP endpoints. It cannot:
1. Retrieve captured data (endpoints are POST-only for ingestion)
2. Invoke MCP tools (stdio-only, no HTTP interface)
3. Control a browser with the extension loaded (no automation hook)

**Solution:** Three complementary features that unlock full AI self-testing.

---

## Feature 32: HTTP GET for Captured Data

### Motivation

Current capture endpoints (`/enhanced-actions`, `/network-bodies`, `/websocket-events`) are POST-only — the extension pushes data to the server. There's no way to retrieve this data via HTTP; it's only accessible through MCP tools.

For AI self-testing, we need HTTP GET endpoints to verify data flows without requiring MCP access.

### Specification

#### New Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `GET /captured/actions` | GET | Return captured user actions |
| `GET /captured/network` | GET | Return captured network requests/responses |
| `GET /captured/websocket` | GET | Return captured WebSocket events |
| `GET /captured/logs` | GET | Return captured console logs |
| `GET /captured/errors` | GET | Return captured errors |

#### Query Parameters

All endpoints support:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 100 | Maximum entries to return |
| `since` | int64 | 0 | Unix timestamp (ms); only entries after this time |
| `format` | string | `json` | Response format: `json` or `jsonl` |

#### Response Format

```json
{
  "entries": [...],
  "count": 42,
  "oldest_ts": 1706123456789,
  "newest_ts": 1706123459012,
  "truncated": false
}
```

#### Security

- Localhost-only (already enforced by existing host permissions)
- No authentication required (same as existing endpoints)
- Optional: respect `--api-key` flag if Feature 20 is implemented

#### Implementation

```go
// In main.go, add route handlers
mux.HandleFunc("/captured/actions", h.handleGetActions)
mux.HandleFunc("/captured/network", h.handleGetNetwork)
mux.HandleFunc("/captured/websocket", h.handleGetWebSocket)
mux.HandleFunc("/captured/logs", h.handleGetLogs)
mux.HandleFunc("/captured/errors", h.handleGetErrors)

// Handler pattern
func (h *Handler) handleGetActions(w http.ResponseWriter, r *http.Request) {
    if r.Method != "GET" {
        http.Error(w, "Method not allowed", 405)
        return
    }
    limit := parseIntParam(r, "limit", 100)
    since := parseInt64Param(r, "since", 0)

    entries := h.capture.GetActions(limit, since)
    writeJSON(w, map[string]any{
        "entries":   entries,
        "count":     len(entries),
        "truncated": len(entries) == limit,
    })
}
```

#### Testing

```bash
# Verify actions captured
curl http://localhost:7777/captured/actions | jq '.count'

# Get recent network requests
curl "http://localhost:7777/captured/network?since=$(date +%s000)&limit=10"

# Stream as JSONL for processing
curl "http://localhost:7777/captured/logs?format=jsonl"
```

---

## Feature 33: HTTP MCP Endpoint (`/mcp`)

### Motivation

MCP tools are only accessible via stdio (JSON-RPC over stdin/stdout). This means:
- AI can't invoke tools from bash during self-testing
- Can't script tool calls in test automation
- Can't verify tool responses without spawning an MCP client

### Specification

#### Endpoint

```
POST /mcp
Content-Type: application/json
```

#### Request Format

Standard MCP JSON-RPC 2.0 request:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "observe",
    "arguments": {
      "what": "errors"
    }
  }
}
```

#### Response Format

Standard MCP JSON-RPC 2.0 response:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"errors\": [...]}"
      }
    ]
  }
}
```

#### Supported Methods

| Method | Description |
|--------|-------------|
| `tools/list` | List all available MCP tools |
| `tools/call` | Invoke an MCP tool |
| `initialize` | Return server capabilities |

#### Security

- Localhost-only (existing enforcement)
- Optional API key (`Authorization: Bearer <key>`) if Feature 20 implemented
- Rate limiting applies (if Feature 21 implemented)
- Audit logging applies (if Feature 12 implemented)

#### Implementation

```go
// Route handler
mux.HandleFunc("/mcp", h.handleHTTPMCP)

func (h *Handler) handleHTTPMCP(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", 405)
        return
    }

    var req JSONRPCRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSONRPCError(w, nil, -32700, "Parse error")
        return
    }

    // Route to existing MCP handler
    resp := h.handleMCPRequest(req)
    writeJSON(w, resp)
}
```

#### Testing

```bash
# List all tools
curl -X POST http://localhost:7777/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Call observe tool
curl -X POST http://localhost:7777/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
      "name": "observe",
      "arguments": {"what": "errors"}
    }
  }' | jq '.result.content[0].text | fromjson'

# Call highlight_element (requires AI Web Pilot enabled)
curl -X POST http://localhost:7777/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "highlight_element",
      "arguments": {"selector": "h1", "duration_ms": 2000}
    }
  }'
```

---

## Feature 34: AI-Runnable Playwright Harness

### Motivation

Features 32 and 33 give AI access to server data, but for true end-to-end testing, AI needs to:
- Launch a browser with the extension loaded
- Navigate to test pages
- Trigger user actions
- Verify capture pipeline works

Playwright can load unpacked extensions and automate Chrome. A harness script would provide AI with a complete testing environment.

### Specification

#### Script Location

```
scripts/uat-runner.js
```

#### Dependencies

```json
{
  "devDependencies": {
    "@anthropic-ai/sdk": "^0.30.0",
    "@playwright/test": "^1.40.0"
  }
}
```

#### Usage

```bash
# Run all UAT tests
node scripts/uat-runner.js

# Run specific phase
node scripts/uat-runner.js --phase 1

# Run with visible browser
node scripts/uat-runner.js --headed

# Output JSON results
node scripts/uat-runner.js --json > results.json
```

#### Architecture

```
┌─────────────────────────────────────────────────────────┐
│                   uat-runner.js                         │
├─────────────────────────────────────────────────────────┤
│  1. Build server (make dev)                             │
│  2. Spawn kaboom server on random port                │
│  3. Launch Playwright Chrome with extension loaded      │
│  4. Navigate to test page (localhost or fixture)        │
│  5. Execute test scenarios                              │
│  6. Verify via /mcp and /captured/* endpoints           │
│  7. Report results as JSON                              │
│  8. Cleanup: kill server, close browser                 │
└─────────────────────────────────────────────────────────┘
```

#### Test Scenarios

The harness runs test scenarios that map to UAT checklist phases:

```javascript
const SCENARIOS = {
  phase1: [
    {
      name: "1.1 Console error capture",
      setup: async (page) => {
        await page.evaluate(() => console.error("UAT test error"));
      },
      verify: async (api) => {
        const result = await api.mcp("observe", { what: "errors" });
        return result.errors?.some(e => e.message?.includes("UAT test"));
      }
    },
    {
      name: "1.2 Console log capture",
      setup: async (page) => {
        await page.evaluate(() => console.log("UAT test log"));
      },
      verify: async (api) => {
        const result = await api.mcp("observe", { what: "logs" });
        return result.logs?.some(l => l.message?.includes("UAT test"));
      }
    },
    // ... more scenarios
  ],

  phase4b: [
    {
      name: "4B.5 highlight_element",
      setup: async (page, api) => {
        // Enable AI Web Pilot toggle
        await page.evaluate(() => {
          chrome.storage.sync.set({ aiWebPilotEnabled: true });
        });
      },
      verify: async (api) => {
        const result = await api.mcp("highlight_element", {
          selector: "h1",
          duration_ms: 1000
        });
        return result.success === true && result.bounds;
      }
    },
    // ... more pilot scenarios
  ]
};
```

#### API Client

The harness provides a helper for calling server endpoints:

```javascript
class KaboomAPI {
  constructor(port) {
    this.baseUrl = `http://localhost:${port}`;
  }

  async mcp(tool, args = {}) {
    const resp = await fetch(`${this.baseUrl}/mcp`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        jsonrpc: '2.0',
        id: Date.now(),
        method: 'tools/call',
        params: { name: tool, arguments: args }
      })
    });
    const json = await resp.json();
    return JSON.parse(json.result?.content?.[0]?.text || '{}');
  }

  async captured(type, opts = {}) {
    const params = new URLSearchParams(opts);
    const resp = await fetch(`${this.baseUrl}/captured/${type}?${params}`);
    return resp.json();
  }

  async diagnostics() {
    const resp = await fetch(`${this.baseUrl}/diagnostics`);
    return resp.json();
  }
}
```

#### Extension Loading

Playwright can load unpacked extensions:

```javascript
const browser = await chromium.launchPersistentContext(userDataDir, {
  headless: false, // Extensions require headed mode
  args: [
    `--disable-extensions-except=${extensionPath}`,
    `--load-extension=${extensionPath}`,
  ],
});
```

#### Output Format

```json
{
  "timestamp": "2024-01-25T10:30:00Z",
  "duration_ms": 12345,
  "server_version": "5.0.0",
  "extension_version": "5.0.0",
  "phases": {
    "phase1": {
      "passed": 8,
      "failed": 0,
      "skipped": 0,
      "tests": [
        { "name": "1.1 Console error capture", "status": "passed", "duration_ms": 234 },
        { "name": "1.2 Console log capture", "status": "passed", "duration_ms": 189 }
      ]
    },
    "phase4b": {
      "passed": 12,
      "failed": 1,
      "skipped": 0,
      "tests": [
        { "name": "4B.5 highlight_element", "status": "passed", "duration_ms": 1023 },
        { "name": "4B.18 execute_javascript", "status": "failed", "error": "timeout", "duration_ms": 5000 }
      ]
    }
  },
  "summary": {
    "total": 45,
    "passed": 44,
    "failed": 1,
    "skipped": 0,
    "pass_rate": "97.8%"
  }
}
```

#### Test Fixture Page

For consistent testing, the harness includes a test fixture page:

```html
<!-- scripts/fixtures/test-page.html -->
<!DOCTYPE html>
<html>
<head><title>Kaboom UAT Test Page</title></head>
<body>
  <h1 id="title">Test Page</h1>
  <button id="action-btn">Click Me</button>
  <input id="input-field" type="text" value="initial">
  <form id="test-form">
    <input name="field1" value="value1">
    <button type="submit">Submit</button>
  </form>

  <script>
    // Error trigger
    window.triggerError = () => { throw new Error("Intentional test error"); };

    // Network trigger
    window.triggerFetch = () => fetch('/api/test').catch(() => {});

    // WebSocket trigger
    window.triggerWS = () => new WebSocket('ws://localhost:8080/ws');

    // Global for execute_javascript tests
    window.testGlobal = { value: 42, nested: { a: 1, b: 2 } };
  </script>
</body>
</html>
```

---

## Implementation Order

1. **Feature 32 (HTTP GET)** — Simplest, no dependencies, immediate value
2. **Feature 33 (/mcp endpoint)** — Medium complexity, enables tool testing
3. **Feature 34 (Playwright harness)** — Most complex, requires 32 and 33

## Success Criteria

After all three features are implemented:

1. AI can run `node scripts/uat-runner.js` during development
2. AI can verify all UAT checklist items without human interaction
3. Failed tests provide actionable error messages
4. Results are machine-readable JSON
5. Harness cleans up properly (no zombie processes)

## Migration Path

These features are additive — they don't change existing behavior. Existing UAT workflows continue to work. Teams can adopt incrementally:

1. Use `/captured/*` endpoints for quick HTTP-based verification
2. Use `/mcp` endpoint for scripted tool testing
3. Use Playwright harness for full E2E automation

---

## Appendix: Full UAT Checklist Mapping

| UAT Item | Verification Method |
|----------|---------------------|
| 1.1-1.8 | `/mcp` → `observe` + `/captured/*` |
| 2.1-2.9 | `/mcp` → `analyze`, `generate`, `configure` |
| 3.1-3.6 | `/mcp` → `security_audit`, `generate_csp`, etc. |
| 4.1-4.4 | `/mcp` → `observe`, `configure` |
| 4B.1-4B.36 | Playwright + `/mcp` → pilot tools |
| 5.1-5.5 | Playwright navigation + `/captured/*` |
| 5B.1-5B.16 | Playwright + `/mcp` → `observe`, `generate` |
| 6.1-6.4 | `/diagnostics`, `--version`, Playwright evaluate |
