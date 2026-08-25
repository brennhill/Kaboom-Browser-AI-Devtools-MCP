/**
 * Purpose: Unified sync client that replaces multiple polling loops with a single /sync endpoint, handling settings, commands, and extension logs.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import { errorMessage } from '../../lib/error-utils.js';
import { fetchWithTimeout } from '../../lib/timeout-utils.js';
import { buildDaemonJSONRequestInit } from '../../lib/daemon-http.js';
import { drainUIFeatures, restoreUIFeatures } from '../ui/ui-usage-tracker.js';
import { getServerInstallId, updateServerInstallId } from './install-identity.js';
import { setConnectionGeneration } from '../runtime-state/connection-generation.js';
import { EXTENSION_COMMAND_CONTRACT_ID } from '../../types/runtime/command-contract.js';
// =============================================================================
// CONSTANTS
// =============================================================================
const BASE_POLL_MS = 1000;
const DEFAULT_COMMAND_TIMEOUT_MS = 65000;
const ACKNOWLEDGED_COMMAND_HISTORY_LIMIT = 5;
const defaultSyncRuntime = {
    now: () => Date.now(),
    random: () => Math.random(),
    setTimer: (callback, delayMs) => setTimeout(callback, delayMs),
    clearTimer: (handle) => clearTimeout(handle),
    request: (url, init, timeoutMs) => fetchWithTimeout(url, init, timeoutMs)
};
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
    connectionGeneration = 0;
    lifecycleEpoch = 0;
    runtime;
    constructor(serverUrl, extSessionId, callbacks, extensionVersion = '', runtime = defaultSyncRuntime) {
        this.serverUrl = serverUrl;
        this.extSessionId = extSessionId;
        this.callbacks = callbacks;
        this.extensionVersion = extensionVersion;
        this.runtime = runtime;
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
        this.lifecycleEpoch++;
        this.running = true;
        this.log('Starting sync client');
        this.scheduleNextSync(0); // Sync immediately
    }
    /** Stop the sync loop */
    stop() {
        this.running = false;
        this.lifecycleEpoch++;
        if (this.intervalId) {
            this.runtime.clearTimer(this.intervalId);
            this.intervalId = null;
        }
        this.log('Stopped sync client');
    }
    /** Queue a command result to send on next sync, then flush immediately */
    queueCommandResult(result) {
        const activeGeneration = this.inProgressById.get(result.id)?.connection_generation;
        this.clearInProgressById(result.id);
        this.pendingResults.push({
            ...result,
            connection_generation: result.connection_generation || activeGeneration || this.connectionGeneration || undefined
        });
        // Terminal ownership is lossless until the daemon acknowledges the batch.
        // The daemon delivers at most its bounded pending-command capacity per
        // successful sync, and an outage cannot deliver additional commands, so
        // destructive client-side truncation is both unnecessary and unsafe.
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
            this.runtime.clearTimer(this.intervalId);
        }
        this.scheduleNextSync(0);
    }
    /** Reset connection state (e.g., when user toggles pilot/tracking) */
    resetConnection() {
        this.lifecycleEpoch++;
        this.state.consecutiveFailures = 0;
        this.log('Connection state reset');
        // Trigger immediate sync if running
        if (this.running && this.syncing) {
            this.flushRequested = true;
        }
        else if (this.running && this.intervalId) {
            this.runtime.clearTimer(this.intervalId);
            this.scheduleNextSync(0);
        }
    }
    /** Update server URL */
    setServerUrl(url) {
        if (url === this.serverUrl)
            return;
        this.serverUrl = url;
        this.resetConnection();
    }
    /** Optional progress updates for long-running commands */
    updateCommandProgress(commandId, progressPct, status = 'running') {
        const current = this.inProgressById.get(commandId);
        if (!current)
            return;
        const progress = typeof progressPct === 'number' && Number.isFinite(progressPct) ? { progress_pct: clampPercent(progressPct) } : {};
        const next = {
            ...current,
            status,
            updated_at: new Date(this.runtime.now()).toISOString(),
            ...progress
        };
        this.inProgressById.set(commandId, next);
    }
    // =============================================================================
    // PRIVATE METHODS
    // =============================================================================
    scheduleNextSync(delayMs) {
        if (!this.running)
            return;
        this.intervalId = this.runtime.setTimer(() => void this.doSync(), delayMs);
    }
    async doSync() {
        if (!this.running)
            return;
        const lifecycleEpoch = this.lifecycleEpoch;
        this.syncing = true;
        this.flushRequested = false;
        let features;
        try {
            // Build request
            const settings = await this.callbacks.getSettings();
            const logs = this.callbacks.getExtensionLogs();
            const resultsSentCount = this.pendingResults.length;
            features = drainUIFeatures();
            const request = this.buildSyncRequest(settings, logs, resultsSentCount, features);
            const response = await this.fetchSyncResponse(request);
            const data = await response.json();
            if (this.rejectStaleSync(lifecycleEpoch))
                return;
            this.applySyncResponse(data, request, logs, resultsSentCount);
            // Schedule next sync — flush immediately if results were queued during this sync
            this.scheduleNextPoll(data);
        }
        catch (err) {
            this.handleSyncFailure(err, features);
        }
    }
    buildSyncRequest(settings, logs, resultsSentCount, features) {
        return {
            ext_session_id: this.extSessionId,
            connection_generation: this.connectionGeneration || undefined,
            extension_version: this.extensionVersion || undefined,
            command_contract_id: EXTENSION_COMMAND_CONTRACT_ID,
            settings,
            in_progress: this.getInProgressSnapshot(),
            ...(logs.length > 0 ? { extension_logs: logs } : {}),
            ...(resultsSentCount > 0 ? { command_results: this.pendingResults.slice(0, resultsSentCount) } : {}),
            ...(this.state.lastCommandAck ? { last_command_ack: this.state.lastCommandAck } : {}),
            ...(features ? { features_used: features } : {})
        };
    }
    async fetchSyncResponse(request) {
        // Make request with timeout to prevent hanging forever (8s: server holds up to 5s + margin)
        const response = await this.runtime.request(`${this.serverUrl}/sync`, buildDaemonJSONRequestInit(request, {
            extensionVersion: this.extensionVersion || undefined
        }), 8000);
        if (!response.ok) {
            throw new Error(`Sync request failed: HTTP ${response.status} ${response.statusText} from ${this.serverUrl}/sync`);
        }
        return response;
    }
    rejectStaleSync(lifecycleEpoch) {
        if (lifecycleEpoch === this.lifecycleEpoch)
            return false;
        this.syncing = false;
        this.log('Rejected stale connection generation', {
            correlation_id: this.extSessionId,
            bridge: 'sync_response',
            received_epoch: lifecycleEpoch,
            current_epoch: this.lifecycleEpoch
        });
        if (this.running) {
            this.flushRequested = false;
            this.scheduleNextSync(0);
        }
        return true;
    }
    applySyncResponse(data, request, logs, resultsSentCount) {
        if (!Number.isSafeInteger(data.connection_generation) || data.connection_generation <= 0) {
            throw new Error('Sync response omitted a valid connection generation');
        }
        this.connectionGeneration = data.connection_generation;
        setConnectionGeneration(data.connection_generation);
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
        if (data.install_id && data.install_id !== getServerInstallId()) {
            updateServerInstallId(data.install_id);
        }
        this.notifyVersionMismatch(data);
        // Clear sent logs and results
        if (logs.length > 0) {
            this.callbacks.acknowledgeExtensionLogs(logs.length);
        }
        if (resultsSentCount > 0) {
            this.pendingResults = this.pendingResults.slice(resultsSentCount);
        }
        this.dispatchSyncCommands(data);
        // Handle capture overrides
        if (data.capture_overrides && this.callbacks.onCaptureOverrides) {
            this.callbacks.onCaptureOverrides(data.capture_overrides);
        }
    }
    notifyVersionMismatch(data) {
        // Check for version mismatch (compare major.minor only, ignore patch)
        if (!data.server_version || !this.extensionVersion || !this.callbacks.onVersionMismatch)
            return;
        const serverMajorMinor = data.server_version.split('.').slice(0, 2).join('.');
        const extensionMajorMinor = this.extensionVersion.split('.').slice(0, 2).join('.');
        if (serverMajorMinor !== extensionMajorMinor) {
            this.callbacks.onVersionMismatch(this.extensionVersion, data.server_version);
        }
    }
    dispatchSyncCommands(data) {
        if (!data.commands || data.commands.length === 0)
            return;
        this.log('Received commands', { count: data.commands.length, ids: data.commands.map((c) => c.id) });
        for (const command of data.commands) {
            if (command.connection_generation !== this.connectionGeneration) {
                this.log('Rejected stale connection generation', {
                    correlation_id: command.correlation_id || command.id,
                    bridge: 'sync_command',
                    received_generation: command.connection_generation,
                    current_generation: this.connectionGeneration
                });
                continue;
            }
            const signature = this.getCommandSignature(command);
            if (command.id && this.inProgressById.has(command.id)) {
                this.log('Skipping command already in flight', {
                    id: command.id,
                    correlation_id: command.correlation_id,
                    type: command.type
                });
                continue;
            }
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
                // This is a short duplicate-delivery guard, not a command archive.
                // Active work is protected separately by inProgressById; retaining a
                // large completed set can suppress valid IDs after a daemon restart.
                if (this.processedCommandSignatures.size > ACKNOWLEDGED_COMMAND_HISTORY_LIMIT) {
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
            // Dispatch commands without blocking the heartbeat loop.
            // Command completion is returned asynchronously via queueCommandResult().
            void this.dispatchCommand(command);
        }
    }
    scheduleNextPoll(data) {
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
    handleSyncFailure(err, features) {
        this.syncing = false;
        this.flushRequested = false;
        this.onFailure();
        // Re-merge drained UI features so they aren't lost on failed sync.
        if (features) {
            restoreUIFeatures(features);
        }
        const retryMs = this.retryDelayMs();
        this.log('Sync failed, retrying', {
            correlation_id: this.extSessionId,
            error: errorMessage(err),
            retry_ms: retryMs
        });
        this.scheduleNextSync(retryMs);
    }
    onSuccess() {
        const wasDisconnected = !this.state.connected;
        this.state.connected = true;
        this.state.lastSyncAt = this.runtime.now();
        this.state.consecutiveFailures = 0;
        if (wasDisconnected) {
            this.log('Connected');
            this.callbacks.onConnectionChange(true);
        }
    }
    onFailure() {
        this.state.consecutiveFailures++;
        // Log at logarithmic intervals (10, 100, 1000) — not every failure.
        if (this.state.consecutiveFailures === 10 ||
            this.state.consecutiveFailures === 100 ||
            this.state.consecutiveFailures === 1000) {
            this.log('Sync failure threshold reached', { failures: this.state.consecutiveFailures });
        }
        // Require 2+ consecutive failures before marking disconnected
        // to prevent a single transient timeout from flipping connection state
        if (this.state.consecutiveFailures >= 2 && this.state.connected) {
            this.state.connected = false;
            this.log('Disconnected');
            this.callbacks.onConnectionChange(false);
        }
    }
    retryDelayMs() {
        const exponent = Math.min(Math.max(this.state.consecutiveFailures - 1, 0), 5);
        const capped = Math.min(BASE_POLL_MS * 2 ** exponent, 30000);
        const jitter = 0.75 + this.runtime.random() * 0.5;
        return Math.round(capped * jitter);
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
        const controller = new AbortController();
        let timeoutHandle = null;
        let timedOut = false;
        try {
            await Promise.race([
                Promise.resolve(this.callbacks.onCommand(command, controller.signal)),
                new Promise((_, reject) => {
                    timeoutHandle = this.runtime.setTimer(() => {
                        timedOut = true;
                        controller.abort();
                        reject(new Error(`Command ${command.id || '(unknown)'} (${command.type || 'unknown'}) timed out after ${timeoutMs}ms`));
                    }, timeoutMs);
                })
            ]);
            this.log('Command completed OK', { id: command.id });
        }
        catch (err) {
            const message = errorMessage(err, 'Command execution failed');
            this.log('Command execution FAILED', { id: command.id, error: message });
            this.queueCommandResult({
                id: command.id,
                correlation_id: command.correlation_id,
                status: timedOut ? 'timeout' : 'error',
                error: message,
                connection_generation: command.connection_generation
            });
        }
        finally {
            if (timeoutHandle) {
                this.runtime.clearTimer(timeoutHandle);
            }
            this.clearInProgressById(command.id);
            // Ack after dispatch completes (success or failure) — not on bare receipt
            if (command.id) {
                this.state.lastCommandAck = command.id;
            }
        }
    }
    markInProgress(command) {
        const now = new Date(this.runtime.now()).toISOString();
        const current = this.inProgressById.get(command.id);
        this.inProgressById.set(command.id, {
            id: command.id,
            correlation_id: command.correlation_id,
            type: command.type,
            status: current?.status || 'running',
            progress_pct: current?.progress_pct,
            started_at: current?.started_at || now,
            updated_at: now,
            connection_generation: command.connection_generation
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
            updated_at: entry.updated_at || new Date(this.runtime.now()).toISOString()
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