/**
 * Purpose: Owns background-to-content-script liveness, broadcast, overlay, and toast messaging.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */
import { scaleTimeout } from '../../lib/timeouts.js';
export async function pingContentScript(tabId, timeoutMs = scaleTimeout(500)) {
    try {
        const response = (await Promise.race([
            chrome.tabs.sendMessage(tabId, { type: 'kaboom_ping' }),
            new Promise((_, reject) => {
                setTimeout(() => reject(new Error(`Content script ping timeout after ${timeoutMs}ms on tab ${tabId}`)), timeoutMs);
            })
        ]));
        return response?.status === 'alive';
    }
    catch {
        return false;
    }
}
export async function forwardToAllContentScripts(message, debugLogFn) {
    if (typeof chrome === 'undefined' || !chrome.tabs)
        return;
    const tabs = await chrome.tabs.query({});
    for (const tab of tabs) {
        if (!tab.id)
            continue;
        chrome.tabs.sendMessage(tab.id, message).catch((err) => {
            if (err.message?.includes('Receiving end does not exist') ||
                err.message?.includes('Could not establish connection')) {
                return;
            }
            debugLogFn?.('error', 'Unexpected error forwarding setting to tab', {
                tabId: tab.id,
                error: err.message
            });
        });
    }
}
export async function setKaboomOverlayVisibility(tabId, visible) {
    try {
        await chrome.scripting.executeScript({
            target: { tabId },
            func: (show) => {
                for (const id of ['kaboom-tracked-hover-launcher', 'kaboom-draw-toolbar']) {
                    const element = document.getElementById(id);
                    if (element)
                        element.style.display = show ? 'flex' : 'none';
                }
            },
            args: [visible]
        });
    }
    catch {
        // The tab may not have a content script.
    }
}
export function sendTabToast(tabId, text, detail = '', state = 'success', duration_ms = 3000) {
    chrome.tabs
        .sendMessage(tabId, {
        type: 'kaboom_action_toast',
        text,
        detail,
        state,
        duration_ms
    })
        .catch(() => {
        // The tab may not have a content script.
    });
}
//# sourceMappingURL=content-script-bridge.js.map