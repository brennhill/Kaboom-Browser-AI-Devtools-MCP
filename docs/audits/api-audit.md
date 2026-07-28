---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Kaboom MCP API Audit -- Findings

> Audit date: 2026-02-14
> Auditor: Automated code analysis
> Scope: All MCP tools, HTTP endpoints, extension-daemon communication

---

## Summary

| Severity | Count |
|----------|-------|
| CRITICAL | 0 |
| HIGH | 5 |
| MEDIUM | 11 |
| LOW | 8 |

---

## Findings

### HIGH-1: Schema/Handler Mismatch -- observe modes `api`, `changes`, `playback_results` are registered in the handler map but missing from the schema enum

**Severity**: HIGH

**Location**: `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_schema.go` (line 18, `what` enum) vs `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_observe.go` (lines 32-44, `observeHandlers` map)

**Description**: The observe tool schema defines a `what` enum with 22 values, but the `observeHandlers` map registers 25 handlers. Three modes are accessible at runtime but invisible to MCP clients:
- `api` -- returns "not_implemented" status
- `changes` -- returns "not_implemented" status
- `playback_results` -- functional handler

**Impact**: MCP clients (Claude, Cursor) will never discover `playback_results` since it is not in the schema. The `api` and `changes` modes are stubs that waste tokens if discovered.

**Recommendation**: Either add `playback_results` to the schema enum or remove it from the handler map. Remove `api` and `changes` from the handler map since they return "not implemented" -- dead code that confuses.

---

### HIGH-2: Schema/Handler Mismatch -- analyze modes `api_validation`, `security_diff`, `draw_history`, `draw_session` are registered but missing from schema enum

**Severity**: HIGH

**Location**: `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_schema.go` (line 98, `what` enum) vs `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_analyze.go` (lines 32-85, `analyzeHandlers` map)

**Description**: The analyze tool schema enum lists 11 values, but the handler map registers 15. Four modes are inaccessible to MCP clients:
- `api_validation` -- functional (analyze/report/clear operations)
- `security_diff` -- stub returning empty differences
- `draw_history` -- functional handler
- `draw_session` -- functional handler

**Impact**: `api_validation`, `draw_history`, and `draw_session` are functional features hidden from MCP discovery. Clients cannot use them unless they guess the names.

**Recommendation**: Add `api_validation`, `draw_history`, and `draw_session` to the schema enum. For `security_diff`, either add it to the enum or remove the stub handler.

---

### HIGH-3: Schema/Handler Mismatch -- generate formats `test`, `pr_summary`, `har`, `sri` are registered but missing from schema enum

**Severity**: HIGH

**Location**: `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_schema.go` (line 206, `format` enum) vs `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_generate.go` (lines 21-35, `generateHandlers` map)

**Description**: The generate tool schema enum lists 9 values, but the handler map registers 14. Five formats are inaccessible to MCP clients:
- `test` -- fully functional Playwright test generation
- `pr_summary` -- fully functional PR summary generation
- `har` -- fully functional HAR export
- `sri` -- fully functional SRI hash generation

**Impact**: These are major, fully functional features that MCP clients cannot discover. `test` and `har` are likely the most commonly needed generate formats.

**Recommendation**: Add `test`, `pr_summary`, `har`, and `sri` to the schema enum immediately. These are production-ready features.

---

### HIGH-4: Schema/Handler Mismatch -- configure actions `diff_sessions`, `audit_log` are registered but missing from schema enum

**Severity**: HIGH

**Location**: `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_schema.go` (line 314, `action` enum) vs `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_configure.go` (lines 33-48, `configureHandlers` map)

**Description**: The configure tool schema enum lists 12 values, but the handler map registers 14. Two actions are hidden:
- `diff_sessions` -- session comparison
- `audit_log` -- tool invocation audit log

**Impact**: `audit_log` provides important observability data that MCP clients cannot discover.

**Recommendation**: Add `diff_sessions` and `audit_log` to the schema enum.

---

### HIGH-5: Extension calls `/api/extension-status` and `/pending-queries` and `/extension-logs` endpoints that have no visible route registration

**Severity**: HIGH

**Location**: `/Users/brenn/dev/kaboom/src/background/sync/server.ts` (lines 410, 449, 379) vs `/Users/brenn/dev/kaboom/cmd/browser-agent/server_routes.go`

**Description**: The extension background script makes fetch calls to three endpoints:
- `POST /api/extension-status`
- `GET /pending-queries`
- `POST /extension-logs`

These endpoints are not visible in `setupHTTPRoutes()` / `registerCaptureRoutes()` / `registerCoreRoutes()`. They may be registered in the capture package's handler methods or via the `/sync` unified endpoint, but the routing is not apparent from the route setup code.

**Impact**: If these routes are truly missing, extension calls silently fail (404). If they are handled via the `/sync` endpoint or capture handler, the routing is unclear and should be documented.

**Recommendation**: Verify these endpoints exist. If they are handled via `/sync`, document this in the route registration comments. If they are standalone routes registered elsewhere (e.g., in capture package), add comments to `server_routes.go` cross-referencing them.

---

### MEDIUM-1: `observe({what: "api"})` and `observe({what: "changes"})` return "not_implemented" but are still in the handler map

**Severity**: MEDIUM

**Location**: `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_observe.go` (lines 505-517)

**Description**: Two observe modes return stub responses with `"status": "not_implemented"` messages. They occupy handler map entries and would confuse callers if they somehow reached them.

**Recommendation**: Remove from handler map, or implement them, or add a clear deprecation notice.

---

### MEDIUM-2: `analyze({what: "security_diff"})` is a stub returning empty data

**Severity**: MEDIUM

**Location**: `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_security.go` (lines 81-97)

**Description**: `toolDiffSecurity` always returns `{"status":"ok","differences":[]}` regardless of input. It ignores the `compare_from` and `compare_to` parameters.

**Recommendation**: Either implement or remove. If the feature is planned, add a `"status": "not_implemented"` response to be explicit.

---

### MEDIUM-3: No validation on `limit` parameter maximum -- potential large response abuse

**Severity**: MEDIUM

**Location**: All observe handlers -- `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_observe.go` (multiple locations)

**Description**: The `limit` parameter defaults to 100 when <= 0, but has no upper bound. A caller can set `limit: 999999` and receive all buffered entries. While buffers are capped, the response could still be very large.

**Recommendation**: Cap `limit` to a reasonable maximum (e.g., 1000) across all observe modes.

---

### MEDIUM-4: `link_validation` uses `ErrInvalidJSON` error code for non-JSON errors

**Severity**: MEDIUM

**Location**: `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_analyze.go` (line 296)

**Description**: When `toolValidateLinks` rejects URLs that are not HTTP/HTTPS, it returns `ErrInvalidJSON` ("invalid_json") as the error code. This is misleading -- the JSON is valid, the URL values are wrong.

**Recommendation**: Use `ErrInvalidParam` instead of `ErrInvalidJSON` for the "No valid HTTP/HTTPS URLs provided" error.

---

### MEDIUM-5: `generate({what: "har"})` uses string literal "export_failed" instead of `ErrExportFailed` constant

**Severity**: MEDIUM

**Location**: `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_generate.go` (line 395)

**Description**: The HAR export error path uses a string literal `"export_failed"` rather than the defined constant `ErrExportFailed`. This breaks the pattern used everywhere else and could lead to inconsistency.

**Recommendation**: Replace `"export_failed"` with `ErrExportFailed`.

---

### MEDIUM-6: Inconsistent response shapes between similar observe modes

**Severity**: MEDIUM

**Location**: Multiple observe handlers in `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_observe.go` and `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_observe_analysis.go`

**Description**: Different observe modes use inconsistent top-level keys for their data arrays:
- `errors` uses key `"errors"`
- `logs` uses key `"logs"`
- `network_bodies` uses key `"entries"`
- `websocket_events` uses key `"entries"`
- `actions` uses key `"entries"`
- `extension_logs` uses key `"logs"`
- `network_waterfall` uses key `"entries"`
- `timeline` uses key `"entries"`
- `error_bundles` uses key `"bundles"`
- `error_clusters` uses key `"clusters"`

While each makes semantic sense, a uniform key (or at least a convention) would make client code simpler.

**Recommendation**: Consider standardizing on a `"data"` or `"entries"` key, or document the convention clearly. Low priority since changing would break clients.

---

### MEDIUM-7: `generate({what: "csp"})` does not validate `mode` parameter against enum

**Severity**: MEDIUM

**Location**: `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_generate.go` (lines 412-443)

**Description**: The `mode` parameter schema specifies enum `["strict", "moderate", "report_only"]`, but the handler code defaults to `"moderate"` when empty and passes the value through without validation. Any arbitrary string is accepted.

**Recommendation**: Add validation: if `mode` is not empty and not in the enum, return `ErrInvalidParam`.

---

### MEDIUM-8: `observe({what: "errors"})` has `tabId` in response but schema shows `tab_id`

**Severity**: MEDIUM

**Location**: `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_observe.go` (line 187)

**Description**: The error response maps `entry["tabId"]` to the output key `"tab_id"`. While this correctly converts the internal camelCase field to snake_case for the API, the source data from the extension uses `tabId`. If the extension ever changes this field name, the mapping would break silently. The same pattern exists in `toolGetBrowserLogs` (line 305).

**Recommendation**: Add a comment documenting this field name conversion, or create a constants map for extension-to-API field mappings.

---

### MEDIUM-9: `configure({what: "store"})` returns `ErrNotInitialized` but `configure({what: "load"})` returns a fallback response

**Severity**: MEDIUM

**Location**: `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_configure.go` (lines 108-109 vs 156-165)

**Description**: When `sessionStoreImpl` is nil:
- `store` action returns an error: `ErrNotInitialized`
- `load` action returns a success response with `"Session store not initialized"` message

This is inconsistent. The caller gets an error for `store` but a misleading success for `load`.

**Recommendation**: Make `load` return `ErrNotInitialized` when the session store is not initialized, matching `store` behavior.

---

### MEDIUM-10: Missing `recording_id` parameter in schema for `configure({what: "event_recording_stop"})`

**Severity**: MEDIUM

**Location**: `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_schema.go` (configure schema) vs `/Users/brenn/dev/kaboom/cmd/browser-agent/recording_handlers.go` (line 98)

**Description**: The `event_recording_stop` handler expects a `recording_id` parameter, but the schema for the `configure` tool does not explicitly include `recording_id` in its properties. While it could be passed as a generic `session_id`, the mapping is unclear.

**Recommendation**: Add `recording_id` to the configure tool schema properties.

---

### MEDIUM-11: `AuthMiddleware` is defined but its usage in route setup is not visible

**Severity**: MEDIUM

**Location**: `/Users/brenn/dev/kaboom/cmd/browser-agent/auth.go` vs `/Users/brenn/dev/kaboom/cmd/browser-agent/server_routes.go`

**Description**: The `AuthMiddleware` function is defined and accepts an `expectedKey` parameter, but `setupHTTPRoutes()` does not visibly apply it. It may be applied at a higher level (e.g., in `main.go`), but the route setup code only shows `corsMiddleware` and `extensionOnly` wrappers.

**Recommendation**: Verify where `AuthMiddleware` is applied and document its activation conditions (e.g., `KABOOM_API_KEY` env var). If it's not applied, this is a security gap for non-extension endpoints.

---

### LOW-1: `kaboom://guide` resource contains `api` and `changes` observe modes that are not implemented

**Severity**: LOW

**Location**: `/Users/brenn/dev/kaboom/cmd/browser-agent/handler.go` (line 340)

**Description**: The guide resource text lists `api` and `changes` as valid observe modes, but these return "not_implemented" stubs.

**Recommendation**: Remove `api` and `changes` from the guide, or mark them as "planned".

---

### LOW-2: `interact` action enum in error message is manually maintained and could drift

**Severity**: LOW

**Location**: `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_interact.go` (line 81)

**Description**: The `ErrMissingParam` error for missing `action` includes a hardcoded list of valid actions. Unlike `observe`/`analyze`/`generate`/`configure` which compute valid values dynamically from their handler maps, `interact` uses a static string.

**Recommendation**: Compute the valid action list dynamically from `interactDispatch()` keys + `domPrimitiveActions` keys, matching the pattern used by other tools.

---

### LOW-3: `observe({what: "logs"})` has undocumented `source` filter parameter

**Severity**: LOW

**Location**: `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_observe.go` (line 264) -- not in schema

**Description**: The `toolGetBrowserLogs` handler accepts and uses a `source` parameter for exact-match filtering, but this parameter is not in the observe tool schema.

**Recommendation**: Add `source` to the schema or remove the filter.

---

### LOW-4: `observe({what: "logs"})` has undocumented `level` (exact match) parameter separate from `min_level`

**Severity**: LOW

**Location**: `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_observe.go` (line 253) -- `level` not in schema

**Description**: The logs handler supports both `level` (exact match) and `min_level` (threshold). Only `min_level` is in the schema. The `level` parameter is silently accepted.

**Recommendation**: Either add `level` to the schema or remove it in favor of `min_level` alone.

---

### LOW-5: No existing `docs/audits/` directory -- API documentation was not previously tracked

**Severity**: LOW

**Location**: `/Users/brenn/dev/kaboom/docs/`

**Description**: The `docs/` directory had no `audits/` subdirectory. The existing `docs/developer-api.md` covers only the `window.__kaboom` in-page JavaScript API, not the MCP or HTTP APIs.

**Recommendation**: Maintain the API reference created by this audit as the source of truth for API documentation.

---

### LOW-6: `observe` warns when extension disconnected but `analyze` does not uniformly do so

**Severity**: LOW

**Location**: `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_observe.go` (lines 82-84) vs `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_analyze.go`

**Description**: The `observe` dispatcher prepends a disconnect warning for non-server-side modes when the extension is disconnected. The `analyze` dispatcher does not have equivalent logic -- individual handlers check extension status inconsistently (e.g., `accessibility` checks tracking status, `security_audit` does not check connectivity).

**Recommendation**: Add a uniform extension connectivity check at the `analyze` dispatcher level for modes that require it.

---

### LOW-7: `generate({what: "sri"})` error uses `ErrInvalidJSON` for non-JSON failures

**Severity**: LOW

**Location**: `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_generate.go` (line 548)

**Description**: When SRI generation fails, the error code is `ErrInvalidJSON` with message "SRI generation failed: ...". The actual error may not be JSON-related.

**Recommendation**: Use `ErrInternal` or `ErrInvalidParam` depending on the actual error cause.

---

### LOW-8: Schema parameters for `observe` tool include filters not applicable to all modes

**Severity**: LOW

**Location**: `/Users/brenn/dev/kaboom/cmd/browser-agent/tools_schema.go` (lines 13-87)

**Description**: The observe tool schema defines all parameters at the top level (url, method, status_min, status_max, connection_id, direction, min_level, etc.) even though most only apply to specific `what` modes. This is a design trade-off for schema simplicity but means clients see 20+ parameters when only 1-3 are relevant for a given mode.

**Recommendation**: No immediate action needed -- this is a deliberate design choice documented in code comments ("Descriptions are kept minimal to reduce token usage"). Consider adding per-mode parameter descriptions in the `kaboom://guide` resource.

---

## Cross-Reference: Extension Fetch Calls vs Daemon Routes

| Extension Fetch URL | Daemon Route | Match |
|--------------------|-------------|-------|
| `/logs` (POST) | `/logs` (POST) | OK |
| `/logs` (DELETE) | `/logs` (DELETE) | OK |
| `/websocket-events` (POST) | `/websocket-events` | OK |
| `/network-bodies` (POST) | `/network-bodies` | OK |
| `/network-waterfall` (POST) | `/network-waterfall` | OK |
| `/enhanced-actions` (POST) | `/enhanced-actions` | OK |
| `/performance-snapshots` (POST) | `/performance-snapshots` | OK |
| `/query-result` (POST) | `/query-result` | OK |
| `/health` (GET) | `/health` | OK |
| `/screenshots` (POST) | `/screenshots` | OK |
| `/sync` (POST) | `/sync` | OK |
| `/recordings/reveal` (POST) | `/recordings/reveal` | OK |
| `/api/file/read` (POST) | `/api/file/read` | OK |
| `/api/os-automation/inject` (POST) | `/api/os-automation/inject` | OK |
| `/api/os-automation/dismiss` (POST) | `/api/os-automation/dismiss` | OK |
| `/draw-mode/complete` (POST) | `/draw-mode/complete` | OK |
| `/api/extension-status` (POST) | **NOT FOUND in routes** | INVESTIGATE (see HIGH-5) |
| `/pending-queries` (GET) | **NOT FOUND in routes** | INVESTIGATE (see HIGH-5) |
| `/extension-logs` (POST) | **NOT FOUND in routes** | INVESTIGATE (see HIGH-5) |

---

## Recommendations Summary

### Immediate Actions (address before next release)

1. **Sync schema enums with handler maps** (HIGH-1 through HIGH-4): Add missing values to MCP tool schema enums. This unlocks functional features that MCP clients cannot currently discover.

2. **Investigate missing HTTP routes** (HIGH-5): Verify whether `/api/extension-status`, `/pending-queries`, and `/extension-logs` are handled by the unified `/sync` endpoint or are genuinely missing routes.

### Short-Term Improvements

3. **Cap `limit` parameter** (MEDIUM-3): Add a maximum (e.g., 1000) to prevent oversized responses.

4. **Fix error code misuse** (MEDIUM-4, MEDIUM-5, LOW-7): Use correct error codes (`ErrInvalidParam` not `ErrInvalidJSON` for non-JSON errors; `ErrExportFailed` constant not string literal).

5. **Fix inconsistent `load` behavior** (MEDIUM-9): Return `ErrNotInitialized` consistently when session store is not initialized.

6. **Add missing schema parameters** (MEDIUM-10, LOW-3, LOW-4): Add `recording_id`, `source`, and `level` to their respective schemas.

### Long-Term Improvements

7. **Standardize response shapes** (MEDIUM-6): Document or unify the top-level data key naming convention.

8. **Add extension disconnect warnings to analyze** (LOW-6): Add dispatcher-level connectivity check.

9. **Dynamic action list for interact errors** (LOW-2): Compute valid actions list from handler maps.

10. **Update guide resource** (LOW-1): Remove unimplemented modes from the guide.
