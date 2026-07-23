/**
 * Purpose: Thin dispatcher shell that delegates pending MCP queries to command modules registered in commands/.
 * Why: Decouples query routing from handler implementations to keep the dispatch table extensible.
 */
import type { PendingQuery } from '../types/index.js';
import type { SyncClient } from './sync-client.js';
import './commands/observe.js';
import './commands/analyze.js';
import './commands/analyze-navigation.js';
import './commands/analyze-page-structure.js';
import './commands/analyze-feature-gates.js';
import './commands/interact.js';
import './commands/interact-content.js';
import './commands/interact-explore.js';
export type { SendAsyncResultFn, ActionToastFn } from './commands/helpers.js';
export { handlePilotCommand } from './commands/interact.js';
export declare function handlePendingQuery(query: PendingQuery, syncClient: SyncClient): Promise<void>;
//# sourceMappingURL=pending-queries.d.ts.map