import { errorMessage } from '../../lib/error-utils.js';
import { pushExtensionLog } from './log-queue.js';
const defaultStatus = {
    connected: false,
    entries: 0,
    maxEntries: 1000,
    errorCount: 0,
    logFile: '',
    securityMode: 'normal',
    productionParity: true,
    insecureRewritesApplied: []
};
let connectionStatus = { ...defaultStatus };
let connectionCheckRunning = false;
const extensionConnectionListeners = new Set();
export function getConnectionStatus() {
    return Object.freeze({ ...connectionStatus });
}
/**
 * Observe the daemon sync connection edge. This is the authoritative live signal
 * that an MCP client session started or ended — owners of session-scoped browser
 * state (the driven tab group) subscribe here instead of mirroring the state in
 * storage, which goes stale when the worker dies without flushing (rule 18).
 * Returns an unsubscribe function.
 */
export function subscribeExtensionConnection(listener) {
    extensionConnectionListeners.add(listener);
    return () => {
        extensionConnectionListeners.delete(listener);
    };
}
function notifyExtensionConnection(connected) {
    for (const listener of extensionConnectionListeners) {
        try {
            listener(connected);
        }
        catch (error) {
            pushExtensionLog({
                timestamp: new Date().toISOString(),
                level: 'warn',
                message: 'Extension connection listener failed',
                source: 'background',
                category: 'lifecycle',
                data: { connected, error: errorMessage(error) }
            });
        }
    }
}
export function setConnectionStatus(patch) {
    const previouslyConnected = connectionStatus.extensionConnected;
    connectionStatus = { ...connectionStatus, ...patch };
    if (patch.extensionConnected === undefined || patch.extensionConnected === previouslyConnected)
        return;
    notifyExtensionConnection(patch.extensionConnected);
}
export function isConnectionCheckRunning() {
    return connectionCheckRunning;
}
export function setConnectionCheckRunning(running) {
    connectionCheckRunning = running;
}
export function applyConnectionOverrides(overrides) {
    const rewrites = (overrides.insecure_rewrites_applied || '')
        .split(',')
        .map((value) => value.trim())
        .filter(Boolean);
    setConnectionStatus({
        securityMode: overrides.security_mode === 'insecure_proxy' ? 'insecure_proxy' : 'normal',
        productionParity: overrides.production_parity !== 'false',
        insecureRewritesApplied: rewrites
    });
}
//# sourceMappingURL=connection-state.js.map