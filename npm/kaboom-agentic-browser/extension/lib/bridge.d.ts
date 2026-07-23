/**
 * Purpose: Posts log events from the inject context to the content script via window.postMessage, enriching errors with context annotations and action replay.
 * Docs: docs/features/feature/observe/index.md
 */
export interface BridgePayload {
    level?: string;
    message?: string;
    error?: string;
    args?: unknown[];
    filename?: string;
    lineno?: number;
    [key: string]: unknown;
}
/**
 * Post a log message to the content script
 */
export declare function postLog(payload: BridgePayload): void;
//# sourceMappingURL=bridge.d.ts.map