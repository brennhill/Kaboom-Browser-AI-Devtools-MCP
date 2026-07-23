/**
 * Purpose: Unified sync client that replaces multiple polling loops with a single /sync endpoint, handling settings, commands, and extension logs.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import { errorMessage } from '../lib/error-utils.js';
import { fetchWithTimeout } from '../lib/timeout-utils.js';
import { buildDaemonJSONRequestInit } from '../lib/daemon-http.js';
import { beacon } from '../lib/telemetry-beacon.js';
import { drainUIFeatures, restoreUIFeatures } from './ui-usage-tracker.js';
// =============================================================================
// SERVER INSTALL ID — single source of truth for all analytics
// =============================================================================
const INSTALL_ID_STORAGE_KEY = 'kaboom_server_install_id';
/** Server's install ID, populated on first successful sync or from storage. */
let serverInstallId;
/** Returns the server's install ID, or undefined if not yet received. */
export function getServerInstallId() {
    return serverInstallId;
}
/** Load persisted install ID from storage (call once on startup). */
export async function loadServerInstallId() {
    if (typeof chrome === 'undefined' || !chrome.storage?.local)
        return;
    try {
        const result = await chrome.storage.local.get(INSTALL_ID_STORAGE_KEY);
        const stored = result[INSTALL_ID_STORAGE_KEY];
        if (stored && !serverInstallId) {
            serverInstallId = stored;
        }
    }
    catch { /* best-effort */ }
}
function persistInstallId(id) {
    if (typeof chrome === 'undefined' || !chrome.storage?.local)
        return;
    chrome.storage.local.set({ [INSTALL_ID_STORAGE_KEY]: id }).catch(() => { });
}
// =============================================================================
// CONSTANTS
// =============================================================================
const BASE_POLL_MS = 1000;
const DEFAULT_COMMAND_TIMEOUT_MS = 65000;
// =============================================================================
// SYNC CLIENT CLASS
// =============================================================================
export class SyncClient {
    serverUrl;
    extSessionId;
    callbacks;
    state;
    intervalId = null;
    running = false;
    syncing = false;
    flushRequested = false;
    pendingResults = [];
    inProgressById = new Map();
    processedCommandSignatures = new Set();
    extensionVersion;
    constructor(serverUrl, extSessionId, callbacks, extensionVersion = '') {
        this.serverUrl = serverUrl;
        this.extSessionId = extSessionId;
        this.callbacks = callbacks;
        this.extensionVersion = extensionVersion;
        this.state = {
            connected: false,
            lastSyncAt: 0,
            consecutiveFailures: 0,
            lastCommandAck: null
        };
    }
    /** Get current sync state */
    getState() {
        return { ...this.state };
    }
    /** Check if connected */
    isConnected() {
        return this.state.connected;
    }
    /** Start the sync loop */
    start() {
        if (this.running)
            return;
        this.running = true;
        this.log('Starting sync client');
        this.scheduleNextSync(0); // Sync immediately
    }
    /** Stop the sync loop */
    stop() {
        this.running = false;
        if (this.intervalId) {
            clearTimeout(this.intervalId);
            this.intervalId = null;
        }
        this.log('Stopped sync client');
    }
    /** Queue a command result to send on next sync, then flush immediately */
    queueCommandResult(result) {
        this.clearInProgressById(result.id);
        this.pendingResults.push(result);
        // Cap queue size to prevent memory leak if server is unreachable
        const MAX_PENDING_RESULTS = 200;
        if (this.pendingResults.length > MAX_PENDING_RESULTS) {
            this.pendingResults = this.pendingResults.slice(-MAX_PENDING_RESULTS);
        }
        this.flush();
    }
    /** Trigger an immediate sync to deliver queued results with minimal latency */
    flush() {
        if (!this.running)
            return;
        if (this.syncing) {
            // Sync in progress — schedule another immediately after it finishes
            this.flushRequested = true;
            return;
        }
        if (this.intervalId) {
            clearTimeout(this.intervalId);
        }
        this.scheduleNextSync(0);
    }
    /** Reset connection state (e.g., when user toggles pilot/tracking) */
    resetConnection() {
        this.state.consecutiveFailures = 0;
        this.log('Connection state reset');
        // Trigger immediate sync if running
        if (this.running && this.intervalId) {
            clearTimeout(this.intervalId);
            this.scheduleNextSync(0);
        }
    }
    /** Update server URL */
    setServerUrl(url) {
        this.serverUrl = url;
    }
    /** Optional progress updates for long-running commands */
    updateCommandProgress(commandId, progressPct, status = 'running') {
        const current = this.inProgressById.get(commandId);
        if (!current)
            return;
        const next = {
            ...current,
            status,
            updated_at: new Date().toISOString()
        };
        if (typeof progressPct === 'number' && Number.isFinite(progressPct)) {
            next.progress_pct = clampPercent(progressPct);
        }
        this.inProgressById.set(commandId, next);
    }
    // =============================================================================
    // PRIVATE METHODS
    // =============================================================================
    scheduleNextSync(delayMs) {
        if (!this.running)
            return;
        this.intervalId = setTimeout(() => this.doSync(), delayMs);
    }
    async doSync() {
        if (!this.running)
            return;
        this.syncing = true;
        this.flushRequested = false;
        let features;
        try {
            // Build request
            const settings = await this.callbacks.getSettings();
            const logs = this.callbacks.getExtensionLogs();
            const request = {
                ext_session_id: this.extSessionId,
                extension_version: this.extensionVersion || undefined,
                settings,
                in_progress: this.getInProgressSnapshot()
            };
            // Include logs if any
            if (logs.length > 0) {
                request.extension_logs = logs;
            }
            // Include pending command results
            const resultsSentCount = this.pendingResults.length;
            if (resultsSentCount > 0) {
                request.command_results = this.pendingResults.slice(0, resultsSentCount);
            }
            // Include last command ack
            if (this.state.lastCommandAck) {
                request.last_command_ack = this.state.lastCommandAck;
            }
            // Include UI-originated feature usage (screenshot button, draw mode, etc.)
            features = drainUIFeatures();
            if (features) {
                request.features_used = features;
            }
            // Make request with timeout to prevent hanging forever (8s: server holds up to 5s + margin)
            const response = await fetchWithTimeout(`${this.serverUrl}/sync`, buildDaemonJSONRequestInit(request, {
                extensionVersion: this.extensionVersion || undefined
            }), 8000);
            if (!response.ok) {
                throw new Error(`Sync request failed: HTTP ${response.status} ${response.statusText} from ${this.serverUrl}/sync`);
            }
            const data = await response.json();
            // Log sync cycle summary
            this.log('Sync OK', {
                commands: data.commands?.length || 0,
                resultsSent: request.command_results?.length || 0,
                logsSent: request.extension_logs?.length || 0,
                nextPollMs: data.next_poll_ms
            });
            // Success - update state
            this.onSuccess();
            // Store server install ID for use as the single analytics identifier.
            if (data.install_id && data.install_id !== serverInstallId) {
                serverInstallId = data.install_id;
                persistInstallId(data.install_id);
            }
            // Check for version mismatch (compare major.minor only, ignore patch)
            if (data.server_version && this.extensionVersion && this.callbacks.onVersionMismatch) {
                const serverMajorMinor = data.server_version.split('.').slice(0, 2).join('.');
                const extensionMajorMinor = this.extensionVersion.split('.').slice(0, 2).join('.');
                if (serverMajorMinor !== extensionMajorMinor) {
                    beacon('extension_version_mismatch', { ext: extensionMajorMinor, srv: serverMajorMinor });
                    this.callbacks.onVersionMismatch(this.extensionVersion, data.server_version);
                }
            }
            // Clear sent logs and results
            if (logs.length > 0) {
                this.callbacks.clearExtensionLogs();
            }
            if (resultsSentCount > 0) {
                this.pendingResults = this.pendingResults.slice(resultsSentCount);
            }
            // Dispatch commands without blocking the heartbeat loop.
            // Command completion is returned asynchronously via queueCommandResult().
            if (data.commands && data.commands.length > 0) {
                this.log('Received commands', { count: data.commands.length, ids: data.commands.map((c) => c.id) });
                for (const command of data.commands) {
                    const signature = this.getCommandSignature(command);
                    if (command.id && this.processedCommandSignatures.has(signature)) {
                        this.log('Skipping already processed command', {
                            id: command.id,
                            correlation_id: command.correlation_id,
                            type: command.type
                        });
                        continue;
                    }
                    // Dedup on RECEIPT — prevents re-execution if server re-sends before ack
                    if (command.id) {
                        this.processedCommandSignatures.add(signature);
                        const MAX_PROCESSED_COMMANDS = 1000;
                        if (this.processedCommandSignatures.size > MAX_PROCESSED_COMMANDS) {
                            const oldest = this.processedCommandSignatures.values().next().value;
                            if (oldest !== undefined) {
                                this.processedCommandSignatures.delete(oldest);
                            }
                        }
                    }
                    this.log('Dispatching command', {
                        id: command.id,
                        type: command.type,
                        correlation_id: command.correlation_id
                    });
                    void this.dispatchCommand(command);
                }
            }
            // Handle capture overrides
            if (data.capture_overrides && this.callbacks.onCaptureOverrides) {
                this.callbacks.onCaptureOverrides(data.capture_overrides);
            }
            // Schedule next sync — flush immediately if results were queued during this sync
            this.syncing = false;
            if (this.flushRequested) {
                this.flushRequested = false;
                this.scheduleNextSync(0);
            }
            else {
                const nextPollMs = data.next_poll_ms || BASE_POLL_MS;
                this.scheduleNextSync(nextPollMs);
            }
        }
        catch (err) {
            // Failure - just retry after 1 second (no exponential backoff needed)
            this.syncing = false;
            this.flushRequested = false;
            this.onFailure();
            // Re-merge drained UI features so they aren't lost on failed sync.
            if (features) {
                restoreUIFeatures(features);
            }
            this.log('Sync failed, retrying', { error: errorMessage(err) });
            this.scheduleNextSync(BASE_POLL_MS);
        }
    }
    onSuccess() {
        const wasDisconnected = !this.state.connected;
        this.state.connected = true;
        this.state.lastSyncAt = Date.now();
        this.state.consecutiveFailures = 0;
        if (wasDisconnected) {
            this.log('Connected');
            this.callbacks.onConnectionChange(true);
        }
    }
    onFailure() {
        this.state.consecutiveFailures++;
        // Beacon at logarithmic intervals (10, 100, 1000) — not every failure
        if (this.state.consecutiveFailures === 10 ||
            this.state.consecutiveFailures === 100 ||
            this.state.consecutiveFailures === 1000) {
            beacon('sync_connect_failed', { failures: String(this.state.consecutiveFailures) });
        }
        // Require 2+ consecutive failures before marking disconnected
        // to prevent a single transient timeout from flipping connection state
        if (this.state.consecutiveFailures >= 2 && this.state.connected) {
            this.state.connected = false;
            this.log('Disconnected');
            this.callbacks.onConnectionChange(false);
        }
    }
    log(message, data) {
        if (this.callbacks.debugLog) {
            this.callbacks.debugLog('sync', message, data);
        }
        else {
            console.log(`[SyncClient] ${message}`, data || ''); // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal sync state, not user-controlled
        }
    }
    getCommandSignature(command) {
        // Include correlation_id and type so command ID reuse after daemon restart
        // does not suppress new commands with the same queue ID.
        const id = command.id || '';
        const correlationID = command.correlation_id || '';
        const type = command.type || '';
        return `${id}::${correlationID}::${type}`;
    }
    commandTimeoutFor(command) {
        if (command.type === 'upload' && typeof this.callbacks.uploadCommandTimeoutMs === 'number') {
            return Math.max(1, this.callbacks.uploadCommandTimeoutMs);
        }
        if (typeof this.callbacks.commandTimeoutMs === 'number') {
            return Math.max(1, this.callbacks.commandTimeoutMs);
        }
        return DEFAULT_COMMAND_TIMEOUT_MS;
    }
    async dispatchCommand(command) {
        this.markInProgress(command);
        const timeoutMs = this.commandTimeoutFor(command);
        let timeoutHandle = null;
        try {
            await Promise.race([
                Promise.resolve(this.callbacks.onCommand(command)),
                new Promise((_, reject) => {
                    timeoutHandle = setTimeout(() => reject(new Error(`Command ${command.id || '(unknown)'} (${command.type || 'unknown'}) timed out after ${timeoutMs}ms`)), timeoutMs);
                })
            ]);
            this.log('Command completed OK', { id: command.id });
        }
        catch (err) {
            const message = errorMessage(err, 'Command execution failed');
            this.log('Command execution FAILED', { id: command.id, error: message });
            this.queueCommandResult({
                id: command.id,
                status: 'error',
                error: message
            });
        }
        finally {
            if (timeoutHandle) {
                clearTimeout(timeoutHandle);
            }
            this.clearInProgressById(command.id);
            // Ack after dispatch completes (success or failure) — not on bare receipt
            if (command.id) {
                this.state.lastCommandAck = command.id;
            }
        }
    }
    markInProgress(command) {
        const now = new Date().toISOString();
        const current = this.inProgressById.get(command.id);
        this.inProgressById.set(command.id, {
            id: command.id,
            correlation_id: command.correlation_id,
            type: command.type,
            status: current?.status || 'running',
            progress_pct: current?.progress_pct,
            started_at: current?.started_at || now,
            updated_at: now
        });
    }
    clearInProgressById(id) {
        if (!id)
            return;
        this.inProgressById.delete(id);
    }
    getInProgressSnapshot() {
        if (this.inProgressById.size === 0) {
            return [];
        }
        return Array.from(this.inProgressById.values()).map((entry) => ({
            ...entry,
            updated_at: entry.updated_at || new Date().toISOString()
        }));
    }
}
function clampPercent(value) {
    if (value < 0)
        return 0;
    if (value > 100)
        return 100;
    return Math.round(value * 100) / 100;
}
// =============================================================================
// FACTORY FUNCTION
// =============================================================================
/**
 * Create a sync client instance
 */
export function createSyncClient(serverUrl, extSessionId, callbacks, extensionVersion = '') {
    return new SyncClient(serverUrl, extSessionId, callbacks, extensionVersion);
}
//# sourceMappingURL=sync-client.js.map