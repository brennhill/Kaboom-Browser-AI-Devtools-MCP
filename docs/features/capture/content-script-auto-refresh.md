---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Content Script Auto-Refresh After Navigation

## Problem

When `browser_action navigate` is called, Chrome navigates to the new URL but the Kaboom content script may not be loaded on the destination page. This causes subsequent AI Web Pilot tools (`highlight_element`, `execute_javascript`, `manage_state`) to fail with "content script not loaded" errors.

Currently, this requires manual page refresh, breaking the autonomous AI workflow.

## Solution

After `browser_action navigate` completes, automatically detect if the content script is loaded and refresh if necessary.

## Specification

### Detection Mechanism

The background script will ping the content script after navigation completes:

```javascript
// Send ping to content script
const response = await chrome.tabs.sendMessage(tabId, { type: 'KABOOM_PING' })
// If response received, content script is loaded
// If error thrown, content script is not loaded
```

### Auto-Refresh Logic

```
1. browser_action navigate is called
2. Wait for navigation to complete (chrome.tabs.update returns)
3. Wait brief delay for page load (500ms)
4. Ping content script
5. If ping succeeds → content script loaded, return success
6. If ping fails → refresh page, wait for load, return success with note
```

### Response Schema

The `browser_action navigate` response will include additional fields:

```json
{
  "action": "navigate",
  "success": true,
  "url": "https://example.com",
  "content_script_status": "loaded" | "refreshed",
  "message": "Content script ready" | "Page refreshed to load content script"
}
```

### Edge Cases

1. **file:// URLs**: Chrome restricts content script injection on file:// URLs unless explicitly enabled. Return clear error message.

2. **chrome:// URLs**: Content scripts cannot run on Chrome internal pages. Return error indicating this.

3. **Timeout**: If content script doesn't respond after refresh + 3 second wait, return error.

4. **Extension reload**: If extension was reloaded, all tabs need refresh. This is handled separately by session tracking.

### Content Script Ping Handler

Add to content.js:

```javascript
if (message.type === 'KABOOM_PING') {
  sendResponse({ status: 'alive', timestamp: Date.now() })
  return true
}
```

### Implementation Notes

- Ping timeout: 500ms (content script should respond immediately if loaded)
- Post-refresh wait: 1000ms (allow page to fully load)
- Maximum retry: 1 refresh attempt
- The refresh is silent to the user but communicated to the AI

## UAT Checklist

| # | Test Case | Expected | Pass |
|---|-----------|----------|------|
| 1 | Navigate to HTTPS page with content script loaded | `content_script_status: "loaded"`, no refresh | [ ] |
| 2 | Navigate to new HTTPS page (content script not loaded) | `content_script_status: "refreshed"`, page refreshed | [ ] |
| 3 | Navigate to file:// URL | Error or refresh with clear status | [ ] |
| 4 | Navigate to chrome:// URL | Error indicating internal page | [ ] |
| 5 | After refresh, execute_javascript works | Script executes successfully | [ ] |
| 6 | After refresh, highlight_element works | Element highlighted | [ ] |
