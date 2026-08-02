export interface ExtensionLogQueueEntry {
    timestamp: string;
    level: string;
    message: string;
    source: string;
    category: string;
    data?: unknown;
}
export interface ExtensionLogQueueMetrics {
    entries: number;
    droppedCount: number;
    saturated: boolean;
    persistenceFailures: number;
}
export interface ExtensionLogQueueStorage {
    read: () => Promise<unknown>;
    write: (value: PersistedExtensionLogQueue) => Promise<void>;
}
export interface ExtensionLogQueueRecovery {
    status: 'empty' | 'restored' | 'recovered';
    restoredEntries: number;
}
interface PersistedExtensionLogQueue {
    version: 1;
    dropped_count: number;
    entries: ExtensionLogQueueEntry[];
    lifecycle_events?: string[];
}
export declare function initializeExtensionLogQueue(storageOverride?: ExtensionLogQueueStorage): Promise<ExtensionLogQueueRecovery>;
export declare function getExtensionLogQueueSnapshot(): ExtensionLogQueueEntry[];
export declare function acknowledgeExtensionLogQueue(sentCount: number): void;
export declare function pushExtensionLog(entry: ExtensionLogQueueEntry): void;
export declare function recordExtensionDiagnosticLifecycle(event: string, correlationId: string, data?: Readonly<Record<string, unknown>>): void;
export declare function getExtensionLogQueueMetrics(): ExtensionLogQueueMetrics;
export declare function flushExtensionLogPersistenceForTesting(): Promise<void>;
export declare function clearExtensionLogsForTesting(): void;
export {};
//# sourceMappingURL=log-queue.d.ts.map