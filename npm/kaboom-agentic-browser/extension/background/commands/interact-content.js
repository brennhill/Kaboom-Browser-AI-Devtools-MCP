// interact-content.ts — Command handlers for content extraction query types (#257).
// Handles: get_readable, get_markdown, page_summary.
// Routes through chrome.tabs.sendMessage to the content script (ISOLATED world, CSP-safe).
// Falls back to chrome.scripting.executeScript when content script is not loaded (#364).
import { registerCommand } from './registry.js';
import { isContentScriptUnreachableError } from './helpers.js';
import { errorMessage } from '../../lib/error-utils.js';
import { FALLBACK_SCRIPTS } from '../content-fallback-scripts.js';
/**
 * Factory for content extraction command handlers.
 * All three extractors share identical structure — they differ only in message type and error code.
 * The lifecycle's sendResult handles sync/async routing via correlation_id internally.
 */
function contentExtractorCommand(messageType, errorCode) {
    return async (ctx) => {
        try {
            const result = await chrome.tabs.sendMessage(ctx.tabId, {
                type: messageType,
                params: ctx.query.params
            });
            ctx.sendResult(result);
        }
        catch (err) {
            // Fallback: inject extraction script directly when content script is not loaded
            if (isContentScriptUnreachableError(err)) {
                const fallbackFn = FALLBACK_SCRIPTS[messageType];
                if (fallbackFn) {
                    try {
                        const results = await chrome.scripting.executeScript({
                            target: { tabId: ctx.tabId },
                            world: 'ISOLATED',
                            func: fallbackFn
                        });
                        const firstResult = results?.[0]?.result;
                        if (firstResult) {
                            ctx.sendResult(firstResult);
                            return;
                        }
                    }
                    catch (fallbackErr) {
                        // Fallback also failed — return error with context
                        ctx.sendResult({
                            error: errorCode,
                            message: `Content script not loaded and fallback injection failed: ${fallbackErr.message || 'unknown error'}. Refresh the page first: interact({what: "refresh"}), then retry.`
                        });
                        return;
                    }
                }
                ctx.sendResult({
                    error: errorCode,
                    message: 'Content script not loaded on this page. Refresh the page first: interact({what: "refresh"}), then retry.'
                });
                return;
            }
            ctx.sendResult({
                error: errorCode,
                message: errorMessage(err) || `${errorCode}`
            });
        }
    };
}
registerCommand('get_readable', contentExtractorCommand('kaboom_get_readable', 'get_readable_failed'));
registerCommand('get_markdown', contentExtractorCommand('kaboom_get_markdown', 'get_markdown_failed'));
registerCommand('page_summary', contentExtractorCommand('kaboom_page_summary', 'page_summary_failed'));
//# sourceMappingURL=interact-content.js.map