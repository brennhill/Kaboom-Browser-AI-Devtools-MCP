/**
 * Purpose: Own the bounded extension diagnostic log queue.
 * Why: Callers receive snapshots so they cannot mutate or retain the live queue.
 */
export interface ExtensionLogQueueEntry {
    timestamp: string;
    level: string;
    message: string;
    source: string;
    category: string;
    data?: unknown;
}
export declare function getExtensionLogQueueSnapshot(): ExtensionLogQueueEntry[];
export declare function acknowledgeExtensionLogQueue(sentCount: number): void;
export declare function pushExtensionLog(entry: ExtensionLogQueueEntry): void;
export declare function capExtensionLogs(maxEntries: number): void;
export declare function clearExtensionLogsForTesting(): void;
//# sourceMappingURL=log-queue.d.ts.map