/**
 * Purpose: Registers and manages runtime observers for DOM mutations, network requests, performance entries, and WebSocket events in the page context.
 * Docs: docs/features/feature/observe/index.md
 */

/**
 * @fileoverview Observers - Observer registration and management for DOM, network,
 * performance, and WebSocket events.
 */

import { installPerformanceCapture, uninstallPerformanceCapture } from '../lib/analysis/performance.js'
import { installPerfObservers } from '../lib/analysis/perf-snapshot.js'
import { installWebSocketCapture, uninstallWebSocketCapture } from '../lib/net/websocket.js'
import {
  wrapFetchWithBodies,
  wrapXHRWithBodies,
  unwrapXHR,
  adoptEarlyBodies,
  sanitizeHeaders
} from '../lib/net/network.js'
import { installConsoleCapture, uninstallConsoleCapture } from '../lib/page/console.js'
import { safeAssignGlobal } from '../lib/page/safe-global-patch.js'
import { installExceptionCapture, uninstallExceptionCapture } from '../lib/page/exceptions.js'
import {
  installActionCapture,
  uninstallActionCapture,
  installNavigationCapture,
  uninstallNavigationCapture
} from '../lib/page/actions.js'
import { installTransientCapture, uninstallTransientCapture } from '../lib/page/transient-capture.js'
import { postLog } from '../lib/page/bridge.js'
import { MAX_RESPONSE_LENGTH, MEMORY_SOFT_LIMIT_MB, MEMORY_HARD_LIMIT_MB } from '../lib/constants.js'
import { errorMessage } from '../lib/error-utils.js'

// Store original fetch for restoration
let originalFetch: typeof fetch | null = null

// Interception deferral state (Phase 1/Phase 2 split)
let deferralEnabled = true
let phase2Installed = false
let injectionTimestamp = 0
let phase2Timestamp = 0

/**
 * Network error log payload
 */
interface NetworkErrorLog {
  level: 'error'
  type: 'network'
  method: string
  url: string
  status?: number
  statusText?: string
  duration: number
  response?: string
  error?: string
  headers?: Record<string, string>
  [key: string]: unknown
}

function resolveFetchUrl(input: RequestInfo | URL): string {
  return typeof input === 'string' ? input : (input as Request).url
}

function resolveFetchMethod(input: RequestInfo | URL, init?: RequestInit): string {
  return init?.method || (typeof input === 'object' && 'method' in input ? (input as Request).method : 'GET') || 'GET'
}

function resolveRawHeaders(input: RequestInfo | URL, init?: RequestInit): HeadersInit | null {
  return init?.headers || (typeof input === 'object' && 'headers' in input ? (input as Request).headers : null)
}

async function readErrorResponseBody(response: Response): Promise<string> {
  try {
    const cloned = response.clone()
    const body = await cloned.text()
    if (body.length > MAX_RESPONSE_LENGTH) {
      return body.slice(0, MAX_RESPONSE_LENGTH) + '... [truncated]'
    }
    return body
  } catch {
    // EXPECTED_ABSENCE: an error response body is optional context for a
    // failure we already report; one-shot bodies normally cannot re-read, and
    // logging it would misleadingly report the already-surfaced error twice.
    return '[Could not read response]'
  }
}

function optionalHeaders(safeHeaders: Record<string, string>): Record<string, Record<string, string>> {
  return Object.keys(safeHeaders).length > 0 ? { headers: safeHeaders } : {}
}

/**
 * Wrap fetch to capture network errors
 */
export function wrapFetch(originalFetchFn: typeof fetch): typeof fetch {
  return async function (input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
    const startTime = Date.now()
    const url = resolveFetchUrl(input)
    const method = resolveFetchMethod(input, init)

    try {
      const response = await originalFetchFn(input, init)
      const duration = Date.now() - startTime

      // Capture errors (4xx, 5xx)
      if (!response.ok) {
        const responseBody = await readErrorResponseBody(response)

        // Filter sensitive headers (check both init.headers and Request object headers)
        const safeHeaders = sanitizeHeaders(resolveRawHeaders(input, init))

        const logPayload: NetworkErrorLog = {
          level: 'error',
          type: 'network',
          method: method.toUpperCase(),
          url,
          status: response.status,
          statusText: response.statusText,
          duration,
          response: responseBody,
          ...optionalHeaders(safeHeaders)
        }

        postLog(logPayload)
      }

      return response
    } catch (error) {
      const duration = Date.now() - startTime

      // Filter sensitive headers for the error path
      const safeHeaders = sanitizeHeaders(resolveRawHeaders(input, init))

      const logPayload: NetworkErrorLog = {
        level: 'error',
        type: 'network',
        method: method.toUpperCase(),
        url,
        error: errorMessage(error),
        duration,
        ...optionalHeaders(safeHeaders)
      }

      postLog(logPayload)

      throw error
    }
  }
}

/**
 * Install fetch capture.
 * Uses wrapFetchWithBodies to capture request/response bodies for all requests,
 * then wraps that with wrapFetch to also capture error details for 4xx/5xx responses.
 * If the early-patch script ran first, uses the saved original fetch (not the early wrapper).
 */
export function installFetchCapture(): void {
  // Check for early-patch: use the saved original, not the early-patch wrapper
  const earlyOriginal = window.__KABOOM_ORIGINAL_FETCH__
  originalFetch = earlyOriginal || window.fetch
  // Layer 1: wrapFetchWithBodies captures request/response bodies for ALL requests
  // Layer 2: wrapFetch captures detailed error logging for 4xx/5xx responses
  // Use unknown intermediate cast to handle TypeScript's strict fetch overload types
  // This is necessary because the DOM lib defines fetch with multiple overloads
  // that TypeScript cannot reconcile with our simpler function signature
  const wrappedWithBodies = wrapFetchWithBodies(originalFetch as unknown as Parameters<typeof wrapFetchWithBodies>[0])
  // Guarded: pages that define fetch as read-only made this throw, and the
  // uncaught error aborted the rest of observer installation.
  if (!safeAssignGlobal(window, 'fetch', wrapFetch(wrappedWithBodies as unknown as typeof window.fetch))) {
    console.warn('[KaBOOM!] fetch is read-only on this page; network capture via fetch is unavailable.')
  }
}

/**
 * Install XHR body capture (wraps XMLHttpRequest.prototype.open/send)
 */
export function installXHRCapture(): void {
  wrapXHRWithBodies()
}

/**
 * Uninstall XHR body capture
 */
export function uninstallXHRCapture(): void {
  unwrapXHR()
}

/**
 * Uninstall fetch capture
 */
export function uninstallFetchCapture(): void {
  if (originalFetch) {
    safeAssignGlobal(window, 'fetch', originalFetch)
    originalFetch = null
  }
}

/**
 * Install all capture hooks
 */
export function install(): void {
  installConsoleCapture()
  installFetchCapture()
  installXHRCapture()
  installExceptionCapture()
  installActionCapture()
  installNavigationCapture()
  installWebSocketCapture()
  installPerformanceCapture()
  installTransientCapture()
}

/**
 * Uninstall all capture hooks
 */
export function uninstall(): void {
  uninstallConsoleCapture()
  uninstallFetchCapture()
  uninstallXHRCapture()
  uninstallExceptionCapture()
  uninstallActionCapture()
  uninstallNavigationCapture()
  uninstallWebSocketCapture()
  uninstallPerformanceCapture()
  uninstallTransientCapture()
}

/**
 * Check if heavy intercepts should be deferred until page load
 */
export function shouldDeferIntercepts(): boolean {
  if (typeof document === 'undefined') return false
  return document.readyState === 'loading'
}

/**
 * Memory pressure check state
 */
interface MemoryPressureState {
  memoryUsageMB: number
  networkBodiesEnabled: boolean
  wsBufferCapacity: number
  networkBufferCapacity: number
}

/**
 * Check memory pressure and adjust buffer capacities
 */
export function checkMemoryPressure(state: MemoryPressureState): MemoryPressureState {
  const result = { ...state }

  if (state.memoryUsageMB >= MEMORY_HARD_LIMIT_MB) {
    // Hard limit: disable network bodies
    result.networkBodiesEnabled = false
    result.wsBufferCapacity = Math.floor(state.wsBufferCapacity * 0.25)
    result.networkBufferCapacity = Math.floor(state.networkBufferCapacity * 0.25)
  } else if (state.memoryUsageMB >= MEMORY_SOFT_LIMIT_MB) {
    // Soft limit: reduce buffers
    result.wsBufferCapacity = Math.floor(state.wsBufferCapacity * 0.5)
    result.networkBufferCapacity = Math.floor(state.networkBufferCapacity * 0.5)
  }

  return result
}

/**
 * Phase 1 (Immediate): Lightweight, non-intercepting setup.
 */
export function installPhase1(): void {
  console.log('[KaBOOM!] Phase 1 installing (lightweight API + perf observers)')
  injectionTimestamp = performance.now()
  phase2Installed = false
  phase2Timestamp = 0

  // Start PerformanceObservers (passive observers, no prototype modification)
  installPerformanceCapture()

  // Now handle Phase 2 scheduling
  if (!deferralEnabled) {
    // Deferral disabled: install Phase 2 immediately
    installPhase2()
  } else {
    const installDeferred = (): void => {
      if (!phase2Installed) setTimeout(installPhase2, 100)
    }
    if (document.readyState === 'complete') {
      // Page already loaded, defer by 100ms
      installDeferred()
    } else {
      // Wait for load event, then defer by 100ms
      window.addEventListener('load', installDeferred, { once: true })
      // 10-second timeout fallback
      setTimeout(() => {
        if (!phase2Installed) installPhase2()
      }, 10000)
    }
  }
}

/**
 * Phase 2 (Deferred): Heavy interceptors.
 */
export function installPhase2(): void {
  // Double-injection guard
  if (phase2Installed) return

  // Environment guard
  if (typeof window === 'undefined' || typeof document === 'undefined') return

  console.log('[KaBOOM!] Phase 2 installing (heavy interceptors: console, fetch, WS, errors, actions)')
  phase2Timestamp = performance.now()
  phase2Installed = true

  // Install all heavy interceptors
  install()

  // Adopt fetch/XHR bodies buffered by the early-patch script
  adoptEarlyBodies()

  // FCP/LCP/CLS/INP/long-task observers (buffered: true replays pre-Phase-2 entries)
  installPerfObservers()
}

/**
 * Get the current deferral state for diagnostics and testing.
 */
export interface DeferralState {
  deferralEnabled: boolean
  phase2Installed: boolean
  injectionTimestamp: number
  phase2Timestamp: number
}

export function getDeferralState(): DeferralState {
  return {
    deferralEnabled,
    phase2Installed,
    injectionTimestamp,
    phase2Timestamp
  }
}

/**
 * Set whether interception deferral is enabled.
 */
export function setDeferralEnabled(enabled: boolean): void {
  deferralEnabled = enabled
}
