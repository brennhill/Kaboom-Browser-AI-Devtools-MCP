---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Kaboom Extension Message Protocol

This document describes the Chrome runtime message types and handlers used internally by the Kaboom extension. This is for extension developers and integration partners.

## Message Architecture

The extension uses `chrome.runtime.sendMessage()` (from content scripts to background) and `chrome.tabs.sendMessage()` (from background to content scripts) for communication. All messages are validated for sender origin before processing.

### Message Flow

```
Content Script → Background Service Worker (chrome.runtime.sendMessage)
                          ↓
                  Message Handler Router
                          ↓
                  Type-specific Handler
                          ↓
                     Processing
                          ↓
                    sendResponse()

Background → Content Script (chrome.tabs.sendMessage)
            or Popup (chrome.runtime.sendMessage)
```

## Background-Bound Messages

Messages sent from content scripts to the background service worker via `chrome.runtime.sendMessage()`.

### Data Capture Messages

These messages carry telemetry data from the page to the background service worker.

#### `ws_event`
WebSocket events captured by the content script.

```typescript
{
  type: 'ws_event',
  payload: WebSocketEvent,
  tabId?: number
}
```

**Handler:** `background/message-handlers.js`
**Processing:** Added to WebSocket event batcher
**Response:** `false` (no response)

#### `enhanced_action`
Enhanced user actions with multi-strategy selectors.

```typescript
{
  type: 'enhanced_action',
  payload: EnhancedAction,
  tabId?: number
}
```

**Payload Example:**
```json
{
  "type": "click",
  "timestamp": 1707123456789,
  "url": "https://example.com/checkout",
  "selectors": {
    "testId": "submit-btn",
    "role": { "role": "button", "name": "Submit" },
    "text": "Submit",
    "cssPath": "main > form > button.primary"
  }
}
```

**Handler:** `background/message-handlers.js`
**Processing:** Added to enhanced action batcher
**Response:** `false` (no response)

#### `network_body`
Network request/response bodies and metadata.

```typescript
{
  type: 'network_body',
  payload: NetworkBodyPayload,
  tabId: number
}
```

**Payload Fields:**
- `url` — Request URL
- `method` — HTTP method (GET, POST, etc.)
- `status` — Response status code
- `requestBody` — Request body (if captured)
- `responseBody` — Response body (if captured)
- `headers` — Response headers
- `duration` — Request duration in ms

**Handler:** `background/message-handlers.js`
**Security:** Capture can be disabled via feature toggle
**Response:** `false` (no response)

#### `performance_snapshot`
Performance timing and metrics snapshot.

```typescript
{
  type: 'performance_snapshot',
  payload: PerformanceSnapshot,
  tabId?: number
}
```

**Handler:** `background/message-handlers.js`
**Processing:** Added to performance batcher
**Response:** `false` (no response)

#### `log`
Console logs, errors, and exceptions from the page.

```typescript
{
  type: 'log',
  payload: LogEntry,
  tabId?: number
}
```

**Payload Fields:**
- `level` — 'debug' | 'log' | 'info' | 'warn' | 'error'
- `message` — Log message
- `stack` — Stack trace (for errors)
- `timestamp` — Milliseconds since epoch
- `source` — 'console' | 'exception'
- `context` — Custom context annotations

**Handler:** `background/message-handlers.js` → `handleLogMessageAsync()`
**Processing:** Async log processing with context enrichment
**Response:** Async

### Settings & Configuration Messages

#### `setLogLevel`
Set the current minimum log level.

```typescript
{
  type: 'setLogLevel',
  level: 'debug' | 'log' | 'info' | 'warn' | 'error'
}
```

**Handler:** `background/message-handlers.js`
**Persistence:** Saved to chrome.storage.local
**Response:** `false` (no response)

#### `setScreenshotOnError`
Enable/disable automatic screenshot capture on error.

```typescript
{
  type: 'setScreenshotOnError',
  enabled: boolean
}
```

**Handler:** `background/message-handlers.js`
**Response:**
```json
{ "success": true }
```

#### `setAiWebPilotEnabled`
Enable/disable AI Web Pilot feature.

```typescript
{
  type: 'setAiWebPilotEnabled',
  enabled: boolean
}
```

**Handler:** `background/message-handlers.js` → `handleSetAiWebPilotEnabled()`
**Persistence:** Saved to chrome.storage.local
**Side Effects:** Broadcasts tracking state to tracked tab
**Response:**
```json
{ "success": true }
```

#### `setSourceMapEnabled`
Enable/disable source map processing.

```typescript
{
  type: 'setSourceMapEnabled',
  enabled: boolean
}
```

**Handler:** `background/message-handlers.js`
**Persistence:** Saved to chrome.storage.local
**Side Effects:** Clears source map cache if disabled
**Response:**
```json
{ "success": true }
```

#### Boolean Feature Toggle Messages

Sent to content scripts to forward to inject script:

```typescript
{
  type: 'setNetworkWaterfallEnabled' |
        'setPerformanceMarksEnabled' |
        'setActionReplayEnabled' |
        'setWebSocketCaptureEnabled' |
        'setPerformanceSnapshotEnabled' |
        'setDeferralEnabled' |
        'setNetworkBodyCaptureEnabled' |
        'setActionToastsEnabled' |
        'setSubtitlesEnabled' |
        'setDebugMode',
  enabled: boolean
}
```

**Handler:** `background/message-handlers.js` → `handleForwardedSetting()`
**Processing:** Forwarded to all tracked content scripts
**Response:**
```json
{ "success": true }
```

#### `setWebSocketCaptureMode`
Set WebSocket capture detail level.

```typescript
{
  type: 'setWebSocketCaptureMode',
  mode: 'minimal' | 'headers' | 'medium' | 'full'
}
```

**Handler:** `background/message-handlers.js` → `handleForwardedSetting()`
**Response:**
```json
{ "success": true }
```

#### `setServerUrl`
Change the MCP server URL.

```typescript
{
  type: 'setServerUrl',
  url: string
}
```

**Handler:** `background/message-handlers.js` → `handleSetServerUrl()`
**Persistence:** Saved to chrome.storage.local
**Side Effects:**
- Broadcasts to all content scripts
- Re-checks connection with new URL
**Response:**
```json
{ "success": true }
```

### Query & State Messages

#### `getStatus`
Get current extension connection and configuration status.

```typescript
{ type: 'getStatus' }
```

**Handler:** `background/message-handlers.js`
**Response:**
```json
{
  "connected": boolean,
  "serverUrl": string,
  "screenshotOnError": boolean,
  "sourceMapEnabled": boolean,
  "debugMode": boolean,
  "contextWarning": string | null,
  "circuitBreakerState": string,
  "memoryPressure": { "high": boolean }
}
```

#### `getAiWebPilotEnabled`
Check if AI Web Pilot is enabled.

```typescript
{ type: 'getAiWebPilotEnabled' }
```

**Handler:** `background/message-handlers.js`
**Response:**
```json
{ "enabled": boolean }
```

#### `getTrackingState`
Get tracking and AI Pilot state (used by favicon replacer).

```typescript
{ type: 'getTrackingState' }
```

**Handler:** `background/message-handlers.js` → `handleGetTrackingState()`
**Response:**
```json
{
  "state": {
    "isTracked": boolean,
    "aiPilotEnabled": boolean
  }
}
```

#### `getDiagnosticState`
Get diagnostic cache/storage state.

```typescript
{ type: 'getDiagnosticState' }
```

**Handler:** `background/message-handlers.js` → `handleGetDiagnosticState()`
**Response:**
```json
{
  "cache": boolean,
  "storage": boolean | undefined,
  "timestamp": string (ISO 8601)
}
```

#### `clearLogs`
Clear all captured logs from the server.

```typescript
{ type: 'clearLogs' }
```

**Handler:** `background/message-handlers.js` → `handleClearLogsAsync()`
**Response:**
```json
{ "success": boolean, "error"?: string }
```

#### `captureScreenshot`
Capture a screenshot of the current active tab.

```typescript
{ type: 'captureScreenshot' }
```

**Handler:** `background/message-handlers.js` → `handleCaptureScreenshot()`
**Processing:** Async screenshot capture and server relay
**Response:**
```json
{ "success": boolean, "error"?: string }
```

#### `GET_TAB_ID`
Get the current tab ID.

```typescript
{ type: 'GET_TAB_ID' }
```

**Handler:** `background/message-handlers.js`
**Response:**
```json
{ "tabId": number }
```

#### Debug Messages

##### `getDebugLog`
```typescript
{ type: 'getDebugLog' }
```

**Response:**
```json
{ "log": string[] }
```

##### `clearDebugLog`
```typescript
{ type: 'clearDebugLog' }
```

**Response:**
```json
{ "success": true }
```

##### `setDebugMode`
```typescript
{
  type: 'setDebugMode',
  enabled: boolean
}
```

**Response:**
```json
{ "success": true }
```

## Content-Bound Messages

Messages sent from background/popup to content scripts via `chrome.tabs.sendMessage()`.

### Control Messages

#### `KABOOM_PING`
Health check to verify content script is loaded.

```typescript
{ type: 'KABOOM_PING' }
```

**Handler:** `content/message-handlers.js`
**Response:**
```json
{ "status": "alive", "timestamp": number }
```

#### Setting Toggle Messages (Content Script)

```typescript
{
  type: 'setNetworkWaterfallEnabled' |
        'setPerformanceMarksEnabled' |
        'setActionReplayEnabled' |
        'setWebSocketCaptureEnabled' |
        'setPerformanceSnapshotEnabled' |
        'setDeferralEnabled' |
        'setNetworkBodyCaptureEnabled' |
        'setActionToastsEnabled' |
        'setSubtitlesEnabled' |
        'setDebugMode',
  enabled?: boolean,
  mode?: string,
  url?: string
}
```

**Handler:** `content/message-handlers.js` → `handleToggleMessage()`
**Processing:** Converted to `KABOOM_SETTING` and posted to inject script
**Response:** (auto-forwarded)

### Query Messages (Content Script)

#### `KABOOM_EXECUTE_JS`
Execute arbitrary JavaScript in page context (MAIN world).

```typescript
{
  type: 'KABOOM_EXECUTE_JS',
  params: {
    script: string,
    timeout_ms?: number
  }
}
```

**Handler:** `content/message-handlers.js` → `handleExecuteJs()`
**Processing:**
1. Checks if inject script is loaded
2. Posts to inject script via postMessage
3. Returns result or 'inject_not_loaded' error
**Response:**
```json
{
  "success": boolean,
  "result": any,
  "error": string,
  "message": string,
  "stack": string
}
```

#### `KABOOM_EXECUTE_QUERY`
Execute query via async polling system.

```typescript
{
  type: 'KABOOM_EXECUTE_QUERY',
  queryId: string,
  params: string | object
}
```

**Handler:** `content/message-handlers.js` → `handleExecuteQuery()`
**Response:** Deferred (via polling system)

#### `DOM_QUERY`
Query DOM and return structured element data.

```typescript
{
  type: 'DOM_QUERY',
  params: {
    selector: string,
    limit?: number,
    includeHtml?: boolean
  }
}
```

**Handler:** `content/message-handlers.js` → `handleDomQuery()`
**Response:**
```json
{
  "url": string,
  "title": string,
  "matchCount": number,
  "returnedCount": number,
  "matches": [
    {
      "tag": string,
      "text": string,
      "visible": boolean,
      "attributes": object,
      "boundingBox": { "x": number, "y": number, "width": number, "height": number }
    }
  ]
}
```

#### `A11Y_QUERY`
Run accessibility audit via axe-core.

```typescript
{
  type: 'A11Y_QUERY',
  params?: {
    selector?: string,
    runOnly?: string[]
  }
}
```

**Handler:** `content/message-handlers.js` → `handleA11yQuery()`
**Response:**
```json
{
  "violations": [
    {
      "id": string,
      "impact": "critical" | "serious" | "moderate" | "minor",
      "nodes": [{ "html": string, "impact": string }]
    }
  ],
  "passes": []
}
```

#### `GET_NETWORK_WATERFALL`
Get network performance entries.

```typescript
{ type: 'GET_NETWORK_WATERFALL' }
```

**Handler:** `content/message-handlers.js` → `handleGetNetworkWaterfall()`
**Response:**
```json
{
  "entries": [
    {
      "name": string,
      "initiator_type": string,
      "start_time": number,
      "duration": number,
      "transfer_size": number,
      "encoded_body_size": number,
      "decoded_body_size": number
    }
  ]
}
```

### State Management Messages

#### `KABOOM_MANAGE_STATE`
Capture or restore browser state.

```typescript
{
  type: 'KABOOM_MANAGE_STATE',
  params: {
    action: 'capture' | 'restore',
    name?: string,
    state?: BrowserStateSnapshot,
    include_url?: boolean
  }
}
```

**Handler:** `content/message-handlers.js` → `handleStateCommand()`
**Response:**
```json
{
  "success": boolean,
  "snapshot_name": string,
  "size_bytes": number,
  "error"?: string
}
```

### UI Messages

#### `KABOOM_ACTION_TOAST`
Display a visual action indicator (color-coded state).

```typescript
{
  type: 'KABOOM_ACTION_TOAST',
  text: string,
  detail?: string,
  state?: 'trying' | 'success' | 'warning' | 'error',
  duration_ms?: number
}
```

**Handler:** `content/message-handlers.js`
**Processing:** Rendered as overlay toast
**States:**
- `trying` — Orange (action in progress)
- `success` — Green (completed successfully)
- `warning` — Amber (warning state)
- `error` — Red (failed)

#### `KABOOM_SUBTITLE`
Display persistent narration text (like closed captions).

```typescript
{
  type: 'KABOOM_SUBTITLE',
  text: string
}
```

**Handler:** `content/message-handlers.js`
**Processing:** Rendered at bottom of viewport
**Notes:** Empty string clears the subtitle

### Highlight Messages

#### `KABOOM_HIGHLIGHT`
Highlight an element by selector.

```typescript
{
  type: 'KABOOM_HIGHLIGHT',
  params: {
    selector: string,
    duration_ms?: number
  }
}
```

**Handler:** `content/message-handlers.js` → `forwardHighlightMessage()`
**Response:**
```json
{
  "success": boolean,
  "selector": string,
  "bounds": { "x": number, "y": number, "width": number, "height": number },
  "error"?: string
}
```

## Inter-Script Communication (postMessage)

The extension also uses `window.postMessage()` for MAIN-world page context communication (inject script ↔ content script).

### Content-to-Inject Messages

```typescript
{
  type: 'KABOOM_SETTING',
  setting: string,
  enabled?: boolean,
  mode?: string,
  url?: string
}
```

```typescript
{
  type: 'KABOOM_EXECUTE_JS',
  requestId: number | string,
  script: string,
  timeoutMs?: number
}
```

```typescript
{
  type: 'KABOOM_A11Y_QUERY',
  requestId: number | string,
  params?: object
}
```

```typescript
{
  type: 'KABOOM_DOM_QUERY',
  requestId: number | string,
  params?: object
}
```

```typescript
{
  type: 'KABOOM_GET_WATERFALL',
  requestId: number | string
}
```

```typescript
{
  type: 'KABOOM_HIGHLIGHT_REQUEST',
  requestId: number | string,
  params: { selector: string, duration_ms?: number }
}
```

```typescript
{
  type: 'KABOOM_STATE_COMMAND',
  messageId: string,
  action: 'capture' | 'restore',
  state?: object,
  include_url?: boolean
}
```

### Inject-to-Content Messages

```typescript
{
  type: 'KABOOM_SETTING',
  setting: string,
  enabled?: boolean
}
```

```typescript
{
  type: 'KABOOM_EXECUTE_JS_RESULT',
  requestId: number | string,
  result: { success: boolean, result?: any, error?: string }
}
```

```typescript
{
  type: 'KABOOM_A11Y_QUERY_RESPONSE',
  requestId: number | string,
  result: { violations?: any[], error?: string }
}
```

```typescript
{
  type: 'KABOOM_DOM_QUERY_RESPONSE',
  requestId: number | string,
  result: { matches?: any[], error?: string }
}
```

```typescript
{
  type: 'KABOOM_STATE_RESPONSE',
  messageId: string,
  result: { success?: boolean, error?: string }
}
```

```typescript
{
  type: 'KABOOM_WATERFALL_RESPONSE',
  requestId: number | string,
  entries: any[],
  pageURL?: string
}
```

```typescript
{
  type: 'KABOOM_HIGHLIGHT_RESPONSE',
  requestId: number | string,
  success: boolean,
  bounds?: { x, y, width, height }
}
```

## Error Handling

### Message Validation

All messages are validated for:
1. **Sender origin** — Ensure message comes from extension/content script
2. **Message type** — Discriminate by `type` field
3. **Payload shape** — Validate required fields and types

Invalid messages are logged and silently ignored.

### Async Timeouts

Messages that expect responses use timeouts:
- DOM/A11y queries: 30 seconds
- State commands: 5 seconds
- Execute JS: User-specified (default 5000ms)
- Network waterfall: 5 seconds

Timeout responses include descriptive error messages.

### Failed Sender Validation

Messages from untrusted sources (unprivileged web pages) are rejected:

```javascript
if (!isValidMessageSender(sender)) {
  console.error('Rejected message from untrusted sender');
  return false;
}
```

## Version Compatibility

**Breaking Changes:**
- v5.3+: Network body messages include `tabId`
- v5.2+: Enhanced actions format changed

**Safe Patterns:**
- Check message type with `message.type === 'expected-type'`
- Handle optional fields gracefully
- Never assume field presence without checking

## Security Considerations

1. **Always validate sender** — Reject messages from web pages
2. **Never execute untrusted code** — Script execution runs in page context
3. **Sanitize inputs** — Validate all message parameters
4. **Rate limit** — Consider message frequency for denial-of-service
5. **No XSS via messages** — All outputs are properly escaped

## Examples

### Sending Message from Content Script

```javascript
chrome.runtime.sendMessage({
  type: 'log',
  payload: {
    level: 'error',
    message: 'Something went wrong',
    timestamp: Date.now()
  }
}, (response) => {
  if (chrome.runtime.lastError) {
    console.error('Message failed:', chrome.runtime.lastError);
  }
});
```

### Listening in Background

```javascript
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (!isValidMessageSender(sender)) return false;

  if (message.type === 'log') {
    handleLogMessage(message.payload, sender);
    return true;
  }
  return false;
});
```

### Sending Message from Background

```javascript
chrome.tabs.sendMessage(tabId, {
  type: 'KABOOM_EXECUTE_JS',
  params: {
    script: 'document.title',
    timeout_ms: 5000
  }
}, (response) => {
  console.log('Script result:', response.result);
});
```
