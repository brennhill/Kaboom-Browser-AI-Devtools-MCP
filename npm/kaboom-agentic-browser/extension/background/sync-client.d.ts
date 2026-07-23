/**
 * Purpose: Unified sync client that replaces multiple polling loops with a single /sync endpoint, handling settings, commands, and extension logs.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
/** Returns the server's install ID, or undefined if not yet received. */
export declare function getServerInstallId(): string | undefined;
/** Load persisted install ID from storage (call once on startup). */
export declare function loadServerInstallId(): Promise<void>;
/** Settings to send to server */
export interface SyncSettings {
    pilot_enabled: boolean;
    tracking_enabled: boolean;
    tracked_tab_id: number;
    tracked_tab_url: string;
    tracked_tab_title: string;
    tab_status?: 'loading' | 'complete';
    tracked_tab_active?: boolean;
    capture_logs: boolean;
    capture_network: boolean;
    capture_websocket: boolean;
    capture_actions: boolean;
    csp_restricted: boolean;
    csp_level: string;
}
/** Extension log entry */
export interface SyncExtensionLog {
    timestamp: string;
    level: string;
    message: string;
    source: string;
    category: string;
    data?: unknown;
}
/** Command result to send to server */
export interface SyncCommandResult {
    id: string;
    correlation_id?: string;
    status: 'complete' | 'error' | 'timeout' | 'cancelled';
    result?: unknown;
    error?: string;
}
/** Active command metadata sent on each sync heartbeat */
export interface SyncInProgress {
    id: string;
    correlation_id?: string;
    type?: string;
    status?: 'running' | 'pending';
    progress_pct?: number;
    started_at?: string;
    updated_at?: string;
}
/** Command from server */
export interface SyncCommand {
    id: string;
    type: string;
    params: unknown;
    tab_id?: number;
    correlation_id?: string;
    trace_id?: string;
}
/** Sync state */
export interface SyncState {
    connected: boolean;
    lastSyncAt: number;
    consecutiveFailures: number;
    lastCommandAck: string | null;
}
/** Callbacks for sync client */
export interface SyncClientCallbacks {
    onCommand: (command: SyncCommand) => Promise<void>;
    onConnectionChange: (connected: boolean) => void;
    onCaptureOverrides?: (overrides: Record<string, string>) => void;
    onVersionMismatch?: (extensionVersion: string, serverVersion: string) => void;
    commandTimeoutMs?: number;
    uploadCommandTimeoutMs?: number;
    getSettings: () => Promise<SyncSettings>;
    getExtensionLogs: () => SyncExtensionLog[];
    clearExtensionLogs: () => void;
    debugLog?: (category: string, message: string, data?: unknown) => void;
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
    constructor(serverUrl: string, extSessionId: string, callbacks: SyncClientCallbacks, extensionVersion?: string);
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
    private onSuccess;
    private onFailure;
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