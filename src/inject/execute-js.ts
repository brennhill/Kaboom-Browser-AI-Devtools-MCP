/**
 * Purpose: JavaScript execution sandbox for evaluating arbitrary scripts in page context with safe serialization and timeout support.
 * Docs: docs/features/feature/interact-explore/index.md
 */

// execute-js.ts — JavaScript execution sandbox for in-page script evaluation.

import type { ExecuteJsResult } from '../types/runtime-messages.js'
import { createDeferredPromise } from '../lib/timeout-utils.js'

/**
 * Safe serialization for complex objects returned from executeJavaScript.
 */
function serializeSpecialValue(obj: object, depth: number, seen: WeakSet<object>): unknown {
  if (Array.isArray(obj)) return obj.slice(0, 100).map((v) => safeSerializeForExecute(v, depth + 1, seen))
  if (obj instanceof Error) return { error: obj.message, stack: obj.stack }
  if (obj instanceof Date) return obj.toISOString()
  if (obj instanceof RegExp) return obj.toString()
  if (typeof Node !== 'undefined' && obj instanceof Node) {
    const node = obj as Node & { id?: string }
    return `[${node.nodeName}${node.id ? '#' + node.id : ''}]`
  }
  return undefined
}

function tryToJSONValue(obj: object, depth: number, seen: WeakSet<object>): { value: unknown } | null {
  // Browser host objects (DOMRect, DOMPoint, DOMMatrix) have prototype getters
  // that Object.keys() misses. Their toJSON() returns a plain object.
  if (typeof (obj as { toJSON?: unknown }).toJSON !== 'function') return null
  try {
    return { value: safeSerializeForExecute((obj as { toJSON: () => unknown }).toJSON(), depth + 1, seen) }
  } catch {
    // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
    // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
    // Fall through to Object.keys() enumeration
    return null
  }
}

function appendHostProperty(hostResult: Record<string, unknown>, obj: Record<string, unknown>, key: string): void {
  try {
    const hostValue = obj[key]
    const hostType = typeof hostValue
    if (hostValue === undefined || hostType === 'function') return
    if (hostType === 'string' || hostType === 'number' || hostType === 'boolean' || hostValue === null) {
      hostResult[key] = hostValue
    }
  } catch {
    // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
    // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
    // Ignore getter access errors and continue.
  }
}

function serializeHostObject(obj: object, keys: string[]): Record<string, unknown> | null {
  // Host objects like DOMRect/CSSStyleDeclaration expose values via prototype getters,
  // so Object.keys() can be empty even when useful primitive fields exist.
  if (keys.length > 0) return null
  try {
    const proto = Object.getPrototypeOf(obj)
    if (proto && proto !== Object.prototype) {
      const hostResult: Record<string, unknown> = {}
      const propNames = Object.getOwnPropertyNames(proto).slice(0, 120)
      for (const key of propNames) {
        if (key === 'constructor') continue
        appendHostProperty(hostResult, obj as Record<string, unknown>, key)
        if (Object.keys(hostResult).length >= 50) break
      }
      if (Object.keys(hostResult).length > 0) return hostResult
    }
  } catch {
    // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
    // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
    // Fall through to default enumeration.
  }
  return null
}

function serializeEnumerableKeys(
  obj: object,
  keys: string[],
  depth: number,
  seen: WeakSet<object>
): Record<string, unknown> {
  const result: Record<string, unknown> = {}
  for (const key of keys) {
    try {
      result[key] = safeSerializeForExecute((obj as Record<string, unknown>)[key], depth + 1, seen)
    } catch {
      result[key] = '[unserializable]'
    }
  }
  if (Object.keys(obj).length > 50) {
    result['...'] = `[${Object.keys(obj).length - 50} more keys]`
  }
  return result
}

function serializeObject(obj: object, depth: number, seen: WeakSet<object>): unknown {
  if (seen.has(obj)) return '[Circular]'
  seen.add(obj)

  const special = serializeSpecialValue(obj, depth, seen)
  if (special !== undefined) return special

  const toJSONValue = tryToJSONValue(obj, depth, seen)
  if (toJSONValue) return toJSONValue.value

  const keys = Object.keys(obj).slice(0, 50)
  const hostResult = serializeHostObject(obj, keys)
  if (hostResult) return hostResult
  return serializeEnumerableKeys(obj, keys, depth, seen)
}

export function safeSerializeForExecute(
  value: unknown,
  depth: number = 0,
  seen: WeakSet<object> = new WeakSet()
): unknown {
  if (depth > 10) return '[max depth exceeded]'
  if (value === null || value === undefined) return value

  const type = typeof value
  if (type === 'string' || type === 'number' || type === 'boolean') return value
  if (type === 'function') return `[Function: ${(value as (...args: unknown[]) => unknown).name || 'anonymous'}]`
  if (type === 'symbol') return (value as symbol).toString()
  if (type === 'object') return serializeObject(value as object, depth, seen)

  return String(value)
}

// AsyncFunction constructor — same family as Function, but its body permits
// top-level `await`. Derived once at module load from an async function's prototype.
const AsyncFunction = Object.getPrototypeOf(async function () {}).constructor as FunctionConstructor

/**
 * Compile a user script into a callable, trying progressively more permissive forms.
 *
 * Order matters: the two synchronous forms are attempted first so that every
 * script that compiled before this function existed keeps its exact prior
 * behavior (a synchronous throw stays a synchronous throw, not a rejected
 * promise). Only scripts the sync forms reject — chiefly those using top-level
 * `await`, which previously failed with a SyntaxError (issue #598) — fall
 * through to the AsyncFunction forms, whose result is always a promise.
 *
 * 1. sync expression  — `return (script)`  captures IIFE/expression values
 * 2. sync statement   — `script`           allows statements + explicit return
 * 3. async expression — `return (script)`  captures a bare `await expr` value
 * 4. async statement  — `script`           allows statements + top-level await
 */
function compileUserScript(cleanScript: string): () => unknown {
  const forms: Array<() => () => unknown> = [
    // eslint-disable-next-line no-new-func
    () => new Function(`"use strict"; return (${cleanScript});`) as () => unknown, // nosemgrep: javascript.lang.security.eval.rule-eval-with-expression -- Function() constructor for controlled sandbox execution
    // eslint-disable-next-line no-new-func
    () => new Function(`"use strict"; ${cleanScript}`) as () => unknown, // nosemgrep: javascript.lang.security.eval.rule-eval-with-expression -- Function() constructor for controlled sandbox execution
    () => new AsyncFunction(`"use strict"; return (${cleanScript});`) as () => unknown,
    () => new AsyncFunction(`"use strict"; ${cleanScript}`) as () => unknown
  ]

  let lastErr: unknown
  for (const compile of forms) {
    try {
      return compile()
    } catch (err) {
      lastErr = err
    }
  }
  // All forms failed (genuine syntax error / CSP block) — surface the final
  // error so the caller can classify it (csp_blocked vs execution_error).
  throw lastErr
}

/**
 * Execute arbitrary JavaScript in the page context with timeout handling.
 */
export function executeJavaScript(script: string, timeoutMs: number = 5000): Promise<ExecuteJsResult> {
  const deferred = createDeferredPromise<ExecuteJsResult>()

  // #lizard forgives
  const executeWithTimeoutProtection = async (): Promise<void> => {
    const timeoutHandle = setTimeout(() => {
      deferred.resolve({
        success: false,
        error: 'execution_timeout',
        message: `Script exceeded ${timeoutMs}ms timeout. RECOMMENDED ACTIONS:

1. Check for infinite loops or blocking operations in your script
2. Break the task into smaller pieces (< 2s execution time works best)
3. Verify the script logic - test with simpler operations first

Tip: Run small test scripts to isolate the issue, then build up complexity.`
      })
    }, timeoutMs)

    try {
      const cleanScript = script.trim()

      // Compile the script, preferring synchronous forms and falling back to
      // AsyncFunction forms (which permit top-level `await`). See compileUserScript.
      const fn = compileUserScript(cleanScript)

      const result = fn()

      // Handle promises
      if (result && typeof (result as Promise<unknown>).then === 'function') {
        ;(result as Promise<unknown>)
          .then((value) => {
            clearTimeout(timeoutHandle)
            deferred.resolve({ success: true, result: safeSerializeForExecute(value) })
          })
          .catch((err: Error) => {
            clearTimeout(timeoutHandle)
            deferred.resolve({
              success: false,
              error: 'promise_rejected',
              message: err.message,
              stack: err.stack
            })
          })
      } else {
        clearTimeout(timeoutHandle)
        deferred.resolve({ success: true, result: safeSerializeForExecute(result) })
      }
    } catch (err) {
      clearTimeout(timeoutHandle)

      const error = err as Error
      if (
        error.message &&
        (error.message.includes('Content Security Policy') ||
          error.message.includes('unsafe-eval') ||
          error.message.includes('Trusted Type'))
      ) {
        deferred.resolve({
          success: false,
          error: 'csp_blocked',
          message:
            'This page has a Content Security Policy that blocks script execution in the MAIN world. ' +
            'Use world: "isolated" to bypass CSP (DOM access only, no page JS globals). ' +
            'With world: "auto" (default), this fallback happens automatically.'
        })
      } else {
        deferred.resolve({
          success: false,
          error: 'execution_error',
          message: error.message,
          stack: error.stack
        })
      }
    }
  }

  executeWithTimeoutProtection().catch((err) => {
    console.error('[KaBOOM!] Unexpected error in executeJavaScript:', err)
    deferred.resolve({
      success: false,
      error: 'execution_error',
      message: 'Unexpected error during script execution'
    })
  })

  return deferred.promise
}
