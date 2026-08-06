---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# CSP & Execute JS Strategies - Kaboom MCP

**Version:** v5.8+
**Last Updated:** 2026-02-07

---

## Overview

Content Security Policy (CSP) blocks many browser APIs used for code execution and DOM manipulation. Kaboom implements multiple execution strategies to work around CSP restrictions while maintaining security and functionality.

This document explains the mechanics of CSP enforcement, why certain patterns fail, and which strategies work on CSP-heavy sites like Gmail.

---

## The Problem: CSP Blocks Dynamic Code Execution

Content Security Policy (CSP) is a security mechanism that restricts which JavaScript can execute on a page. The relevant directives are:

```
script-src: Controls which scripts can execute
frame-ancestors: Controls which origins can frame the page
```

CSP uses the `unsafe-eval` keyword to allow/deny `new Function()` and `eval()`. When CSP is active and `unsafe-eval` is NOT present:

- ❌ `new Function()` - Blocked
- ❌ `eval()` - Blocked
- ❌ `Function.prototype.constructor()` - Blocked
- ✅ Pre-compiled functions - Allowed
- ✅ Native APIs - Allowed
- ✅ Event handlers - Allowed

---

## Execution Contexts & CSP Scope

JavaScript execution in Chrome extensions happens in different contexts, each with different CSP enforcement:

### Context 1: MAIN World (Page Context)

**What:** Runs in the same JavaScript context as the page
- Shares global scope with page
- Can access page's `window`, `document`, `variables`
- **Subject to page CSP**
- Has access to page APIs: `fetch()`, XHR, WebSocket, `localStorage`, etc.

**CSP Enforcement:** Page CSP applies
- Blocks `new Function()`, `eval()`, etc.
- Gmail Trusted Types CSP blocks all dynamic code evaluation

**Example:**
```typescript
// MAIN world execution (via chrome.scripting.executeScript with world: MAIN)
const script = `
  new Function('return document.title')()  // ❌ BLOCKED by page CSP
  document.title  // ✅ Works - native API access
`
```

### Context 2: ISOLATED World (Content Script)

**What:** Runs in a separate JavaScript context from the page
- Isolated global scope (no `window` pollution)
- **NOT subject to page CSP**
- Limited DOM access (can read, sometimes write)
- Cannot access page variables or functions

**CSP Enforcement:** Extension CSP applies (very permissive)
- Allows `new Function()` unless extension CSP blocks it
- Kaboom has `"script-src": "self"` (most permissive)
- Can execute arbitrary code safely

**Example:**
```typescript
// ISOLATED world execution
const script = `
  new Function('return 42')()  // ✅ Works - no page CSP
  window.pageVar  // ❌ undefined - isolated scope
`
```

### Context 3: Chrome Debugger API

**What:** Only way to truly bypass CSP
- Requires `"debugger"` permission in manifest
- Shows a small debugging banner in corner
- Can execute in MAIN world without CSP restrictions

**When to use:** Only for sites with Trusted Types CSP + no eval allowed

---

## How CSP Blocks `new Function()` Everywhere

This is the critical insight: **`new Function()` is blocked by the PAGE CSP regardless of where you call it.**

#### Why?

`new Function()` creates JavaScript code at runtime. CSP treats this the same as `eval()` — both are "unsafe-eval" operations that circumvent static code analysis.

#### Where It's Blocked

```typescript
// MAIN world: Page CSP blocks this ❌
chrome.scripting.executeScript({
  target: { tabId },
  world: 'MAIN',
  func: () => {
    const fn = new Function('return document.title')  // ❌ CSP violation
  }
})

// ISOLATED world: Extension CSP permits, but...
chrome.scripting.executeScript({
  target: { tabId },
  world: 'ISOLATED',
  func: () => {
    const fn = new Function('return document.title')  // ✅ Works (isolated scope)
  }
})

// Content script in isolated world: Same as above ✅
// (Content scripts run in ISOLATED world by default)
```

#### Gmail's Trusted Types CSP

Gmail uses an aggressive CSP that goes beyond blocking `unsafe-eval`:

```
Content-Security-Policy:
  default-src 'none';
  script-src 'strict-inline';
  require-trusted-types-for 'script';
  trusted-types strict
```

This blocks:
- ❌ `eval()` - Obviously unsafe
- ❌ `new Function()` - Creates code at runtime
- ❌ `Function.prototype.constructor()` - Indirect `new Function()`
- ❌ `setTimeout('code')` - String-based execution
- ❌ Even `<script>` tag insertion - Requires trusted type

---

## Kaboom's Multi-Strategy Approach

Kaboom implements a tiered strategy to handle different CSP levels:

### Strategy 1: MAIN World Execution (Default)

**When:** Page has permissive CSP (most sites)

```typescript
// In src/background/tools_interact.ts
const executeJS = (req) => {
  const params = {
    script: req.script,
    world: 'MAIN',  // DEFAULT
    timeout_ms: req.timeout_ms || 5000
  }
  // Sends to extension...
}

// In src/inject/message-handlers.ts
export function executeJavaScript(script: string, timeoutMs = 5000) {
  try {
    // ✅ Works on most sites
    const fn = new Function(`"use strict"; return (${script})`)
    const result = fn()
    return { success: true, result }
  } catch (err) {
    if (err.message.includes('Content Security Policy')) {
      // ❌ CSP blocked execution
      // Fall back to ISOLATED world
      return { success: false, error: 'csp_blocked' }
    }
  }
}
```

**Pros:**
- Has full access to page context (`window`, `document`, page variables)
- Can access page APIs (fetch, XHR, WebSocket)
- No extra marshaling needed

**Cons:**
- Blocked by CSP on Gmail, GitHub, Discord, etc.

### Strategy 2: ISOLATED World (Fallback)

**When:** Page has CSP that blocks `unsafe-eval`

```typescript
// In src/background/tools_interact.ts
const executeJS = (req) => {
  const params = {
    script: req.script,
    world: 'ISOLATED',  // FALLBACK
    timeout_ms: req.timeout_ms || 5000
  }
  // Sends to extension...
}
```

**How it works:**
- Extension calls `chrome.scripting.executeScript()` with `world: 'ISOLATED'`
- Code runs in isolated world, not subject to page CSP
- `new Function()` works because extension CSP allows it
- Limited DOM access via pre-compiled primitives

**Pros:**
- Bypasses page CSP completely
- No security warnings

**Cons:**
- No access to page variables/functions
- DOM access limited to pre-compiled queries (see `analyze({what: "dom"})`)
- Cannot use page APIs that require page context

**DOM Query Alternative:**
```typescript
// ISOLATED world execution (no page context)
const script = `
  window.myVar  // ❌ undefined (isolated scope)
  document.querySelectorAll('button').length  // ✅ DOM works
`
```

### Strategy 3: Query DOM Primitives (No Eval)

**When:** Need DOM queries without execute_js**

Pre-compiled DOM primitives don't use `new Function()`, so they work everywhere:

```typescript
// No CSP issue - uses native DOM APIs
const result = await analyze({
  what: 'dom',
  selector: 'button'
})
```

**Available without eval:**
- `analyze({what: "dom"})` - Query elements, get text/values, and count matches
- `accessibility` - Run axe-core a11y audit
- `network_waterfall` - Get performance timing data

**Why it works:**
```typescript
// This compiles to native code at build time, no eval
export function executeDOMQuery(script: string): Result {
  // Pre-compiled switch statement
  switch (script.type) {
    case 'count': return document.querySelectorAll(selector).length
    case 'text': return document.querySelector(selector)?.textContent
    case 'exists': return !!document.querySelector(selector)
    // ...no Function() anywhere
  }
}
```

### Strategy 4: Chrome Debugger API (Nuclear Option)

**When:** Site has Trusted Types CSP (Gmail) AND no other option works

```go
// In cmd/browser-agent/tools_interact.go
// Only used as last resort on high-CSP sites
func executeWithDebugger(tabID int, script string) error {
  // Attaches debugger to tab (shows yellow banner)
  // Executes script with zero CSP restrictions
  // Detaches debugger
  return nil
}
```

**Pros:**
- Only way to bypass Trusted Types CSP
- Completely unrestricted code execution

**Cons:**
- Shows yellow "DevTools is listening" banner (bad UX)
- Slower (attaching debugger has overhead)
- Only use as absolute last resort

**When to use:**
```javascript
// If MAIN fails (CSP) and ISOLATED fails (needs page context)
// AND site uses Trusted Types CSP...
// Then: Use debugger API (only option)

// Example: Script needs to call page function
const script = `
  window.pageFunction()  // ❌ ISOLATED world doesn't have this
  // Must use debugger API
`
```

---

## The `world: "auto"` Fallback Behavior

Kaboom's default is `world: "auto"`, which implements intelligent fallback:

```typescript
// User specifies world: "auto" (default)
const executeJS = (req) => {
  const params = {
    script: req.script,
    world: req.world || 'auto',  // AUTO fallback
    timeout_ms: req.timeout_ms || 5000
  }
  // Sends to extension...
}

// In inject script
export function executeJavaScript(script: string, timeout = 5000) {
  try {
    // Step 1: Try MAIN world execution
    const fn = new Function(`"use strict"; return (${script})`)
    return { success: true, result: fn() }
  } catch (err) {
    if (err.message.includes('Content Security Policy')) {
      // Step 2: CSP blocked, try ISOLATED world
      // Content script calls chrome.scripting.executeScript with ISOLATED
      return { success: false, error: 'csp_blocked', retry: 'isolated' }
    }
    // Other errors are fatal
    throw err
  }
}
```

**Auto-fallback flow:**

```
User calls execute_js with world: "auto"
  ↓
1st attempt: Try MAIN world (full context)
  ✓ Success → Return result immediately
  ✗ CSP blocked → Fallback to ISOLATED
    ↓
2nd attempt: Try ISOLATED world (DOM-only)
    ✓ Success → Return result
    ✗ No page context → Return error
      ↓
3rd attempt: Suggest using analyze({what: "dom"}) or debugger API
```

---

## Decision Tree: Which Execution Strategy to Use

```
┌─ Do you need to access page variables/functions?
│  │
│  ├─ YES → Try world: "MAIN"
│  │  ├─ ✅ Works → Use MAIN (full context)
│  │  ├─ ❌ CSP blocked → Try world: "ISOLATED"
│  │  │  ├─ ✅ Works → Use ISOLATED (DOM-only)
│  │  │  ├─ ❌ Still need page context → Use debugger API (last resort)
│  │
│  └─ NO → Use analyze({what: "dom"})
│     ✅ DOM queries without eval → Use analyze({what: "dom"})
│
└─ Is this Gmail/GitHub/Discord (high CSP)?
   ├─ YES → Try ISOLATED first, analyze({what: "dom"}) for DOM, debugger for page context
   └─ NO → Use MAIN, fall back to ISOLATED automatically
```

---

## Code Examples: What Works & What Fails

### Example 1: Simple Math (Works Everywhere)

```typescript
// Request
execute_js({
  script: "2 + 2",
  world: "auto"
})

// Result
{ success: true, result: 4 }

// ✅ Works on all sites (even Trusted Types CSP)
// Reason: No page context needed, pure computation
```

### Example 2: DOM Query (Works with analyze({what: "dom"}) workaround)

```typescript
// Request - DON'T use execute_js for this
execute_js({
  script: "document.querySelectorAll('button').length",
  world: "MAIN"
})

// Result
{ success: true, result: 42 }  // On permissive CSP sites

// Result (CSP site)
{ success: false, error: 'csp_blocked', message: '...' }

// ✅ Better: Use analyze({what: "dom"}) instead
analyze({
  what: 'dom',
  selector: 'button'
})
// Result: { count: 42 }  // Works everywhere, even Gmail
```

### Example 3: Page API Access (Context Dependent)

```typescript
// Request
execute_js({
  script: "window.myGlobalVar",
  world: "MAIN"
})

// Result (Permissive CSP)
{ success: true, result: 'some value' }

// Result (Gmail/High CSP)
{ success: false, error: 'csp_blocked' }

// Fallback (ISOLATED world)
{ success: false, result: undefined }  // Isolated scope doesn't have it

// ✅ If needed on Gmail: Use Chrome debugger API
// (Shows yellow banner, but works)
```

### Example 4: Async Operation (Promise Handling)

```typescript
// Request
execute_js({
  script: `
    fetch('/api/data')
      .then(r => r.json())
      .then(d => d.length)
  `,
  world: "MAIN",
  timeout_ms: 10000
})

// Result (Permissive CSP)
{ success: true, result: 42 }

// Result (CSP blocks, retry ISOLATED)
{ success: false, error: 'csp_blocked' }

// ✅ Note: fetch() IS a page API (not blocked by CSP)
// ✅ new Function() IS blocked (the evaluation itself)
// This fails at the "new Function()" step, before fetch runs
```

### Example 5: Gmail Trusted Types Workaround

```typescript
// ❌ Doesn't work on Gmail (Trusted Types CSP)
execute_js({
  script: "document.innerHTML = '<script>alert(1)</script>'",
  world: "MAIN"
})
// Error: Content Security Policy: The page's settings blocked...

// ✅ Works on Gmail (uses debugger API, no CSP)
execute_js({
  script: "document.innerHTML = '<div>safe</div>'",
  world: "MAIN",
  force_debugger: true  // Last resort
})
// Result: { success: true, result: undefined }
// (Shows yellow DevTools banner)
```

---

## CSP Bypass Techniques & Their Limitations

### Technique 1: String-based Code ❌

```typescript
// Won't work - CSP blocks string-based code
const code = `
  console.log('hello')
`
setTimeout(code)  // ❌ CSP violation
eval(code)        // ❌ CSP violation
new Function(code)()  // ❌ CSP violation
```

### Technique 2: Attribute-based Execution ❌

```typescript
// Won't work - CSP blocks inline event handlers
const el = document.createElement('div')
el.onclick = function() { alert(1) }  // ❌ Event handler
el.setAttribute('onload', 'alert(1)')  // ❌ Inline handler
```

### Technique 3: Service Worker ❌

```typescript
// Can't register service workers on Gmail/Discord
// CSP blocks Service Worker registration in high-security contexts
navigator.serviceWorker.register(...)  // ❌ CSP blocks on Gmail
```

### Technique 4: Isolated World + Pre-compiled Primitives ✅

```typescript
// Works because no Function() involved
export function queryDOM(selector, action) {
  // Pre-compiled at build time, not eval
  switch (action) {
    case 'count':
      return document.querySelectorAll(selector).length
    case 'exists':
      return !!document.querySelector(selector)
    case 'text':
      return document.querySelector(selector)?.textContent
  }
}

// ✅ Works on Gmail - no eval, pre-compiled DOM access
```

### Technique 5: Chrome Debugger API ✅

```go
// Only guaranteed way to bypass Trusted Types CSP
// (Shows debugging banner - bad UX but necessary)
func attachDebugger(tabID int) error {
  // After this, no CSP restrictions on code execution
  return chrome.debugger.attach(target)
}

// ✅ Works on Gmail (but shows yellow banner)
// ❌ Slow, intrusive, last resort only
```

---

## CSP Headers Kaboom Encounters

### Permissive CSP (Most Sites)

```
Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-eval'
```
**Result:** ✅ MAIN world works, ✅ `new Function()` allowed

### Moderate CSP (GitHub, Reddit, etc.)

```
Content-Security-Policy: default-src 'self'; script-src 'self'
```
**Result:** ❌ MAIN world fails, ✅ ISOLATED world works, ✅ Pre-compiled DOM works

### Strict CSP (Discord, Slack)

```
Content-Security-Policy: script-src 'self' 'nonce-xyz123'
```
**Result:** ❌ MAIN world fails, ✅ ISOLATED world works, ✅ Pre-compiled DOM works

### Trusted Types CSP (Gmail, Google Workspace)

```
Content-Security-Policy:
  default-src 'none';
  script-src 'strict-inline';
  require-trusted-types-for 'script';
  trusted-types strict
```
**Result:** ❌ MAIN world fails, ❌ ISOLATED world limited, ✅ Pre-compiled DOM works, ⚠️ Debugger API required for custom code

---

## Implementation: World Parameter Handling

### Server Side (cmd/browser-agent/tools_interact.ts)

```go
func (h *MCPHandler) ExecuteJS(req ExecuteJSRequest) {
  world := req.World
  if world == "" || world == "auto" {
    world = "MAIN"  // Default to MAIN, extension handles fallback
  }

  query := &queries.PendingQuery{
    Type: "execute_js",
    Params: json.RawMessage{
      "script": req.Script,
      "world": world,
      "timeout_ms": req.TimeoutMs,
    },
  }

  h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout)
}
```

### Extension Side (src/background/pending-queries.ts)

```typescript
async function executeJs(params: ExecuteJsParams) {
  const { script, world = 'MAIN', timeout_ms = 5000 } = params

  if (world === 'MAIN') {
    // Try MAIN world first
    try {
      return await chrome.scripting.executeScript({
        target: { tabId },
        world: 'MAIN',
        func: async () => await executeJavaScript(script, timeout_ms),
      })
    } catch (err) {
      if (err.message.includes('CSP')) {
        // Fall back to ISOLATED
        return await chrome.scripting.executeScript({
          target: { tabId },
          world: 'ISOLATED',
          func: async () => await executeJavaScript(script, timeout_ms),
        })
      }
      throw err
    }
  } else if (world === 'ISOLATED') {
    // Direct ISOLATED execution
    return await chrome.scripting.executeScript({
      target: { tabId },
      world: 'ISOLATED',
      func: async () => await executeJavaScript(script, timeout_ms),
    })
  }
}
```

### Inject Script (src/inject/message-handlers.ts)

```typescript
export function executeJavaScript(script: string, timeoutMs = 5000) {
  return new Promise<ExecuteJsResult>((resolve) => {
    const timeoutHandle = setTimeout(() => {
      resolve({ success: false, error: 'execution_timeout' })
    }, timeoutMs)

    try {
      // This runs in whatever world was specified
      const fn = new Function(`"use strict"; return (${script})`)
      const result = fn()

      // Handle promise results
      if (result && typeof result.then === 'function') {
        result
          .then((value) => {
            clearTimeout(timeoutHandle)
            resolve({ success: true, result: safeSerializeForExecute(value) })
          })
          .catch((err) => {
            clearTimeout(timeoutHandle)
            resolve({ success: false, error: 'promise_rejected' })
          })
      } else {
        clearTimeout(timeoutHandle)
        resolve({ success: true, result: safeSerializeForExecute(result) })
      }
    } catch (err) {
      clearTimeout(timeoutHandle)

      // Detect CSP errors
      if (err.message.includes('Content Security Policy')) {
        resolve({
          success: false,
          error: 'csp_blocked',
          message: 'Use world: "isolated" for CSP-protected pages'
        })
      } else {
        resolve({
          success: false,
          error: 'execution_error',
          message: err.message
        })
      }
    }
  })
}
```

---

## Testing CSP Scenarios

### Test 1: Permissive Site (Works with MAIN)

```bash
# Open devtools console on most sites
> new Function('return 2+2')()
// 4 (works)

# In Kaboom MCP
execute_js({ script: "2+2", world: "MAIN" })
// { success: true, result: 4 }
```

### Test 2: CSP Site (Falls back to ISOLATED)

```bash
# Open devtools console on GitHub
> new Function('return 2+2')()
// VM:1 Uncaught SyntaxError: Unexpected token ')'
# (This is the CSP block message)

# In Kaboom MCP
execute_js({ script: "2+2", world: "auto" })
// MAIN fails → falls back to ISOLATED
// { success: true, result: 4 }
```

### Test 3: DOM Query (Works Everywhere)

```bash
# Works on all sites (Gmail, GitHub, Discord)
analyze({ what: 'dom', selector: 'button' })
// { count: 42 }  ✅ Always works
```

### Test 4: Gmail Trusted Types (Only Debugger Works)

```bash
# MAIN fails
execute_js({ script: "document.title", world: "MAIN" })
// { success: false, error: 'csp_blocked' }

# ISOLATED works if it's just DOM
execute_js({ script: "document.title", world: "ISOLATED" })
// { success: true, result: "Gmail" }

# But if you need page context (window.userEmail), only debugger works
execute_js({ script: "window.userEmail", world: "auto", force_debugger: true })
// { success: true, result: "user@gmail.com" }
// (Shows yellow DevTools banner)
```

---

## Related Documents

- [Error Recovery Strategy](./error-recovery.md) - Handles CSP errors in retry logic
- [Async Command Architecture](../../../.claude/refs/async-command-architecture.md)
- [Extension Architecture](../../../.claude/refs/extension-architecture.md) - World parameter details
- [MDN: Content Security Policy](https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP)
- [Google: Trusted Types](https://web.dev/trusted-types/)

---

## Troubleshooting CSP Issues

| Symptom | Cause | Solution |
|---------|-------|----------|
| Script always fails | Page has strict CSP | Try `world: "isolated"` or `analyze({what: "dom"})` |
| "CSP_BLOCKED" error | Page blocks unsafe-eval | Use pre-compiled DOM primitives |
| Code works in console but fails in extension | Running in different world | Use `world: "MAIN"` for page context |
| Gmail scripts don't work | Trusted Types CSP | Use `analyze({what: "dom"})` for DOM, accept limitation for page access |
| Yellow "DevTools" banner appears | Using debugger API | Expected on high-CSP sites; only way to bypass Trusted Types |

