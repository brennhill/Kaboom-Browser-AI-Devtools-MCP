/**
 * Purpose: Shared infrastructure for command dispatch -- result helpers, target tab resolution, action toast, and type aliases.
 */
import type { PendingQuery } from '../../types/index.js';
import type { SyncClient } from '../sync-client.js';
/** Callback signature for sending async command results back through /sync */
export type SendAsyncResultFn = (syncClient: SyncClient, queryId: string, correlationId: string, status: 'complete' | 'error' | 'timeout' | 'cancelled', result?: unknown, error?: string) => void;
/** Callback signature for showing visual action toasts */
export type ActionToastFn = (tabId: number, text: string, detail?: string, state?: 'trying' | 'success' | 'warning' | 'error', durationMs?: number) => void;
export type QueryParamsObject = Record<string, unknown>;
type TargetResolutionSource = 'explicit_tab' | 'tracked_tab' | 'active_tab' | 'active_tab_fallback' | 'auto_tracked_active_tab' | 'auto_tracked_random_tab' | 'auto_tracked_new_tab';
export interface TargetResolution {
    tabId: number;
    url: string;
    source: TargetResolutionSource;
    requestedTabId?: number;
    trackedTabId?: number | null;
    useActiveTab: boolean;
}
interface TargetResolutionError {
    payload: Record<string, unknown>;
    message: string;
}
export declare function debugLog(category: string, message: string, data?: unknown): void;
/** Send a query result back through /sync */
export declare function sendResult(syncClient: SyncClient, queryId: string, result: unknown): void;
/** Send an async command result back through /sync */
export declare function sendAsyncResult(syncClient: SyncClient, queryId: string, correlationId: string, status: 'complete' | 'error' | 'timeout' | 'cancelled', result?: unknown, error?: string): void;
/** Show a visual action toast on the tracked tab */
export declare function actionToast(tabId: number, action: string, detail?: string, state?: 'trying' | 'success' | 'warning' | 'error', durationMs?: number): void;
export declare function parseQueryParamsObject(params: PendingQuery['params']): QueryParamsObject;
export declare function withTargetContext(result: unknown, target: TargetResolution): Record<string, unknown>;
export declare function requiresTargetTab(queryType: string): boolean;
export declare function isBrowserEscapeAction(queryType: string, paramsObj: QueryParamsObject): boolean;
export declare function persistTrackedTab(tab: chrome.tabs.Tab): Promise<void>;
export declare function resolveTargetTab(query: PendingQuery, paramsObj: QueryParamsObject): Promise<{
    target?: TargetResolution;
    error?: TargetResolutionError;
}>;
/**
 * Check if a URL is restricted — content scripts cannot run on these pages.
 * Covers internal browser pages and known CSP-restricted origins.
 */
export declare function isRestrictedUrl(url: string | undefined): boolean;
/** Check if an error indicates the content script is not loaded on the target page. */
export declare function isContentScriptUnreachableError(err: unknown): boolean;
/**
 * Minimal context shape needed by requireAiWebPilot.
 * Avoids circular import with registry.ts (which defines CommandContext).
 */
interface AiWebPilotGuardContext {
    sendResult: (result: unknown) => void;
}
/**
 * Guard that checks AI Web Pilot is enabled.
 * Returns true if enabled and the caller should proceed.
 * Returns false if disabled — the error response has already been sent.
 */
export declare function requireAiWebPilot(ctx: AiWebPilotGuardContext): boolean;
export {};
//# sourceMappingURL=helpers.d.ts.map