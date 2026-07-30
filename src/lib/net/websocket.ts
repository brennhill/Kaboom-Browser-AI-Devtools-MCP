/**
 * Purpose: Wraps the WebSocket constructor to intercept lifecycle events and messages, delegating tracking and sampling to websocket-tracking.ts.
 * Docs: docs/features/feature/observe/index.md
 */

// websocket.ts — WebSocket constructor instrumentation and capture installation.

/**
 * @fileoverview WebSocket capture.
 * Wraps the WebSocket constructor to intercept lifecycle events and messages.
 * Delegates tracking, sampling, and schema detection to websocket-tracking.ts.
 */

import type { WebSocketCaptureMode } from '../../types/capture/websocket.js'

import {
  type WebSocketMessageData,
  type ConnectionTracker,
  getSize,
  formatPayload,
  truncateWsMessage,
  createConnectionTracker,
  setWebSocketCaptureModeInternal,
  getWebSocketCaptureModeInternal,
  resetCaptureModeForTesting
} from './websocket-tracking.js'

// =============================================================================
// MODULE STATE (instrumentation-specific)
// =============================================================================

let originalWebSocket: typeof WebSocket | null = null
let webSocketCaptureEnabled = true

// =============================================================================
// INTERNAL TYPES
// =============================================================================

/** WebSocket event payload type */
interface WsEventPayload {
  type: 'websocket'
  event: string
  id: string
  url: string
  ts: string
  code?: number
  reason?: string
  direction?: 'incoming' | 'outgoing'
  data?: string
  size?: number
  truncated?: boolean
}

/** PostMessage payload type */
interface KaboomWsMessage {
  type: 'kaboom_ws'
  payload: WsEventPayload
}

// =============================================================================
// CAPTURE EVENT HELPERS
// =============================================================================

/**
 * Copy mutable binary data before capture work crosses the microtask boundary.
 * ArrayBuffer views are normalized to the exact byte range supplied to send().
 */
function snapshotDeferredPayload(data: WebSocketMessageData | ArrayBufferView): WebSocketMessageData {
  if (data instanceof ArrayBuffer) {
    return data.slice(0)
  }
  if (ArrayBuffer.isView(data)) {
    const snapshot = new Uint8Array(data.byteLength)
    snapshot.set(new Uint8Array(data.buffer, data.byteOffset, data.byteLength))
    return snapshot.buffer
  }
  return data
}

/** Post a WebSocket lifecycle event (open/close/error) */
function postLifecycleEvent(
  event: string,
  connectionId: string,
  urlString: string,
  extra?: { code?: number; reason?: string; ts?: string }
): void {
  window.postMessage(
    {
      type: 'kaboom_ws',
      payload: {
        type: 'websocket',
        event,
        id: connectionId,
        url: urlString,
        ts: extra?.ts || new Date().toISOString(),
        ...(extra?.code !== undefined && { code: extra.code }),
        ...(extra?.reason !== undefined && { reason: extra.reason })
      }
    } as KaboomWsMessage,
    window.location.origin
  )
}

/** Post a WebSocket message event */
function postMessageEvent(
  connectionId: string,
  urlString: string,
  direction: 'incoming' | 'outgoing',
  data: WebSocketMessageData,
  tracker: ConnectionTracker
): void {
  tracker.recordSampledMessage(direction, data)
  const size = getSize(data)
  const formatted = formatPayload(data)
  const { data: truncatedData, truncated } = truncateWsMessage(formatted)

  window.postMessage(
    {
      type: 'kaboom_ws',
      payload: {
        type: 'websocket',
        event: 'message',
        id: connectionId,
        url: urlString,
        direction,
        data: truncatedData,
        size,
        truncated: truncated || undefined,
        ts: new Date().toISOString()
      }
    } as KaboomWsMessage,
    window.location.origin
  )
}

/** Attach message and send capture to a WebSocket instance */
function attachMessageCapture(
  ws: WebSocket,
  connectionId: string,
  urlString: string,
  tracker: ConnectionTracker
): void {
  ws.addEventListener('message', (event: MessageEvent<WebSocketMessageData>) => {
    if (!webSocketCaptureEnabled) return
    tracker.recordMessage('incoming', event.data)
    if (!tracker.shouldSample('incoming')) return
    const snapshot = snapshotDeferredPayload(event.data)
    queueMicrotask(() => postMessageEvent(connectionId, urlString, 'incoming', snapshot, tracker))
  })

  const originalSend = ws.send.bind(ws)
  // Match the native WebSocket.send parameter type exactly so the wrapper stays a
  // transparent pass-through. Typing `data` as the broader ArrayBufferLike pulls in
  // SharedArrayBuffer, which lib.dom's send() overloads reject (TS 6+).
  ws.send = function (data: Parameters<typeof originalSend>[0]): void {
    if (webSocketCaptureEnabled) {
      tracker.recordMessage('outgoing', data as WebSocketMessageData)
    }
    if (webSocketCaptureEnabled && tracker.shouldSample('outgoing')) {
      const snapshot = snapshotDeferredPayload(data)
      queueMicrotask(() => postMessageEvent(connectionId, urlString, 'outgoing', snapshot, tracker))
    }
    return originalSend(data)
  }
}

/** Attach lifecycle (close/error) capture to a WebSocket instance */
function attachLifecycleCapture(ws: WebSocket, connectionId: string, urlString: string): void {
  ws.addEventListener('close', (event: CloseEvent) => {
    if (!webSocketCaptureEnabled) return
    postLifecycleEvent('close', connectionId, urlString, {
      code: event.code,
      reason: event.reason
    })
  })

  ws.addEventListener('error', () => {
    if (!webSocketCaptureEnabled) return
    postLifecycleEvent('error', connectionId, urlString)
  })
}

// =============================================================================
// INSTALLATION
// =============================================================================

/**
 * Install WebSocket capture by wrapping the WebSocket constructor.
 * If the early-patch script ran first (world: "MAIN", document_start),
 * uses the saved original constructor and adopts buffered connections.
 */
export function installWebSocketCapture(): void {
  if (typeof window === 'undefined') return
  if (!window.WebSocket) return // No WebSocket support
  if (originalWebSocket) return // Already installed
  webSocketCaptureEnabled = true // Ensure capture is enabled when installing

  // Check for early-patch: use the saved original, not the early-patch wrapper
  const earlyOriginal = window.__KABOOM_ORIGINAL_WS__
  originalWebSocket = earlyOriginal || window.WebSocket

  const OriginalWS = originalWebSocket

  function KaboomWebSocket(this: WebSocket, url: string | URL, protocols?: string | string[]): WebSocket {
    const ws = new OriginalWS(url, protocols)
    const connectionId = crypto.randomUUID()
    const urlString = url.toString()
    const tracker = createConnectionTracker(connectionId, urlString)

    ws.addEventListener('open', () => {
      if (!webSocketCaptureEnabled) return
      postLifecycleEvent('open', connectionId, urlString)
    })

    attachLifecycleCapture(ws, connectionId, urlString)
    attachMessageCapture(ws, connectionId, urlString, tracker)

    return ws
  }

  // Set up prototype chain and static properties
  KaboomWebSocket.prototype = OriginalWS.prototype
  Object.defineProperty(KaboomWebSocket, 'CONNECTING', { value: OriginalWS.CONNECTING, writable: false })
  Object.defineProperty(KaboomWebSocket, 'OPEN', { value: OriginalWS.OPEN, writable: false })
  Object.defineProperty(KaboomWebSocket, 'CLOSING', { value: OriginalWS.CLOSING, writable: false })
  Object.defineProperty(KaboomWebSocket, 'CLOSED', { value: OriginalWS.CLOSED, writable: false })

  window.WebSocket = KaboomWebSocket as unknown as typeof WebSocket

  // Adopt connections buffered by the early-patch script
  adoptEarlyConnections()
}

// =============================================================================
// EARLY CONNECTION ADOPTION
// =============================================================================

/**
 * Adopt WebSocket connections buffered by the early-patch script.
 * For each still-active connection, creates a tracker and attaches event listeners
 * so ongoing messages are captured. Posts synthetic "open" events for connections
 * that opened before the inject script loaded.
 */
function adoptEarlyConnections(): void {
  const earlyConnections = window.__KABOOM_EARLY_WS__
  if (!earlyConnections || earlyConnections.length === 0) {
    // Clean up globals even if no connections
    delete window.__KABOOM_ORIGINAL_WS__
    delete window.__KABOOM_EARLY_WS__
    return
  }

  let adopted = 0

  for (const conn of earlyConnections) {
    const ws = conn.ws

    // Skip fully closed connections
    if (ws.readyState === WebSocket.CLOSED) continue

    adopted++
    const connectionId = crypto.randomUUID()
    const urlString = conn.url
    const tracker = createConnectionTracker(connectionId, urlString)

    // Post synthetic "open" event for connections that already opened
    const hasOpened = conn.events.some((e) => e.type === 'open')
    if (hasOpened && webSocketCaptureEnabled) {
      const openEvent = conn.events.find((e) => e.type === 'open')
      postLifecycleEvent('open', connectionId, urlString, {
        ts: openEvent ? new Date(openEvent.ts).toISOString() : undefined
      })
    }

    attachLifecycleCapture(ws, connectionId, urlString)
    attachMessageCapture(ws, connectionId, urlString, tracker)
  }

  if (adopted > 0) {
    console.log(`[KaBOOM!] Adopted ${adopted} early WebSocket connection(s)`)
  }

  // Clean up early-patch globals
  delete window.__KABOOM_ORIGINAL_WS__
  delete window.__KABOOM_EARLY_WS__
}

// =============================================================================
// CONFIGURATION
// =============================================================================

/**
 * Set the WebSocket capture mode
 */
export function setWebSocketCaptureMode(mode: WebSocketCaptureMode): void {
  setWebSocketCaptureModeInternal(mode)
}

/**
 * Set WebSocket capture enabled state
 */
export function setWebSocketCaptureEnabled(enabled: boolean): void {
  webSocketCaptureEnabled = enabled
}

/**
 * Get the current WebSocket capture mode
 */
export function getWebSocketCaptureMode(): WebSocketCaptureMode {
  return getWebSocketCaptureModeInternal()
}

/**
 * Uninstall WebSocket capture, restoring the original constructor
 */
export function uninstallWebSocketCapture(): void {
  if (typeof window === 'undefined') return
  if (originalWebSocket) {
    window.WebSocket = originalWebSocket
    originalWebSocket = null
  }
}

/**
 * Reset all module state for testing purposes
 * Restores original WebSocket if installed, resets capture settings to defaults.
 * Call this in beforeEach/afterEach test hooks to prevent test pollution.
 */
export function resetForTesting(): void {
  uninstallWebSocketCapture()
  webSocketCaptureEnabled = false
  resetCaptureModeForTesting()
  originalWebSocket = null
  // Clean up early-patch globals if present
  if (typeof window !== 'undefined') {
    delete window.__KABOOM_ORIGINAL_WS__
    delete window.__KABOOM_EARLY_WS__
  }
}
