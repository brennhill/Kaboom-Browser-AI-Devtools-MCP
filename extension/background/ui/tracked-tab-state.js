/**
 * Purpose: Owns live tracked-tab lookup, tab readiness, active-tab selection, and capture of a
 *          tab the user is not looking at.
 * Why: `chrome.tabs.captureVisibleTab` can only photograph the tab that is visible, so every
 *      screenshot used to activate the target tab and put the user's tab back afterwards.
 *      That stole the foreground once per capture and dropped whatever the person was typing.
 *      `Page.captureScreenshot` over the tab's persistent CDP lease has no such constraint.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
import { delay } from '../../lib/timeout-utils.js';
import { scaleTimeout } from '../../lib/timeouts.js';
import { readTrackedTab } from '../../lib/tabs/tracked-tab-storage.js';
import { errorMessage } from '../../lib/error-utils.js';
import { KABOOM_LOG_PREFIX } from '../../lib/brand.js';
import { DebugCategory, debugLog } from '../debug.js';
import { cdpSessions, CDP_SESSION_ERRORS } from '../dom/cdp/cdp-session.js';
import { setKaboomOverlayVisibility } from './content-script-bridge.js';
export async function waitForTabLoad(tabId, timeoutMs = scaleTimeout(5000)) {
    const startTime = Date.now();
    while (Date.now() - startTime < timeoutMs) {
        try {
            if ((await chrome.tabs.get(tabId)).status === 'complete')
                return true;
        }
        catch {
            // EXPECTED_ABSENCE: tracked-tab closure during polling is normal; logging would duplicate the resulting recovery state.
            return false;
        }
        await delay(scaleTimeout(100));
    }
    return false;
}
export async function getTrackedTabInfo() {
    const result = await readTrackedTab();
    const tabId = result.id || null;
    let tabStatus = null;
    let trackedTabActive = null;
    if (tabId && typeof chrome !== 'undefined' && chrome.tabs) {
        try {
            const tab = await chrome.tabs.get(tabId);
            if (tab.status === 'loading' || tab.status === 'complete')
                tabStatus = tab.status;
            trackedTabActive = !!tab.active;
        }
        catch {
            // EXPECTED_ABSENCE: UI recipients can normally disappear during navigation
            // or teardown; logging it would misleadingly report a normal lifecycle race as failure.
            // The tracked tab may have closed.
        }
    }
    return {
        trackedTabId: tabId,
        trackedTabUrl: result.url || null,
        trackedTabTitle: result.title || null,
        tabStatus,
        trackedTabActive
    };
}
export async function getActiveTab() {
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    return tab?.id ? tab : null;
}
/**
 * Photograph a tab WITHOUT bringing it to the foreground.
 *
 * The CDP path is the ordinary one: it works on a tab the user is not looking at, so the
 * agent can drive and observe in the background while the person keeps working. The
 * visible-tab path is the fallback for a context with no `chrome.debugger`, and is the only
 * remaining place that touches the foreground.
 *
 * Kaboom's own overlays (`data-kaboom-overlay`) are hidden across BOTH paths, so a screenshot
 * never contains the supervision badge or phantom cursor that Kaboom itself drew.
 */
export async function captureTabImage(tabId, windowId, options) {
    await setKaboomOverlayVisibility(tabId, false);
    try {
        // A tab that is already the active one in its window is reachable through
        // captureVisibleTab with no debugger at all, and those are the same pixels CDP would
        // return. Attaching anyway raises Chrome's "Kaboom is debugging this browser" infobar
        // over the user's own browsing for the lease's idle grace — and screenshot_on_error
        // fires on any page error, so that banner would appear unprompted while someone is
        // simply using their browser. CDP is for tabs the user is NOT looking at.
        const alreadyVisible = await captureIfActive(tabId, windowId, options);
        if (alreadyVisible !== null)
            return alreadyVisible;
        const sessions = cdpSessions();
        if (!sessions) {
            reportForegroundFallback(tabId, 'no_debugger_api', 'chrome.debugger is not available in this context');
            return await captureVisibleTabActivating(tabId, windowId, options);
        }
        try {
            return await captureOverCDP(await sessions.acquire(tabId), options);
        }
        catch (err) {
            reportForegroundFallback(tabId, classifyCaptureFailure(err), errorMessage(err));
        }
        return await captureVisibleTabActivating(tabId, windowId, options);
    }
    finally {
        await setKaboomOverlayVisibility(tabId, true);
    }
}
/**
 * Capture a tab that is already the active one in its window, or return null when it is
 * not — leaving the caller to reach it over CDP.
 *
 * A capture failure here is NOT swallowed into the null: it is reported and returns null
 * so CDP still gets its turn, because "this tab is backgrounded" and "captureVisibleTab
 * refused" are different facts and only the second is a defect (rule 25).
 */
async function captureIfActive(tabId, windowId, options) {
    const [activeTab] = await chrome.tabs.query({ active: true, windowId });
    if (activeTab?.id !== tabId)
        return null;
    try {
        return await chrome.tabs.captureVisibleTab(windowId, options);
    }
    catch (err) {
        debugLog(DebugCategory.CAPTURE, 'Visible-tab capture failed on the active tab; falling back to CDP', {
            tab_id: tabId,
            browser_error: errorMessage(err)
        });
        return null;
    }
}
function classifyCaptureFailure(err) {
    const message = errorMessage(err);
    const expected = [
        CDP_SESSION_ERRORS.EXCLUSIVE_HELD,
        CDP_SESSION_ERRORS.DRAIN_TIMEOUT,
        CDP_SESSION_ERRORS.ATTACH_FAILED,
        CDP_SESSION_ERRORS.INVALIDATED
    ];
    return expected.some((code) => message.includes(code)) ? 'session_unavailable' : 'cdp_capture_failed';
}
function reportForegroundFallback(tabId, reason, detail) {
    debugLog(DebugCategory.CAPTURE, 'Capture fell back to the visible tab', {
        tab_id: tabId,
        reason,
        detail
    });
    if (reason !== 'cdp_capture_failed')
        return;
    // console.warn (not debugLog alone) so a broken lease is visible in the service-worker
    // console with extension debug logging off. The user's window just jumped for a reason
    // they cannot otherwise see, and the capture they got is not the one they asked for.
    console.warn(`${KABOOM_LOG_PREFIX} capture(tab ${tabId}): CDP capture failed, activating the tab instead — ${detail}`);
}
/**
 * Capture over an already-acquired lease, always releasing it.
 *
 * The clip is the *visual* viewport so the image matches what `captureVisibleTab` produced,
 * and `scale` is the page's device pixel ratio so a HiDPI capture keeps its resolution —
 * `observe(screenshot, selector)` crops by that same ratio, and a 1x image would place the
 * crop at half the intended coordinates.
 */
async function captureOverCDP(lease, options) {
    try {
        const clip = await resolveViewportClip(lease);
        const shot = (await lease.send('Page.captureScreenshot', {
            format: options.format,
            ...(options.format === 'jpeg' && typeof options.quality === 'number' ? { quality: options.quality } : {}),
            ...(clip ? { clip } : {})
        }));
        if (typeof shot?.data !== 'string' || shot.data.length === 0) {
            throw new Error('cdp_capture_empty: Page.captureScreenshot returned no image data');
        }
        return `data:image/${options.format === 'png' ? 'png' : 'jpeg'};base64,${shot.data}`;
    }
    finally {
        lease.release();
    }
}
async function resolveViewportClip(lease) {
    const metrics = (await lease.send('Page.getLayoutMetrics', {}));
    const viewport = metrics?.cssVisualViewport;
    const width = viewport?.clientWidth ?? 0;
    const height = viewport?.clientHeight ?? 0;
    if (!(width > 0) || !(height > 0))
        return null;
    return { x: viewport?.pageX ?? 0, y: viewport?.pageY ?? 0, width, height, scale: await devicePixelScale(lease) };
}
async function devicePixelScale(lease) {
    const evaluated = (await lease.send('Runtime.evaluate', {
        expression: 'window.devicePixelRatio',
        returnByValue: true
    }));
    const ratio = evaluated?.result?.value;
    return typeof ratio === 'number' && ratio > 0 ? ratio : 1;
}
/**
 * Fallback only. `chrome.tabs.captureVisibleTab` photographs whatever is visible in the
 * window, so a background tab is simply not reachable through it: the tab must be brought
 * forward and the user's tab handed straight back.
 */
async function captureVisibleTabActivating(tabId, windowId, options) {
    const [activeTab] = await chrome.tabs.query({ active: true, windowId });
    const wasActive = activeTab?.id === tabId;
    if (!wasActive)
        await chrome.tabs.update(tabId, { active: true });
    try {
        return await chrome.tabs.captureVisibleTab(windowId, options);
    }
    finally {
        if (!wasActive && activeTab?.id) {
            await chrome.tabs.update(activeTab.id, { active: true }).catch(() => {
                // EXPECTED_ABSENCE: closure of the prior active tab is normal during
                // capture; logging it would misleadingly mark the successful capture failed.
            });
        }
    }
}
//# sourceMappingURL=tracked-tab-state.js.map