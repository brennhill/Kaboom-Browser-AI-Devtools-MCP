/**
 * Purpose: Captures and uploads visible-tab screenshots for background error enrichment.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import { errorMessage } from '../../lib/error-utils.js';
import { captureVisibleTabSafe } from '../ui/tracked-tab-state.js';
import { getRequestHeaders } from './server.js';
export async function captureScreenshot(tabId, serverUrl, relatedErrorId, canTakeScreenshot, recordScreenshot, debugLog) {
    const rateCheck = canTakeScreenshot(tabId);
    if (!rateCheck.allowed) {
        debugLog?.('capture', `Screenshot rate limited: ${rateCheck.reason}`, {
            tabId,
            nextAllowedIn: rateCheck.nextAllowedIn
        });
        return {
            success: false,
            error: `Rate limited: ${rateCheck.reason}`,
            nextAllowedIn: rateCheck.nextAllowedIn
        };
    }
    try {
        const tab = await chrome.tabs.get(tabId);
        const dataUrl = await captureVisibleTabSafe(tabId, tab.windowId, { format: 'jpeg', quality: 80 });
        recordScreenshot(tabId);
        const response = await fetch(`${serverUrl}/screenshots`, {
            method: 'POST',
            headers: getRequestHeaders(),
            body: JSON.stringify({
                data_url: dataUrl,
                url: tab.url,
                correlation_id: relatedErrorId || ''
            })
        });
        if (!response.ok) {
            throw new Error(`Failed to upload screenshot: server returned HTTP ${response.status} ${response.statusText}`);
        }
        const result = (await response.json());
        const entry = {
            ts: new Date().toISOString(),
            type: 'screenshot',
            level: 'info',
            url: tab.url,
            _enrichments: ['screenshot'],
            screenshotFile: result.filename,
            trigger: relatedErrorId ? 'error' : 'manual',
            ...(relatedErrorId ? { relatedErrorId } : {})
        };
        debugLog?.('capture', `Screenshot saved: ${result.filename}`, {
            trigger: relatedErrorId ? 'error' : 'manual',
            relatedErrorId
        });
        return { success: true, entry };
    }
    catch (error) {
        debugLog?.('error', 'Screenshot capture failed', { error: errorMessage(error) });
        return { success: false, error: errorMessage(error) };
    }
}
//# sourceMappingURL=screenshot.js.map