---
status: proposed
scope: feature/enterprise-audit/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-enterprise-audit
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-enterprise-audit.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Enterprise Audit Review](./index.md).

# Technical Spec: Enterprise Audit & Governance

## Purpose

Kaboom captures sensitive browser state — console logs, network bodies, WebSocket payloads, DOM content, and accessibility trees. In enterprise environments, AI tools accessing this data create legitimate questions: Who called which tool? What data was exposed? How long is it retained? Can runaway AI agents be constrained?

This specification defines four tiers of enterprise-readiness features that give teams visibility, control, and auditability over how AI agents interact with captured browser data through Kaboom's MCP interface.

---

## Compliance Posture

Kaboom runs exclusively on the developer's machine. Data never leaves localhost. This architecture provides a natural compliance advantage: captured browser state is never transmitted to external services, never stored in cloud infrastructure, and never accessible to other users or processes.

The enterprise features in this spec do not claim to make Kaboom "SOC2-compliant" or "HIPAA-compliant" — compliance is an organizational property, not a tool property. What these features provide is **evidence and control**: audit trails that demonstrate what data an AI agent accessed, retention policies that limit exposure windows, and redaction that prevents sensitive patterns from reaching AI clients. These are building blocks that help organizations satisfy their own compliance requirements, not compliance certifications themselves.

For teams operating in regulated environments, Kaboom's localhost-only architecture means it sits outside the scope of most data-handling regulations (data is not "in transit" or "at rest" in a regulated sense — it is ephemeral runtime state on a developer workstation). The audit and governance features exist to satisfy internal security policies and provide forensic evidence if needed, not to meet external audit frameworks.

---

## Tier 1: AI Audit Trail

### 1.1 Tool Invocation Log

Every MCP tool call produces an audit entry. The log captures what was called, by whom, when, and the outcome — without storing the full response content (which may be large and sensitive).

#### How It Works

When the MCP dispatcher receives a tool call, it records an entry before executing the tool. After execution, it updates the entry with the result metadata (duration, response size, error if any). The log is a fixed-size ring buffer (configurable, default 10,000 entries) stored in memory. When the buffer is full, the oldest entries are evicted — this is a recent-history query tool, not a permanent archive.

Each audit entry contains:
- Timestamp (ISO 8601 with millisecond precision)
- Session ID (see 1.3)
- Client identity (see 1.2)
- Tool name (e.g., `observe`, `analyze`, `generate`)
- Parameter summary (tool name and key parameters, not full request body)
- Response size in bytes
- Duration in milliseconds
- Status (success, error, rate-limited, redacted)
- Redaction count (how many fields were redacted in the response, see 1.4)

The log is queryable via `configure({what:"audit_log"})` with filters for time range, tool name, session ID, and status. Responses return entries in reverse chronological order with pagination.

#### What the Log Does NOT Store

- Full request parameters (may contain sensitive queries)
- Full response bodies (may contain captured PII)
- Authentication credentials
- Raw network bodies or DOM content

The log stores metadata about access, not the accessed data itself. This keeps the audit log small and non-sensitive while still providing a complete access trail.

#### Lifecycle

The audit log lives in memory and is lost on server restart. This matches Kaboom's ephemeral design — the server is spawned for a session and dies when done. For teams that need a durable record, the `export_data` tool (see 2.3) can dump the current audit buffer to a file before the session ends. This is an explicit action, not a background persistence mechanism.

---

### 1.2 Client Identification

Each MCP connection identifies the connecting client. This information is recorded on every audit entry and available in health metrics.

#### How It Works

When an MCP client connects (over stdio), Kaboom identifies it through the MCP handshake. During the MCP `initialize` request, the client sends its `clientInfo` object (part of the MCP specification). Kaboom extracts:

- Client name (e.g., "claude-code", "cursor", "windsurf", "continue")
- Client version

If the client doesn't send `clientInfo` (older clients), Kaboom labels the connection as "unknown".

#### Storage

Client identity is stored per-session (see 1.3) and referenced by all audit entries for that session. It is not separately persisted — it lives as a field on the session record.

---

### 1.3 Session ID Assignment

Each MCP connection receives a unique session ID that correlates all tool calls within that connection's lifetime.

#### How It Works

When an MCP client sends the `initialize` request, Kaboom generates a session ID using a compact format: a base-36 timestamp prefix plus 6 random characters (e.g., `s_m5kq2a_7f3xab`). This provides chronological sortability, uniqueness (36^6 = 2.2 billion combinations per timestamp unit), and human-readability.

The session ID is:
- Returned in the `initialize` response as a server capability extension
- Included in every audit log entry for that connection
- Available via `configure({what:"health"})` for active session listing
- Logged on connection close with total tool call count and duration

Sessions end when the stdio pipe closes (MCP client disconnects or process dies). The server detects this via EOF on stdin.

#### Current Limitation: Single Session

The current architecture supports exactly one MCP session over stdio (one `bufio.Scanner` reading stdin). This means one AI client per Kaboom server instance. The session ID is still valuable for correlating tool calls within that session's lifetime and distinguishing across server restarts.

#### Future: Multi-Session Support

To support multiple concurrent AI clients, the server would need to move from stdio-based MCP to an HTTP-based MCP transport (JSON-RPC over HTTP POST). Each HTTP connection would carry a session token (assigned during `initialize`), and the server would maintain a map of active sessions. This is a protocol-layer change that does not affect the audit, redaction, or rate-limiting logic — those already reference session IDs abstractly. Multi-session support is not in scope for the initial implementation.

---

### 1.4 Redaction Audit Log

When data is redacted from a tool response (by configured redaction patterns, see Tier 2.4), the redaction itself is logged. This creates an audit trail proving that sensitive data was protected, without storing the sensitive data.

#### How It Works

Each time a redaction pattern matches content in a tool response, a redaction event is recorded containing:
- Timestamp
- Session ID
- Tool name that produced the response
- Field path where the match occurred (e.g., `entries[3].message`, `body.response`)
- Pattern name that triggered the redaction (e.g., "credit-card", "bearer-token")
- Number of characters redacted (length only, not content)

Redaction events are included in the audit log as a sub-entry of the tool invocation that triggered them. They are also separately queryable via `get_audit_log` with a `type: "redaction"` filter.

#### What Is NOT Stored

The redacted content itself is never stored anywhere — not in the audit log, not in a side channel, nowhere. The audit log proves "a credit card pattern was found and removed from field X" without revealing the card number.

---

## Tier 2: Data Governance

### 2.1 TTL-Based Retention

All captured data (console logs, network entries, WebSocket events, actions, network bodies) has a configurable time-to-live. Entries older than the TTL are automatically evicted during normal buffer operations.

#### How It Works

The server maintains a TTL value (default: unlimited, meaning data lives until buffer rotation evicts it). When TTL is set (via `--ttl` flag or config), every buffer read operation includes an age check. Entries with timestamps older than `now - TTL` are skipped on read — they are effectively invisible to tool responses even though they may still occupy buffer space briefly until the next write evicts them.

TTL values:
- Minimum: 1 minute
- Default: unlimited (existing behavior — buffer size limits still apply)
- Typical values: 1 hour for normal use, 15 minutes for sensitive environments

TTL applies to:
- Console log entries
- Network request/response metadata
- Network body storage
- WebSocket event buffer
- Action buffer
- Performance snapshots
- Checkpoint data

TTL does NOT apply to:
- Audit log entries (those have their own retention, governed by buffer size)
- Noise rules (configuration, not captured data)
- Server configuration state

#### TTL is Set at Startup

TTL is configured at server start and does not change during a session. This matches the ephemeral server design — if you need a different TTL, restart the server with the new value. Runtime configuration changes add complexity without clear value for a tool that runs for the duration of a coding session.

---

### 2.2 Configuration Profiles

Named configuration bundles that set multiple enterprise features to sensible defaults for common security postures. Profiles are a convenience — they set the same flags that operators could set individually. They describe a behavior level, not a compliance framework.

#### Available Profiles

**`short-lived`** — Minimal data retention for sensitive work:
- TTL: 15 minutes
- Redaction: bearer tokens, API keys, JWTs, session cookies
- Rate limits: default

**`restricted`** — Limited AI access, aggressive redaction:
- TTL: 30 minutes
- Redaction: all built-in patterns enabled
- Rate limits: `analyze` limited to 10/min, `observe` limited to 10/min
- Read-only mode: enabled (no mutation tools)

**`paranoid`** — Maximum restriction:
- TTL: 5 minutes
- Redaction: all built-in patterns enabled
- Rate limits: 30 calls/min per tool (global cap)
- Read-only mode: enabled
- Tool allowlist: `observe`, `analyze`, `configure` only

#### How Profiles Work

Profiles are applied at server start via `--profile=restricted`. They set defaults for all related flags. Individual flags can still override profile values (e.g., `--profile=restricted --ttl=1h` uses restricted defaults but overrides TTL to 1 hour).

Profiles are not runtime-changeable — they are a startup configuration. The active profile name is exposed in the health endpoint.

---

### 2.3 Data Export

An MCP tool for exporting current buffer state and audit entries as structured data. This keeps the server ephemeral — no background file writes, no persistent state — while still allowing teams to preserve session data when needed.

#### How It Works

A new MCP tool `export_data` returns the requested data as the tool response (JSON Lines format, one JSON object per line). The tool accepts:
- Scope: `audit`, `captures`, `all`
- Time range: start/end timestamps (optional, defaults to all available data)

For `captures` scope, the response includes all buffer contents (console, network, WebSocket, actions, performance) serialized as JSON objects, one per line. Each line includes a `type` field identifying the data category.

For `audit` scope, the response includes all audit log entries currently in the ring buffer.

The export is a point-in-time snapshot — data continues to be captured and evicted normally during the export. The AI client receiving the response can write it to a file, send it to a log aggregator, or process it however the team's tooling requires. Kaboom does not write files or manage export directories.

---

### 2.4 Configurable Redaction Patterns

Operators define regex patterns that automatically redact sensitive data from MCP tool responses before they reach the AI client. Redaction happens at the response boundary — the server stores unredacted data internally but never exposes it through MCP.

#### How It Works

Redaction patterns are configured via a JSON configuration file (`--redaction-config`) or CLI flags. Each pattern has:
- Name: human-readable identifier (e.g., "credit-card", "bearer-token")
- Pattern: regex applied to string fields in tool responses
- Replacement: what to substitute (default: `[REDACTED:{name}]`)
- Scope: which tool responses to apply to (`all`, or specific tool names)

Patterns are applied after the tool generates its response but before serialization to the MCP client. The server walks all string fields in the response JSON, applying each active pattern. Matches are replaced with the configured replacement string.

#### Limitation: No Retroactive Protection

Redaction only applies to tool responses generated after patterns are configured. If an AI client has already received unredacted data from earlier tool calls (before redaction was enabled), that data is already in the client's context and cannot be recalled. This is an inherent limitation of response-boundary redaction.

To avoid this gap, configure redaction patterns at server start (via `--redaction-config` or `--profile`) rather than enabling them mid-session. When patterns are active from the first tool call, no unredacted data is ever exposed to the AI client through MCP.

#### Built-In Patterns

The server ships with these patterns (disabled by default, enabled by profiles or explicit config):

- `bearer-token`: `Bearer [A-Za-z0-9\-._~+/]+=*` — OAuth bearer tokens
- `api-key`: `(?i)(api[_-]?key|apikey|secret[_-]?key)\s*[:=]\s*\S+` — API keys in various formats
- `credit-card`: `\b[0-9]{4}[- ]?[0-9]{4}[- ]?[0-9]{4}[- ]?[0-9]{4}\b` — Credit card number patterns
- `ssn`: `\b[0-9]{3}-[0-9]{2}-[0-9]{4}\b` — US Social Security Numbers
- `email`: `(?i)\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b` — Email addresses
- `jwt`: `eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*` — JSON Web Tokens
- `session-cookie`: `(?i)(session|sid|token)\s*=\s*[A-Za-z0-9+/=_-]{16,}` — Session identifiers

All patterns are pure RE2 regex — no post-match validation functions. This keeps the redaction path simple and predictable. False positives are acceptable for a dev tool (better to over-redact than under-redact).

#### Custom Patterns

Operators add custom patterns in the config file:

```json
{
  "patterns": [
    {"name": "internal-id", "pattern": "CUST-[0-9]{8}", "replacement": "[REDACTED:customer-id]"},
    {"name": "mrn", "pattern": "MRN[0-9]{7,10}", "replacement": "[REDACTED:medical-record]"}
  ]
}
```

#### Performance

Redaction adds latency to tool responses. With the built-in patterns on a typical response (< 5KB text), the overhead is under 2ms. Large responses (100KB+ network bodies, full DOM trees) may see 10-30ms. This is acceptable given the security benefit and because these large responses are already slow to generate. RE2 guarantees linear-time matching — no catastrophic backtracking regardless of input.

---

## Tier 3: Operational Safety

### 3.1 API Key Authentication

Optional authentication for the HTTP API that prevents unauthorized clients from connecting to the Kaboom server. When enabled, all HTTP requests (from extension, direct API) must include the key.

#### How It Works

When started with `--api-key=<secret>` or `KABOOM_API_KEY` environment variable, the server requires an `Authorization: Bearer <secret>` header on all HTTP requests. Requests without the header or with the wrong key receive 401 Unauthorized.

The key is a shared secret — the same key is configured in the extension settings and the server. This is not intended for multi-user authentication (see Tier 4 for multi-tenant), but for preventing accidental connections from other tools on localhost.

MCP connections (stdio) are not affected by API key auth — stdio inherently authenticates via the process model (only the parent process can connect).

#### Extension Configuration

The extension's options page gains a "Server API Key" field. When set, the extension includes the key in all POST requests to the server. The key is stored in `chrome.storage.local` (encrypted at rest by Chrome).

#### Key Rotation

To rotate the key, restart the server with the new key and update the extension setting. During the brief window between server restart and extension update, the extension will receive 401s and enter its normal backoff behavior.

---

### 3.2 Per-Tool Rate Limits

Configurable rate limits that apply per MCP tool. This prevents a malfunctioning or malicious AI agent from hammering expensive paths (like `analyze` calls for DOM analysis or `observe` calls for `network_bodies`) in a tight loop.

#### How It Works

Each tool has a configurable maximum calls per minute. When exceeded, the tool responds with an MCP error containing a retry-after hint. The rate limit resets every 60 seconds on a sliding window.

Default limits (can be overridden):
- `observe`: 60/min (high — common read operation)
- `analyze`: 20/min (expensive — triggers round-trip to browser for extension-backed modes like `what:"dom"`)
- `generate`: 30/min (moderate — codegen is CPU-bound)
- `analyze`: 30/min (moderate)
- `get_audit_log`: 10/min (low — audit queries shouldn't be in hot loops)
- `export_data`: 2/min (very low — exports serialize all buffer data)

Limits are configured via `--rate-limits` flag or config file:
```
--rate-limits="analyze=10,generate=5,observe=120"
```

#### Rate Limit Response

When rate-limited, the tool returns a standard MCP error response:
```json
{
  "error": {
    "code": -32029,
    "message": "Rate limit exceeded for tool 'analyze': 20/min. Retry after 45s.",
    "data": {"tool": "analyze", "limit": 20, "window": "1m", "retry_after_seconds": 45}
  }
}
```

The error code -32029 is in the MCP application error range. Well-behaved AI clients should respect the retry hint.

#### Interaction with Global Rate Limiter

Per-tool rate limits operate at the MCP tool layer (calls per minute). The existing global rate limiter operates at the HTTP event layer (events per second for data ingestion). These are independent:

- Per-tool limits constrain how often an AI client can query data
- Global rate limits constrain how fast the extension can push data

If the global circuit breaker opens (HTTP 429), MCP tool calls are unaffected — they read from in-memory buffers, not from incoming HTTP data. If a per-tool limit is hit, it does not affect other tools or the global rate limiter. The two systems do not conflict because they operate on different paths (MCP reads vs. HTTP writes).

---

### 3.3 Configurable Thresholds

All hardcoded server limits become configurable via CLI flags, environment variables, or a config file. This replaces the current approach of fixed constants in source code.

#### Configurable Values

| Parameter | Default | Flag | Env |
|-----------|---------|------|-----|
| Console buffer size | 1000 entries | `--buffer-console` | `KABOOM_BUFFER_CONSOLE` |
| Network buffer size | 500 entries | `--buffer-network` | `KABOOM_BUFFER_NETWORK` |
| WebSocket buffer size | 1000 events | `--buffer-websocket` | `KABOOM_BUFFER_WEBSOCKET` |
| Action buffer size | 200 actions | `--buffer-actions` | `KABOOM_BUFFER_ACTIONS` |
| Network body max size | 100KB | `--body-max-size` | `KABOOM_BODY_MAX_SIZE` |
| Network body store count | 50 bodies | `--body-max-count` | `KABOOM_BODY_MAX_COUNT` |
| Memory soft limit | 20MB | `--memory-soft` | `KABOOM_MEMORY_SOFT` |
| Memory hard limit | 50MB | `--memory-hard` | `KABOOM_MEMORY_HARD` |
| Global rate limit | 1000 events/sec | `--rate-limit` | `KABOOM_RATE_LIMIT` |
| Server port | 7890 | `--port` | `KABOOM_PORT` |
| TTL | unlimited | `--ttl` | `KABOOM_TTL` |
| Audit log size | 10000 entries | `--audit-size` | `KABOOM_AUDIT_SIZE` |

#### Config File

As an alternative to flags, a JSON config file can be specified via `--config`:

```json
{
  "buffers": {
    "console": 2000,
    "network": 1000,
    "websocket": 5000,
    "actions": 500
  },
  "memory": {
    "soft_limit": "40MB",
    "hard_limit": "100MB"
  },
  "retention": {
    "ttl": "2h",
    "audit_size": 50000
  },
  "profile": "restricted"
}
```

JSON is used because `encoding/json` is in Go stdlib — no parsing library needed. The config file should NOT contain secrets (API keys, etc.) — those belong in environment variables.

Priority order: CLI flags > environment variables > config file > defaults.

---

### 3.4 Health & SLA Metrics

`configure({what:"health"})` exposes server operational state for monitoring and alerting. This is distinct from the existing `/health` HTTP endpoint (which only returns circuit breaker state) — it provides comprehensive server metrics.

#### What It Reports

The tool returns:
- **Server**: version, uptime, PID, platform, go version
- **Memory**: current allocation, buffer breakdown by category, percent of hard limit used
- **Buffers**: entries/capacity for each buffer type, eviction counts since startup
- **Rate limiting**: current event rate, circuit breaker state, total 429s issued
- **Session**: session ID, client identity, connection duration, tool call count
- **Audit**: total tool calls since startup, calls per tool breakdown, error count
- **Configuration**: active profile name (if any), TTL value, redaction patterns active count, read-only status

#### Use Cases

- **Debugging**: When an AI agent reports missing data, check buffer utilization and eviction counts
- **Monitoring**: Track memory usage trends across long sessions
- **Configuration verification**: Confirm TTL, redaction, and rate limits are active as expected

---

## Tier 4: Multi-Tenant & Access Control

### 4.1 Project Isolation

Multiple isolated capture contexts on a single Kaboom server. Each project has independent buffers and noise rules. The primary use case is a developer working on multiple applications simultaneously (e.g., a frontend and a backend admin panel in separate browser tabs) who wants clean separation of captured data.

#### How It Works

Projects are created via a new HTTP endpoint `POST /projects` with a name and optional configuration overrides. Each project gets:
- Independent buffer sets (console, network, WebSocket, actions, performance)
- Independent checkpoint storage
- Independent noise rules
- Optional per-project TTL

The extension specifies which project to send data to via a project ID in its configuration (options page). MCP clients specify the project via a parameter on the `initialize` request.

Default behavior (no project specified) uses a "default" project, maintaining backward compatibility with existing single-project deployments.

#### Memory Model

Projects share a single global memory budget — they do not each get an independent hard limit. The server maintains one global allocator that tracks total memory across all projects. When total memory approaches the hard limit, eviction is triggered in the project with the highest memory usage (largest-first eviction).

This means:
- 1 project uses up to 50MB (the full hard limit) — identical to today
- 3 projects share 50MB with largest-first eviction — each effectively gets ~16MB under balanced load
- The global hard limit is never exceeded regardless of project count

Maximum number of projects is configurable (default 5, max 10). Each project has a minimum guaranteed allocation of 2MB — if adding a project would drop per-project minimums below 2MB, creation fails. This prevents the "10 projects × 5MB = useless" problem — fewer, larger projects are better than many tiny ones.

#### Project Lifecycle

Projects persist in memory for the server's lifetime. They can be deleted via `DELETE /projects/{id}`, which immediately frees all buffers. There is no persistence across server restarts — projects are recreated on next connection.

---

### 4.2 Read-Only Mode

A server mode that accepts captured data from the extension but disables all MCP tools that modify server state. This creates a "observe but don't touch" deployment suitable for shared environments or untrusted AI clients.

#### What Is Disabled

In read-only mode, these operations return an error:
- `configure` with action `clear` (clearing buffers)
- `configure` with action `dismiss` (modifying noise rules)
- `configure` with action `noise_rule` (adding/removing noise rules)
- Checkpoint deletion
- Any future mutation tool

#### What Remains Active

- All observe tools (reading logs, network, WebSocket events, actions)
- All analyze tools (performance, API schema, accessibility, changes, timeline)
- All generate tools (reproduction scripts, tests, PR summaries, SARIF, HAR)
- Query DOM (read-only browser query)
- Audit log reads
- Health metrics

#### Activation

Read-only mode is set at server start via `--read-only` flag. It cannot be toggled at runtime — this prevents an AI agent from disabling read-only mode via a tool call.

---

### 4.3 Tool Allowlisting

Restrict which MCP tools are available to connected clients. Tools not on the allowlist are completely hidden — they don't appear in the `tools/list` response and calls to them return "unknown tool" errors.

#### How It Works

An allowlist is configured via `--tools-allow` flag or config file. Only the listed tools are exposed:

```
--tools-allow="observe,analyze,configure"
```

If no allowlist is set, all tools are available (default behavior).

There is also a blocklist (`--tools-block`) for the inverse case — allow everything except specific tools:

```
--tools-block="configure,generate,analyze"
```

If both are specified, the allowlist takes priority (blocklist is ignored).

#### Tool Visibility

Hidden tools don't appear in the MCP `tools/list` response. An AI client that doesn't see a tool in the list won't try to call it. If somehow called directly, the server responds with the standard MCP "method not found" error — it doesn't reveal that the tool exists but is blocked.


---

## Configuration Summary

All enterprise features are opt-in. A default Kaboom installation behaves identically to today — no audit log, no TTL, no redaction, no auth. Enterprise features activate via explicit configuration.

### Minimal Enterprise Setup

```bash
kaboom --api-key=$KABOOM_KEY --ttl=1h
```

### Restricted Setup

```bash
kaboom \
  --profile=restricted \
  --api-key=$KABOOM_KEY \
  --redaction-config=./redaction-rules.json \
  --config=./kaboom.json
```

---

## Performance Constraints

Enterprise features must not violate existing SLOs:
- Audit logging adds < 0.1ms per tool call (append to in-memory ring buffer)
- Redaction on typical responses (< 5KB text): < 2ms (7 compiled RE2 patterns)
- Redaction on large responses (100KB+ DOM/network bodies): may add 10-30ms — this is acceptable as these responses are already slow to generate
- TTL eviction piggybacks on buffer reads, never blocking request processing
- Rate limit checks add < 0.01ms per tool call (atomic counter read)
- Config file parsing happens once at startup (not on every request)
- Health metrics are computed on-demand when the tool is called (no background polling overhead)

Total overhead for typical tool calls with all Tier 1-3 features active: < 3ms.

---

## Test Scenarios

### Tier 1
- Verify audit log records every tool call with correct metadata
- Verify client identity is extracted from MCP `initialize` request
- Verify session IDs are unique across server restarts
- Verify session ID format is sortable and human-readable
- Verify redaction events are logged without storing redacted content
- Verify audit log ring buffer evicts oldest entries when full
- Verify `get_audit_log` filters work (time range, tool name, status)

### Tier 2
- Verify TTL eviction skips entries older than configured TTL on read
- Verify TTL of zero (default) means no eviction (existing behavior)
- Verify profiles set all expected values for each level
- Verify profile values can be overridden by explicit flags
- Verify export_data returns valid JSON Lines with all buffer types
- Verify redaction patterns match and replace correctly
- Verify redacted responses don't contain original sensitive data
- Verify custom redaction patterns load from JSON config file
- Verify redaction does not apply retroactively to prior responses (limitation test)
- Verify RE2 patterns run in linear time on adversarial input (no backtracking)

### Tier 3
- Verify API key auth rejects requests without key
- Verify API key auth accepts requests with correct key
- Verify MCP stdio connections bypass API key auth
- Verify per-tool rate limits enforce configured thresholds
- Verify rate limit error response includes retry hint
- Verify per-tool rate limits and global circuit breaker do not conflict (global wins)
- Verify configurable thresholds override defaults
- Verify JSON config file parsing with all supported values
- Verify priority order: CLI > env > config file > defaults
- Verify health metrics report accurate values for all categories
- Verify API key is NOT accepted from JSON config file (must be env var or flag)

### Tier 4
- Verify project isolation (data in project A not visible in project B)
- Verify global memory limit is respected across all projects (largest-first eviction)
- Verify project creation fails when minimum 2MB per-project guarantee would be violated
- Verify read-only mode blocks all mutation operations
- Verify read-only mode allows all read/analyze/generate operations
- Verify tool allowlisting hides tools from `tools/list`
- Verify hidden tools return "method not found" on direct call
- Verify blocklist hides specified tools
- Verify default behavior (no enterprise config) is unchanged

---

## Zero-Dependency Constraint

All enterprise features are implemented using Go stdlib only. Specific implications:
- Regex: `regexp` package (RE2 engine — guaranteed linear time, no catastrophic backtracking)
- JSON: `encoding/json` for config file parsing, export serialization, and audit entries
- Crypto: `crypto/rand` for session ID generation
- Config: JSON config file parsed with `encoding/json` — no custom parser needed
- No external auth libraries — API key is a simple string comparison
- No external logging libraries — audit entries are JSON-serialized structs

The config file format is JSON specifically because it requires zero implementation effort with Go stdlib. No TOML, no YAML, no INI — those all require custom parsers or third-party libraries.

---

## Migration & Backward Compatibility

All features are additive. No existing behavior changes without explicit opt-in. Specifically:
- No audit entries are recorded unless the server sees tool calls (passive — always active but zero-cost if unused)
- No TTL is enforced unless `--ttl` is set
- No redaction occurs unless patterns are configured (via `--redaction-config` or `--profile`)
- No auth is required unless `--api-key` is set
- No per-tool rate limits apply unless configured
- All tools remain available unless allowlisting is configured
- The extension works unchanged — new auth header is only sent if configured

The MCP protocol version does not change. New configure actions and report modes may be added; existing tool response formats remain stable.
