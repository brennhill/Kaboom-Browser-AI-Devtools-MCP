---
feature: auto-paste-screenshots
status: proposed
version: null
tool: observe / configure
mode: (cross-cutting) errors, page, + configure screenshot_mode
authors: []
created: 2026-01-28
updated: 2026-01-28
doc_type: product-spec
feature_id: feature-auto-paste-screenshots
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Auto-Paste Screenshots to IDE

> Attach browser screenshots as base64 image content blocks in MCP tool responses, giving AI coding assistants visual context alongside telemetry data.

## Problem

AI coding assistants working through MCP receive structured text data (errors, logs, DOM state) but never see what the user sees. Visual context is critical for:

1. **UI bugs** -- CSS layout issues, z-index problems, missing icons, and rendering glitches are invisible in text telemetry. An AI agent debugging "button not clickable" cannot distinguish between a z-index overlap and a missing event handler without seeing the page.
2. **Error context** -- A JavaScript exception is more actionable when the AI can see the visual state that triggered it (e.g., a half-rendered form, a broken modal overlay).
3. **Verification loops** -- After the AI suggests a fix and the developer applies it, the AI currently must ask "did that work?" and parse text descriptions. With screenshots, it can verify visually.

Today, Kaboom captures screenshots to disk (JPEG files) via the extension's `captureVisibleTab` API and stores filename references in log entries. But these files are inaccessible to MCP clients -- the AI agent only sees a filename string like `example.com-20260128-143022.jpg`, with no way to view the actual image. The screenshot capture infrastructure exists but delivers zero value to the AI.

MCP supports returning images as base64-encoded content blocks alongside text. This feature bridges the gap: screenshots flow directly into MCP responses where multimodal AI models can interpret them.

## Solution

Extend Kaboom's MCP response pipeline to optionally include a screenshot as a base64 image content block appended to any `observe` or `generate` tool response. The screenshot is captured on-demand at response time (not from a stale cache), ensuring the AI sees the current page state.

### Design decision: `configure`-based global setting, not per-request parameter.

Rationale:
- A per-request `include_screenshot: true` parameter on `observe` would add a boolean to every single observe call, inflating every request. Most AI agents would either always want screenshots or never want them -- a per-call toggle adds friction without flexibility.
- A `configure` setting (`screenshot_mode`) lets the AI turn screenshots on once and receive them on all subsequent responses. This matches how `screenshot_on_error` and `log_level` already work as session-scoped capture settings.
- The AI can still disable screenshots mid-session (e.g., when debugging non-visual issues) with a single `configure` call.

**MCP image content block format** (per MCP specification):
```json
{
  "type": "image",
  "data": "<base64-encoded-jpeg>",
  "mimeType": "image/jpeg"
}
```

This is appended as an additional content block after the text block(s) in the `MCPToolResult.Content` array.

## User Stories

- As an AI coding agent, I want to see a screenshot of the browser alongside error data so that I can correlate visual state with exceptions.
- As an AI coding agent, I want to toggle screenshots on/off for a session so that I only pay the token cost when visual context is needed.
- As a developer using Kaboom, I want my AI assistant to see what I see in the browser so that I spend less time describing visual bugs.
- As an AI coding agent, I want screenshots automatically included with `observe({what: "page"})` responses when screenshot mode is enabled so that I get a complete picture of the current page.

## MCP Interface

### Enabling screenshots via `configure`

**Tool:** `configure`
**Action:** `capture` (existing action -- add `screenshot_mode` to supported settings)

#### Request
```json
{
  "tool": "configure",
  "arguments": {
    "action": "capture",
    "settings": {
      "screenshot_mode": "on"
    }
  }
}
```

#### Valid values for `screenshot_mode`
| Value | Behavior |
|-------|----------|
| `"off"` | No screenshots in MCP responses (default) |
| `"on"` | Append screenshot to every `observe` and `generate` response |
| `"errors_only"` | Append screenshot only to `observe({what: "errors"})` responses |

#### Response
```json
{
  "content": [
    {"type": "text", "text": "Capture settings updated: screenshot_mode=on"}
  ]
}
```

### Observe response with screenshot (when enabled)

**Tool:** `observe`
**Mode:** Any (e.g., `errors`, `page`, `logs`)

#### Request (unchanged)
```json
{
  "tool": "observe",
  "arguments": {
    "what": "errors"
  }
}
```

#### Response (with screenshot appended)
```json
{
  "content": [
    {
      "type": "text",
      "text": "3 browser error(s)\n\n| # | Type | Message | URL | Line |\n..."
    },
    {
      "type": "image",
      "data": "/9j/4AAQSkZJRg...<base64 JPEG>...",
      "mimeType": "image/jpeg"
    }
  ]
}
```

The image block is always the **last** content block in the array, after all text blocks. If screenshot capture fails (no tracked tab, permission denied, rate-limited), the response includes a text note instead of silently omitting it:

```json
{
  "type": "text",
  "text": "[Screenshot unavailable: no tracked tab]"
}
```

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | Add `screenshot_mode` setting to `configure({action: "capture"})` with values `off`, `on`, `errors_only` | must |
| R2 | When `screenshot_mode` is `on`, append a base64 JPEG image content block to every `observe` and `generate` response | must |
| R3 | When `screenshot_mode` is `errors_only`, append screenshot only to `observe({what: "errors"})` responses | must |
| R4 | Use existing `captureVisibleTab` mechanism in the extension (JPEG, quality 80) | must |
| R5 | Screenshot is captured on-demand at response time via the async command pipeline (not from disk cache) | must |
| R6 | Respect existing rate limits: 5s cooldown between screenshots, 10/session max | must |
| R7 | If screenshot capture fails or is rate-limited, append a text block explaining why (never silently omit) | must |
| R8 | Add `MCPImageContentBlock` struct to Go server with fields: `Type`, `Data`, `MimeType` | must |
| R9 | Screenshot JPEG quality reduced to 60 (from 80) when delivered via MCP to reduce payload size | should |
| R10 | Server logs a warning if screenshot payload exceeds 500KB | should |
| R11 | `screenshot_mode` setting resets to `off` on server restart (session-scoped, matches existing capture overrides) | must |
| R12 | Default `screenshot_mode` is `off` to avoid unexpected token costs | must |

## Non-Goals

- This feature does NOT paste screenshots into the IDE via clipboard or OS-level paste actions. The "auto-paste" is achieved by including the image in the MCP response, which the MCP client (Cursor, Claude Code, etc.) renders natively. No clipboard, no AppleScript, no accessibility API.
- This feature does NOT add a new MCP tool. It extends existing `observe`/`generate` response payloads and adds a new value to the existing `configure({action: "capture"})` settings.
- Out of scope: Video capture or animated screenshots (GIFs). A single static JPEG is sufficient.
- Out of scope: Screenshot annotation, cropping, or highlighting. The raw browser viewport capture is returned as-is.
- Out of scope: Serving screenshots from disk. The existing disk-saved screenshots (from `handleScreenshot`) are a separate feature. This feature captures a fresh screenshot on-demand for each MCP response.

## Performance SLOs

| Metric | Target | Notes |
|--------|--------|-------|
| Screenshot capture latency | < 500ms | `captureVisibleTab` is typically ~100-200ms; async pipeline adds ~200ms |
| JPEG payload size | < 500KB typical | 1920x1080 JPEG at quality 60 is ~150-300KB |
| Base64 encoding overhead | < 50ms | Standard library base64 encoding of ~300KB |
| Total response time increase | < 1s | Acceptable for visual debugging; AI agents are not latency-sensitive |
| Memory impact (server) | < 2MB transient | Base64 string held in memory only during response serialization, then GC'd |

### Payload Size Analysis

A screenshot at 1920x1080 resolution:
- JPEG quality 80: ~200-400KB raw, ~270-530KB base64
- JPEG quality 60: ~100-250KB raw, ~135-335KB base64
- JPEG quality 40: ~60-150KB raw, ~80-200KB base64

At quality 60 (R9), typical screenshots will be 150-300KB base64. This is significant but acceptable:
- MCP responses for `observe({what: "network_waterfall"})` with 1000 entries can already exceed 200KB of text
- Claude and other multimodal models handle image inputs of this size routinely
- The setting defaults to `off`, so users opt in knowing the cost

## Security Considerations

- **Data captured:** The full visible browser viewport, which may contain sensitive information (PII, passwords, financial data). This is the same data already captured by the existing `captureVisibleTab` screenshot feature.
- **No new attack surface:** Screenshots are captured by the same extension code that already exists. The only new path is encoding to base64 and including in MCP responses (localhost-only, same as all other Kaboom data).
- **Privacy:** Screenshot mode defaults to `off`. The AI agent must explicitly enable it. The existing `screenshotOnError` setting and this new `screenshot_mode` are independent controls.
- **Sensitive content warning:** When `screenshot_mode` is first set to `on` or `errors_only`, the response should include a note: "Screenshots may contain sensitive page content (passwords, PII). Ensure your MCP client handles image data appropriately."
- **No persistence change:** Screenshots included in MCP responses are transient (in-memory only during serialization). They are NOT saved to disk by this feature. The existing disk-save behavior via `/screenshots` endpoint is unchanged and independent.

## Edge Cases

- **No tracked tab:** If no tab is being tracked, screenshot capture is skipped. A text block `[Screenshot unavailable: no tracked tab]` is appended instead. Expected behavior: response still contains all text data; only the image block is replaced with an explanation.
- **Extension disconnected:** If the extension is not connected, the async command to capture a screenshot will time out. Expected behavior: append `[Screenshot unavailable: extension not connected]` text block after the standard async timeout window.
- **Tab closed during capture:** The `chrome.tabs.captureVisibleTab` call may fail if the tab closes mid-capture. Expected behavior: graceful failure, append text explanation, do not crash or corrupt the response.
- **Rate-limited:** If screenshots are requested faster than the 5s cooldown or the session max (10) is exceeded, capture is skipped. Expected behavior: append `[Screenshot unavailable: rate-limited (5s cooldown)]` or `[Screenshot unavailable: session limit reached (10/10)]`.
- **Concurrent MCP requests:** Two MCP requests arriving simultaneously both want screenshots. Expected behavior: first gets the screenshot, second may be rate-limited. This is acceptable -- the data is near-identical at sub-5s intervals.
- **Very large viewport:** On 4K displays (3840x2160), JPEG at quality 60 could be ~500KB-1MB base64. Expected behavior: capture proceeds but server logs a warning (R10). No hard cap -- the AI model can handle it.
- **`observe` modes with no visual relevance:** Modes like `extension_logs`, `websocket_status`, or `pending_commands` have no visual component. Expected behavior: screenshot is still captured when `screenshot_mode` is `on` (the page state is always relevant context). The AI can configure `errors_only` if it wants to be selective.

## Architecture

### Go Server Changes

1. **New content block type:** Extend `MCPContentBlock` (or add `MCPImageContentBlock`) to support `type: "image"` with `data` and `mimeType` fields:

```go
// MCPImageContentBlock represents a base64-encoded image in an MCP tool result.
type MCPImageContentBlock struct {
    Type     string `json:"type"`     // "image"
    Data     string `json:"data"`     // base64-encoded JPEG (no data URL prefix)
    MimeType string `json:"mimeType"` // "image/jpeg"
}
```

Since the existing `MCPContentBlock` struct has only `Type` and `Text` fields, the cleanest approach is to use `interface{}` in the `MCPToolResult.Content` slice, or define a union type. Implementation detail to resolve during development.

2. **Screenshot capture via async command pipeline:** The server already has the async command infrastructure (pending queries, correlation IDs, polling). To capture a screenshot on-demand:
   - Server creates a pending query of type `screenshot` with a correlation ID
   - Extension polls `/pending-queries`, sees the screenshot request, calls `captureVisibleTab`
   - Extension POSTs the base64 data URL back (via existing `/screenshots` or a new result endpoint)
   - Server extracts the base64 data and appends it to the MCP response

3. **Response augmentation:** After the primary tool handler produces its `JSONRPCResponse`, a post-processing step checks `screenshot_mode`. If enabled (and applicable), it triggers the screenshot capture and appends the image block.

### Extension Changes

Minimal. The extension already:
- Captures screenshots via `captureVisibleTab` (JPEG, quality 80)
- Rate-limits captures (5s cooldown, 10/session max)
- Responds to pending queries from the server

The only new work is handling a `screenshot` query type in the pending-query poll handler and returning the raw base64 data (without saving to disk).

### Estimated Implementation Effort

| Component | Lines | Description |
|-----------|-------|-------------|
| Go: `MCPImageContentBlock` struct + serialization | ~20 | New struct, update `MCPToolResult.Content` to support mixed types |
| Go: `screenshot_mode` capture setting | ~15 | Add to `validSettings`, `defaultSettings` in `capture_control.go` |
| Go: Response augmentation pipeline | ~50 | Post-processing step to capture + append screenshot |
| Go: Async screenshot request/response | ~30 | Pending query creation, result handling, base64 extraction |
| Extension: Pending query handler for screenshots | ~30 | Handle `screenshot` query type, return base64 data |
| Tests | ~80 | Unit tests for settings, content block serialization, rate limiting, edge cases |
| **Total** | **~225** | Modest increase from initial ~150 estimate due to async pipeline |

## Dependencies

- Depends on: Async command pipeline (v6.0.0) for on-demand screenshot capture
- Depends on: `captureVisibleTab` extension permission (already granted in manifest)
- Depends on: `configure({action: "capture"})` settings infrastructure (already exists)
- Depended on by: Future visual regression features (e.g., `generate({format: "visual_diff"})`)

## Assumptions

- A1: The extension is connected and tracking a tab (required for `captureVisibleTab`)
- A2: The MCP client supports `image` content blocks (Claude Code, Cursor do; others may not)
- A3: The tracked tab is visible (not minimized) -- `captureVisibleTab` captures the visible area of the window
- A4: AI models receiving the screenshots are multimodal (can process image inputs)
- A5: The async command pipeline can complete a screenshot capture within ~500ms typical

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Should `MCPToolResult.Content` use `interface{}` slices or a tagged union type? | open | `interface{}` is simpler but loses type safety. A tagged union (with `Type` discriminator) is cleaner but requires custom JSON marshaling. |
| OI-2 | Should screenshots also attach to `generate` responses (reproduction, test, HAR)? | open | Generate outputs are code/data artifacts -- visual context may be less useful. Recommend `on` mode includes them, `errors_only` excludes them. |
| OI-3 | Should there be a `screenshot_quality` configure setting (40/60/80)? | open | Could help users on slow connections or with token-cost concerns. Low priority -- quality 60 is a good default. |
| OI-4 | Should the server cache the last screenshot for ~2s to avoid redundant captures on rapid successive observe calls? | open | Would reduce extension load but adds staleness. The 5s rate limit already handles rapid calls. |
| OI-5 | How do non-multimodal MCP clients handle image content blocks? | open | MCP spec says clients should ignore unknown content types. Need to verify Claude Code and Cursor behavior. |
