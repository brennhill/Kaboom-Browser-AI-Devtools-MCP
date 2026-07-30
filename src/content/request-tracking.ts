/**
 * Purpose: Manages pending request/response pairs (highlight, execute_js, a11y, DOM queries) with timeout cleanup for AI Web Pilot features.
 * Docs: docs/features/feature/interact-explore/index.md
 */

/**
 * @fileoverview Request Tracking Module
 * Manages pending requests for AI Web Pilot features
 * Includes periodic cleanup timer to handle edge cases where pagehide/beforeunload don't fire.
 */

import type { HighlightResponse, ExecuteJsResult } from '../types/runtime-messages.js'
import type { A11yAuditResult } from '../types/capture/accessibility.js'
import type { DomQueryResult } from '../types/capture/dom.js'
import type { PendingRequestStats } from './types.js'

// Pending highlight response resolvers (keyed by request ID)
const pendingHighlightRequests = new Map<number, (result: HighlightResponse) => void>()
let highlightRequestId = 0

// Pending execute requests waiting for responses from inject.js
const pendingExecuteRequests = new Map<number, (result: ExecuteJsResult) => void>()
let executeRequestId = 0

// Pending a11y audit requests waiting for responses from inject.js
const pendingA11yRequests = new Map<number, (result: A11yAuditResult) => void>()
let a11yRequestId = 0

// Pending DOM query requests waiting for responses from inject.js
const pendingDomRequests = new Map<number, (result: DomQueryResult) => void>()
let domRequestId = 0

// Periodic cleanup timer (Issue #2 fix)
const CLEANUP_INTERVAL_MS = 30000 // 30 seconds
let cleanupTimer: ReturnType<typeof setInterval> | null = null

// Track request timestamps for stale detection
const requestTimestamps = new Map<number, number>()

/**
 * Get request timestamps for stale detection (Issue #2 fix).
 * Returns array of [requestId, timestamp] pairs for cleanup.
 */
function getRequestTimestamps(): [number, number][] {
  const timestamps: [number, number][] = []
  for (const [id, timestamp] of requestTimestamps) {
    timestamps.push([id, timestamp])
  }
  return timestamps
}

/**
 * Clear all pending request Maps on page unload (Issue 2 fix).
 * Prevents memory leaks and stale request accumulation across navigations.
 */
export function clearPendingRequests(): void {
  pendingHighlightRequests.clear()
  pendingExecuteRequests.clear()
  pendingA11yRequests.clear()
  pendingDomRequests.clear()
  requestTimestamps.clear()
}

/**
 * Perform periodic cleanup of stale requests (Issue #2 fix).
 * Removes requests older than 60 seconds as a fallback when pagehide/beforeunload don't fire.
 */
function performPeriodicCleanup(): void {
  const now = Date.now()
  const staleThreshold = 60000 // 60 seconds

  for (const [id, timestamp] of getRequestTimestamps()) {
    if (now - timestamp > staleThreshold) {
      // Remove stale request from all maps
      pendingHighlightRequests.delete(id)
      pendingExecuteRequests.delete(id)
      pendingA11yRequests.delete(id)
      pendingDomRequests.delete(id)
      requestTimestamps.delete(id)
    }
  }
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
export function registerHighlightRequest(resolve: (result: HighlightResponse) => void): number {
  const requestId = ++highlightRequestId
  pendingHighlightRequests.set(requestId, resolve)
  return requestId
}

/**
 * Resolve a highlight request
 */
export function resolveHighlightRequest(requestId: number, result: HighlightResponse): void {
  const resolve = pendingHighlightRequests.get(requestId)
  if (resolve) {
    pendingHighlightRequests.delete(requestId)
    resolve(result)
  }
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
  pendingHighlightRequests.delete(requestId)
}

/**
 * Get the next execute request ID and register a resolver
 */
export function registerExecuteRequest(resolve: (result: ExecuteJsResult) => void): number {
  const requestId = ++executeRequestId
  pendingExecuteRequests.set(requestId, resolve)
  return requestId
}

/**
 * Resolve an execute request
 */
export function resolveExecuteRequest(requestId: number, result: ExecuteJsResult): void {
  const resolve = pendingExecuteRequests.get(requestId)
  if (resolve) {
    pendingExecuteRequests.delete(requestId)
    resolve(result)
  }
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
  pendingExecuteRequests.delete(requestId)
}

/**
 * Get the next a11y request ID and register a resolver
 */
export function registerA11yRequest(resolve: (result: A11yAuditResult) => void): number {
  const requestId = ++a11yRequestId
  pendingA11yRequests.set(requestId, resolve)
  return requestId
}

/**
 * Resolve an a11y request
 */
export function resolveA11yRequest(requestId: number, result: A11yAuditResult): void {
  const resolve = pendingA11yRequests.get(requestId)
  if (resolve) {
    pendingA11yRequests.delete(requestId)
    resolve(result)
  }
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
  pendingA11yRequests.delete(requestId)
}

/**
 * Get the next DOM request ID and register a resolver
 */
export function registerDomRequest(resolve: (result: DomQueryResult) => void): number {
  const requestId = ++domRequestId
  pendingDomRequests.set(requestId, resolve)
  return requestId
}

/**
 * Resolve a DOM request
 */
export function resolveDomRequest(requestId: number, result: DomQueryResult): void {
  const resolve = pendingDomRequests.get(requestId)
  if (resolve) {
    pendingDomRequests.delete(requestId)
    resolve(result)
  }
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
  pendingDomRequests.delete(requestId)
}

/**
 * Cleanup periodic timer (Issue #2 fix).
 * Should be called when content script is shutting down.
 */
export function cleanupRequestTracking(): void {
  if (cleanupTimer) {
    clearInterval(cleanupTimer)
    cleanupTimer = null
  }
  clearPendingRequests()
}

/**
 * Initialize request tracking (register cleanup handlers)
 */
export function initRequestTracking(): void {
  // Register cleanup handlers for page unload/navigation (Issue 2 fix)
  // Using 'pagehide' (modern, fires on both close and navigation) + 'beforeunload' (legacy fallback)
  window.addEventListener('pagehide', clearPendingRequests)
  window.addEventListener('beforeunload', clearPendingRequests)

  // Start periodic cleanup timer (Issue #2 fix)
  // Provides fallback when pagehide/beforeunload don't fire (e.g., page crash)
  cleanupTimer = setInterval(performPeriodicCleanup, CLEANUP_INTERVAL_MS)
}
