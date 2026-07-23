// analyze-feature-gates.ts — Feature gate detection command handler (#345).
import { analyzeFeatureGates } from '../analyze-feature-gates.js';
import { registerCommand } from './registry.js';
import { errorMessage } from '../../lib/error-utils.js';
// =============================================================================
// FEATURE GATES DETECTION (#345)
// =============================================================================
registerCommand('feature_gates', async (ctx) => {
    try {
        const results = await chrome.scripting.executeScript({
            target: { tabId: ctx.tabId },
            world: 'MAIN',
            func: analyzeFeatureGates
        });
        const result = results?.[0]?.result;
        if (!result) {
            ctx.sendResult({
                error: 'feature_gates_failed',
                message: 'Feature gates scan returned no result'
            });
            return;
        }
        ctx.sendResult(result);
    }
    catch (err) {
        const message = errorMessage(err, 'Feature gates scan failed');
        ctx.sendResult({
            error: 'feature_gates_failed',
            message
        });
    }
});
//# sourceMappingURL=analyze-feature-gates.js.map