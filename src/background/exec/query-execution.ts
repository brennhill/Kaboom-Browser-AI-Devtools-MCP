/**
 * Purpose: Executes JavaScript in page context with world-aware routing (content script relay, chrome.scripting, or CSP-safe structured executor).
 * Docs: docs/features/feature/csp-safe-execution/index.md
 */

// query-execution.ts — JavaScript execution with world-aware routing and CSP fallback.
// Handles execute_js queries via content script relay, chrome.scripting API, or structured executor.

import { debugLog } from '../index.js'
import { DebugCategory } from '../debug.js'
import { scaleTimeout } from '../../lib/timeouts.js'
import { parseExpression } from './csp-safe/parser.js'
import { cspSafeExecutor } from './csp-safe/executor.js'
import { errorMessage } from '../../lib/error-utils.js'

// =============================================================================
// CSP PROBE
// =============================================================================

/** Result of probing a tab's Content Security Policy restrictions */
export interface CSPProbeResult {
  csp_restricted: boolean
  csp_level: 'none' | 'script_exec' | 'page_blocked'
}

/**
 * Probe whether a tab's CSP blocks dynamic script execution (new Function).
 * Returns one of three levels:
 * - "none": No CSP restrictions — execute_js is safe
 * - "script_exec": new Function() blocked — use dom/get_readable instead
 * - "page_blocked": Privileged URL (chrome://, devtools://) — no extension access
 */
export async function probeCSPStatus(tabId: number): Promise<CSPProbeResult> {
  try {
    const results = await chrome.scripting.executeScript({
      target: { tabId },
      world: 'MAIN',
      func: () => {
        try {
          new Function('return 1')()
          return 'ok'
        } catch {
          return 'csp_blocked'
        }
      }
    })
    const val = results?.[0]?.result
    if (val === 'ok') return { csp_restricted: false, csp_level: 'none' }
    if (val === 'csp_blocked') return { csp_restricted: true, csp_level: 'script_exec' }
    return { csp_restricted: true, csp_level: 'page_blocked' }
  } catch {
    return { csp_restricted: true, csp_level: 'page_blocked' }
  }
}

// =============================================================================
// ISOLATED WORLD EXECUTION (chrome.scripting API)
// =============================================================================

/** Result shape from JS execution */
export interface ExecutionResult {
  success: boolean
  error?: string
  message?: string
  result?: unknown
  stack?: string
  execution_mode?: string
}

/**
 * Execute JavaScript via chrome.scripting.executeScript.
 * Used as fallback when MAIN world execution fails due to page CSP,
 * or when inject script is not loaded.
 * The func is injected natively by Chrome's extension system.
 */
export async function executeViaScriptingAPI(
  tabId: number,
  script: string,
  timeoutMs: number,
  world: 'MAIN' | 'ISOLATED' = 'MAIN'
): Promise<ExecutionResult> {
  const timeoutPromise = new Promise<never>((_, reject) => {
    setTimeout(() => reject(new Error(`Script exceeded ${timeoutMs}ms timeout`)), timeoutMs + 2000)
  })

  const executionPromise = chrome.scripting.executeScript({
    target: { tabId },
    world: world,
    func: (code: string) => {
      try {
        const cleaned = code.trim()

        // Try expression form first (captures return values from IIFEs, expressions).
        // If SyntaxError (statements like try/catch, if/else), fall back to statement form.
        let fn: () => unknown
        try {
          // eslint-disable-next-line no-new-func
          fn = new Function(`"use strict"; return (${cleaned});`) as () => unknown // nosemgrep: javascript.lang.security.eval.rule-eval-with-expression -- chrome.scripting.executeScript API, not eval()
        } catch {
          // eslint-disable-next-line no-new-func
          fn = new Function(`"use strict"; ${cleaned}`) as () => unknown // nosemgrep: javascript.lang.security.eval.rule-eval-with-expression -- chrome.scripting.executeScript API, not eval()
        }
        const result = fn()

        if (result !== null && result !== undefined && typeof (result as { then?: unknown }).then === 'function') {
          return (result as Promise<unknown>)
            .then((v: unknown) => {
              return { success: true as const, result: serialize(v) }
            })
            .catch((err: unknown) => {
              const e = err as Error
              return { success: false as const, error: 'promise_rejected', message: e.message }
            })
        }

        return { success: true as const, result: serialize(result) }
      } catch (err) {
        const e = err as Error
        const msg = e.message || ''
        if (msg.includes('Content Security Policy') || msg.includes('Trusted Type') || msg.includes('unsafe-eval')) {
          return {
            success: false as const,
            error: 'csp_blocked_all_worlds',
            message:
              'Page CSP blocks dynamic script execution. ' +
              'Use query_dom for DOM operations or navigate away from this CSP-restricted page.'
          }
        }
        return { success: false as const, error: 'execution_error', message: msg, stack: e.stack }
      }

      function serialize(value: unknown, depth = 0, seen = new WeakSet<object>()): unknown {
        if (depth > 10) return '[max depth]'
        if (value === null || value === undefined) return value
        const t = typeof value
        if (t === 'string' || t === 'number' || t === 'boolean') return value
        if (t === 'function') return '[Function]'
        if (t === 'symbol') return String(value)
        if (t === 'object') {
          const obj = value as object
          if (seen.has(obj)) return '[Circular]'
          seen.add(obj)
          if (Array.isArray(obj)) return obj.slice(0, 100).map((v) => serialize(v, depth + 1, seen))
          if (obj instanceof Error) return { error: (obj as Error).message }
          if (obj instanceof Date) return (obj as Date).toISOString()
          if (obj instanceof RegExp) return String(obj)
          // DOM node duck-type check (works across worlds)
          if ('nodeType' in obj && 'nodeName' in obj) {
            const node = obj as { nodeName: string; id?: string }
            return `[${node.nodeName}${node.id ? '#' + node.id : ''}]`
          }
          // Browser host objects (DOMRect, DOMPoint, DOMMatrix) have prototype getters
          // that Object.keys() misses. Their toJSON() returns a plain object.
          if (typeof (obj as { toJSON?: unknown }).toJSON === 'function') {
            try {
              return serialize((obj as { toJSON: () => unknown }).toJSON(), depth + 1, seen)
            } catch {
              // Fall through to Object.keys() enumeration
            }
          }
          const keys = Object.keys(obj).slice(0, 50)
          // #389: Some host objects expose data only via prototype getters
          // (DOMRect/CSSStyleDeclaration-like values). If no enumerable keys exist,
          // introspect prototype property names and capture primitive getter values.
          if (keys.length === 0) {
            try {
              const proto = Object.getPrototypeOf(obj)
              if (proto && proto !== Object.prototype) {
                const hostResult: Record<string, unknown> = {}
                const propNames = Object.getOwnPropertyNames(proto).slice(0, 120)
                for (const key of propNames) {
                  if (key === 'constructor') continue
                  try {
                    const value = (obj as Record<string, unknown>)[key]
                    const valueType = typeof value
                    if (value === undefined || valueType === 'function') continue
                    if (valueType === 'string' || valueType === 'number' || valueType === 'boolean' || value === null) {
                      hostResult[key] = value
                    }
                  } catch {
                    // Ignore getter access errors.
                  }
                  if (Object.keys(hostResult).length >= 50) break
                }
                if (Object.keys(hostResult).length > 0) return hostResult
              }
            } catch {
              // Fall through to default object key enumeration.
            }
          }
          const result: Record<string, unknown> = {}
          for (const key of keys) {
            try {
              result[key] = serialize((obj as Record<string, unknown>)[key], depth + 1, seen)
            } catch {
              result[key] = '[unserializable]'
            }
          }
          return result
        }
        return String(value)
      }
    },
    args: [script]
  })

  try {
    const results = await Promise.race([executionPromise, timeoutPromise])
    const firstResult = results?.[0]?.result
    if (firstResult && typeof firstResult === 'object') {
      return firstResult as ExecutionResult
    }
    return { success: false, error: 'no_result', message: 'chrome.scripting.executeScript produced no result' }
  } catch (err) {
    const msg = errorMessage(err) || ''
    if (msg.includes('timeout')) {
      return { success: false, error: 'execution_timeout', message: msg }
    }
    return { success: false, error: 'scripting_api_error', message: msg }
  }
}

// =============================================================================
// CSP-SAFE STRUCTURED EXECUTION (tier 3)
// =============================================================================

/**
 * Execute JavaScript by parsing it into a structured command and running
 * a pre-compiled executor function via chrome.scripting.executeScript.
 * This bypasses CSP because no string-to-code conversion happens.
 */
async function executeViaStructuredCommand(
  tabId: number,
  script: string,
  timeoutMs: number,
  world: 'MAIN' | 'ISOLATED' = 'MAIN'
): Promise<ExecutionResult> {
  const modeTag = world === 'ISOLATED' ? 'isolated_structured' : 'csp_safe_structured'

  const parseResult = parseExpression(script)
  if (!parseResult.ok) {
    return {
      success: false,
      error: 'csp_blocked_unparseable',
      message:
        `This expression cannot be converted to a structured command: ${parseResult.reason}. ` +
        'Only simple property access, method calls, and assignments are supported. ' +
        'Use interact DOM primitives (click, type, get_text, get_attribute, list_interactive) instead.',
      execution_mode: modeTag
    }
  }

  const timeoutPromise = new Promise<never>((_, reject) => {
    setTimeout(() => reject(new Error(`Structured execution exceeded ${timeoutMs}ms timeout`)), timeoutMs + 2000)
  })

  const executionPromise = chrome.scripting.executeScript({
    target: { tabId },
    world: world,
    func: cspSafeExecutor,
    args: [parseResult.command]
  })

  try {
    const results = await Promise.race([executionPromise, timeoutPromise])
    const firstResult = results?.[0]?.result
    if (firstResult && typeof firstResult === 'object') {
      const execResult = firstResult as ExecutionResult
      return { ...execResult, execution_mode: modeTag }
    }
    return {
      success: false,
      error: 'no_result',
      message: 'Structured executor produced no result',
      execution_mode: modeTag
    }
  } catch (err) {
    const msg = errorMessage(err) || ''
    if (msg.includes('timeout')) {
      return { success: false, error: 'execution_timeout', message: msg, execution_mode: modeTag }
    }
    return { success: false, error: 'structured_execution_error', message: msg, execution_mode: modeTag }
  }
}

/**
 * Execute JS with world-aware routing.
 * - isolated: structured executor in ISOLATED world (skips new Function — always fails in MV3)
 * - main: send to content script (MAIN world via inject)
 * - auto: try content script → scripting API MAIN → structured executor MAIN
 */
// #lizard forgives
export async function executeWithWorldRouting(
  tabId: number,
  queryParams: string | Record<string, unknown>,
  world: string
): Promise<ExecutionResult> {
  let parsedParams: { script?: string; timeout_ms?: number }
  try {
    parsedParams = typeof queryParams === 'string' ? JSON.parse(queryParams) : queryParams
  } catch {
    parsedParams = {}
  }
  const script = parsedParams.script || ''
  const timeoutMs = parsedParams.timeout_ms || scaleTimeout(5000)

  // ISOLATED: go directly to structured executor — new Function() always fails in MV3's ISOLATED world
  if (world === 'isolated') {
    return executeViaStructuredCommand(tabId, script, timeoutMs, 'ISOLATED')
  }

  // MAIN or AUTO: try content script (MAIN world) first
  try {
    const result = (await chrome.tabs.sendMessage(tabId, {
      type: 'kaboom_execute_query',
      params: queryParams
    })) as ExecutionResult

    // Auto-fallback: split by error type
    if (world === 'auto' && result && !result.success) {
      // CSP errors → structured executor in MAIN world
      if (result.error === 'csp_blocked') {
        debugLog(DebugCategory.CONNECTION, 'CSP fallback: structured executor in MAIN world', { tabId })
        return executeViaStructuredCommand(tabId, script, timeoutMs, 'MAIN')
      }

      // Inject not loaded/responding → try MAIN world via scripting API
      if (result.error === 'inject_not_loaded' || result.error === 'inject_not_responding') {
        debugLog(DebugCategory.CONNECTION, 'Auto-fallback to chrome.scripting API (MAIN)', {
          error: result.error,
          tabId
        })
        return executeViaScriptingAPI(tabId, script, timeoutMs, 'MAIN')
      }
    }

    return result
  } catch (err) {
    let message = errorMessage(err, 'Tab communication failed')

    // Auto-fallback: content script not reachable — try scripting API MAIN, then structured
    if (world === 'auto' && message.includes('Receiving end does not exist')) {
      debugLog(DebugCategory.CONNECTION, 'Auto-fallback (content script unreachable)', { tabId })
      const mainResult = await executeViaScriptingAPI(tabId, script, timeoutMs, 'MAIN')
      if (mainResult.success) return mainResult
      if (mainResult.error === 'csp_blocked_all_worlds') {
        return executeViaStructuredCommand(tabId, script, timeoutMs, 'MAIN')
      }
      return mainResult
    }

    if (message.includes('Receiving end does not exist')) {
      message =
        'Content script not loaded. REQUIRED ACTION: Refresh the page first using this command:\n\ninteract({what: "refresh"})\n\nThen retry your command.'
    }
    return { success: false, error: 'content_script_not_loaded', message }
  }
}
