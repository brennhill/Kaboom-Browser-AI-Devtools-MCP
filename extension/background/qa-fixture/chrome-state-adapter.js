/**
 * Purpose: Adapts Chrome tab, cookie, scripting, and window APIs to the QA fixture state driver.
 * Why: Keeps browser API details out of transaction policy and makes every I/O boundary replaceable in tests.
 * Docs: docs/features/feature/environment-manipulation/index.md
 */
import { createBrowserStateDriver } from './browser-state-driver.js';
const EMPTY_KEYS = {
    local_storage: [],
    session_storage: [],
    feature_flags: [],
    seed_data: []
};
export function createChromeBrowserStateDriver() {
    return createBrowserStateDriver(chromeDriverDeps());
}
export function chromeDriverDeps() {
    return {
        getTab: (tabId) => chrome.tabs.get(tabId),
        getWindow: (windowId) => chrome.windows.get(windowId),
        capturePageState: (tabId, fixture) => executePageFunction(tabId, captureNamedPageState, fixtureKeys(fixture)),
        getCookie: async (url, name) => {
            const cookie = await chrome.cookies.get({ url, name });
            return cookie ? fromChromeCookie(cookie) : null;
        },
        navigate: navigateAndWait,
        resizeViewport: resizeViewport,
        restoreWindow: async (windowId, width, height) => {
            await chrome.windows.update(windowId, { width, height });
        },
        setCookie: async (cookie, url) => {
            await chrome.cookies.set(toChromeCookie(cookie, url));
        },
        removeCookie: async (url, name) => {
            await chrome.cookies.remove({ url, name });
        },
        applyPageState: async (tabId, fixture) => {
            await executePageFunction(tabId, applyFixturePageState, fixture);
        },
        restorePageState: async (tabId, state) => {
            await executePageFunction(tabId, restoreFixturePageState, state);
        }
    };
}
function fixtureKeys(fixture) {
    return {
        local_storage: Object.keys(fixture.local_storage ?? {}),
        session_storage: Object.keys(fixture.session_storage ?? {}),
        feature_flags: Object.keys(fixture.feature_flags ?? {}),
        seed_data: Object.keys(fixture.seed_data ?? {})
    };
}
async function executePageFunction(tabId, func, arg) {
    const results = await chrome.scripting.executeScript({
        target: { tabId },
        world: 'MAIN',
        func,
        args: [arg]
    });
    const first = results[0];
    if (!first || first.result === undefined)
        throw new Error('fixture_page_execution_failed');
    return first.result;
}
async function navigateAndWait(tabId, url, timeoutMs) {
    await new Promise((resolve, reject) => {
        let settled = false;
        const finish = (error) => {
            if (settled)
                return;
            settled = true;
            clearTimeout(timer);
            chrome.tabs.onUpdated.removeListener(onUpdated);
            if (error)
                reject(error);
            else
                resolve();
        };
        const onUpdated = (updatedTabId, changeInfo) => {
            if (updatedTabId === tabId && changeInfo.status === 'complete')
                finish();
        };
        const timer = setTimeout(() => finish(new Error('fixture_navigation_timeout')), timeoutMs);
        chrome.tabs.onUpdated.addListener(onUpdated);
        chrome.tabs
            .update(tabId, { url })
            .then((tab) => {
            if (tab?.status === 'complete')
                finish();
        })
            .catch(() => finish(new Error('fixture_navigation_failed')));
    });
}
async function resizeViewport(tabId, windowId, desiredWidth, desiredHeight) {
    const currentWindow = await chrome.windows.get(windowId);
    const currentViewport = await executePageFunction(tabId, readViewport, EMPTY_KEYS);
    if (currentWindow.width === undefined || currentWindow.height === undefined) {
        throw new Error('window_bounds_unavailable');
    }
    const chromeWidth = currentWindow.width - currentViewport.width;
    const chromeHeight = currentWindow.height - currentViewport.height;
    await chrome.windows.update(windowId, {
        width: desiredWidth + chromeWidth,
        height: desiredHeight + chromeHeight
    });
}
function readViewport(_unused) {
    return { width: window.innerWidth, height: window.innerHeight };
}
function captureNamedPageState(keys) {
    const capture = (storage, names) => {
        const result = {};
        for (const name of names)
            result[name] = storage.getItem(name);
        return result;
    };
    return {
        local_storage: capture(localStorage, keys.local_storage),
        session_storage: capture(sessionStorage, keys.session_storage),
        feature_flags: capture(localStorage, keys.feature_flags),
        seed_data: capture(localStorage, keys.seed_data)
    };
}
function applyFixturePageState(fixture) {
    for (const [key, value] of Object.entries(fixture.local_storage ?? {}))
        localStorage.setItem(key, value);
    for (const [key, value] of Object.entries(fixture.session_storage ?? {}))
        sessionStorage.setItem(key, value);
    for (const [key, value] of Object.entries(fixture.feature_flags ?? {}))
        localStorage.setItem(key, JSON.stringify(value));
    for (const [key, value] of Object.entries(fixture.seed_data ?? {}))
        localStorage.setItem(key, JSON.stringify(value));
    return true;
}
function restoreFixturePageState(state) {
    const restore = (storage, values) => {
        for (const [key, value] of Object.entries(values)) {
            if (value === null)
                storage.removeItem(key);
            else
                storage.setItem(key, value);
        }
    };
    restore(localStorage, state.local_storage);
    restore(sessionStorage, state.session_storage);
    restore(localStorage, state.feature_flags);
    restore(localStorage, state.seed_data);
    return true;
}
function fromChromeCookie(cookie) {
    return {
        name: cookie.name,
        value: cookie.value,
        domain: cookie.domain,
        path: cookie.path,
        secure: cookie.secure,
        http_only: cookie.httpOnly,
        same_site: cookie.sameSite === 'no_restriction' ? 'none' : cookie.sameSite
    };
}
function toChromeCookie(cookie, url) {
    let sameSite;
    if (cookie.same_site === 'none')
        sameSite = 'no_restriction';
    else if (cookie.same_site === 'lax' || cookie.same_site === 'strict')
        sameSite = cookie.same_site;
    return {
        url,
        name: cookie.name,
        value: cookie.value,
        ...(cookie.domain ? { domain: cookie.domain } : {}),
        ...(cookie.path ? { path: cookie.path } : {}),
        ...(cookie.secure !== undefined ? { secure: cookie.secure } : {}),
        ...(cookie.http_only !== undefined ? { httpOnly: cookie.http_only } : {}),
        ...(sameSite ? { sameSite } : {})
    };
}
//# sourceMappingURL=chrome-state-adapter.js.map