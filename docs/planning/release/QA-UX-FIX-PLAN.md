---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# QA/UX Fix Plan — Kaboom MCP Tools

> **Status: COMPLETED** — All P0–P4 fixes implemented. Documentation updated.

## Context

QA/UX audit of the 5 MCP tools found schema mismatches, missing filters, stubs confusing LLMs, and workflow gaps. This plan fixes all issues to make tools reliable and easy for LLMs to use.

---

## P0 — Schema Bugs (4 fixes)

### P0-1: Fix `network_waterfall` URL filter (SILENT BUG)

Schema declares `url` param but handler reads `url_filter`. Filtering silently does nothing.

- **File:** `cmd/browser-agent/tools_observe_analysis.go:19`
- **Fix:** Change struct tag `json:"url_filter"` → `json:"url"`

### P0-2: Add `link_validation` to analyze schema

- **File:** `cmd/browser-agent/tools_schema.go:97`
- **Fix:** Add `"link_validation"` to analyze `what` enum

### P0-3: Add `test_from_context`, `test_heal`, `test_classify` to generate schema

- **File:** `cmd/browser-agent/tools_schema.go:205`
- **Fix:** Add all three to generate `format` enum

### P0-4: Add `upload` to interact schema

- **File:** `cmd/browser-agent/tools_schema.go:447`
- **Fix:** Add `"upload"` to interact `action` enum

---

## P1 — Filter Gaps (6 fixes)

Schema advertises filters that handlers silently ignore. Fix all.

### P1-1: `network_bodies` — add `url`, `method`, `status_min`, `status_max`, `limit`

- **File:** `cmd/browser-agent/tools_observe.go` (toolGetNetworkBodies)
- **Current:** Dumps entire buffer, no filtering
- **Fix:** Parse params, iterate backwards, apply URL substring + method match + status range, cap at limit (default 100)

### P1-2: `websocket_events` — use existing filter infrastructure

- **File:** `cmd/browser-agent/tools_observe.go` (toolGetWSEvents)
- **Current:** Calls `GetAllWebSocketEvents()` ignoring all filter params
- **Fix:** Parse `limit`, `url`, `connection_id`, `direction`. Build `WebSocketEventFilter`, call `GetWebSocketEvents(filter)` — capture layer already implements this

### P1-3: `actions` — add `url`, `limit`

- **File:** `cmd/browser-agent/tools_observe.go` (toolGetEnhancedActions)
- **Current:** Dumps entire buffer
- **Fix:** Parse params, iterate backwards, URL substring match, cap at limit (default 100)

### P1-4: `errors` — add `url` filter

- **File:** `cmd/browser-agent/tools_observe.go` (toolGetBrowserErrors)
- **Current:** Has `limit` only
- **Fix:** Add `URL string` to params, filter by `containsIgnoreCase(entry["url"], params.URL)`

### P1-5: `logs` — add `url` filter

- **File:** `cmd/browser-agent/tools_observe.go` (toolGetBrowserLogs)
- **Current:** Has limit/level/min_level/source but no url
- **Fix:** Same as errors

### P1-6: `error_bundles` — add `url` filter

- **File:** `cmd/browser-agent/tools_observe_bundling.go`
- **Current:** Has limit/window_seconds but no url
- **Fix:** Filter error entries by URL substring before bundling

---

## P2 — Workflow Improvements (2 items)

### P2-1: Keep interact async-only — NO CODE CHANGE

Async pattern is correct. "Completion" semantics are ambiguous for DOM actions. Skip for now.

### P2-2: Add network waterfall data to `error_bundles`

- **File:** `cmd/browser-agent/tools_observe_bundling.go`
- **Current:** Only includes `network_bodies` (fetch-only). XHR/image/script failures invisible.
- **Fix:** Also fetch `GetNetworkWaterfallEntries()`, match within time window, add `"waterfall"` array to each bundle

---

## P3 — Remove Stubs from Schema (11 removals)

Stubs waste LLM tokens. Remove from schema enum arrays. Keep handler code for future re-enable.

| Tool | Mode to remove | Current behavior |
|------|---------------|-----------------|
| observe | `api` | returns "not_implemented" |
| observe | `changes` | returns "not_implemented" |
| observe | `playback_results` | returns placeholder |
| generate | `test` | returns `{script: ""}` |
| generate | `pr_summary` | returns `{summary: ""}` |
| generate | `har` | returns empty HAR |
| generate | `sri` | returns empty resources |
| analyze | `api_validation` | returns `{violations: []}` |
| analyze | `security_diff` | returns `{differences: []}` |
| configure | `diff_sessions` | returns `{status: "ok"}` |
| configure | `audit_log` | returns `{entries: []}` |

**Also update generate tool description** to remove references to removed formats (test, pr_summary, har, sri).

---

## P4 — Polish (3 items)

### P4-1: Add `count` field to responses missing it

- `network_bodies`, `websocket_events`, `actions` — add `"count": len(filtered)` to response

### P4-2: Fix hardcoded User-Agent version

- **File:** `cmd/browser-agent/tools_analyze.go:287`
- **Fix:** Replace `"Kaboom/0.7.12"` with `fmt.Sprintf("Kaboom/%s", version)`

### P4-3: Enrich `observe({what: "page"})` response

- Currently returns only `url` + `title`
- Add `tab_id` and `tracked` status from existing server-side data

---

## Files Modified

| File | Changes |
|------|---------|
| `tools_schema.go` | P0 enum additions, P3 enum removals |
| `tools_observe.go` | P1 filters (errors, logs, network_bodies, websocket_events, actions), P4 count fields |
| `tools_observe_analysis.go` | P0 url_filter fix, P4 page enrichment |
| `tools_observe_bundling.go` | P1 url filter, P2 waterfall enrichment |
| `tools_analyze.go` | P4 version string |
| `tools_generate.go` | (schema only, handler code stays) |
| `tools_configure.go` | (schema only, handler code stays) |
| `tools_security.go` | (schema only, handler code stays) |

---

## Verification

After each priority level:
```bash
go build ./cmd/browser-agent/
go test ./cmd/browser-agent/...
```

Final:
```bash
make ci-local
```
