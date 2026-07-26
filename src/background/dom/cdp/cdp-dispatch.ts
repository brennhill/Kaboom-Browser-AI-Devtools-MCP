/**
 * Purpose: Dispatches hardware-level input via Chrome DevTools Protocol.
 * Why: Synthetic DOM events have isTrusted:false which anti-bot systems and complex SPAs ignore.
 *      CDP Input.dispatch* commands produce true hardware events indistinguishable from real user input.
 * Docs: docs/features/feature/interact-explore/index.md
 */

// cdp-dispatch.ts — CDP executor for hardware-level clicks and keypresses.
// Manages chrome.debugger attach/detach lifecycle and dispatches CDP Input.* commands.

import type { PendingQuery } from '../../../types/queries.js'
import type { SyncClient } from '../../sync/sync-client.js'
import type { DOMActionParams, DOMResult } from '../dom-types.js'
import type { SendAsyncResultFn, ActionToastFn } from '../../commands/helpers.js'
import { CDP_VERSION } from '../../../lib/constants.js'
import { errorMessage } from '../../../lib/error-utils.js'
import { KEY_CODES, charToKeyInfo } from './cdp-key-mappings.js'
import { resolveElement, buildCDPResult } from './cdp-element-resolve.js'

interface CDPActionParams {
  action: string
  x?: number
  y?: number
  selector?: string
  text?: string
  key?: string
  modifiers?: number
}

async function cdpSend(tabId: number, method: string, params: Record<string, unknown>): Promise<void> {
  await chrome.debugger.sendCommand({ tabId }, method, params)
}

async function resolveCoordinates(tabId: number, params: CDPActionParams): Promise<{ x: number; y: number }> {
  if (typeof params.x === 'number' && typeof params.y === 'number') {
    return { x: params.x, y: params.y }
  }

  if (!params.selector) {
    throw new Error('click requires x/y coordinates or a selector')
  }

  // Use Runtime.evaluate to get element center coordinates
  const expression = `(() => {
    const el = document.querySelector(${JSON.stringify(params.selector)});
    if (!el) return null;
    const r = el.getBoundingClientRect();
    return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
  })()`

  const evalResult = (await chrome.debugger.sendCommand({ tabId }, 'Runtime.evaluate', {
    expression,
    returnByValue: true
  })) as { result?: { value?: { x: number; y: number } | null } }

  const coords = evalResult?.result?.value
  if (!coords) {
    throw new Error(`Element not found: ${params.selector}`)
  }

  return coords
}

async function cdpClick(tabId: number, params: CDPActionParams): Promise<Record<string, unknown>> {
  const { x, y } = await resolveCoordinates(tabId, params)

  await cdpSend(tabId, 'Input.dispatchMouseEvent', {
    type: 'mousePressed',
    x,
    y,
    button: 'left',
    clickCount: 1
  })
  await cdpSend(tabId, 'Input.dispatchMouseEvent', {
    type: 'mouseReleased',
    x,
    y,
    button: 'left',
    clickCount: 1
  })

  return {
    success: true,
    action: 'hardware_click',
    x,
    y,
    method: 'cdp'
  }
}

interface CDPKeyEventPayload {
  key: string
  code: string
  keyCode: number
  text?: string
  unmodifiedText?: string
  modifiers?: number
}

async function dispatchCDPKeyPair(tabId: number, payload: CDPKeyEventPayload): Promise<void> {
  const common = {
    key: payload.key,
    code: payload.code,
    windowsVirtualKeyCode: payload.keyCode,
    nativeVirtualKeyCode: payload.keyCode
  }
  await cdpSend(tabId, 'Input.dispatchKeyEvent', {
    type: 'keyDown',
    ...common,
    ...(payload.text !== undefined ? { text: payload.text } : {}),
    ...(payload.unmodifiedText !== undefined ? { unmodifiedText: payload.unmodifiedText } : {}),
    ...(payload.modifiers !== undefined ? { modifiers: payload.modifiers } : {})
  })
  await cdpSend(tabId, 'Input.dispatchKeyEvent', {
    type: 'keyUp',
    ...common,
    ...(payload.modifiers !== undefined ? { modifiers: payload.modifiers } : {})
  })
}

async function cdpType(tabId: number, params: CDPActionParams): Promise<Record<string, unknown>> {
  const text = params.text || ''
  if (!text) {
    throw new Error('type requires text parameter')
  }

  for (const char of text) {
    const info = charToKeyInfo(char)
    await dispatchCDPKeyPair(tabId, {
      key: info.key,
      code: info.code,
      keyCode: info.keyCode,
      text: char,
      unmodifiedText: info.shiftKey ? char.toLowerCase() : char,
      modifiers: info.shiftKey ? 8 : 0
    })
  }

  return {
    success: true,
    action: 'hardware_type',
    char_count: text.length,
    method: 'cdp'
  }
}

async function cdpKeyPress(tabId: number, params: CDPActionParams): Promise<Record<string, unknown>> {
  const key = params.text || params.key || ''
  if (!key) {
    throw new Error('key_press requires text or key parameter')
  }

  const mapped = KEY_CODES[key]
  if (mapped) {
    // Named key (Enter, Tab, etc.)
    await dispatchCDPKeyPair(tabId, {
      key,
      code: mapped.code,
      keyCode: mapped.keyCode
    })
  } else {
    // Single character
    const info = charToKeyInfo(key)
    await dispatchCDPKeyPair(tabId, {
      key: info.key,
      code: info.code,
      keyCode: info.keyCode,
      text: key,
      unmodifiedText: info.shiftKey ? key.toLowerCase() : key,
      modifiers: info.shiftKey ? 8 : 0
    })
  }

  return {
    success: true,
    action: 'hardware_key_press',
    key,
    method: 'cdp'
  }
}

function parseCDPParams(query: PendingQuery): CDPActionParams | null {
  try {
    const raw = typeof query.params === 'string' ? JSON.parse(query.params) : query.params
    if (!raw || typeof raw !== 'object' || !('action' in raw)) return null
    return raw as CDPActionParams
  } catch {
    return null
  }
}

function mapCDPError(err: unknown): string {
  const msg = errorMessage(err, 'unknown_error')
  if (msg.includes('Cannot attach to this target')) {
    return 'cdp_attach_failed: Cannot attach debugger to this tab. It may be an internal browser page.'
  }
  if (msg.includes('Another debugger is already attached')) {
    return 'cdp_already_attached: Another debugger session is active. Close DevTools or other debugging sessions.'
  }
  if (msg.includes('Debugger is not attached')) {
    return 'cdp_not_attached: Debugger was detached during execution.'
  }
  return `cdp_error: ${msg}`
}

// =============================================================================
// AUTO-ESCALATION: CDP-first for click/type/key_press, fallback to DOM
// =============================================================================

// Platform-specific modifier for select-all (Meta on macOS, Ctrl elsewhere).
// Guard `navigator`: it is absent in the Node 20 test runner (global navigator
// only landed in Node 21), and an unguarded module-scope read makes importing
// this module throw ReferenceError there — taking down every test that touches
// it. The service worker always has navigator, so behavior is unchanged.
const SELECT_ALL_MODIFIER =
  typeof navigator !== 'undefined' && /mac/i.test(navigator.platform || '') ? 4 : 2

/** Actions that auto-escalate to CDP. */
const CDP_ESCALATABLE = new Set(['click', 'type', 'key_press'])

/** Check whether an action should attempt CDP before DOM primitives. */
export function isCDPEscalatable(action: string): boolean {
  return CDP_ESCALATABLE.has(action)
}

/**
 * Decide whether an action should attempt CDP hardware events before falling
 * back to DOM primitives. Callers should route through this single predicate.
 *
 * `dispatch: "dom"` is the #599 escape hatch: it forces the DOM-primitives path
 * (native-setter value + real element.click()), which drives React/Vue/Svelte
 * controlled inputs and delegated onClick handlers reliably, at the cost of
 * CDP's trusted (isTrusted:true) events. Frame/nth-scoped actions never use CDP
 * because CDP input targets the main frame by coordinate only.
 */
export function shouldEscalateToCDP(action: string, params: DOMActionParams): boolean {
  return (
    isCDPEscalatable(action) &&
    params.dispatch !== 'dom' &&
    !params.frame &&
    params.nth === undefined
  )
}

/**
 * Build an in-page JS expression that reconciles a controlled-input framework's
 * value tracker after a CDP type (#599).
 *
 * CDP `Input.dispatchKeyEvent` updates the element's DOM value, but React tracks
 * controlled-input values with a private `_valueTracker` and only fires its
 * synthetic `onChange` when a native `input` event reports a value that differs
 * from the tracked one. Re-applying the current value through the *prototype*
 * `value` setter (which bypasses React's instance-level override) and dispatching
 * a bubbling `input`/`change` makes React observe the change and fire onChange.
 *
 * The reconciliation is gated on a detected React tracker/fiber so it is a no-op
 * on plain inputs (no spurious double input/change), and idempotent on React
 * inputs whose onChange already fired (the tracker is current → no second fire).
 */
export function buildReactValueReconcileExpression(selector: string): string {
  const sel = JSON.stringify(selector)
  return `(() => {
    const el = document.querySelector(${sel});
    if (!el || (!(el instanceof HTMLInputElement) && !(el instanceof HTMLTextAreaElement))) return { reconciled: false };
    const keys = Object.keys(el);
    const hasReactTracker = !!el._valueTracker ||
      keys.some(k => k.indexOf('__reactFiber$') === 0 || k.indexOf('__reactProps$') === 0);
    if (!hasReactTracker) return { reconciled: false };
    const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
    const desc = Object.getOwnPropertyDescriptor(proto, 'value');
    if (desc && typeof desc.set === 'function') desc.set.call(el, el.value);
    el.dispatchEvent(new Event('input', { bubbles: true }));
    el.dispatchEvent(new Event('change', { bubbles: true }));
    return { reconciled: true };
  })()`
}

async function cdpClearField(tabId: number): Promise<void> {
  // Select all then delete — works cross-platform
  await cdpSend(tabId, 'Input.dispatchKeyEvent', {
    type: 'keyDown',
    key: 'a',
    code: 'KeyA',
    windowsVirtualKeyCode: 65,
    modifiers: SELECT_ALL_MODIFIER
  })
  await cdpSend(tabId, 'Input.dispatchKeyEvent', {
    type: 'keyUp',
    key: 'a',
    code: 'KeyA',
    windowsVirtualKeyCode: 65,
    modifiers: SELECT_ALL_MODIFIER
  })
  await cdpSend(tabId, 'Input.dispatchKeyEvent', {
    type: 'keyDown',
    key: 'Backspace',
    code: 'Backspace',
    windowsVirtualKeyCode: 8
  })
  await cdpSend(tabId, 'Input.dispatchKeyEvent', {
    type: 'keyUp',
    key: 'Backspace',
    code: 'Backspace',
    windowsVirtualKeyCode: 8
  })
}

async function cdpDispatchKeySequence(tabId: number, text: string): Promise<void> {
  for (const char of text) {
    const info = charToKeyInfo(char)
    const modifiers = info.shiftKey ? 8 : 0
    await cdpSend(tabId, 'Input.dispatchKeyEvent', {
      type: 'keyDown',
      key: info.key,
      code: info.code,
      text: char,
      unmodifiedText: info.shiftKey ? char.toLowerCase() : char,
      windowsVirtualKeyCode: info.keyCode,
      nativeVirtualKeyCode: info.keyCode,
      modifiers
    })
    await cdpSend(tabId, 'Input.dispatchKeyEvent', {
      type: 'keyUp',
      key: info.key,
      code: info.code,
      windowsVirtualKeyCode: info.keyCode,
      nativeVirtualKeyCode: info.keyCode,
      modifiers
    })
  }
}

async function cdpDispatchSingleKey(tabId: number, key: string): Promise<void> {
  const mapped = KEY_CODES[key]
  if (mapped) {
    await cdpSend(tabId, 'Input.dispatchKeyEvent', {
      type: 'keyDown',
      key,
      code: mapped.code,
      windowsVirtualKeyCode: mapped.keyCode,
      nativeVirtualKeyCode: mapped.keyCode
    })
    await cdpSend(tabId, 'Input.dispatchKeyEvent', {
      type: 'keyUp',
      key,
      code: mapped.code,
      windowsVirtualKeyCode: mapped.keyCode,
      nativeVirtualKeyCode: mapped.keyCode
    })
  } else {
    const info = charToKeyInfo(key)
    const modifiers = info.shiftKey ? 8 : 0
    await cdpSend(tabId, 'Input.dispatchKeyEvent', {
      type: 'keyDown',
      key: info.key,
      code: info.code,
      text: key,
      unmodifiedText: info.shiftKey ? key.toLowerCase() : key,
      windowsVirtualKeyCode: info.keyCode,
      nativeVirtualKeyCode: info.keyCode,
      modifiers
    })
    await cdpSend(tabId, 'Input.dispatchKeyEvent', {
      type: 'keyUp',
      key: info.key,
      code: info.code,
      windowsVirtualKeyCode: info.keyCode,
      nativeVirtualKeyCode: info.keyCode,
      modifiers
    })
  }
}

/**
 * Attempt CDP-first execution for click/type/key_press.
 * Returns a DOMResult on success, or null to signal fallback to DOM primitives.
 * Any error is caught internally — callers just check for null.
 */
export async function tryCDPEscalation(
  tabId: number,
  action: string,
  params: DOMActionParams
): Promise<DOMResult | null> {
  if (!CDP_ESCALATABLE.has(action)) return null
  // If CDP is unavailable in this runtime (tests, constrained extension contexts),
  // skip escalation before any DOM probing so normal DOM primitives remain deterministic.
  if (!chrome?.debugger?.attach || !chrome?.debugger?.sendCommand || !chrome?.debugger?.detach) {
    return null
  }

  const selector = params.selector || ''
  const startTime = Date.now()

  try {
    // Step 1: Resolve element via page script (also focuses for type/key_press)
    const resolved = await resolveElement(tabId, params)
    if (!resolved) return null

    // Step 2: Attach debugger
    await chrome.debugger.attach({ tabId }, CDP_VERSION)

    try {
      // Step 3: Execute CDP action
      if (action === 'click') {
        await cdpSend(tabId, 'Input.dispatchMouseEvent', {
          type: 'mousePressed',
          x: resolved.x,
          y: resolved.y,
          button: 'left',
          clickCount: 1
        })
        await cdpSend(tabId, 'Input.dispatchMouseEvent', {
          type: 'mouseReleased',
          x: resolved.x,
          y: resolved.y,
          button: 'left',
          clickCount: 1
        })
      } else if (action === 'type') {
        const text = params.text || ''
        if (!text) return null
        if (params.clear) await cdpClearField(tabId)
        await cdpDispatchKeySequence(tabId, text)
        // #599: CDP keystrokes leave React's controlled-input value tracker stale,
        // suppressing onChange. Reconcile through the native setter so onChange fires.
        // Best-effort: the value is already typed even if reconciliation is skipped.
        if (selector) {
          try {
            await cdpSend(tabId, 'Runtime.evaluate', {
              expression: buildReactValueReconcileExpression(selector),
              returnByValue: true
            })
          } catch {
            /* reconciliation failed — value is still typed into the DOM */
          }
        }
      } else if (action === 'key_press') {
        const key = params.text || ''
        if (!key) return null
        await cdpDispatchSingleKey(tabId, key)
      }

      // Step 4: Build DOMResult with matched evidence
      return buildCDPResult(action, selector, resolved, Date.now() - startTime)
    } finally {
      try {
        await chrome.debugger.detach({ tabId })
      } catch {
        /* already detached */
      }
    }
  } catch {
    // CDP unavailable or failed — fall back to DOM primitives silently
    return null
  }
}

// =============================================================================
// DIRECT CDP QUERIES (hardware_click via Go-side cdp_action)
// =============================================================================

export async function executeCDPAction(
  query: PendingQuery,
  tabId: number,
  syncClient: SyncClient,
  sendAsyncResult: SendAsyncResultFn,
  actionToast: ActionToastFn
): Promise<void> {
  const params = parseCDPParams(query)
  if (!params) {
    sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, 'invalid_params')
    return
  }

  const { action } = params
  if (!action) {
    sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, 'missing_action')
    return
  }

  const toastLabel = action === 'key_press' ? 'Typing...' : `CDP ${action}`
  actionToast(tabId, toastLabel, undefined, 'trying', 10000)

  try {
    await chrome.debugger.attach({ tabId }, CDP_VERSION)
  } catch (err) {
    const errorMsg = mapCDPError(err)
    actionToast(tabId, toastLabel, errorMsg, 'error')
    sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, errorMsg)
    return
  }

  try {
    let result: Record<string, unknown>

    switch (action) {
      case 'click':
        result = await cdpClick(tabId, params)
        break
      case 'type':
        result = await cdpType(tabId, params)
        break
      case 'key_press':
        result = await cdpKeyPress(tabId, params)
        break
      default:
        throw new Error(`Unknown CDP action: ${action}`)
    }

    actionToast(tabId, toastLabel, undefined, 'success')
    sendAsyncResult(syncClient, query.id, query.correlation_id!, 'complete', result)
  } catch (err) {
    const errorMsg = mapCDPError(err)
    actionToast(tabId, toastLabel, errorMsg, 'error')
    sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, errorMsg)
  } finally {
    try {
      await chrome.debugger.detach({ tabId })
    } catch {
      // Already detached or tab closed — safe to ignore
    }
  }
}
