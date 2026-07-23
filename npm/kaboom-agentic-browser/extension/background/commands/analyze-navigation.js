// analyze-navigation.ts — Navigation/SPA route discovery command handler (#335).
// Uses shared domPrimitiveNavDiscovery from dom-primitives-nav-discovery.ts (#9.6).
import { domPrimitiveNavDiscovery } from '../dom-primitives-nav-discovery.js';
import { registerCommand } from './registry.js';
import { errorMessage } from '../../lib/error-utils.js';
// =============================================================================
// NAVIGATION / SPA ROUTE DISCOVERY (#335)
// =============================================================================
registerCommand('navigation', async (ctx) => {
    try {
        const results = await chrome.scripting.executeScript({
            target: { tabId: ctx.tabId },
            world: 'ISOLATED',
            func: domPrimitiveNavDiscovery
        });
        const first = results?.[0]?.result;
        const navData = first && typeof first === 'object' ? first : {};
        // Add url/title from tab info (not available inside the injected script's return)
        const tab = await chrome.tabs.get(ctx.tabId);
        const payload = {
            url: tab.url || '',
            title: tab.title || '',
            ...navData
        };
        ctx.sendResult(payload);
    }
    catch (err) {
        const message = errorMessage(err, 'Navigation discovery failed');
        ctx.sendResult({
            error: 'navigation_discovery_failed',
            message
        });
    }
});
//# sourceMappingURL=analyze-navigation.js.map