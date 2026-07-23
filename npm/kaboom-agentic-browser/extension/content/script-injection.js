/**
 * Purpose: Injects inject.bundled.js into the page MAIN world and syncs stored settings to the inject context.
 * Docs: docs/features/feature/csp-safe-execution/index.md
 */
import { SettingName } from '../lib/constants.js';
import { getLocals } from '../lib/storage-utils.js';
/** Whether inject.bundled.js has been injected into the page (MAIN world) */
let injected = false;
/** Whether inject.js has responded to a bridge ping for this page load */
let bridgeReady = false;
/** Shared in-flight promise for initial inject load */
let injectionPromise = null;
/** Shared in-flight promise for bridge probe */
let bridgeProbePromise = null;
/** Monotonic ID for bridge probe request IDs */
let bridgeProbeCounter = 0;
const NONCE_ATTR = 'data-kaboom-nonce';
/** Per-page-load nonce for authenticating postMessages to inject.js */
const pageNonce = crypto
    .getRandomValues(new Uint8Array(16))
    .reduce((s, b) => s + b.toString(16).padStart(2, '0'), '');
/** Get the page nonce for authenticating postMessages to inject.js */
export function getPageNonce() {
    return pageNonce;
}
/** Check if inject script has been loaded into the page context */
export function isInjectScriptLoaded() {
    return injected;
}
/** Check if inject bridge has acknowledged a readiness ping */
function isInjectBridgeReady() {
    return bridgeReady;
}
/** Settings that need to be synced to inject script on page load */
const SYNC_SETTINGS = [
    { storageKey: 'webSocketCaptureEnabled', messageType: SettingName.WEBSOCKET_CAPTURE },
    { storageKey: 'webSocketCaptureMode', messageType: SettingName.WEBSOCKET_CAPTURE_MODE, isMode: true },
    { storageKey: 'networkWaterfallEnabled', messageType: SettingName.NETWORK_WATERFALL },
    { storageKey: 'performanceMarksEnabled', messageType: SettingName.PERFORMANCE_MARKS },
    { storageKey: 'actionReplayEnabled', messageType: SettingName.ACTION_REPLAY },
    { storageKey: 'networkBodyCaptureEnabled', messageType: SettingName.NETWORK_BODY_CAPTURE }
];
/**
 * Sync stored settings to the inject script after it loads.
 * This ensures new pages receive the current settings state.
 */
async function syncStoredSettings() {
    const storageKeys = SYNC_SETTINGS.map((s) => s.storageKey);
    const result = await getLocals(storageKeys);
    for (const setting of SYNC_SETTINGS) {
        const value = result[setting.storageKey];
        if (value === undefined)
            continue; // Use default if not set
        if (setting.isMode) {
            window.postMessage({
                type: 'kaboom_setting',
                setting: setting.messageType,
                mode: value,
                _nonce: pageNonce
            }, window.location.origin);
        }
        else {
            window.postMessage({ type: 'kaboom_setting', setting: setting.messageType, enabled: value, _nonce: pageNonce }, window.location.origin);
        }
    }
}
/**
 * Inject axe-core library into the page
 * Must be called from content script context (has chrome.runtime API access)
 */
function injectAxeCore() {
    if (document.getElementById('kaboom-axe-loader'))
        return;
    const script = document.createElement('script');
    script.id = 'kaboom-axe-loader';
    script.src = chrome.runtime.getURL('lib/axe.min.js');
    script.onload = () => script.remove();
    (document.head || document.documentElement).appendChild(script);
}
/**
 * Inject the capture script into the page
 */
function injectScript() {
    // Remove stale nonce-bearing script nodes so inject resolves the current nonce.
    document.querySelectorAll(`script[${NONCE_ATTR}]`).forEach((el) => {
        if (typeof el.remove === 'function')
            el.remove();
    });
    document.documentElement?.setAttribute?.(NONCE_ATTR, pageNonce);
    const script = document.createElement('script');
    script.src = chrome.runtime.getURL('inject.bundled.js');
    script.type = 'module';
    script.dataset.kaboomNonce = pageNonce;
    return new Promise((resolve) => {
        script.onload = () => {
            script.remove();
            injected = true;
            bridgeReady = false;
            // Sync stored settings after inject script loads.
            // Small delay to ensure inject script has initialized its message listeners.
            setTimeout(syncStoredSettings, 50);
            resolve(true);
        };
        script.onerror = () => {
            script.remove();
            injected = false;
            bridgeReady = false;
            resolve(false);
        };
        (document.head || document.documentElement).appendChild(script);
    });
}
function beginInjection(force = false) {
    if (!force) {
        if (injected)
            return Promise.resolve(true);
        if (injectionPromise)
            return injectionPromise;
    }
    else if (injectionPromise) {
        return injectionPromise;
    }
    injectionPromise = new Promise((resolve) => {
        const runInjection = () => {
            injectAxeCore(); // Inject axe-core first (needed by inject script)
            injectScript()
                .then((ok) => resolve(ok))
                .finally(() => {
                injectionPromise = null;
            });
        };
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', runInjection, { once: true });
            return;
        }
        runInjection();
    });
    return injectionPromise;
}
/**
 * Ensure inject script is present, deduplicating concurrent inject attempts.
 * Optionally force a fresh reinjection attempt.
 */
export async function ensureInjectScriptReady(timeoutMs = 2000, force = false) {
    if (!force && injected)
        return true;
    const injection = beginInjection(force);
    if (timeoutMs <= 0)
        return injection;
    return Promise.race([
        injection,
        new Promise((resolve) => {
            setTimeout(() => resolve(injected), timeoutMs);
        })
    ]);
}
/**
 * Ensure inject bridge responds to a ping, proving MAIN-world messaging is live.
 */
export async function ensureInjectBridgeReady(timeoutMs = 350) {
    if (bridgeReady)
        return true;
    const injectReady = await ensureInjectScriptReady(timeoutMs);
    if (!injectReady)
        return false;
    if (bridgeReady)
        return true;
    if (bridgeProbePromise)
        return bridgeProbePromise;
    bridgeProbePromise = new Promise((resolve) => {
        const requestId = `inject_bridge_${Date.now()}_${++bridgeProbeCounter}`;
        let settled = false;
        let timer;
        const cleanup = () => {
            if (timer)
                clearTimeout(timer);
            window.removeEventListener('message', onMessage);
            bridgeProbePromise = null;
        };
        const finish = (ok) => {
            if (settled)
                return;
            settled = true;
            if (ok)
                bridgeReady = true;
            cleanup();
            resolve(ok);
        };
        const onMessage = (event) => {
            if (event.source !== window || event.origin !== window.location.origin)
                return;
            if (event.data?.type !== 'kaboom_inject_bridge_pong')
                return;
            if (event.data?.requestId !== requestId)
                return;
            if (event.data?._nonce && event.data._nonce !== pageNonce)
                return;
            finish(true);
        };
        window.addEventListener('message', onMessage);
        timer = setTimeout(() => finish(false), Math.max(25, timeoutMs));
        try {
            window.postMessage({
                type: 'kaboom_inject_bridge_ping',
                requestId,
                _nonce: pageNonce
            }, window.location.origin);
        }
        catch {
            finish(false);
        }
    });
    return bridgeProbePromise;
}
/**
 * Initialize script injection (call when DOM is ready)
 */
export function initScriptInjection(force = false) {
    void beginInjection(force);
}
//# sourceMappingURL=script-injection.js.map