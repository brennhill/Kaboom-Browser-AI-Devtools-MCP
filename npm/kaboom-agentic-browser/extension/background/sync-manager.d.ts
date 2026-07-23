/**
 * Purpose: Manages sync client instance lifecycle (start/stop/reset) and wires dependencies to avoid circular imports with index.ts.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
type DebugLogFn = (category: string, message: string, data?: unknown) => void;
/** Mutable connection status (same shape as index.ts) */
export interface SyncConnectionStatusRef {
    connected: boolean;
    entries: number;
    maxEntries: number;
    errorCount: number;
    logFile: string;
    logFileSize?: number;
    serverVersion?: string;
    extensionVersion?: string;
    versionMismatch?: boolean;
}
/** Extension log queue entry */
export interface ExtensionLogEntry {
    timestamp: string;
    level: string;
    message: string;
    source: string;
    category: string;
    data?: unknown;
}
/** Dependencies injected by index.ts to avoid circular imports */
export interface SyncManagerDeps {
    getServerUrl: () => string;
    getExtSessionId: () => string;
    getConnectionStatus: () => SyncConnectionStatusRef;
    setConnectionStatus: (patch: Partial<SyncConnectionStatusRef>) => void;
    getAiControlled: () => boolean;
    getAiWebPilotEnabledCache: () => boolean;
    getExtensionLogQueue: () => ExtensionLogEntry[];
    clearExtensionLogQueue: () => void;
    applyCaptureOverrides: (overrides: Record<string, string>) => void;
    debugLog: DebugLogFn;
}
/**
 * Start the sync client (unified /sync endpoint).
 * Safe to call multiple times — will no-op if already running.
 */
export declare function startSyncClient(deps: SyncManagerDeps): void;
/**
 * Stop the sync client
 */
export declare function stopSyncClient(debugLog: DebugLogFn): void;
/**
 * Reset sync client connection (call when user enables pilot/tracking)
 */
export declare function resetSyncClientConnection(debugLog: DebugLogFn): void;
export {};
//# sourceMappingURL=sync-manager.d.ts.map