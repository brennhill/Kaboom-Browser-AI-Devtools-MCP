/**
 * Purpose: Manages sync client instance lifecycle (start/stop/reset) and wires dependencies to avoid circular imports with index.ts.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import { createSyncClient } from './sync-client.js';
import { getLastCSPStatus } from './browser-actions.js';
import { DebugCategory } from './debug.js';
import { updateBadge } from './communication.js';
import { isQueryProcessing, addProcessingQuery, removeProcessingQuery } from './state-manager.js';
import { getTrackedTabInfo } from './event-listeners.js';
import { handlePendingQuery as handlePendingQueryImpl } from './pending-queries.js';
import { errorMessage } from '../lib/error-utils.js';
// =============================================================================
// MODULE STATE
// =============================================================================
/** Sync client instance (initialized lazily) */
let syncClient = null;
// =============================================================================
// HELPERS
// =============================================================================
/**
 * Get extension version safely
 */
function getExtensionVersion() {
    if (typeof chrome !== 'undefined' && chrome.runtime?.getManifest) {
        return chrome.runtime.getManifest().version;
    }
    return '';
}
// =============================================================================
// SYNC CLIENT LIFECYCLE
// =============================================================================
/**
 * Start the sync client (unified /sync endpoint).
 * Safe to call multiple times — will no-op if already running.
 */
// #lizard forgives
export function startSyncClient(deps) {
    if (syncClient) {
        // Already running, nothing to do
        return;
    }
    syncClient = createSyncClient(deps.getServerUrl(), deps.getExtSessionId(), {
        // Handle commands from server
        // #lizard forgives
        onCommand: async (command) => {
            deps.debugLog(DebugCategory.CONNECTION, 'Processing sync command', { type: command.type, id: command.id });
            if (isQueryProcessing(command.id)) {
                deps.debugLog(DebugCategory.CONNECTION, 'Skipping already processing command', { id: command.id });
                return;
            }
            addProcessingQuery(command.id);
            try {
                await handlePendingQueryImpl(command, syncClient);
            }
            catch (err) {
                deps.debugLog(DebugCategory.CONNECTION, 'Error processing sync command', {
                    type: command.type,
                    error: errorMessage(err)
                });
            }
            finally {
                removeProcessingQuery(command.id);
            }
        },
        // Handle connection state changes
        onConnectionChange: (connected) => {
            deps.setConnectionStatus({ connected });
            updateBadge(deps.getConnectionStatus());
            deps.debugLog(DebugCategory.CONNECTION, connected ? 'Sync connected' : 'Sync disconnected');
            // Notify popup
            if (typeof chrome !== 'undefined' && chrome.runtime) {
                chrome.runtime
                    .sendMessage({
                    type: 'status_update',
                    status: { ...deps.getConnectionStatus(), aiControlled: deps.getAiControlled() }
                })
                    .catch(() => {
                    /* popup may not be open */
                });
            }
        },
        // Handle capture overrides from server
        onCaptureOverrides: (overrides) => {
            deps.applyCaptureOverrides(overrides);
            if (typeof chrome !== 'undefined' && chrome.runtime) {
                chrome.runtime
                    .sendMessage({
                    type: 'status_update',
                    status: { ...deps.getConnectionStatus(), aiControlled: deps.getAiControlled() }
                })
                    .catch(() => {
                    /* popup may not be open */
                });
            }
        },
        // Handle version mismatch between extension and server
        onVersionMismatch: (extensionVersion, serverVersion) => {
            deps.debugLog(DebugCategory.CONNECTION, 'Version mismatch detected', { extensionVersion, serverVersion });
            // Update connection status with version info
            deps.setConnectionStatus({
                serverVersion,
                extensionVersion,
                versionMismatch: extensionVersion !== serverVersion
            });
            // Notify popup about version mismatch
            if (typeof chrome !== 'undefined' && chrome.runtime) {
                chrome.runtime
                    .sendMessage({
                    type: 'version_mismatch',
                    extensionVersion,
                    serverVersion
                })
                    .catch(() => {
                    /* popup may not be open */
                });
            }
        },
        // Get current settings to send to server
        getSettings: async () => {
            const trackingInfo = await getTrackedTabInfo();
            const csp = getLastCSPStatus();
            return {
                pilot_enabled: deps.getAiWebPilotEnabledCache(),
                tracking_enabled: !!trackingInfo.trackedTabId,
                tracked_tab_id: trackingInfo.trackedTabId || 0,
                tracked_tab_url: trackingInfo.trackedTabUrl || '',
                tracked_tab_title: trackingInfo.trackedTabTitle || '',
                tab_status: trackingInfo.tabStatus || undefined,
                tracked_tab_active: trackingInfo.trackedTabActive ?? undefined,
                capture_logs: true,
                capture_network: true,
                capture_websocket: true,
                capture_actions: true,
                csp_restricted: csp.csp_restricted,
                csp_level: csp.csp_level
            };
        },
        // Get pending extension logs
        getExtensionLogs: () => {
            return deps.getExtensionLogQueue().map((log) => ({
                timestamp: log.timestamp,
                level: log.level,
                message: log.message,
                source: log.source,
                category: log.category,
                data: log.data
            }));
        },
        // Clear extension logs after sending
        clearExtensionLogs: () => {
            deps.clearExtensionLogQueue();
        },
        // Debug logging
        debugLog: (category, message, data) => {
            deps.debugLog(DebugCategory.CONNECTION, `[Sync] ${message}`, data);
        }
    }, getExtensionVersion());
    syncClient.start();
    deps.debugLog(DebugCategory.CONNECTION, 'Sync client started');
}
/**
 * Stop the sync client
 */
export function stopSyncClient(debugLog) {
    if (syncClient) {
        syncClient.stop();
        debugLog(DebugCategory.CONNECTION, 'Sync client stopped');
    }
}
/**
 * Reset sync client connection (call when user enables pilot/tracking)
 */
export function resetSyncClientConnection(debugLog) {
    if (syncClient) {
        syncClient.resetConnection();
        debugLog(DebugCategory.CONNECTION, 'Sync client connection reset');
    }
}
//# sourceMappingURL=sync-manager.js.map