---
title: "Observe — Read Browser State"
description: "Complete reference for the observe tool. 30 modes for reading errors, network traffic, WebSocket messages, user actions, recordings, storage, page inventory, inbox messages, and more."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['reference', 'observe']
---

The `observe` tool reads the current browser state. It's your AI's eyes into the browser — errors, network traffic, WebSocket messages, performance metrics, visual state, and more.

**Always call `observe()` before `interact()` or `generate()`** to give the AI context about the current page.

Need one runnable call + response shape + failure fix for every mode? See [Observe Executable Examples](/reference/examples/observe-examples/).

## Quick Reference

```js
observe({what: "errors"})                    // Console errors
observe({what: "error_bundles"})             // Errors with full context
observe({what: "logs", min_level: "warn"})   // Console output
observe({what: "network_bodies", url: "/api", status_min: 400})  // Failed API calls
observe({what: "websocket_events", last_n: 10})                  // WebSocket messages
observe({what: "screenshot"})                // Page screenshot
observe({what: "vitals"})                    // Web Vitals (LCP, CLS, INP)
observe({what: "recordings"})               // Recording metadata
observe({what: "log_diff_report", original_id: "rec-1", replay_id: "rec-2"})  // Compare recordings
```

## Common Parameters

These parameters work across multiple modes:

| Parameter | Type | Description |
|-----------|------|-------------|
| `what` | string (required) | Which mode to use (see sections below) |
| `limit` | number | Maximum entries to return |
| `last_n` | number | Return only the last N items |
| `url` | string | Filter by URL substring |
| `after_cursor` | string | Backward pagination — entries older than cursor |
| `before_cursor` | string | Forward pagination — entries newer than cursor |
| `since_cursor` | string | All entries newer than cursor (inclusive, no limit) |
| `restart_on_eviction` | boolean | Auto-restart if cursor expired from buffer overflow |
| `summary` | boolean | Return compact summary (~60-70% smaller). Works with errors, logs, network_waterfall, network_bodies, websocket_events, actions, error_bundles, timeline, history, transients |
| `scope` | string | Filter scope: `current_page` (default) or `all` — applies to errors, logs |

---

## Errors & Logs

### `errors`

Console errors with deduplication. The starting point for any debugging session.

```js
observe({what: "errors"})
```

Returns deduplicated errors with stack traces, source locations, and occurrence counts.

### `error_bundles`

Pre-assembled debugging context per error. Each bundle includes the error plus the network requests, user actions, and console logs that happened near it.

```js
observe({what: "error_bundles", window_seconds: 5})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `window_seconds` | number | 3 | How far back to look for related context (max 10) |

This is the most powerful debugging mode — it gives the AI a complete incident report instead of a bare stack trace.

### `logs`

All console output (log, warn, error, info, debug) with level filtering.

```js
observe({what: "logs", min_level: "warn", limit: 50})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `min_level` | string | Minimum level threshold: `debug` < `log` < `info` < `warn` < `error` |
| `source` | string | Exact source filter |
| `include_extension_logs` | boolean | Include extension debug logs alongside console output |
| `extension_limit` | number | Max extension logs when `include_extension_logs: true` |
| `include_internal` | boolean | Include daemon lifecycle/transport diagnostics |
| `limit` | number | Maximum entries to return |

### `extension_logs`

Internal KaBOOM extension debug logs. **Not** browser console output — use `logs` for that. Only useful for troubleshooting the KaBOOM extension itself.

```js
observe({what: "extension_logs"})
```

---

## Network

### `network_waterfall`

Resource timing data for all network requests (XHR, fetch, scripts, stylesheets, images). Shows URL, method, status, duration, and size.

```js
observe({what: "network_waterfall", limit: 30})
observe({what: "network_waterfall", url: "/api"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `limit` | number | Maximum entries |
| `url` | string | Filter by URL substring |

:::note
This captures **all** network requests. For request/response bodies, use `network_bodies` (which only captures `fetch()` calls).
:::

### `network_bodies`

Full request and response payloads for `fetch()` calls. Filtered by URL, method, or status code.

```js
observe({what: "network_bodies", url: "/api/users", status_min: 400})
observe({what: "network_bodies", method: "POST", limit: 5})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `url` | string | Filter by URL substring |
| `method` | string | Filter by HTTP method (GET, POST, etc.) |
| `status_min` | number | Minimum status code (e.g., 400 for errors only) |
| `status_max` | number | Maximum status code |
| `body_path` | string | Extract JSON value using dot-path (e.g., `data.items[0].id`) |
| `limit` | number | Maximum entries |

**Buffer:** 100 recent requests, 8 MB total memory. Auth headers are always stripped.

<!-- Screenshot: Example network_bodies output showing a failed API call with request and response payloads -->

---

## WebSocket

### `websocket_events`

Captured WebSocket messages and lifecycle events (open, close, error).

```js
observe({what: "websocket_events", last_n: 20})
observe({what: "websocket_events", connection_id: "ws-3", direction: "incoming"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `connection_id` | string | Filter by specific WebSocket connection |
| `direction` | string | `incoming` or `outgoing` |
| `last_n` | number | Return only the last N events |
| `limit` | number | Maximum entries |

**Buffer:** 500 events max, 4 MB limit. High-frequency streams are adaptively sampled.

### `websocket_status`

Live connection health — active connections, message rates, duration.

```js
observe({what: "websocket_status"})
```

Returns connection ID, URL, state (open/closed), duration, and message rates per direction.

---

## Page State

### `page`

Current URL and page title.

```js
observe({what: "page"})
```

### `tabs`

All browser tabs with their URLs and titles.

```js
observe({what: "tabs"})
```

### `screenshot`

Captures a screenshot of the current viewport. The AI receives the image and can reason about visual state — broken layouts, stuck spinners, hidden elements.

```js
observe({what: "screenshot"})
observe({what: "screenshot", selector: ".error-panel", format: "png"})
observe({what: "screenshot", full_page: true})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `format` | string | `png` | Image format: `png` or `jpeg` |
| `quality` | number | — | JPEG quality 1-100 (only for `jpeg`) |
| `full_page` | boolean | false | Capture the full scrollable page |
| `selector` | string | — | Capture a specific element by CSS selector |
| `wait_for_stable` | boolean | false | Wait for layout to stabilize before capture |

<!-- Screenshot: Example of observe screenshot output showing a web page capture -->

### `pilot`

Current state of AI Web Pilot — whether browser control is enabled or disabled.

```js
observe({what: "pilot"})
```

---

## User Actions

### `actions`

Recorded user interactions — clicks, typing, navigation, scrolling. Each action includes a timestamp, the element targeted, and the selector used.

```js
observe({what: "actions", last_n: 10})
observe({what: "actions", limit: 50})
```

Useful for understanding what the user did before an error occurred, or for generating reproduction scripts.

---

## Performance

### `vitals`

Core Web Vitals: LCP (Largest Contentful Paint), CLS (Cumulative Layout Shift), INP (Interaction to Next Paint), FCP (First Contentful Paint).

```js
observe({what: "vitals"})
```

Each metric includes the value, the rating (good/needs-improvement/poor), and thresholds.

---

## Recordings

### `saved_videos`

List recorded browser session videos.

```js
observe({what: "saved_videos"})
```

### `recordings`

Recording metadata — lists all recordings with their IDs, timestamps, and duration.

```js
observe({what: "recordings"})
```

### `recording_actions`

Actions captured during a specific recording session.

```js
observe({what: "recording_actions"})
```

### `playback_results`

Results from a recording playback session.

```js
observe({what: "playback_results"})
```

### `log_diff_report`

Compare error states between two recordings. Useful for verifying that a bug fix actually resolved the issue.

```js
observe({what: "log_diff_report", original_id: "rec-abc", replay_id: "rec-xyz"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `original_id` | string | Original recording ID (baseline) |
| `replay_id` | string | Replay recording ID (comparison) |

---

## Timeline & Aggregation

### `timeline`

Merged chronological view of all events — errors, network requests, user actions, console logs — interleaved by timestamp.

```js
observe({what: "timeline"})
observe({what: "timeline", include: ["errors", "network"]})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `include` | array | Categories to include (e.g., `errors`, `network`, `actions`, `logs`) |

---

## Async Commands

### `command_result`

Retrieve the result of a previously-issued async command by correlation ID.

```js
observe({what: "command_result", correlation_id: "abc123"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `correlation_id` | string | The ID returned when the async command was issued |

### `pending_commands`

List commands that are still waiting for execution by the extension.

```js
observe({what: "pending_commands"})
```

### `failed_commands`

List commands that failed during execution.

```js
observe({what: "failed_commands"})
```

---

## Browser Storage

### `storage`

Read localStorage, sessionStorage, or cookies for the current page.

```js
observe({what: "storage", storage_type: "local"})
observe({what: "storage", storage_type: "cookies"})
observe({what: "storage", storage_type: "session", key: "auth_token"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `storage_type` | string | `local`, `session`, or `cookies` |
| `key` | string | Read a specific key (omit for all) |

### `indexeddb`

Read IndexedDB databases and object stores.

```js
observe({what: "indexeddb"})
observe({what: "indexeddb", database: "myDB", store: "users"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `database` | string | Database name |
| `store` | string | Object store name |

---

## Aggregation & Summarization

### `history`

Navigation history from recorded user actions.

```js
observe({what: "history"})
```

### `summarized_logs`

Groups and summarizes repeated log patterns. Useful for noisy applications where the same log message appears hundreds of times.

```js
observe({what: "summarized_logs"})
observe({what: "summarized_logs", min_group_size: 5})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `min_group_size` | number | 2 | Minimum occurrences to form a group |

### `page_inventory`

Inventory of interactive elements on the page — forms, buttons, links, inputs.

```js
observe({what: "page_inventory"})
observe({what: "page_inventory", visible_only: true})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `visible_only` | boolean | Only return visible elements |

### `transients`

Capture transient UI elements — toasts, alerts, snackbars, and notifications that appear briefly and disappear. KaBOOM intercepts these before they vanish.

```js
observe({what: "transients"})
observe({what: "transients", last_n: 5})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `classification` | string | Filter by type: `alert`, `toast`, `snackbar`, `notification`, `tooltip`, `banner`, `flash` |

### `inbox`

Read queued push notifications and alert events emitted by streaming workflows.

```js
observe({what: "inbox"})
observe({what: "inbox", limit: 20})
```

### `site_menus`

Discover the site's navigation menus as structured data — without relying on landmark or ARIA markup. KaBOOM dispatches `list_interactive`, then applies a menu heuristic in Go to group links into menus (primary nav, footer, dropdowns) and remove those elements from the flat interactable list.

```js
observe({what: "site_menus"})
observe({what: "site_menus", summary: true})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `summary` | boolean | Return a compact list of menus instead of full per-item detail |

---

## Pagination

All buffer-backed modes support cursor-based pagination. The response includes a `metadata` object with cursor values:

```json
{
  "data": [...],
  "metadata": {
    "cursor": "1707325200:42",
    "has_more": true
  }
}
```

Pass the cursor back on the next request:

```js
// Get next page (older entries)
observe({what: "logs", after_cursor: "1707325200:42"})

// Get new entries since last check
observe({what: "errors", since_cursor: "1707325200:42"})
```

If the cursor expires (buffer overflowed since last read), set `restart_on_eviction: true` to automatically restart from the oldest available entry instead of erroring.
