/**
 * Purpose: Handles browser navigation actions (navigate, refresh, back, forward, tab management) with CSP probing and async timeouts.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import type { PendingQuery } from '../types/index.js';
import type { SyncClient } from './sync-client.js';
import { type CSPProbeResult } from './query-execution.js';
import type { SendAsyncResultFn, ActionToastFn } from './pending-queries.js';
export type BrowserActionResult = {
    success: boolean;
    action?: string;
    url?: string;
    final_url?: string;
    title?: string;
    tab_id?: number;
    tab_index?: number;
    closed_tab_id?: number;
    content_script_status?: string;
    message?: string;
    error?: string;
    csp_blocked?: boolean;
    csp_restricted?: boolean;
    csp_level?: string;
    failure_cause?: string;
};
/** Get the CSP status from the most recent navigation (for sync layer) */
export declare function getLastCSPStatus(): CSPProbeResult;
export declare function handleBrowserAction(tabId: number, params: {
    action?: string;
    what?: string;
    url?: string;
    reason?: string;
    tab_id?: number;
    tab_index?: number;
    new_tab?: boolean;
}, actionToast: ActionToastFn): Promise<BrowserActionResult>;
export declare function handleAsyncExecuteCommand(query: PendingQuery, tabId: number, world: string, syncClient: SyncClient, sendAsyncResult: SendAsyncResultFn, actionToast: ActionToastFn): Promise<void>;
export declare function handleAsyncBrowserAction(query: PendingQuery, tabId: number, params: {
    action?: string;
    what?: string;
    url?: string;
    tab_id?: number;
    tab_index?: number;
    new_tab?: boolean;
}, syncClient: SyncClient, sendAsyncResult: SendAsyncResultFn, actionToast: ActionToastFn): Promise<void>;
//# sourceMappingURL=browser-actions.d.ts.map