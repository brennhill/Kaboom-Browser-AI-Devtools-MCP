import { errorMessage } from '../../lib/error-utils.js';
import { KABOOM_LOG_PREFIX } from '../../lib/brand.js';
import { postDaemonJSON } from '../../lib/daemon-http.js';
import { setKaboomOverlayVisibility } from '../ui/content-script-bridge.js';
import { trackUIFeature } from '../ui/ui-usage-tracker.js';
function captureActiveTab(sendResponse, deps) {
    if (typeof chrome === 'undefined' || !chrome.tabs) {
        sendResponse({ success: false, error: 'Chrome tabs API not available' });
        return;
    }
    chrome.tabs.query({ active: true, currentWindow: true }, async (tabs) => {
        const tabId = tabs[0]?.id;
        if (!tabId) {
            sendResponse({ success: false, error: 'No active tab' });
            return;
        }
        try {
            const result = await deps.captureScreenshot(tabId, null);
            if (result.success && result.entry)
                deps.addLog(result.entry);
            sendResponse(result);
        }
        catch (error) {
            sendResponse({ success: false, error: errorMessage(error) });
        }
    });
}
/**
 * Draw mode's backdrop. Deliberately stays on `captureVisibleTab`: the request arrives from a
 * content script the user is drawing on, so the tab is already in front of them, and the
 * annotation must be laid over exactly the pixels they are looking at. Background capture
 * (`captureTabImage`) is for tabs nobody is watching; this one has a watcher by definition.
 */
async function captureDrawOverlay(tabId, sendResponse) {
    if (!tabId) {
        sendResponse({ dataUrl: '' });
        return;
    }
    try {
        const tab = await chrome.tabs.get(tabId);
        await setKaboomOverlayVisibility(tabId, false);
        const dataUrl = await chrome.tabs.captureVisibleTab(tab.windowId, { format: 'png' });
        await setKaboomOverlayVisibility(tabId, true);
        sendResponse({ dataUrl });
    }
    catch {
        await setKaboomOverlayVisibility(tabId, true).catch(() => {
            console.error(`${KABOOM_LOG_PREFIX} Failed to restore overlay after screenshot capture error`);
        });
        sendResponse({ dataUrl: '' });
    }
}
async function deliverDrawResult(message, tabId, deps) {
    if (!tabId)
        return;
    const body = {
        screenshot_data_url: message.screenshot_data_url || '',
        annotations: message.annotations || [],
        element_details: message.elementDetails || {},
        page_url: message.page_url || '',
        tab_id: tabId,
        correlation_id: message.correlation_id || ''
    };
    if (message.annot_session_name)
        body.annot_session_name = message.annot_session_name;
    try {
        const response = await postDaemonJSON(`${deps.getServerUrl()}/draw-mode/complete`, body);
        if (!response.ok)
            deps.debugLog('error', `Draw mode POST failed: ${response.status}`);
    }
    catch (error) {
        deps.debugLog('error', `Draw mode completion error: ${errorMessage(error)}. Server may be unreachable.`);
    }
}
export function createCaptureMessageHandler(deps) {
    return {
        feature: 'capture',
        handle(message, sender, sendResponse) {
            switch (message.type) {
                case 'capture_screenshot':
                    trackUIFeature('screenshot');
                    captureActiveTab(sendResponse, deps);
                    return true;
                case 'kaboom_capture_screenshot':
                    void captureDrawOverlay(sender.tab?.id, sendResponse);
                    return true;
                case 'draw_mode_completed':
                    void deliverDrawResult(message, sender.tab?.id, deps);
                    return false;
                case 'track_ui_feature':
                    trackUIFeature(message.feature);
                    return false;
                default:
                    return undefined;
            }
        }
    };
}
//# sourceMappingURL=capture-handler.js.map