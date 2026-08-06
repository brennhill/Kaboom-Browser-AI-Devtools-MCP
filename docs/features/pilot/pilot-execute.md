---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Agent Assignment: execute_javascript

**Branch:** `feature/pilot-execute`
**Worktree:** `../kaboom-pilot-execute`
**Priority:** P4 Phase 2 (parallel — requires Phase 1 complete)
**Dependency:** Merge `feature/pilot-toggle` first

---

## Objective

Implement `execute_javascript` MCP tool that runs arbitrary JavaScript in the browser context and returns JSON-serialized results. Enables AI to inspect Redux stores, globals, and framework state without human intervention.

---

## Deliverables

### 1. Inject.js Handler

**File:** `extension/inject.js`

Add in AI Web Pilot section:
```javascript
// ============================================================================
// AI WEB PILOT: EXECUTE JAVASCRIPT
// ============================================================================

function executeJavaScript(script, timeoutMs = 5000) {
  return new Promise((resolve) => {
    const timeoutId = setTimeout(() => {
      resolve({
        success: false,
        error: 'execution_timeout',
        message: `Script exceeded ${timeoutMs}ms timeout`
      })
    }, timeoutMs)

    try {
      // Use Function constructor to execute in global scope
      // This runs in page context (inject.js), not extension context
      const fn = new Function(`
        "use strict";
        return (${script});
      `)

      const result = fn()

      clearTimeout(timeoutId)

      // Handle promises
      if (result && typeof result.then === 'function') {
        result
          .then(value => {
            resolve({ success: true, result: safeSerialize(value) })
          })
          .catch(err => {
            resolve({
              success: false,
              error: 'promise_rejected',
              message: err.message,
              stack: err.stack
            })
          })
      } else {
        resolve({ success: true, result: safeSerialize(result) })
      }
    } catch (err) {
      clearTimeout(timeoutId)
      resolve({
        success: false,
        error: 'execution_error',
        message: err.message,
        stack: err.stack
      })
    }
  })
}

// Safe serialization for complex objects
function safeSerializeForExecute(value, depth = 0, seen = new WeakSet()) {
  if (depth > 10) return '[max depth exceeded]'
  if (value === null) return null
  if (value === undefined) return undefined

  const type = typeof value
  if (type === 'string' || type === 'number' || type === 'boolean') {
    return value
  }

  if (type === 'function') {
    return `[Function: ${value.name || 'anonymous'}]`
  }

  if (type === 'symbol') {
    return value.toString()
  }

  if (type === 'object') {
    if (seen.has(value)) return '[Circular]'
    seen.add(value)

    if (Array.isArray(value)) {
      return value.slice(0, 100).map(v => safeSerializeForExecute(v, depth + 1, seen))
    }

    if (value instanceof Error) {
      return { error: value.message, stack: value.stack }
    }

    if (value instanceof Date) {
      return value.toISOString()
    }

    if (value instanceof RegExp) {
      return value.toString()
    }

    // DOM nodes
    if (value instanceof Node) {
      return `[${value.nodeName}${value.id ? '#' + value.id : ''}]`
    }

    // Plain objects
    const result = {}
    const keys = Object.keys(value).slice(0, 50)
    for (const key of keys) {
      try {
        result[key] = safeSerializeForExecute(value[key], depth + 1, seen)
      } catch {
        result[key] = '[unserializable]'
      }
    }
    if (Object.keys(value).length > 50) {
      result['...'] = `[${Object.keys(value).length - 50} more keys]`
    }
    return result
  }

  return String(value)
}
```

### 2. Message Routing

**File:** `extension/background.js`

Handle `KABOOM_EXECUTE_JS`:
- Check `isAiWebPilotEnabled()`
- Generate unique `request_id`
- Forward to content script → inject.js
- Wait for response with matching `request_id`
- Return result to server

**File:** `extension/content.js`

Forward `KABOOM_EXECUTE_JS` to page, return response.

### 3. MCP Tool Handler

**File:** `cmd/browser-agent/pilot.go`

```go
func (v *Capture) handleExecuteJavaScript(params map[string]any) (any, error) {
    script, ok := params["script"].(string)
    if !ok || script == "" {
        return nil, errors.New("script is required")
    }

    timeoutMs := 5000
    if t, ok := params["timeout_ms"].(float64); ok {
        timeoutMs = int(t)
    }

    result := v.sendPilotCommand("execute", map[string]any{
        "script":     script,
        "timeout_ms": timeoutMs,
    })

    return result, nil
}
```

---

## Tests

**File:** `extension-tests/pilot-execute.test.js` (new)

1. Simple expression: `1 + 1` → `{ result: 2 }`
2. Access globals: `window.location.href` → returns URL
3. Object serialization: `{ a: 1, b: [2, 3] }` → properly serialized
4. Function return: `(() => 42)()` → `{ result: 42 }`
5. Error handling: `throw new Error('test')` → error response with stack
6. Timeout: `while(true){}` → timeout error (mock with delay)
7. Promise resolution: `Promise.resolve(42)` → `{ result: 42 }`
8. Promise rejection: `Promise.reject(new Error('fail'))` → error response
9. Circular reference handling: doesn't crash
10. DOM node serialization: returns descriptive string

---

## Verification

```bash
node --test extension-tests/pilot-execute.test.js
go test -v ./cmd/browser-agent/ -run ExecuteJavaScript
```

---

## Security Notes

- Localhost-only (Kaboom binds to 127.0.0.1)
- Human opt-in required (AI Web Pilot toggle)
- No sandboxing — runs with full page privileges
- User responsibility for side effects

---

## Files Modified

| File | Change |
|------|--------|
| `extension/inject.js` | `executeJavaScript()`, `safeSerializeForExecute()` |
| `extension/background.js` | Route KABOOM_EXECUTE_JS |
| `extension/content.js` | Forward execute message |
| `cmd/browser-agent/pilot.go` | `handleExecuteJavaScript()` |
| `extension-tests/pilot-execute.test.js` | New file |
