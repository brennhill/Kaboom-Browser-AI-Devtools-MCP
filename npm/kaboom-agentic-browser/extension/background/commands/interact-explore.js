// interact-explore.ts — explore_page compound command handler (#338).
// Combines page metadata, interactive elements, readable text, and navigation
// links into a single extension response, reducing MCP round-trips for AI agents.
import { domPrimitiveListInteractive } from '../dom-primitives-list-interactive.js';
import { domPrimitiveNavDiscovery } from '../dom-primitives-nav-discovery.js';
import { readableFallbackScript } from '../content-fallback-scripts.js';
import { registerCommand } from './registry.js';
import { errorMessage } from '../../lib/error-utils.js';
// =============================================================================
// EXPLORE_PAGE COMMAND (#338)
// =============================================================================
registerCommand('explore_page', async (ctx) => {
    try {
        // If URL is provided, navigate first
        const targetUrl = typeof ctx.params.url === 'string' ? ctx.params.url : undefined;
        if (targetUrl) {
            // Validate URL scheme — only http/https allowed (security: prevent javascript:/data:/chrome: injection)
            if (!/^https?:\/\//i.test(targetUrl)) {
                throw new Error('Only http/https URLs are supported for explore_page navigation, got: ' + targetUrl.split(':')[0] + ':');
            }
            // Register onUpdated listener BEFORE calling tabs.update to prevent race condition
            // where the page load completes before the listener is attached (#9.3.2, #9.7.5).
            await new Promise((resolve) => {
                const timeout = setTimeout(() => {
                    chrome.tabs.onUpdated.removeListener(onUpdated);
                    resolve();
                }, 15000);
                const onUpdated = (tabId, changeInfo) => {
                    if (tabId === ctx.tabId && changeInfo.status === 'complete') {
                        chrome.tabs.onUpdated.removeListener(onUpdated);
                        clearTimeout(timeout);
                        resolve();
                    }
                };
                chrome.tabs.onUpdated.addListener(onUpdated);
                chrome.tabs.update(ctx.tabId, { url: targetUrl }).catch(() => {
                    chrome.tabs.onUpdated.removeListener(onUpdated);
                    clearTimeout(timeout);
                    resolve(); // continue with current page state
                });
            });
        }
        // 1. Get tab info (page metadata)
        const tab = await chrome.tabs.get(ctx.tabId);
        // 2. Run all data collection in parallel — capture errors for _errors array (#9.7.4)
        const [interactiveResults, readableResults, navResults] = await Promise.all([
            // Interactive elements
            chrome.scripting
                .executeScript({
                target: { tabId: ctx.tabId, allFrames: true },
                world: 'MAIN',
                func: domPrimitiveListInteractive,
                args: ['']
            })
                .catch((err) => [{ result: { success: false, error: err.message, _source: 'interactive' } }]),
            // Readable content
            chrome.scripting
                .executeScript({
                target: { tabId: ctx.tabId },
                world: 'ISOLATED',
                func: readableFallbackScript
            })
                .catch((err) => [{ result: { error: 'extraction_failed', _reason: err.message, _source: 'readable' } }]),
            // Navigation links (uses shared dom-primitives-nav-discovery)
            chrome.scripting
                .executeScript({
                target: { tabId: ctx.tabId },
                world: 'ISOLATED',
                func: domPrimitiveNavDiscovery
            })
                .catch((err) => [
                { result: { error: 'extraction_failed', _reason: err.message, _source: 'navigation' } }
            ])
        ]);
        // Process interactive elements (capped at 100)
        const elements = [];
        let interactiveError;
        for (const r of interactiveResults) {
            const res = r.result;
            if (res?.success === false) {
                if (!interactiveError)
                    interactiveError = res.error || res.message;
                continue;
            }
            if (res?.elements) {
                elements.push(...res.elements);
                if (elements.length >= 100)
                    break;
            }
        }
        let cappedElements = elements.slice(0, 100);
        // Apply visible_only filter if requested
        if (ctx.params.visible_only === true) {
            cappedElements = cappedElements.filter((el) => {
                const elem = el;
                return elem.visible !== false;
            });
        }
        // Apply limit if specified
        const limit = typeof ctx.params.limit === 'number' && ctx.params.limit > 0 ? ctx.params.limit : cappedElements.length;
        const finalElements = cappedElements.slice(0, limit);
        // Process readable content
        const readableFirst = readableResults?.[0]?.result;
        const readable = readableFirst && typeof readableFirst === 'object' ? readableFirst : null;
        // Process navigation links
        const navFirst = navResults?.[0]?.result;
        const navigation = navFirst && typeof navFirst === 'object' ? navFirst : null;
        // Build composite payload
        const payload = {
            // Page metadata
            url: tab.url || '',
            title: tab.title || '',
            tab_status: tab.status || '',
            favicon: tab.favIconUrl || '',
            viewport: {
                width: tab.width,
                height: tab.height
            },
            // Interactive elements
            interactive_elements: finalElements,
            interactive_count: finalElements.length,
            // Readable text
            readable: readable || { error: 'extraction_failed' },
            // Navigation links
            navigation: navigation || { error: 'extraction_failed' }
        };
        // Build unified _errors array for partial failures (UX Review R6)
        const errors = [];
        if (interactiveError && finalElements.length === 0) {
            payload.interactive_error = interactiveError;
            errors.push({ component: 'interactive', error: interactiveError });
        }
        if (readable && typeof readable === 'object' && 'error' in readable) {
            const reason = readable._reason;
            errors.push({ component: 'readable', error: String(reason || readable.error) });
        }
        if (navigation && typeof navigation === 'object' && 'error' in navigation) {
            const reason = navigation._reason;
            errors.push({ component: 'navigation', error: String(reason || navigation.error) });
        }
        if (errors.length > 0) {
            payload._errors = errors;
        }
        ctx.sendResult(payload);
    }
    catch (err) {
        const message = errorMessage(err, 'Explore page failed');
        ctx.sendResult({
            error: 'explore_page_failed',
            message
        });
    }
});
//# sourceMappingURL=interact-explore.js.map