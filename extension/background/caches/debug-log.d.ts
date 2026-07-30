/**
 * Purpose: Owns the bounded in-memory background debug log.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import type { DebugLogEntry } from '../../types/runtime/debug.js';
export declare function getDebugLog(): DebugLogEntry[];
export declare function addDebugLogEntry(entry: DebugLogEntry): void;
export declare function clearDebugLog(): void;
//# sourceMappingURL=debug-log.d.ts.map