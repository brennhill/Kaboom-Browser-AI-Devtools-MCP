/**
 * Purpose: Unified sync client that replaces multiple polling loops with a single /sync endpoint, handling settings, commands, and extension logs.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import type { ExtensionLog } from '../../types/wire/wire-extension-log.js';
import type { SyncCommand, SyncCommandResult, SyncSettings } from '../../types/wire/wire-sync.js';
/** Sync state */
export interface SyncState {
    connected: boolean;
    lastSyncAt: number;
    consecutiveFailures: number;
    lastCommandAck: string | null;
}
/** Callbacks for sync client */
export interface SyncClientCallbacks {
    onCommand: (command: SyncCommand, signal: AbortSignal) => Promise<void>;
    onConnectionChange: (connected: boolean) => void;
    onCaptureOverrides?: (overrides: Record<string, string>) => void;
    onVersionMismatch?: (extensionVersion: string, serverVersion: string) => void;
    commandTimeoutMs?: number;
    uploadCommandTimeoutMs?: number;
    getSettings: () => Promise<SyncSettings>;
    getExtensionLogs: () => ExtensionLog[];
    acknowledgeExtensionLogs: (sentCount: number) => void;
    debugLog?: (category: string, message: string, data?: unknown) => void;
}
/** Controllable runtime boundary for deterministic sync lifecycle tests. */
export interface SyncRuntime {
    now: () => number;
    random: () => number;
    setTimer: (callback: () => void, delayMs: number) => ReturnType<typeof setTimeout>;
    clearTimer: (handle: ReturnType<typeof setTimeout>) => void;
    request: (url: string, init: RequestInit, timeoutMs: number) => Promise<Response>;
}
export declare class SyncClient {
    private serverUrl;
    private extSessionId;
    private callbacks;
    private state;
    private intervalId;
    private running;
    private syncing;
    private flushRequested;
    private pendingResults;
    private inProgressById;
    private processedCommandSignatures;
    private extensionVersion;
    private connectionGeneration;
    private lifecycleEpoch;
    private runtime;
    constructor(serverUrl: string, extSessionId: string, callbacks: SyncClientCallbacks, extensionVersion?: string, runtime?: SyncRuntime);
    /** Get current sync state */
    getState(): SyncState;
    /** Check if connected */
    isConnected(): boolean;
    /** Start the sync loop */
    start(): void;
    /** Stop the sync loop */
    stop(): void;
    /** Queue a command result to send on next sync, then flush immediately */
    queueCommandResult(result: SyncCommandResult): void;
    /** Trigger an immediate sync to deliver queued results with minimal latency */
    flush(): void;
    /** Reset connection state (e.g., when user toggles pilot/tracking) */
    resetConnection(): void;
    /** Update server URL */
    setServerUrl(url: string): void;
    /** Optional progress updates for long-running commands */
    updateCommandProgress(commandId: string, progressPct?: number, status?: 'running' | 'pending'): void;
    private scheduleNextSync;
    private doSync;
    private buildSyncRequest;
    private fetchSyncResponse;
    private rejectStaleSync;
    private applySyncResponse;
    private notifyVersionMismatch;
    private dispatchSyncCommands;
    private scheduleNextPoll;
    private handleSyncFailure;
    private onSuccess;
    private onFailure;
    private retryDelayMs;
    private log;
    private getCommandSignature;
    private commandTimeoutFor;
    private dispatchCommand;
    private markInProgress;
    private clearInProgressById;
    private getInProgressSnapshot;
}
/**
 * Create a sync client instance
 */
export declare function createSyncClient(serverUrl: string, extSessionId: string, callbacks: SyncClientCallbacks, extensionVersion?: string): SyncClient;
//# sourceMappingURL=sync-client.d.ts.map