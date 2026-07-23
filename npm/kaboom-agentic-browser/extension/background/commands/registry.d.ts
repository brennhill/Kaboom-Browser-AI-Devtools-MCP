/**
 * Purpose: Map-based command registry and dispatch loop that replaces the monolithic if-chain for routing pending queries to handlers.
 * Why: Extensible design lets new command modules register themselves without modifying central dispatch.
 */
import type { PendingQuery } from '../../types/index.js';
import type { SyncClient } from '../sync-client.js';
import type { SendAsyncResultFn, QueryParamsObject, TargetResolution } from './helpers.js';
import { actionToast } from './helpers.js';
export interface CommandContext {
    query: PendingQuery;
    syncClient: SyncClient;
    tabId: number;
    params: QueryParamsObject;
    target: TargetResolution | undefined;
    /** Send a sync result, wrapped with target context */
    sendResult: (result: unknown) => void;
    /** Send an async result, wrapped with target context */
    sendAsyncResult: SendAsyncResultFn;
    /** Show action toast on the target tab */
    actionToast: typeof actionToast;
}
export type CommandHandler = (ctx: CommandContext) => Promise<void>;
export declare function registerCommand(type: string, handler: CommandHandler): void;
export declare function dispatch(query: PendingQuery, syncClient: SyncClient): Promise<void>;
//# sourceMappingURL=registry.d.ts.map