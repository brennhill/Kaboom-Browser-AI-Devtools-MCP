/**
 * Purpose: Forwards window.postMessage events from the inject context to the background script via chrome.runtime.sendMessage.
 * Docs: docs/features/feature/observe/index.md
 */
import { getReloadedExtensionWarning } from '../lib/brand.js';
// Dispatch table: page postMessage type -> background message type
export const MESSAGE_MAP = {
    kaboom_log: 'log',
    kaboom_ws: 'ws_event',
    kaboom_network_body: 'network_body',
    kaboom_enhanced_action: 'enhanced_action',
    kaboom_performance_snapshot: 'performance_snapshot'
};
// Track whether the extension context is still valid
let contextValid = true;
/**
 * Safely send a message to the background script
 * Handles extension context invalidation gracefully
 */
export function safeSendMessage(msg) {
    if (!contextValid)
        return;
    try {
        chrome.runtime.sendMessage(msg);
    }
    catch (e) {
        if (e instanceof Error && e.message?.includes('Extension context invalidated')) {
            contextValid = false;
            console.warn(getReloadedExtensionWarning());
        }
    }
}
/**
 * Check if the extension context is still valid
 */
function isContextValid() {
    return contextValid;
}
//# sourceMappingURL=message-forwarding.js.map