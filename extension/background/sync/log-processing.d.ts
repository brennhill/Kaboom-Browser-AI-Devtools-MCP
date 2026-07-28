/**
 * Purpose: Normalizes captured log entries and applies level filtering.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import type { LogEntry } from '../../types/index.js';
export declare function formatLogEntry(entry: LogEntry): LogEntry;
export declare function shouldCaptureLog(logLevel: string, filterLevel: string, logType?: string): boolean;
//# sourceMappingURL=log-processing.d.ts.map