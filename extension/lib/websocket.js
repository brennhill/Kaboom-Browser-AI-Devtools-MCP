/**
 * Purpose: Wraps the WebSocket constructor to intercept lifecycle events and messages, delegating tracking and sampling to websocket-tracking.ts.
 * Docs: docs/features/feature/observe/index.md
 */
import { getSize, formatPayload, truncateWsMessage, createConnectionTracker, setWebSocketCaptureModeInternal, getWebSocketCaptureModeInternal, resetCaptureModeForTesting } from './websocket-tracking.js';
// Re-export everything from tracking so existing import paths work unchanged
export { getSize, formatPayload, truncateWsMessage, createConnectionTracker } from './websocket-tracking.js';
// =============================================================================
// MODULE STATE (instrumentation-specific)
// =============================================================================
let originalWebSocket = null;
let webSocketCaptureEnabled = true;
// =============================================================================
// CAPTURE EVENT HELPERS
// =============================================================================
/** Post a WebSocket lifecycle event (open/close/error) */
function postLifecycleEvent(event, connectionId, urlString, extra) {
    window.postMessage({
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
    }, window.location.origin);
}
/** Post a WebSocket message event */
function postMessageEvent(connectionId, urlString, direction, data) {
    const size = getSize(data);
    const formatted = formatPayload(data);
    const { data: truncatedData, truncated } = truncateWsMessage(formatted);
    window.postMessage({
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
    }, window.location.origin);
}
/** Attach message and send capture to a WebSocket instance */
function attachMessageCapture(ws, connectionId, urlString, tracker) {
    ws.addEventListener('message', (event) => {
        if (!webSocketCaptureEnabled)
            return;
        tracker.recordMessage('incoming', event.data);
        if (!tracker.shouldSample('incoming'))
            return;
        postMessageEvent(connectionId, urlString, 'incoming', event.data);
    });
    const originalSend = ws.send.bind(ws);
    // Match the native WebSocket.send parameter type exactly so the wrapper stays a
    // transparent pass-through. Typing `data` as the broader ArrayBufferLike pulls in
    // SharedArrayBuffer, which lib.dom's send() overloads reject (TS 6+).
    ws.send = function (data) {
        if (webSocketCaptureEnabled) {
            tracker.recordMessage('outgoing', data);
        }
        if (webSocketCaptureEnabled && tracker.shouldSample('outgoing')) {
            postMessageEvent(connectionId, urlString, 'outgoing', data);
        }
        return originalSend(data);
    };
}
/** Attach lifecycle (close/error) capture to a WebSocket instance */
function attachLifecycleCapture(ws, connectionId, urlString) {
    ws.addEventListener('close', (event) => {
        if (!webSocketCaptureEnabled)
            return;
        postLifecycleEvent('close', connectionId, urlString, {
            code: event.code,
            reason: event.reason
        });
    });
    ws.addEventListener('error', () => {
        if (!webSocketCaptureEnabled)
            return;
        postLifecycleEvent('error', connectionId, urlString);
    });
}
// =============================================================================
// INSTALLATION
// =============================================================================
/**
 * Install WebSocket capture by wrapping the WebSocket constructor.
 * If the early-patch script ran first (world: "MAIN", document_start),
 * uses the saved original constructor and adopts buffered connections.
 */
export function installWebSocketCapture() {
    if (typeof window === 'undefined')
        return;
    if (!window.WebSocket)
        return; // No WebSocket support
    if (originalWebSocket)
        return; // Already installed
    webSocketCaptureEnabled = true; // Ensure capture is enabled when installing
    // Check for early-patch: use the saved original, not the early-patch wrapper
    const earlyOriginal = window.__KABOOM_ORIGINAL_WS__;
    originalWebSocket = earlyOriginal || window.WebSocket;
    const OriginalWS = originalWebSocket;
    function KaboomWebSocket(url, protocols) {
        const ws = new OriginalWS(url, protocols);
        const connectionId = crypto.randomUUID();
        const urlString = url.toString();
        const tracker = createConnectionTracker(connectionId, urlString);
        ws.addEventListener('open', () => {
            if (!webSocketCaptureEnabled)
                return;
            postLifecycleEvent('open', connectionId, urlString);
        });
        attachLifecycleCapture(ws, connectionId, urlString);
        attachMessageCapture(ws, connectionId, urlString, tracker);
        return ws;
    }
    // Set up prototype chain and static properties
    KaboomWebSocket.prototype = OriginalWS.prototype;
    Object.defineProperty(KaboomWebSocket, 'CONNECTING', { value: OriginalWS.CONNECTING, writable: false });
    Object.defineProperty(KaboomWebSocket, 'OPEN', { value: OriginalWS.OPEN, writable: false });
    Object.defineProperty(KaboomWebSocket, 'CLOSING', { value: OriginalWS.CLOSING, writable: false });
    Object.defineProperty(KaboomWebSocket, 'CLOSED', { value: OriginalWS.CLOSED, writable: false });
    window.WebSocket = KaboomWebSocket;
    // Adopt connections buffered by the early-patch script
    adoptEarlyConnections();
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
function adoptEarlyConnections() {
    const earlyConnections = window.__KABOOM_EARLY_WS__;
    if (!earlyConnections || earlyConnections.length === 0) {
        // Clean up globals even if no connections
        delete window.__KABOOM_ORIGINAL_WS__;
        delete window.__KABOOM_EARLY_WS__;
        return;
    }
    let adopted = 0;
    for (const conn of earlyConnections) {
        const ws = conn.ws;
        // Skip fully closed connections
        if (ws.readyState === WebSocket.CLOSED)
            continue;
        adopted++;
        const connectionId = crypto.randomUUID();
        const urlString = conn.url;
        const tracker = createConnectionTracker(connectionId, urlString);
        // Post synthetic "open" event for connections that already opened
        const hasOpened = conn.events.some((e) => e.type === 'open');
        if (hasOpened && webSocketCaptureEnabled) {
            const openEvent = conn.events.find((e) => e.type === 'open');
            postLifecycleEvent('open', connectionId, urlString, {
                ts: openEvent ? new Date(openEvent.ts).toISOString() : undefined
            });
        }
        attachLifecycleCapture(ws, connectionId, urlString);
        attachMessageCapture(ws, connectionId, urlString, tracker);
    }
    if (adopted > 0) {
        console.log(`[KaBOOM!] Adopted ${adopted} early WebSocket connection(s)`);
    }
    // Clean up early-patch globals
    delete window.__KABOOM_ORIGINAL_WS__;
    delete window.__KABOOM_EARLY_WS__;
}
// =============================================================================
// CONFIGURATION
// =============================================================================
/**
 * Set the WebSocket capture mode
 */
export function setWebSocketCaptureMode(mode) {
    setWebSocketCaptureModeInternal(mode);
}
/**
 * Set WebSocket capture enabled state
 */
export function setWebSocketCaptureEnabled(enabled) {
    webSocketCaptureEnabled = enabled;
}
/**
 * Get the current WebSocket capture mode
 */
export function getWebSocketCaptureMode() {
    return getWebSocketCaptureModeInternal();
}
/**
 * Uninstall WebSocket capture, restoring the original constructor
 */
export function uninstallWebSocketCapture() {
    if (typeof window === 'undefined')
        return;
    if (originalWebSocket) {
        window.WebSocket = originalWebSocket;
        originalWebSocket = null;
    }
}
/**
 * Reset all module state for testing purposes
 * Restores original WebSocket if installed, resets capture settings to defaults.
 * Call this in beforeEach/afterEach test hooks to prevent test pollution.
 */
export function resetForTesting() {
    uninstallWebSocketCapture();
    webSocketCaptureEnabled = false;
    resetCaptureModeForTesting();
    originalWebSocket = null;
    // Clean up early-patch globals if present
    if (typeof window !== 'undefined') {
        delete window.__KABOOM_ORIGINAL_WS__;
        delete window.__KABOOM_EARLY_WS__;
    }
}
//# sourceMappingURL=websocket.js.map