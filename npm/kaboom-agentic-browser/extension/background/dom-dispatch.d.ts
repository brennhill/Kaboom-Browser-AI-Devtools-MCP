/**
 * Purpose: Dispatches DOM actions (click, type, wait_for, list_interactive, query) to injected page scripts with frame targeting and CDP escalation.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import type { PendingQuery } from '../types/queries.js';
import type { SyncClient } from './sync-client.js';
import type { SendAsyncResultFn, ActionToastFn } from './commands/helpers.js';
export declare function executeDOMAction(query: PendingQuery, tabId: number, syncClient: SyncClient, sendAsyncResult: SendAsyncResultFn, actionToast: ActionToastFn): Promise<void>;
//# sourceMappingURL=dom-dispatch.d.ts.map