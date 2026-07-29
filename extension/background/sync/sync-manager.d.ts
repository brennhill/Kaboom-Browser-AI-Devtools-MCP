/**
 * Purpose: Manages sync client instance lifecycle (start/stop/reset) and wires dependencies to avoid circular imports with index.ts.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import type { ConnectionStatus } from '../../types/index.js';
import type { ExtensionLogQueueEntry } from '../runtime-state/log-queue.js';
type DebugLogFn = (category: string, message: string, data?: unknown) => void;
/** Mutable connection status (same shape as index.ts) */
export type SyncConnectionStatusRef = Pick<ConnectionStatus, 'connected' | 'entries' | 'maxEntries' | 'errorCount' | 'logFile' | 'logFileSize' | 'serverVersion' | 'extensionVersion' | 'versionMismatch' | 'securityMode' | 'productionParity' | 'insecureRewritesApplied'>;
/** Dependencies injected by index.ts to avoid circular imports */
export interface SyncManagerDeps {
    getServerUrl: () => string;
    getExtSessionId: () => string;
    getConnectionStatus: () => SyncConnectionStatusRef;
    setConnectionStatus: (patch: Partial<SyncConnectionStatusRef>) => void;
    getAiControlled: () => boolean;
    getAiWebPilotEnabledCache: () => boolean;
    getExtensionLogQueue: () => ExtensionLogQueueEntry[];
    acknowledgeExtensionLogQueue: (sentCount: number) => void;
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