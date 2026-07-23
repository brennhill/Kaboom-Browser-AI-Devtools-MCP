/**
 * Purpose: HTTP functions for sending telemetry data (logs, WebSocket events, network bodies, actions, performance) to the Kaboom MCP server.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import { KABOOM_LOG_PREFIX } from '../lib/brand.js';
import { getExtensionVersion } from './version-check.js';
import { errorMessage } from '../lib/error-utils.js';
import { buildDaemonHeaders } from '../lib/daemon-http.js';
function buildHeartbeatStatusError(capture) {
    if (!capture || typeof capture.extension_connected !== 'boolean') {
        return 'Server reachable, but extension heartbeat status is unavailable. Update the server and extension, then reopen the popup.';
    }
    if (capture.extension_last_seen && capture.extension_last_seen.trim().length > 0) {
        return `Server reachable, but extension heartbeat is stale (last seen ${capture.extension_last_seen}). Reopen the KaBOOM! popup and click "Track This Tab".`;
    }
    return 'Server reachable, but extension heartbeat is missing. Open the KaBOOM! popup and click "Track This Tab".';
}
/**
 * Get standard headers for API requests including version header
 */
export function getRequestHeaders(additionalHeaders = {}) {
    return buildDaemonHeaders({
        extensionVersion: getExtensionVersion(),
        additionalHeaders
    });
}
/**
 * Generic telemetry batch sender. All telemetry POST endpoints follow the same
 * pattern: log count, POST JSON, check response.ok, log acceptance.
 * Includes AbortSignal.timeout(10000) to prevent hanging requests.
 */
async function sendTelemetryBatch(serverUrl, endpoint, payloadKey, items, label, debugLogFn) {
    if (debugLogFn)
        debugLogFn('connection', `Sending ${items.length} ${label} to server`);
    const response = await fetch(`${serverUrl}${endpoint}`, {
        method: 'POST',
        headers: getRequestHeaders(),
        body: JSON.stringify({ [payloadKey]: items }),
        signal: AbortSignal.timeout(10000)
    });
    if (!response.ok) {
        const error = `Server error (${label}): ${response.status} ${response.statusText}`;
        if (debugLogFn)
            debugLogFn('error', error);
        throw new Error(error);
    }
    if (debugLogFn)
        debugLogFn('connection', `Server accepted ${items.length} ${label}`);
    return response;
}
/**
 * Send log entries to the server
 */
export async function sendLogsToServer(serverUrl, entries, debugLogFn) {
    const response = await sendTelemetryBatch(serverUrl, '/logs', 'entries', entries, 'entries', debugLogFn);
    const result = (await response.json());
    if (debugLogFn)
        debugLogFn('connection', `Server accepted entries, total: ${result.entries}`);
    return result;
}
/**
 * Send WebSocket events to the server
 */
export async function sendWSEventsToServer(serverUrl, events, debugLogFn) {
    await sendTelemetryBatch(serverUrl, '/websocket-events', 'events', events, 'WS events', debugLogFn);
}
/**
 * Send network bodies to the server
 */
export async function sendNetworkBodiesToServer(serverUrl, bodies, debugLogFn) {
    await sendTelemetryBatch(serverUrl, '/network-bodies', 'bodies', bodies, 'network bodies', debugLogFn);
}
/**
 * Send enhanced actions to server
 */
export async function sendEnhancedActionsToServer(serverUrl, actions, debugLogFn) {
    await sendTelemetryBatch(serverUrl, '/enhanced-actions', 'actions', actions, 'enhanced actions', debugLogFn);
}
/**
 * Send performance snapshots to server
 */
export async function sendPerformanceSnapshotsToServer(serverUrl, snapshots, debugLogFn) {
    await sendTelemetryBatch(serverUrl, '/performance-snapshots', 'snapshots', snapshots, 'performance snapshots', debugLogFn);
}
/**
 * Check server health
 */
export async function checkServerHealth(serverUrl) {
    try {
        const response = await fetch(`${serverUrl}/health`);
        if (!response.ok) {
            return { connected: false, error: `HTTP ${response.status}` };
        }
        let data;
        try {
            data = (await response.json());
        }
        catch {
            return {
                connected: false,
                error: 'Server returned invalid response - check Server URL in options'
            };
        }
        if (data.capture?.extension_connected !== true) {
            return {
                ...data,
                connected: false,
                error: buildHeartbeatStatusError(data.capture)
            };
        }
        return {
            ...data,
            connected: true
        };
    }
    catch (error) {
        return {
            connected: false,
            error: errorMessage(error)
        };
    }
}
/**
 * Update extension badge.
 * Uses Promise.all to ensure both text and color are applied atomically
 * before the MV3 service worker can be suspended.
 */
export function updateBadge(status) {
    if (typeof chrome === 'undefined' || !chrome.action)
        return;
    if (status.connected) {
        const errorCount = status.errorCount || 0;
        Promise.all([
            chrome.action.setBadgeText({
                text: errorCount === 0 ? '' : errorCount > 99 ? '99+' : String(errorCount)
            }),
            chrome.action.setBadgeBackgroundColor({
                color: '#3fb950'
            })
        ]).catch(() => {
            /* badge update failed — SW may be shutting down */
        });
    }
    else {
        Promise.all([
            chrome.action.setBadgeText({ text: '!' }),
            chrome.action.setBadgeBackgroundColor({
                color: '#f85149'
            })
        ]).catch(() => {
            /* badge update failed — SW may be shutting down */
        });
    }
}
/**
 * Send status ping to server
 */
export async function sendStatusPing(serverUrl, statusMessage, diagnosticLogFn) {
    try {
        const response = await fetch(`${serverUrl}/api/extension-status`, {
            method: 'POST',
            headers: getRequestHeaders(),
            body: JSON.stringify(statusMessage)
        });
        if (!response.ok) {
            console.error(`${KABOOM_LOG_PREFIX} Failed to send status ping: HTTP ${response.status}`, { type: statusMessage.type }); // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal server state, not user-controlled format string
        }
    }
    catch (err) {
        console.error(`${KABOOM_LOG_PREFIX} Error sending status ping:`, { type: statusMessage.type, error: errorMessage(err) });
        if (diagnosticLogFn) {
            diagnosticLogFn(`${KABOOM_LOG_PREFIX} Status ping error: ${errorMessage(err)}`);
        }
    }
}
//# sourceMappingURL=server.js.map