/**
 * Purpose: Dispatches hardware-level input via Chrome DevTools Protocol.
 * Why: Synthetic DOM events have isTrusted:false which anti-bot systems and complex SPAs ignore.
 *      CDP Input.dispatch* commands produce true hardware events indistinguishable from real user input.
 * Docs: docs/features/feature/interact-explore/index.md
 */

// cdp-dispatch.ts — CDP executor for hardware-level clicks and keypresses.
// Dispatches CDP Input.* commands over a lease from cdp-session.ts, which owns attach/detach.

import type { PendingQuery } from '../../../types/runtime/queries.js'
import type { SyncClient } from '../../sync/sync-client.js'
import type { DOMActionParams, DOMResult } from '../dom-types.js'
import type { SendAsyncResultFn, ActionToastFn } from '../../commands/helpers.js'
import { errorMessage } from '../../../lib/error-utils.js'
import {
  KEY_CODES,
  charToKeyInfo,
  isModifierShortcut,
  keyEventsForText,
  modifierBitmask,
  SHIFT_BIT
} from './cdp-key-mappings.js'
import { cdpSessions, CDP_SESSION_ERRORS, type Lease } from './cdp-session.js'
import { drivingSessions } from '../../supervision/driving-session.js'
import { resolveElement, buildCDPResult, buildCoordinateCDPResult, type ResolvedElement } from './cdp-element-resolve.js'
import { coordinateOutOfViewport } from '../viewport-bounds.js'
import {
  CDP_GESTURE_ACTIONS,
  isCDPGesture,
  explicitGesturePoint,
  executeCDPGesture,
  dispatchSingleClick,
  deliverZoomRegion,
  type GestureContext,
  type GesturePoint
} from './cdp-gestures.js'

interface CDPActionParams {
  action: string
  x?: number
  y?: number
  width?: number
  height?: number
  scale?: number
  selector?: string
  text?: string
  key?: string
  /** ctrl | shift | alt | cmd, combinable. Folded into Chrome's bitmask at dispatch. */
  modifiers?: string[]
}

async function cdpSend(lease: Lease, method: string, params: Record<string, unknown>): Promise<void> {
  await lease.send(method, params)
}

/**
 * The gesture surface a held lease exposes: send commands, and move the phantom cursor the
 * user watches. Built here so every CDP input path — direct action, escalation, gesture —
 * drives the same supervision cursor instead of some of them silently skipping it.
 */
function leaseGestureContext(lease: Lease, tabId: number): GestureContext {
  const driving = drivingSessions()
  return {
    send: (method, params) => lease.send(method, params),
    cursor: (x, y) => driving.cursor(tabId, x, y)
  }
}

async function resolveCoordinates(lease: Lease, params: CDPActionParams): Promise<{ x: number; y: number }> {
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

  const evalResult = (await lease.send('Runtime.evaluate', {
    expression,
    returnByValue: true
  })) as { result?: { value?: { x: number; y: number } | null } }

  const coords = evalResult?.result?.value
  if (!coords) {
    throw new Error(`Element not found: ${params.selector}`)
  }

  return coords
}

async function cdpClick(lease: Lease, tabId: number, params: CDPActionParams): Promise<Record<string, unknown>> {
  const { x, y } = await resolveCoordinates(lease, params)
  // A modifier the agent asked for and Chrome never saw is a silently different click:
  // ctrl+click opens a tab, plain click navigates in place. Carry the bitmask through.
  const modifiers = await dispatchSingleClick(leaseGestureContext(lease, tabId), { x, y }, params.modifiers)

  return {
    success: true,
    action: 'click',
    x,
    y,
    modifiers,
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

async function dispatchCDPKeyPair(lease: Lease, payload: CDPKeyEventPayload): Promise<void> {
  const common = {
    key: payload.key,
    code: payload.code,
    windowsVirtualKeyCode: payload.keyCode,
    nativeVirtualKeyCode: payload.keyCode
  }
  await cdpSend(lease, 'Input.dispatchKeyEvent', {
    type: 'keyDown',
    ...common,
    ...(payload.text !== undefined ? { text: payload.text } : {}),
    ...(payload.unmodifiedText !== undefined ? { unmodifiedText: payload.unmodifiedText } : {}),
    ...(payload.modifiers !== undefined ? { modifiers: payload.modifiers } : {})
  })
  await cdpSend(lease, 'Input.dispatchKeyEvent', {
    type: 'keyUp',
    ...common,
    ...(payload.modifiers !== undefined ? { modifiers: payload.modifiers } : {})
  })
}

async function cdpType(lease: Lease, params: CDPActionParams): Promise<Record<string, unknown>> {
  const text = params.text || ''
  if (!text) {
    throw new Error('type requires text parameter')
  }

  // A modifier the agent asked for and Chrome never saw is a silently different keystroke:
  // ctrl+a selects the field, a plain "a" appends a character. Carry the mask through.
  const modifiers = modifierBitmask(params.modifiers)
  await cdpDispatchKeySequence(lease, text, params.modifiers)

  return {
    success: true,
    action: 'hardware_type',
    char_count: text.length,
    modifiers,
    method: 'cdp'
  }
}

async function cdpKeyPress(lease: Lease, params: CDPActionParams): Promise<Record<string, unknown>> {
  const key = params.text || params.key || ''
  if (!key) {
    throw new Error('key_press requires text or key parameter')
  }

  const mapped = KEY_CODES[key]
  if (mapped) {
    // Named key (Enter, Tab, etc.)
    await dispatchCDPKeyPair(lease, {
      key,
      code: mapped.code,
      keyCode: mapped.keyCode
    })
  } else {
    // Single character
    const info = charToKeyInfo(key)
    await dispatchCDPKeyPair(lease, {
      key: info.key,
      code: info.code,
      keyCode: info.keyCode,
      text: key,
      unmodifiedText: info.shiftKey ? key.toLowerCase() : key,
      modifiers: info.shiftKey ? SHIFT_BIT : 0
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
    // EXPECTED_ABSENCE: malformed external parameters are an expected validation case; logging would duplicate the client error.
    return null
  }
}

/**
 * Terminal state for an action the USER interrupted. Distinct from any CDP fault: it is not
 * retryable, and the agent must be told a person stopped it rather than that the browser
 * misbehaved.
 */
export const STOPPED_BY_USER = 'stopped_by_user: the user stopped this action from the browser'

function mapCDPError(err: unknown): string {
  const msg = errorMessage(err, 'unknown_error')
  if (msg.includes('Cannot attach to this target')) {
    return 'cdp_attach_failed: Cannot attach debugger to this tab. It may be an internal browser page.'
  }
  if (msg.includes('Another debugger is already attached')) {
    return 'cdp_already_attached: Another debugger session is active. Close DevTools or other debugging sessions.'
  }
  if (msg.includes('Debugger is not attached') || msg.includes(CDP_SESSION_ERRORS.INVALIDATED)) {
    return 'cdp_not_attached: Debugger was detached during execution.'
  }
  if (msg.includes(CDP_SESSION_ERRORS.EXCLUSIVE_HELD)) {
    return 'cdp_busy: A performance trace holds this tab exclusively. Stop the trace and retry.'
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
const SELECT_ALL_MODIFIER = typeof navigator !== 'undefined' && /mac/i.test(navigator.platform || '') ? 4 : 2

/**
 * Actions that auto-escalate to CDP.
 *
 * The pointer gestures joined this set in kaboom-05ue.5. Before that, 3 of 58 interact actions
 * reached CDP and every other gesture was a synthetic DOM event with isTrusted:false.
 */
const CDP_ESCALATABLE = new Set(['click', 'type', 'key_press', ...CDP_GESTURE_ACTIONS])

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
  return isCDPEscalatable(action) && params.dispatch !== 'dom' && !params.frame && params.nth === undefined
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

async function cdpClearField(lease: Lease): Promise<void> {
  // Select all then delete — works cross-platform
  await cdpSend(lease, 'Input.dispatchKeyEvent', {
    type: 'keyDown',
    key: 'a',
    code: 'KeyA',
    windowsVirtualKeyCode: 65,
    modifiers: SELECT_ALL_MODIFIER
  })
  await cdpSend(lease, 'Input.dispatchKeyEvent', {
    type: 'keyUp',
    key: 'a',
    code: 'KeyA',
    windowsVirtualKeyCode: 65,
    modifiers: SELECT_ALL_MODIFIER
  })
  await cdpSend(lease, 'Input.dispatchKeyEvent', {
    type: 'keyDown',
    key: 'Backspace',
    code: 'Backspace',
    windowsVirtualKeyCode: 8
  })
  await cdpSend(lease, 'Input.dispatchKeyEvent', {
    type: 'keyUp',
    key: 'Backspace',
    code: 'Backspace',
    windowsVirtualKeyCode: 8
  })
}

/**
 * Type `text` over the lease, holding `held` for every keystroke.
 *
 * Exported so the modifier contract can be checked against the commands actually dispatched:
 * a mask that never reaches Input.dispatchKeyEvent produces an ordinary keystroke and still
 * reports success (kaboom-wpyt).
 */
export async function cdpDispatchKeySequence(
  lease: Lease,
  text: string,
  held?: readonly string[]
): Promise<void> {
  for (const event of keyEventsForText(text, held)) {
    await cdpSend(lease, 'Input.dispatchKeyEvent', event)
  }
}

async function cdpDispatchSingleKey(lease: Lease, key: string): Promise<void> {
  const mapped = KEY_CODES[key]
  if (mapped) {
    await cdpSend(lease, 'Input.dispatchKeyEvent', {
      type: 'keyDown',
      key,
      code: mapped.code,
      windowsVirtualKeyCode: mapped.keyCode,
      nativeVirtualKeyCode: mapped.keyCode
    })
    await cdpSend(lease, 'Input.dispatchKeyEvent', {
      type: 'keyUp',
      key,
      code: mapped.code,
      windowsVirtualKeyCode: mapped.keyCode,
      nativeVirtualKeyCode: mapped.keyCode
    })
  } else {
    const info = charToKeyInfo(key)
    const modifiers = info.shiftKey ? SHIFT_BIT : 0
    await cdpSend(lease, 'Input.dispatchKeyEvent', {
      type: 'keyDown',
      key: info.key,
      code: info.code,
      text: key,
      unmodifiedText: info.shiftKey ? key.toLowerCase() : key,
      windowsVirtualKeyCode: info.keyCode,
      nativeVirtualKeyCode: info.keyCode,
      modifiers
    })
    await cdpSend(lease, 'Input.dispatchKeyEvent', {
      type: 'keyUp',
      key: info.key,
      code: info.code,
      windowsVirtualKeyCode: info.keyCode,
      nativeVirtualKeyCode: info.keyCode,
      modifiers
    })
  }
}

/** Execute the CDP input action for click/type/key_press. Returns false when the action's payload is unusable. */
async function cdpExecuteAction(
  ctx: GestureContext,
  lease: Lease,
  action: string,
  params: DOMActionParams,
  selector: string,
  resolved: NonNullable<Awaited<ReturnType<typeof resolveElement>>>
): Promise<boolean> {
  if (action === 'click') {
    // Same click as every other CDP path, modifiers included. This branch used to build its
    // own press/release pair and drop params.modifiers on the floor.
    await dispatchSingleClick(ctx, { x: resolved.x, y: resolved.y }, params.modifiers)
    return true
  }
  if (action === 'type') {
    const text = params.text || ''
    if (!text) return false
    if (params.clear) await cdpClearField(lease)
    await cdpDispatchKeySequence(lease, text, params.modifiers)
    // #599: CDP keystrokes leave React's controlled-input value tracker stale,
    // suppressing onChange. Reconcile through the native setter so onChange fires.
    // Best-effort: the value is already typed even if reconciliation is skipped.
    // Skipped for a shortcut (ctrl/alt/cmd held): nothing was typed, so reconciling would
    // fire a spurious input/change on a React field the keystroke never edited.
    if (selector && !isModifierShortcut(modifierBitmask(params.modifiers))) {
      try {
        await cdpSend(lease, 'Runtime.evaluate', {
          expression: buildReactValueReconcileExpression(selector),
          returnByValue: true
        })
      } catch {
        // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
        // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
        /* reconciliation failed — value is still typed into the DOM */
      }
    }
    return true
  }
  if (action === 'key_press') {
    const key = params.text || ''
    if (!key) return false
    await cdpDispatchSingleKey(lease, key)
    return true
  }
  return true
}

/** Where a gesture lands, and the element behind it when one was resolved. */
interface GestureTarget {
  point: GesturePoint
  resolved: ResolvedElement | null
}

/**
 * Resolve the coordinate an action drives.
 *
 * Explicit coordinates win outright: hover_at, scroll_at and drag address a pixel, and running
 * selector resolution for them would fail on pages where nothing matches an empty selector and
 * silently downgrade the gesture to a synthetic DOM event.
 */
async function resolveGestureTarget(tabId: number, params: DOMActionParams): Promise<GestureTarget | null> {
  const explicit = explicitGesturePoint(params)
  if (explicit) return { point: explicit, resolved: null }
  const resolved = await resolveElement(tabId, params)
  if (!resolved) return null
  return { point: { x: resolved.x, y: resolved.y }, resolved }
}

/** Run one pointer gesture over the lease and turn its evidence into a DOMResult. */
async function runCDPGesture(
  lease: Lease,
  tabId: number,
  action: string,
  params: DOMActionParams,
  target: GestureTarget,
  startTime: number
): Promise<DOMResult> {
  // The phantom cursor follows a drag point by point, so the user watches the route the
  // agent is taking rather than seeing the pointer teleport after the fact.
  const ctx = leaseGestureContext(lease, tabId)
  const evidence = await executeCDPGesture(ctx, action, params, target.point)
  const elapsed = Date.now() - startTime
  const withPoint = { x: target.point.x, y: target.point.y, ...evidence }
  return target.resolved
    ? buildCDPResult(action, params.selector || '', target.resolved, elapsed, withPoint)
    : buildCoordinateCDPResult(action, target.point, elapsed, evidence)
}

/**
 * Attempt CDP-first execution for click/type/key_press and the pointer gestures.
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
  const sessions = cdpSessions()
  if (!sessions) return null

  const selector = params.selector || ''
  const startTime = Date.now()

  try {
    // Step 1: Resolve the target coordinate (also focuses the element for type/key_press)
    const target = await resolveGestureTarget(tabId, params)
    if (!target) return null
    // Selector-addressed actions need the matched element for their evidence; without one
    // there is nothing to report a match against, so the DOM path takes over.
    if (!target.resolved && !isCDPGesture(action)) return null

    // Step 2: Take a reference to the tab's shared CDP session. The session outlives this
    // action, so a click that starts a navigation is no longer racing its own teardown.
    const lease = await sessions.acquire(tabId)

    // Step 2a: Show the human what is about to happen, BEFORE the input dispatches. The
    // phantom cursor lands on the coordinate CDP is about to click, so what they see is
    // intent rather than history. The session also starts the heartbeat that keeps the
    // overlay from tearing itself down mid-action.
    const driving = drivingSessions()
    driving.start(tabId, action)
    driving.cursor(tabId, target.point.x, target.point.y)

    try {
      // Step 3: Execute CDP action
      if (isCDPGesture(action)) {
        return await runCDPGesture(lease, tabId, action, params, target, startTime)
      }
      const ctx = leaseGestureContext(lease, tabId)
      if (!(await cdpExecuteAction(ctx, lease, action, params, selector, target.resolved!))) return null

      // Step 4: Build DOMResult with matched evidence
      return buildCDPResult(action, selector, target.resolved!, Date.now() - startTime)
    } finally {
      driving.stop(tabId)
      lease.release()
    }
  } catch {
    // A user stop must NOT return null. Null means "CDP did not handle this", and the
    // caller then performs the same action with synthetic DOM events — so a stop would
    // re-execute the very action being stopped, and the user would be told they prevented
    // something that happened anyway. Return a result so the caller reports it instead.
    if (drivingSessions().consumeStopRequest(tabId)) {
      return {
        success: false,
        action,
        selector: params.selector || '',
        error: STOPPED_BY_USER
      }
    }
    // EXPECTED_ABSENCE: unavailable optional CDP is normal; logging would mislabel the canonical DOM fallback as failure.
    return null
  }
}

// =============================================================================
// DIRECT CDP QUERIES (a coordinate-addressed click via Go-side cdp_action)
// =============================================================================

/** Direct CDP actions that only read pixels. They dispatch no input and drive nothing. */
const CDP_CAPTURE_ACTIONS: ReadonlySet<string> = new Set(['zoom_region'])

function isCDPCapture(action: string): boolean {
  return CDP_CAPTURE_ACTIONS.has(action)
}

/** Run one directly-addressed CDP action and return the terminal payload to send back. */
async function dispatchDirectCDPAction(
  lease: Lease,
  target: { tabId: number; queryId: string },
  action: string,
  params: CDPActionParams
): Promise<Record<string, unknown>> {
  switch (action) {
    case 'click':
      return cdpClick(lease, target.tabId, params)
    case 'type':
      return cdpType(lease, params)
    case 'key_press':
      return cdpKeyPress(lease, params)
    case 'zoom_region':
      await lease.ensureDomain('Page')
      return deliverZoomRegion((method, sendParams) => lease.send(method, sendParams), params, target)
    default:
      throw new Error(`Unknown CDP action: ${action}`)
  }
}

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

  // A capture may legitimately name a rectangle outside the visible area; input may not. Chrome
  // clamps an out-of-range Input.dispatchMouseEvent onto the nearest edge and reports success, so
  // the point is checked against the page's own viewport before the debugger is even attached.
  if (!isCDPCapture(action)) {
    const offScreen = await coordinateOutOfViewport(tabId, action, params)
    if (offScreen) {
      actionToast(tabId, toastLabel, offScreen, 'error')
      sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, offScreen)
      return
    }
  }

  const sessions = cdpSessions()
  if (!sessions) {
    const errorMsg = 'cdp_unavailable: chrome.debugger is not available in this context'
    actionToast(tabId, toastLabel, errorMsg, 'error')
    sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, errorMsg)
    return
  }

  const driving = drivingSessions()
  let lease: Lease
  try {
    lease = await sessions.acquire(tabId)
  } catch (err) {
    const errorMsg = mapCDPError(err)
    actionToast(tabId, toastLabel, errorMsg, 'error')
    sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, errorMsg)
    return
  }

  // A capture is not input. Starting a driving session paints the supervision indicator and
  // phantom cursor into the page milliseconds before Page.captureScreenshot reads the pixels,
  // so the region the agent asked to read back would sometimes contain Kaboom's own overlay.
  const drivesInput = !isCDPCapture(action)
  if (drivesInput) {
    driving.start(tabId, action)
    if (typeof params.x === 'number' && typeof params.y === 'number') {
      driving.cursor(tabId, params.x, params.y)
    }
  }

  try {
    const result = await dispatchDirectCDPAction(lease, { tabId, queryId: query.id }, action, params)

    actionToast(tabId, toastLabel, undefined, 'success')
    sendAsyncResult(syncClient, query.id, query.correlation_id!, 'complete', result)
  } catch (err) {
    // A session invalidated by the user's Stop is not a CDP fault. Reporting it as one
    // would send the agent into a retry against a tab whose owner just told it to stop.
    const errorMsg = driving.consumeStopRequest(tabId) ? STOPPED_BY_USER : mapCDPError(err)
    actionToast(tabId, toastLabel, errorMsg, 'error')
    sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, errorMsg)
  } finally {
    if (drivesInput) driving.stop(tabId)
    lease.release()
  }
}
