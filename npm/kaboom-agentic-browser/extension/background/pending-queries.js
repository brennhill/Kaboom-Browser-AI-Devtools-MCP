/**
 * Purpose: Thin dispatcher shell that delegates pending MCP queries to command modules registered in commands/.
 * Why: Decouples query routing from handler implementations to keep the dispatch table extensible.
 */
import { dispatch } from './commands/registry.js';
// Import command modules to trigger handler registration
import './commands/observe.js';
import './commands/analyze.js';
import './commands/analyze-navigation.js';
import './commands/analyze-page-structure.js';
import './commands/analyze-feature-gates.js';
import './commands/interact.js';
import './commands/interact-content.js';
import './commands/interact-explore.js';
// Re-export handlePilotCommand (used by index.ts re-export chain)
export { handlePilotCommand } from './commands/interact.js';
export async function handlePendingQuery(query, syncClient) {
    return dispatch(query, syncClient);
}
//# sourceMappingURL=pending-queries.js.map