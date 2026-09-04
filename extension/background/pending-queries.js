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
export async function handlePendingQuery(query, syncClient, signal = new AbortController().signal) {
    return dispatch(query, syncClient, signal);
}
//# sourceMappingURL=pending-queries.js.map