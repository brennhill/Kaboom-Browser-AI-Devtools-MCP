/**
 * Purpose: Manages pending request/response pairs (highlight, execute_js, a11y, DOM queries) with timeout cleanup for AI Web Pilot features.
 * Docs: docs/features/feature/interact-explore/index.md
 */

/**
 * @fileoverview Request Tracking Module
 * Manages pending requests for AI Web Pilot features
 * Each request owns its expiry timer; page lifecycle cleanup cancels every timer.
 */

import type { HighlightResponse, ExecuteJsResult } from '../types/runtime-messages.js'
import type { A11yAuditResult } from '../types/capture/accessibility.js'
import type { DomQueryResult } from '../types/capture/dom.js'
import type { PendingRequestStats } from './types.js'

// Pending highlight response resolvers (keyed by request ID)
type PendingRequest<T> = {
  resolve: (result: T) => void
  timer: ReturnType<typeof setTimeout> | null
  cancel?: () => void
}

const pendingHighlightRequests = new Map<number, PendingRequest<HighlightResponse>>()
let highlightRequestId = 0

// Pending execute requests waiting for responses from inject.js
const pendingExecuteRequests = new Map<number, PendingRequest<ExecuteJsResult>>()
let executeRequestId = 0

// Pending a11y audit requests waiting for responses from inject.js
const pendingA11yRequests = new Map<number, PendingRequest<A11yAuditResult>>()
let a11yRequestId = 0

// Pending DOM query requests waiting for responses from inject.js
const pendingDomRequests = new Map<number, PendingRequest<DomQueryResult>>()
let domRequestId = 0

let initialized = false

function registerRequest<T>(
  requests: Map<number, PendingRequest<T>>,
  requestId: number,
  resolve: (result: T) => void,
  timeoutMs?: number,
  onTimeout?: () => void,
  onCancel?: () => void
): number {
  const request: PendingRequest<T> = { resolve, timer: null, cancel: onCancel }
  requests.set(requestId, request)
  if (timeoutMs !== undefined && onTimeout !== undefined) {
    request.timer = setTimeout(() => {
      if (!requests.delete(requestId)) return
      request.timer = null
      onTimeout()
    }, timeoutMs)
  }
  return requestId
}

function resolveRequest<T>(requests: Map<number, PendingRequest<T>>, requestId: number, result: T): void {
  const request = requests.get(requestId)
  if (!request) return
  requests.delete(requestId)
  if (request.timer !== null) clearTimeout(request.timer)
  request.resolve(result)
}

function deleteRequest<T>(requests: Map<number, PendingRequest<T>>, requestId: number): void {
  const request = requests.get(requestId)
  if (!request) return
  requests.delete(requestId)
  if (request.timer !== null) clearTimeout(request.timer)
}

function clearRequests<T>(requests: Map<number, PendingRequest<T>>): void {
  for (const request of requests.values()) {
    if (request.timer !== null) clearTimeout(request.timer)
    request.cancel?.()
  }
  requests.clear()
}

/**
 * Clear all pending request Maps on page unload (Issue 2 fix).
 * Prevents memory leaks and stale request accumulation across navigations.
 */
export function clearPendingRequests(): void {
  clearRequests(pendingHighlightRequests)
  clearRequests(pendingExecuteRequests)
  clearRequests(pendingA11yRequests)
  clearRequests(pendingDomRequests)
}

/**
 * Get statistics about pending requests (for testing/debugging)
 * @returns Counts of pending requests by type
 */
export function getPendingRequestStats(): PendingRequestStats {
  return {
    highlight: pendingHighlightRequests.size,
    execute: pendingExecuteRequests.size,
    a11y: pendingA11yRequests.size,
    dom: pendingDomRequests.size
  }
}

/**
 * Get the next highlight request ID and register a resolver
 */
export function registerHighlightRequest(resolve: (result: HighlightResponse) => void, onCancel?: () => void): number {
  const requestId = ++highlightRequestId
  return registerRequest(pendingHighlightRequests, requestId, resolve, undefined, undefined, onCancel)
}

/**
 * Resolve a highlight request
 */
export function resolveHighlightRequest(requestId: number, result: HighlightResponse): void {
  resolveRequest(pendingHighlightRequests, requestId, result)
}

/**
 * Check if a highlight request exists
 */
export function hasHighlightRequest(requestId: number): boolean {
  return pendingHighlightRequests.has(requestId)
}

/**
 * Delete a highlight request without resolving
 */
export function deleteHighlightRequest(requestId: number): void {
  deleteRequest(pendingHighlightRequests, requestId)
}

/**
 * Get the next execute request ID and register a resolver
 */
export function registerExecuteRequest(
  resolve: (result: ExecuteJsResult) => void,
  timeoutMs?: number,
  onTimeout?: () => void
): number {
  const requestId = ++executeRequestId
  return registerRequest(pendingExecuteRequests, requestId, resolve, timeoutMs, onTimeout, onTimeout)
}

/**
 * Resolve an execute request
 */
export function resolveExecuteRequest(requestId: number, result: ExecuteJsResult): void {
  resolveRequest(pendingExecuteRequests, requestId, result)
}

/**
 * Check if an execute request exists
 */
export function hasExecuteRequest(requestId: number): boolean {
  return pendingExecuteRequests.has(requestId)
}

/**
 * Delete an execute request without resolving
 */
export function deleteExecuteRequest(requestId: number): void {
  deleteRequest(pendingExecuteRequests, requestId)
}

/**
 * Get the next a11y request ID and register a resolver
 */
export function registerA11yRequest(
  resolve: (result: A11yAuditResult) => void,
  timeoutMs?: number,
  onTimeout?: () => void
): number {
  const requestId = ++a11yRequestId
  return registerRequest(pendingA11yRequests, requestId, resolve, timeoutMs, onTimeout, onTimeout)
}

/**
 * Resolve an a11y request
 */
export function resolveA11yRequest(requestId: number, result: A11yAuditResult): void {
  resolveRequest(pendingA11yRequests, requestId, result)
}

/**
 * Check if an a11y request exists
 */
export function hasA11yRequest(requestId: number): boolean {
  return pendingA11yRequests.has(requestId)
}

/**
 * Delete an a11y request without resolving
 */
export function deleteA11yRequest(requestId: number): void {
  deleteRequest(pendingA11yRequests, requestId)
}

/**
 * Get the next DOM request ID and register a resolver
 */
export function registerDomRequest(
  resolve: (result: DomQueryResult) => void,
  timeoutMs?: number,
  onTimeout?: () => void
): number {
  const requestId = ++domRequestId
  return registerRequest(pendingDomRequests, requestId, resolve, timeoutMs, onTimeout, onTimeout)
}

/**
 * Resolve a DOM request
 */
export function resolveDomRequest(requestId: number, result: DomQueryResult): void {
  resolveRequest(pendingDomRequests, requestId, result)
}

/**
 * Check if a DOM request exists
 */
export function hasDomRequest(requestId: number): boolean {
  return pendingDomRequests.has(requestId)
}

/**
 * Delete a DOM request without resolving
 */
export function deleteDomRequest(requestId: number): void {
  deleteRequest(pendingDomRequests, requestId)
}

/**
 * Cleanup periodic timer (Issue #2 fix).
 * Should be called when content script is shutting down.
 */
export function cleanupRequestTracking(): void {
  if (initialized) {
    window.removeEventListener('pagehide', clearPendingRequests)
    window.removeEventListener('beforeunload', clearPendingRequests)
    initialized = false
  }
  clearPendingRequests()
}

/**
 * Initialize request tracking (register cleanup handlers)
 */
export function initRequestTracking(): void {
  if (initialized) return
  // Register cleanup handlers for page unload/navigation (Issue 2 fix)
  // Using 'pagehide' (modern, fires on both close and navigation) + 'beforeunload' (legacy fallback)
  window.addEventListener('pagehide', clearPendingRequests)
  window.addEventListener('beforeunload', clearPendingRequests)
  initialized = true
}
