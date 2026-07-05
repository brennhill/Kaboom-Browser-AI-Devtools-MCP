---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Agent Assignment: highlight_element

**Branch:** `feature/pilot-highlight`
**Worktree:** `../kaboom-pilot-highlight`
**Priority:** P4 Phase 2 (parallel — requires Phase 1 complete)
**Dependency:** Merge `feature/pilot-toggle` first

---

## Objective

Implement `highlight_element` MCP tool that injects a visual overlay on DOM elements so the AI can point at things for human verification.

---

## Deliverables

### 1. Inject.js Handler

**File:** `extension/inject.js`

Add in AI Web Pilot section:
```javascript
// ============================================================================
// AI WEB PILOT: HIGHLIGHT
// ============================================================================

let kaboomHighlighter = null

function highlightElement(selector, durationMs = 5000) {
  // Remove existing highlight
  if (kaboomHighlighter) {
    kaboomHighlighter.remove()
    kaboomHighlighter = null
  }

  const element = document.querySelector(selector)
  if (!element) {
    return { success: false, error: 'element_not_found', selector }
  }

  const rect = element.getBoundingClientRect()

  kaboomHighlighter = document.createElement('div')
  kaboomHighlighter.id = 'kaboom-highlighter'
  Object.assign(kaboomHighlighter.style, {
    position: 'fixed',
    top: `${rect.top}px`,
    left: `${rect.left}px`,
    width: `${rect.width}px`,
    height: `${rect.height}px`,
    border: '4px solid red',
    borderRadius: '4px',
    backgroundColor: 'rgba(255, 0, 0, 0.1)',
    zIndex: '2147483647',
    pointerEvents: 'none',
    boxSizing: 'border-box'
  })

  document.body.appendChild(kaboomHighlighter)

  setTimeout(() => {
    if (kaboomHighlighter) {
      kaboomHighlighter.remove()
      kaboomHighlighter = null
    }
  }, durationMs)

  return {
    success: true,
    selector,
    bounds: { x: rect.x, y: rect.y, width: rect.width, height: rect.height }
  }
}

// Handle scroll — update position
window.addEventListener('scroll', () => {
  if (kaboomHighlighter) {
    const selector = kaboomHighlighter.dataset.selector
    if (selector) {
      const el = document.querySelector(selector)
      if (el) {
        const rect = el.getBoundingClientRect()
        kaboomHighlighter.style.top = `${rect.top}px`
        kaboomHighlighter.style.left = `${rect.left}px`
      }
    }
  }
}, { passive: true })
```

### 2. Message Routing

**File:** `extension/background.js`

Handle `KABOOM_HIGHLIGHT` message:
- Check `isAiWebPilotEnabled()`
- Forward to content script → inject.js
- Return result to server

**File:** `extension/content.js`

Forward `KABOOM_HIGHLIGHT` to page context, return response.

### 3. MCP Tool Handler

**File:** `cmd/browser-agent/pilot.go`

```go
func (v *Capture) handleHighlightElement(params map[string]any) (any, error) {
    selector, ok := params["selector"].(string)
    if !ok || selector == "" {
        return nil, errors.New("selector is required")
    }

    durationMs := 5000
    if d, ok := params["duration_ms"].(float64); ok {
        durationMs = int(d)
    }

    // Send to extension, wait for response
    result := v.sendPilotCommand("highlight", map[string]any{
        "selector":    selector,
        "duration_ms": durationMs,
    })

    return result, nil
}
```

---

## Tests

**File:** `extension-tests/pilot-highlight.test.js` (new)

1. Highlight creates div with correct styles
2. Highlight positions on element bounds
3. Highlight auto-removes after duration
4. Second highlight removes first
5. Returns error for non-existent selector
6. Highlight updates position on scroll

---

## Verification

```bash
node --test extension-tests/pilot-highlight.test.js
go test -v ./cmd/browser-agent/ -run Highlight
```

---

## Files Modified

| File | Change |
|------|--------|
| `extension/inject.js` | `highlightElement()` function |
| `extension/background.js` | Route KABOOM_HIGHLIGHT |
| `extension/content.js` | Forward highlight message |
| `cmd/browser-agent/pilot.go` | `handleHighlightElement()` |
| `extension-tests/pilot-highlight.test.js` | New file |
