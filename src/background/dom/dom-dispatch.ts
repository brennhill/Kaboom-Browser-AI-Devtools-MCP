/**
 * Purpose: Dispatches DOM actions (click, type, wait_for, list_interactive, query) to injected page scripts with frame targeting and CDP escalation.
 * Docs: docs/features/feature/interact-explore/index.md
 */

// dom-dispatch.ts — DOM action dispatcher and utilities.
// Owns action-family routing independently from the self-contained injected functions.
// Script builders stay self-contained because chrome.scripting.executeScript
// serializes injected functions independently.

import type { PendingQuery } from '../../types/runtime/queries.js'
import type { SyncClient } from '../sync/sync-client.js'
import type { DOMActionParams, DOMResult } from './dom-types.js'
import type { SendAsyncResultFn, ActionToastFn } from '../commands/helpers.js'
import { domFrameProbe, domFrameOriginProbe } from './primitives/dom-frame-probe.js'
import { domPrimitivePointer } from './primitives/dom-primitives-pointer.js'
import { domPrimitiveForm } from './primitives/dom-primitives-form.js'
import { domPrimitiveRead } from './primitives/dom-primitives-read.js'
import { domPrimitiveListInteractive } from './primitives/dom-primitives-list-interactive.js'
import { domPrimitiveQuery } from './primitives/dom-primitives-query.js'
import { domPrimitiveWaitForStable, domPrimitiveActionDiff } from './primitives/dom-primitives-stability.js'
import { domPrimitiveOverlay } from './primitives/dom-primitives-overlay.js'
import { domPrimitiveIntent } from './primitives/dom-primitives-intent.js'
import {
  gesturePrimitiveFor,
  gestureModifierMask,
  buildGestureDOMResult,
  needsGestureDispatch
} from './primitives/gestures/dom-primitives-gestures.js'
import { resolveElement, type ResolvedElement } from './cdp/cdp-element-resolve.js'
import { explicitGesturePoint } from './cdp/cdp-gestures.js'
import { shouldEscalateToCDP, tryCDPEscalation } from './cdp/cdp-dispatch.js'
import { coordinateOutOfViewport } from './viewport-bounds.js'
import { isReadOnlyAction } from '../exec/action-metadata.js'
import { errorMessage } from '../../lib/error-utils.js'
import { delay } from '../../lib/timeout-utils.js'
import { normalizeFrameArg, resolveMatchedFrameIds } from '../exec/frame-targeting.js'
import { DebugCategory, debugLog } from '../debug.js'
import type { FrameOriginInfo, FrameOriginMap } from './dom-result-reconcile.js'
import {
  toDOMResult,
  pickFrameResult,
  mergeListInteractive,
  deriveAsyncStatusFromDOMResult,
  enrichWithEffectiveContext,
  sendToastForResult
} from './dom-result-reconcile.js'

function parseDOMParams(query: PendingQuery): DOMActionParams | null {
  try {
    return typeof query.params === 'string' ? JSON.parse(query.params) : (query.params as DOMActionParams)
  } catch {
    // EXPECTED_ABSENCE: malformed external parameters are an expected validation case; logging would duplicate the client error.
    return null
  }
}

type DOMExecutionTarget = { tabId: number; allFrames: true } | { tabId: number; frameIds: number[] }

async function resolveExecutionTarget(tabId: number, frame: unknown): Promise<DOMExecutionTarget> {
  const normalized = normalizeFrameArg(frame)

  if (normalized === undefined || normalized === 'all') {
    return { tabId, allFrames: true }
  }
  const frameIds = await resolveMatchedFrameIds(tabId, normalized, domFrameProbe)

  return { tabId, frameIds }
}

const WAIT_FOR_POLL_INTERVAL_MS = 80

/** Resolve which DOM action name to dispatch for wait_for based on params.
 *  Callers must validate mutual exclusivity before calling this. */
function resolveWaitForAction(params: DOMActionParams): string {
  if (params.absent) return 'wait_for_absent'
  if (params.text) return 'wait_for_text'
  return 'wait_for'
}

async function executeWaitForURL(tabId: number, params: DOMActionParams): Promise<DOMResult> {
  const urlSubstring = params.url_contains!
  const timeoutMs = Math.max(1, params.timeout_ms ?? 5000)
  const startedAt = Date.now()

  while (true) {
    const tab = await chrome.tabs.get(tabId)
    if (tab.url && tab.url.includes(urlSubstring)) {
      return {
        success: true,
        action: 'wait_for',
        selector: '',
        value: tab.url
      }
    }
    if (Date.now() - startedAt >= timeoutMs) {
      return {
        success: false,
        action: 'wait_for',
        selector: '',
        error: 'timeout',
        message: `URL did not contain "${urlSubstring}" within ${timeoutMs}ms`
      }
    }
    const remaining = timeoutMs - (Date.now() - startedAt)
    await delay(Math.min(WAIT_FOR_POLL_INTERVAL_MS, Math.max(1, remaining)))
  }
}

async function executeWaitFor(target: DOMExecutionTarget, params: DOMActionParams): Promise<DOMResult> {
  const selector = params.selector || ''
  const timeoutMs = Math.max(1, params.timeout_ms ?? 5000)
  const domAction = resolveWaitForAction(params)
  const domOpts = { timeout_ms: timeoutMs, text: params.text }
  const startedAt = Date.now()
  const quickCheck = await chrome.scripting.executeScript({
    target,
    world: 'MAIN',
    func: domPrimitiveRead,
    args: [domAction, selector, domOpts]
  })
  const quickPicked = pickFrameResult(quickCheck)
  const quickResult = toDOMResult(quickPicked?.result)
  if (quickResult?.success) {
    return quickResult
  }

  let lastResult: DOMResult | null = toDOMResult(quickPicked?.result) ?? null
  while (Date.now() - startedAt < timeoutMs) {
    const remaining = timeoutMs - (Date.now() - startedAt)
    await delay(Math.min(WAIT_FOR_POLL_INTERVAL_MS, Math.max(1, remaining)))

    const probeResults = await chrome.scripting.executeScript({
      target,
      world: 'MAIN',
      func: domPrimitiveRead,
      args: [domAction, selector, domOpts]
    })

    const picked = pickFrameResult(probeResults)
    const result = toDOMResult(picked?.result)
    if (result) lastResult = result
    if (result?.success) {
      return result
    }
  }

  const label =
    domAction === 'wait_for_text'
      ? `Text "${params.text}" not found within ${timeoutMs}ms`
      : domAction === 'wait_for_absent'
        ? `Element still present within ${timeoutMs}ms: ${selector}`
        : undefined
  if (lastResult?.error === 'timeout') {
    return lastResult
  }
  return {
    success: false,
    action: 'wait_for',
    selector,
    error: 'timeout',
    message: label || `Element not found within ${timeoutMs}ms: ${selector}`
  }
}

async function executeStandardAction(
  target: DOMExecutionTarget,
  params: DOMActionParams
): Promise<chrome.scripting.InjectionResult[]> {
  const primitive = POINTER_ACTIONS.has(params.action!)
    ? domPrimitivePointer
    : FORM_ACTIONS.has(params.action!)
      ? domPrimitiveForm
      : domPrimitiveRead
  return chrome.scripting.executeScript({
    target,
    world: 'MAIN',
    func: primitive,
    args: [
      params.action!,
      params.selector || '',
      {
        text: params.text,
        key: params.key,
        value: params.value,
        direction: params.direction,
        clear: params.clear,
        checked: params.checked,
        name: params.name,
        timeout_ms: params.timeout_ms,
        stability_ms: params.stability_ms,
        analyze: params.analyze,
        observe_mutations: params.observe_mutations,
        element_id: params.element_id,
        scope_selector: params.scope_selector,
        scope_rect: params.scope_rect,
        nth: params.nth,
        new_tab: params.new_tab,
        structured: params.structured,
        // A modifier stripped here never reaches the page: the injected keystroke would carry
        // no ctrl/alt/cmd and the shortcut the agent asked for would silently not happen.
        modifiers: params.modifiers
      }
    ]
  })
}

async function executeListInteractive(
  target: DOMExecutionTarget,
  params: DOMActionParams
): Promise<chrome.scripting.InjectionResult[]> {
  // Build options object with scope_rect and filter params (#369)
  const opts: Record<string, unknown> = {}
  if (params.scope_rect) opts.scope_rect = params.scope_rect
  if (params.text_contains) opts.text_contains = params.text_contains
  if (params.role) opts.role = params.role
  if (params.visible_only) opts.visible_only = params.visible_only
  if (params.exclude_nav) opts.exclude_nav = params.exclude_nav

  const hasOpts = Object.keys(opts).length > 0
  const args: [string] | [string, Record<string, unknown>] = hasOpts
    ? [params.selector || '', opts]
    : [params.selector || '']
  return chrome.scripting.executeScript({
    target,
    world: 'MAIN',
    func: domPrimitiveListInteractive,
    args
  })
}

// #370: Execute DOM query (exists, count, text, text_all, attributes)
async function executeQuery(
  target: DOMExecutionTarget,
  params: DOMActionParams
): Promise<chrome.scripting.InjectionResult[]> {
  const opts: Record<string, unknown> = {}
  if (params.query_type) opts.query_type = params.query_type
  if (params.attribute_names) opts.attribute_names = params.attribute_names
  if (params.scope_selector) opts.scope_selector = params.scope_selector

  return chrome.scripting.executeScript({
    target,
    world: 'MAIN',
    func: domPrimitiveQuery,
    args: [params.selector || '', Object.keys(opts).length > 0 ? opts : undefined]
  })
}

// #502: Execute stability actions (wait_for_stable, action_diff) via extracted self-contained functions
async function executeStabilityAction(
  target: DOMExecutionTarget,
  params: DOMActionParams
): Promise<chrome.scripting.InjectionResult[]> {
  if (params.action === 'wait_for_stable') {
    return chrome.scripting.executeScript({
      target,
      world: 'MAIN',
      func: domPrimitiveWaitForStable,
      args: [{ stability_ms: params.stability_ms, timeout_ms: params.timeout_ms }]
    })
  }
  // action_diff
  return chrome.scripting.executeScript({
    target,
    world: 'MAIN',
    func: domPrimitiveActionDiff,
    args: [{ timeout_ms: params.timeout_ms }]
  })
}

// #502: Execute overlay actions (dismiss_top_overlay, auto_dismiss_overlays) via extracted self-contained function
async function executeOverlayAction(
  target: DOMExecutionTarget,
  params: DOMActionParams
): Promise<chrome.scripting.InjectionResult[]> {
  return chrome.scripting.executeScript({
    target,
    world: 'MAIN',
    func: domPrimitiveOverlay,
    args: [
      params.action as 'dismiss_top_overlay' | 'auto_dismiss_overlays',
      { scope_selector: params.scope_selector, timeout_ms: params.timeout_ms }
    ]
  })
}

// #502: Execute intent actions (open_composer, submit_active_composer, confirm_top_dialog) via extracted self-contained function
async function executeIntentAction(
  target: DOMExecutionTarget,
  params: DOMActionParams
): Promise<chrome.scripting.InjectionResult[]> {
  return chrome.scripting.executeScript({
    target,
    world: 'MAIN',
    func: domPrimitiveIntent,
    args: [
      params.action as 'open_composer' | 'submit_active_composer' | 'confirm_top_dialog',
      { scope_selector: params.scope_selector }
    ]
  })
}

// #599 escape hatch and no-CDP fallback for the pointer gestures. A gesture that reaches here
// still drives the page — it loses isTrusted:true, not the interaction.
//
// The target is resolved here rather than inside the injected primitive: resolveElement already
// owns the selector engine, and a second copy in page scope would be one more place for
// "which element did we hit" to diverge from what the CDP path reports.
async function executeGestureAction(target: DOMExecutionTarget, params: DOMActionParams): Promise<DOMResult> {
  const action = params.action!
  const selector = params.selector || ''
  const explicit = explicitGesturePoint(params)
  let resolved: ResolvedElement | null = null
  if (!explicit) {
    resolved = await resolveElement(target.tabId, params)
    if (!resolved) {
      return {
        success: false,
        action,
        selector,
        error: 'element_not_found',
        message: `No element resolved for ${action} (selector=${selector || '<none>'})`
      }
    }
  }
  const point = explicit ?? { x: resolved!.x, y: resolved!.y }
  const modifiers = gestureModifierMask(params.modifiers)
  const results = await chrome.scripting.executeScript({
    target,
    world: 'MAIN',
    func: gesturePrimitiveFor(action),
    args: [action, point, { modifiers, drag_path: params.drag_path, delta_x: params.delta_x, delta_y: params.delta_y }]
  })
  return buildGestureDOMResult(action, selector, point, resolved, results?.[0]?.result ?? undefined, modifiers)
}

const STABILITY_ACTIONS = new Set(['wait_for_stable', 'action_diff'])
const OVERLAY_ACTIONS = new Set(['dismiss_top_overlay', 'auto_dismiss_overlays'])
const INTENT_ACTIONS = new Set(['open_composer', 'submit_active_composer', 'confirm_top_dialog'])
const POINTER_ACTIONS = new Set(['click', 'hover', 'focus', 'scroll_to'])
const FORM_ACTIONS = new Set(['type', 'paste', 'select', 'check', 'key_press', 'set_attribute'])

function sendDOMError(
  syncClient: SyncClient,
  query: PendingQuery,
  message: string,
  sendAsyncResult: SendAsyncResultFn
): void {
  sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, message)
}

/** Validate wait_for condition exclusivity; returns the rejection message or null. */
function waitForParamsError(params: DOMActionParams, selector: string): string | null {
  const hasSelector = !!(selector || params.element_id)
  const hasText = !!params.text
  const hasURL = !!params.url_contains
  const condCount = (hasSelector || params.absent ? 1 : 0) + (hasText ? 1 : 0) + (hasURL ? 1 : 0)
  if (condCount === 0) return 'wait_for requires selector, text, or url_contains'
  if (condCount > 1) return 'wait_for conditions are mutually exclusive'
  if (params.absent && !hasSelector) return 'wait_for with absent requires a selector'
  return null
}

/** Returns true when the wait_for params are invalid and the query has been rejected. */
function rejectInvalidWaitFor(
  params: DOMActionParams,
  syncClient: SyncClient,
  query: PendingQuery,
  sendAsyncResult: SendAsyncResultFn
): boolean {
  const error = waitForParamsError(params, params.selector || '')
  if (!error) return false
  sendDOMError(syncClient, query, error, sendAsyncResult)
  return true
}

function resolveToastContext(params: DOMActionParams): { label: string; detail: string | undefined } {
  const label = params.reason || params.action!
  const detail = params.reason ? undefined : params.selector || 'page'
  return { label, detail }
}

async function executeWaitForURLFlow(
  tabId: number,
  params: DOMActionParams,
  syncClient: SyncClient,
  query: PendingQuery,
  actionToast: ActionToastFn,
  sendAsyncResult: SendAsyncResultFn
): Promise<void> {
  try {
    const urlResult = await executeWaitForURL(tabId, params)
    sendAsyncResult(
      syncClient,
      query.id,
      query.correlation_id!,
      urlResult.success ? 'complete' : 'error',
      await enrichWithEffectiveContext(tabId, urlResult),
      urlResult.success ? undefined : urlResult.error
    )
  } catch (err) {
    actionToast(tabId, 'wait_for', errorMessage(err), 'error')
    sendDOMError(syncClient, query, errorMessage(err), sendAsyncResult)
  }
}

/** Shared context for reporting a DOM action outcome: toast surface plus async result routing. */
interface DOMOutcomeContext {
  tabId: number
  readOnly: boolean
  toastLabel: string
  toastDetail: string | undefined
  syncClient: SyncClient
  query: PendingQuery
  actionToast: ActionToastFn
  sendAsyncResult: SendAsyncResultFn
}

/** Dispatch a reconciled DOM result to the client, enriched with effective context. */
async function sendReconciledAsyncResult(
  ctx: DOMOutcomeContext,
  status: 'complete' | 'error',
  reconciledResult: unknown,
  error: string | undefined
): Promise<void> {
  ctx.sendAsyncResult(
    ctx.syncClient,
    ctx.query.id,
    ctx.query.correlation_id!,
    status,
    await enrichWithEffectiveContext(ctx.tabId, reconciledResult),
    error
  )
}

async function attemptCDPEscalation(
  ctx: DOMOutcomeContext,
  action: string,
  params: DOMActionParams,
  selector: string
): Promise<boolean> {
  // CDP auto-escalation: try hardware events first for click/type/key_press (main frame only).
  // Falls back to DOM primitives silently if CDP is unavailable or fails.
  // `dispatch: "dom"` opts out entirely (React escape hatch, #599).
  if (!shouldEscalateToCDP(action, params)) return false
  try {
    const cdpResult = await tryCDPEscalation(ctx.tabId, action, params)
    if (!cdpResult) return false
    const { result: reconciledResult, status, error } = deriveAsyncStatusFromDOMResult(action, selector, cdpResult)
    const domResult = toDOMResult(reconciledResult)
    if (domResult) {
      sendToastForResult(ctx.tabId, false, domResult, ctx.actionToast, ctx.toastLabel, ctx.toastDetail)
    } else {
      ctx.actionToast(ctx.tabId, ctx.toastLabel, ctx.toastDetail, 'success')
    }
    await sendReconciledAsyncResult(ctx, status, reconciledResult, error)
    return true
  } catch {
    // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
    // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
    // CDP failed — fall through to DOM primitives
    return false
  }
}

async function routeDOMExecution(
  action: string,
  target: DOMExecutionTarget,
  params: DOMActionParams
): Promise<chrome.scripting.InjectionResult[] | DOMResult> {
  if (action === 'list_interactive') return executeListInteractive(target, params)
  if (action === 'query') return executeQuery(target, params)
  if (action === 'wait_for') return executeWaitFor(target, params)
  if (needsGestureDispatch(action, params.modifiers)) return executeGestureAction(target, params)
  if (STABILITY_ACTIONS.has(action)) return executeStabilityAction(target, params)
  if (OVERLAY_ACTIONS.has(action)) return executeOverlayAction(target, params)
  if (INTENT_ACTIONS.has(action)) return executeIntentAction(target, params)
  return executeStandardAction(target, params)
}

function notifyMissingResult(ctx: DOMOutcomeContext): void {
  if (!ctx.readOnly) ctx.actionToast(ctx.tabId, ctx.toastLabel, 'no result', 'error')
  sendDOMError(ctx.syncClient, ctx.query, 'no_result', ctx.sendAsyncResult)
}

async function sendDirectDOMResult(
  ctx: DOMOutcomeContext,
  action: string,
  selector: string,
  rawResult: DOMResult
): Promise<void> {
  const { result: reconciledResult, status, error } = deriveAsyncStatusFromDOMResult(action, selector, rawResult)
  const domResult = toDOMResult(reconciledResult)
  if (domResult) {
    sendToastForResult(ctx.tabId, ctx.readOnly, domResult, ctx.actionToast, ctx.toastLabel, ctx.toastDetail)
  } else if (!ctx.readOnly && status === 'complete') {
    ctx.actionToast(ctx.tabId, ctx.toastLabel, ctx.toastDetail, 'success')
  } else if (!ctx.readOnly && status === 'error') {
    ctx.actionToast(ctx.tabId, ctx.toastLabel, error || 'failed', 'error')
  }
  await sendReconciledAsyncResult(ctx, status, reconciledResult, error)
}

/**
 * Ask every frame which origin it is.
 *
 * One extra injected call per list_interactive, run against the same frames the elements came
 * from. On failure the merge reports attribution as unavailable rather than guessing an origin:
 * a wrong origin would misattribute a control an agent is about to click.
 */
async function probeFrameOrigins(tabId: number): Promise<FrameOriginMap | undefined> {
  try {
    const probed = await chrome.scripting.executeScript({
      target: { tabId, allFrames: true },
      world: 'MAIN',
      func: domFrameOriginProbe
    })
    const origins = new Map<number, FrameOriginInfo>()
    for (const entry of probed) {
      const value = entry.result as FrameOriginInfo | undefined
      if (value && typeof value.origin === 'string') origins.set(entry.frameId, value)
    }
    return origins.size > 0 ? origins : undefined
  } catch (err) {
    debugLog(DebugCategory.QUERY, 'Frame origin probe failed; element provenance is unavailable', {
      error: errorMessage(err, 'frame_origin_probe_failed')
    })
    return undefined
  }
}

async function sendListInteractiveResult(
  tabId: number,
  rawResult: chrome.scripting.InjectionResult[],
  syncClient: SyncClient,
  query: PendingQuery,
  sendAsyncResult: SendAsyncResultFn
): Promise<void> {
  const merged = mergeListInteractive(rawResult, await probeFrameOrigins(tabId))
  sendAsyncResult(
    syncClient,
    query.id,
    query.correlation_id!,
    merged.success ? 'complete' : 'error',
    await enrichWithEffectiveContext(tabId, merged),
    merged.success ? undefined : merged.error || 'list_interactive_failed'
  )
}

function buildFramePayload(picked: { result: unknown; frameId: number }, firstResult: object): Record<string, unknown> {
  const base: Record<string, unknown> = { ...(firstResult as Record<string, unknown>), frame_id: picked.frameId }
  const matched = base['matched']
  if (matched && typeof matched === 'object' && !Array.isArray(matched)) {
    base['matched'] = { ...(matched as Record<string, unknown>), frame_id: picked.frameId }
  }
  return base
}

async function sendFrameResult(
  ctx: DOMOutcomeContext,
  action: string,
  selector: string,
  resultPayload: unknown
): Promise<void> {
  const { result: reconciledResult, status, error } = deriveAsyncStatusFromDOMResult(action, selector, resultPayload)
  const domResult = toDOMResult(reconciledResult)
  if (domResult) {
    sendToastForResult(ctx.tabId, ctx.readOnly, domResult, ctx.actionToast, ctx.toastLabel, ctx.toastDetail)
  } else if (!ctx.readOnly && status === 'error') {
    ctx.actionToast(ctx.tabId, ctx.toastLabel, error || 'failed', 'error')
  }
  await sendReconciledAsyncResult(ctx, status, reconciledResult, error)
}

async function finalizeFrameResults(
  ctx: DOMOutcomeContext,
  action: string,
  selector: string,
  rawResult: chrome.scripting.InjectionResult[],
  tryingShownAt: number
): Promise<void> {
  // Ensure "trying" toast is visible for at least 500ms
  const MIN_TOAST_MS = 500
  const elapsed = Date.now() - tryingShownAt
  if (!ctx.readOnly && elapsed < MIN_TOAST_MS) await delay(MIN_TOAST_MS - elapsed)

  // list_interactive: merge elements from all frames
  if (action === 'list_interactive') {
    await sendListInteractiveResult(ctx.tabId, rawResult, ctx.syncClient, ctx.query, ctx.sendAsyncResult)
    return
  }

  const picked = pickFrameResult(rawResult)
  const firstResult = picked?.result
  if (firstResult && typeof firstResult === 'object') {
    const resultPayload = picked ? buildFramePayload(picked, firstResult) : firstResult
    await sendFrameResult(ctx, action, selector, resultPayload)
  } else {
    notifyMissingResult(ctx)
  }
}

export async function executeDOMAction(
  query: PendingQuery,
  tabId: number,
  syncClient: SyncClient,
  sendAsyncResult: SendAsyncResultFn,
  actionToast: ActionToastFn
): Promise<void> {
  const params = parseDOMParams(query)
  if (!params) {
    sendDOMError(syncClient, query, 'invalid_params', sendAsyncResult)
    return
  }

  const { action, selector } = params
  if (!action) {
    sendDOMError(syncClient, query, 'missing_action', sendAsyncResult)
    return
  }
  if (action === 'wait_for' && rejectInvalidWaitFor(params, syncClient, query, sendAsyncResult)) {
    return
  }

  // A point off the screen is refused BEFORE either dispatch path. Neither would refuse it
  // itself: CDP clamps an out-of-range Input.dispatchMouseEvent to the nearest edge and reports
  // success, and the DOM fallback's elementFromPoint returns null, which reads as "nothing is
  // there" rather than "you pointed off the screen".
  const offScreen = await coordinateOutOfViewport(tabId, action, params)
  if (offScreen) {
    actionToast(tabId, action, offScreen, 'error')
    sendDOMError(syncClient, query, offScreen, sendAsyncResult)
    return
  }

  const { label: toastLabel, detail: toastDetail } = resolveToastContext(params)
  const readOnly = isReadOnlyAction(action)
  const selectorArg = selector || ''
  const outcome: DOMOutcomeContext = {
    tabId,
    readOnly,
    toastLabel,
    toastDetail,
    syncClient,
    query,
    actionToast,
    sendAsyncResult
  }

  // URL-based wait_for: polls chrome.tabs.get from background — no page injection needed.
  if (action === 'wait_for' && params.url_contains) {
    await executeWaitForURLFlow(tabId, params, syncClient, query, actionToast, sendAsyncResult)
    return
  }

  try {
    const executionTarget = await resolveExecutionTarget(tabId, params.frame)
    const tryingShownAt = Date.now()
    if (!readOnly) actionToast(tabId, toastLabel, toastDetail, 'trying', 10000)

    if (await attemptCDPEscalation(outcome, action, params, selectorArg)) {
      return
    }

    const rawResult = await routeDOMExecution(action, executionTarget, params)

    // wait_for quick-check can return a DOMResult directly
    if (!Array.isArray(rawResult)) {
      if (!rawResult) {
        notifyMissingResult(outcome)
        return
      }
      await sendDirectDOMResult(outcome, action, selectorArg, rawResult)
      return
    }

    await finalizeFrameResults(outcome, action, selectorArg, rawResult, tryingShownAt)
  } catch (err) {
    actionToast(tabId, action, errorMessage(err), 'error')
    sendDOMError(syncClient, query, errorMessage(err), sendAsyncResult)
  }
}
