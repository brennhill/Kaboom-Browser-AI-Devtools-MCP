---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Agent Assignment: AI Web Pilot Toggle Infrastructure

**Branch:** `feature/pilot-toggle`
**Worktree:** `../kaboom-pilot-toggle`
**Priority:** P4 Phase 1 (blocking — must complete before Phase 2 agents)

---

## Objective

Build the human opt-in toggle infrastructure for AI Web Pilot features. This is the safety gate that prevents AI from self-authorizing code execution.

---

## Deliverables

### 1. Extension Popup UI

**File:** `extension/popup.html`

Add toggle after existing settings:
```html
<div class="setting">
  <label>
    <input type="checkbox" id="aiWebPilotEnabled">
    AI Web Pilot
  </label>
  <span class="warning">⚠️ Allows AI to interact with page</span>
</div>
```

**File:** `extension/popup.js`

- Load state from `chrome.storage.sync.get('aiWebPilotEnabled')`
- Save on change: `chrome.storage.sync.set({ aiWebPilotEnabled: checked })`
- Default: `false` (disabled)

### 2. Background Script Gate

**File:** `extension/background.js`

Add function to check toggle before routing pilot commands:
```javascript
async function isAiWebPilotEnabled() {
  const { aiWebPilotEnabled } = await chrome.storage.sync.get('aiWebPilotEnabled')
  return aiWebPilotEnabled === true
}
```

When receiving `KABOOM_HIGHLIGHT`, `KABOOM_MANAGE_STATE`, or `KABOOM_EXECUTE_JS`:
- Check `isAiWebPilotEnabled()`
- If false, respond with `{ error: 'ai_web_pilot_disabled' }`
- If true, forward to content script / inject.js

### 3. Server-Side Check

**File:** `cmd/browser-agent/pilot.go` (new)

```go
// pilot.go — AI Web Pilot feature handlers.
// Implements highlight_element, manage_state, execute_javascript.
// All features require human opt-in via extension popup.

package main

// PilotDisabledError returned when toggle is off
var ErrPilotDisabled = errors.New("ai_web_pilot_disabled: enable 'AI Web Pilot' in extension popup")

func (v *Capture) handlePilotCommand(cmd string, params map[string]any) (any, error) {
    // Extension will return error if disabled
    // Server passes through the error to MCP response
}
```

### 4. MCP Tool Stubs

**File:** `cmd/browser-agent/tools_core.go`

Add tool schemas (implementation in Phase 2):
- `highlight_element` — selector (required), duration_ms (default 5000)
- `manage_state` — action (save|load|list|delete), snapshot_name
- `execute_javascript` — script (required), timeout_ms (default 5000)

All return "not enabled" error until Phase 2 agents implement handlers.

---

## Tests

**File:** `extension-tests/pilot-toggle.test.js` (new)

1. Toggle defaults to false
2. Toggle persists across popup open/close
3. Pilot commands rejected when toggle off
4. Pilot commands accepted when toggle on

**File:** `cmd/browser-agent/pilot_test.go` (new)

1. Tool schema validation
2. Error response format when disabled

---

## Verification

```bash
# Extension tests
node --test extension-tests/pilot-toggle.test.js

# Go tests
go test -v ./cmd/browser-agent/ -run Pilot

# Manual: Open popup, verify toggle exists, verify default off
```

---

## Files Modified

| File | Change |
|------|--------|
| `extension/popup.html` | Add toggle UI |
| `extension/popup.js` | Toggle state management |
| `extension/background.js` | Gate function + command routing |
| `cmd/browser-agent/pilot.go` | New file — feature stubs |
| `cmd/browser-agent/tools_core.go` | Add tool schemas |
| `extension-tests/pilot-toggle.test.js` | New file |
| `cmd/browser-agent/pilot_test.go` | New file |
