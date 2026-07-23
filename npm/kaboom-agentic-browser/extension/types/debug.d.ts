/**
 * Purpose: Defines structured debug categories and debug-log entry contracts for extension diagnostics.
 * Why: Keeps diagnostic logging shape consistent so debug pipelines can filter and analyze reliably.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
/**
 * @fileoverview Debug Types
 * Debug logging and categorization
 */
/**
 * Debug log categories
 */
export type DebugCategory = 'connection' | 'capture' | 'error' | 'lifecycle' | 'settings' | 'sourcemap' | 'query';
/**
 * Debug log entry
 */
export interface DebugLogEntry {
    readonly ts: string;
    readonly category: DebugCategory;
    readonly message: string;
    readonly data?: unknown;
}
//# sourceMappingURL=debug.d.ts.map