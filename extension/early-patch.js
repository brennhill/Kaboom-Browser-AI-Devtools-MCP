/**
 * Purpose: Installs early WebSocket, fetch, and XHR shims before page scripts run and buffers pre-inject events.
 * Why: Prevents loss of startup network activity that occurs before the main inject capture bundle is initialized.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
// early-patch.ts — Lightweight WebSocket, fetch, and XHR patches.
// Runs in MAIN world at document_start before any page scripts.
// Saves originals and buffers connections/bodies for handoff to inject.bundled.js.
// Must be self-contained: no imports, no chrome.* APIs (MAIN world).
;
(function () {
    'use strict';
    if (typeof window === 'undefined')
        return;
    /**
     * Assign a global that the page may have made read-only.
     *
     * Duplicated from src/lib/safe-global-patch.ts on purpose: this file runs in
     * the MAIN world at document_start and is required to be self-contained (see
     * the header). Keep the two in sync.
     *
     * Hardened pages define fetch/WebSocket as non-writable, so a plain assignment
     * throws "Cannot assign to read only property 'fetch' of object '#<Window>'".
     * Unguarded, that throw escaped and aborted every patch after it.
     */
    function safeAssignGlobal(target, key, value) {
        try {
            // eslint-disable-next-line security/detect-object-injection -- key is a keyof T supplied by our own call sites
            target[key] = value;
            // eslint-disable-next-line security/detect-object-injection -- same key, read back to confirm the write landed
            if (target[key] === value)
                return true;
        }
        catch {
            // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
            // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
            // Non-writable — try defineProperty below.
        }
        try {
            Object.defineProperty(target, key, { value, writable: true, configurable: true });
            // eslint-disable-next-line security/detect-object-injection -- same key, read back to confirm the write landed
            return target[key] === value;
        }
        catch {
            // EXPECTED_ABSENCE: a non-configurable page-owned global is normal;
            // logging would mislabel the documented no-capture fallback as failure.
            return false;
        }
    }
    // Cloaked domains — bail out before patching any globals.
    // Must be sync (MAIN world, no chrome APIs). Manifest exclude_matches is
    // the primary guard; this is a defense-in-depth fallback.
    const CLOAKED_DOMAINS = ['cloudflare.com'];
    const host = location.hostname;
    if (CLOAKED_DOMAINS.some((d) => host === d || host.endsWith('.' + d)))
        return;
    // Guard: only install once (extension reloads, multiple frames)
    if (window.__KABOOM_ORIGINAL_WS__ || window.__KABOOM_ORIGINAL_FETCH__ || window.__KABOOM_EARLY_BODIES__)
        return;
    // =========================================================================
    // SHARED: Early body buffer (used by fetch + XHR patches)
    // =========================================================================
    const EARLY_BODY_MAX = 50;
    const BODY_SIZE_CAP = 16384; // 16KB per body
    const BODY_READ_TIMEOUT = 5000; // 5s timeout for async body read
    // Only capture text/json-like responses
    const TEXT_CONTENT_RE = /^(text\/|application\/json|application\/.*\+json|application\/xml|application\/.*\+xml)/i;
    const earlyBodies = [];
    window.__KABOOM_EARLY_BODIES__ = earlyBodies;
    /** Push a body entry with FIFO eviction */
    function pushBody(entry) {
        earlyBodies.push(entry);
        if (earlyBodies.length > EARLY_BODY_MAX) {
            earlyBodies.shift();
        }
    }
    // =========================================================================
    // WEBSOCKET PATCH (existing)
    // =========================================================================
    if (window.WebSocket) {
        const OriginalWS = window.WebSocket;
        // Store original for inject script to retrieve
        window.__KABOOM_ORIGINAL_WS__ = OriginalWS;
        // Buffer for early connections
        const earlyConnections = [];
        window.__KABOOM_EARLY_WS__ = earlyConnections;
        // Thin wrapper: creates real WebSocket + buffers lifecycle events
        function EarlyWebSocket(url, protocols) {
            const ws = protocols !== undefined ? new OriginalWS(url, protocols) : new OriginalWS(url);
            const conn = { ws, url: url.toString(), createdAt: Date.now(), events: [] };
            ws.addEventListener('open', () => {
                conn.events.push({ type: 'open', ts: Date.now() });
            });
            ws.addEventListener('close', (e) => {
                conn.events.push({ type: 'close', ts: Date.now(), code: e.code, reason: e.reason });
            });
            ws.addEventListener('error', () => {
                conn.events.push({ type: 'error', ts: Date.now() });
            });
            earlyConnections.push(conn);
            // Cap buffer to bound memory
            if (earlyConnections.length > 50) {
                earlyConnections.shift();
            }
            return ws;
        }
        // Preserve prototype chain: instanceof WebSocket still works
        EarlyWebSocket.prototype = OriginalWS.prototype;
        // Preserve static constants
        Object.defineProperty(EarlyWebSocket, 'CONNECTING', { value: 0, writable: false });
        Object.defineProperty(EarlyWebSocket, 'OPEN', { value: 1, writable: false });
        Object.defineProperty(EarlyWebSocket, 'CLOSING', { value: 2, writable: false });
        Object.defineProperty(EarlyWebSocket, 'CLOSED', { value: 3, writable: false });
        if (!safeAssignGlobal(window, 'WebSocket', EarlyWebSocket)) {
            // Read-only WebSocket: leave the page's own alone and skip early WS capture.
            // Mirror the fetch path — drop the stashes so inject's Phase 2 does not adopt
            // a shim we never installed, and so the buffer is not left dangling.
            delete window.__KABOOM_ORIGINAL_WS__;
            delete window.__KABOOM_EARLY_WS__;
        }
    }
    // =========================================================================
    // FETCH PATCH
    // =========================================================================
    if (typeof window.fetch === 'function') {
        const OriginalFetch = window.fetch;
        // Store original for Phase 2 adoption
        window.__KABOOM_ORIGINAL_FETCH__ = OriginalFetch;
        const patchedFetch = function (input, init) {
            // Determine URL and method
            let url = '';
            let method = 'GET';
            if (typeof input === 'string') {
                url = input;
            }
            else if (input instanceof URL) {
                url = input.toString();
            }
            else if (input && typeof input.url === 'string') {
                url = input.url;
                method = input.method || 'GET';
            }
            if (init?.method) {
                method = init.method;
            }
            // Call original fetch — return the original promise unchanged
            const responsePromise = OriginalFetch.call(window, input, init);
            // Async body read in microtask — does NOT block the fetch return
            responsePromise
                .then((response) => {
                try {
                    const contentType = response.headers?.get?.('content-type') || '';
                    // Only capture text/json responses
                    if (!TEXT_CONTENT_RE.test(contentType))
                        return;
                    // Clone to avoid consuming the body
                    const cloned = response.clone();
                    const status = response.status;
                    // Race body read against timeout
                    Promise.race([
                        cloned.text(),
                        new Promise((resolve) => {
                            setTimeout(() => resolve('[Skipped: body read timeout]'), BODY_READ_TIMEOUT);
                        })
                    ])
                        .then((body) => {
                        const truncated = body.length > BODY_SIZE_CAP ? body.slice(0, BODY_SIZE_CAP) : body;
                        pushBody({
                            url,
                            method: method.toUpperCase(),
                            status,
                            content_type: contentType,
                            response_body: truncated,
                            timestamp: Date.now()
                        });
                    })
                        .catch(() => {
                        // EXPECTED_ABSENCE: unreadable cloned bodies are normal page
                        // behavior; logging them would misleadingly attribute page data limits to Kaboom.
                    });
                }
                catch {
                    // EXPECTED_ABSENCE: hostile response accessors are normal page
                    // behavior; logging them would misleadingly blame Kaboom for page code.
                }
            })
                .catch(() => {
                // EXPECTED_ABSENCE: rejected page fetches are normal application behavior;
                // logging them would misleadingly duplicate the page's own network failure.
            });
            return responsePromise;
        };
        if (!safeAssignGlobal(window, 'fetch', patchedFetch)) {
            // Read-only fetch: leave the page's own alone and skip early fetch capture.
            // Everything below (XHR, buffering) still installs.
            delete window.__KABOOM_ORIGINAL_FETCH__;
        }
    }
    // =========================================================================
    // XHR PATCH
    // =========================================================================
    if (typeof XMLHttpRequest !== 'undefined') {
        const OriginalOpen = XMLHttpRequest.prototype.open;
        const OriginalSend = XMLHttpRequest.prototype.send;
        // Store originals for Phase 2 adoption
        window.__KABOOM_ORIGINAL_XHR_OPEN__ = OriginalOpen;
        window.__KABOOM_ORIGINAL_XHR_SEND__ = OriginalSend;
        XMLHttpRequest.prototype.open = function (method, url, ...rest) {
            ;
            this.__kaboomEarlyMethod = method;
            this.__kaboomEarlyUrl =
                typeof url === 'string' ? url : url.toString();
            return OriginalOpen.apply(this, [method, url, ...rest]);
        };
        XMLHttpRequest.prototype.send = function (body) {
            const xhrUrl = this.__kaboomEarlyUrl || '';
            const xhrMethod = this.__kaboomEarlyMethod || 'GET';
            this.addEventListener('load', function () {
                try {
                    const contentType = this.getResponseHeader('content-type') || '';
                    // Only capture text/json responses
                    if (!TEXT_CONTENT_RE.test(contentType))
                        return;
                    const responseType = this.responseType;
                    // Skip non-text response types (blob, arraybuffer, document)
                    if (responseType && responseType !== '' && responseType !== 'text' && responseType !== 'json')
                        return;
                    let responseBody = null;
                    try {
                        responseBody = this.responseText;
                    }
                    catch {
                        // EXPECTED_ABSENCE: inaccessible responseText is normal for opaque/non-text XHR;
                        // logging would mislabel the expected skipped capture as failure.
                        return;
                    }
                    if (responseBody === null)
                        return;
                    const truncated = responseBody.length > BODY_SIZE_CAP ? responseBody.slice(0, BODY_SIZE_CAP) : responseBody;
                    pushBody({
                        url: xhrUrl,
                        method: xhrMethod.toUpperCase(),
                        status: this.status,
                        content_type: contentType,
                        response_body: truncated,
                        timestamp: Date.now()
                    });
                }
                catch {
                    // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
                    // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
                    /* silent — early body capture must not affect page */
                }
            });
            return OriginalSend.call(this, body);
        };
    }
    // =========================================================================
    // SELF-CLEANUP: If Phase 2 never adopts early patches (e.g., CSP blocks
    // inject bundle), restore originals and free buffers after 30 seconds.
    // Bounds worst-case memory leak to ~800KB for the 30-second window.
    // =========================================================================
    setTimeout(() => {
        // Phase 2 deletes __KABOOM_EARLY_BODIES__ on adoption — if it still
        // exists, Phase 2 never ran and we must clean up.
        if (window.__KABOOM_EARLY_BODIES__) {
            delete window.__KABOOM_EARLY_BODIES__;
            // Restore fetch
            if (window.__KABOOM_ORIGINAL_FETCH__) {
                safeAssignGlobal(window, 'fetch', window.__KABOOM_ORIGINAL_FETCH__);
                delete window.__KABOOM_ORIGINAL_FETCH__;
            }
            // Restore XHR
            if (window.__KABOOM_ORIGINAL_XHR_OPEN__) {
                XMLHttpRequest.prototype.open = window.__KABOOM_ORIGINAL_XHR_OPEN__;
                delete window.__KABOOM_ORIGINAL_XHR_OPEN__;
            }
            if (window.__KABOOM_ORIGINAL_XHR_SEND__) {
                XMLHttpRequest.prototype.send = window.__KABOOM_ORIGINAL_XHR_SEND__;
                delete window.__KABOOM_ORIGINAL_XHR_SEND__;
            }
            // Restore WebSocket through the same guard as the install and the fetch
            // restore: a plain assignment throws on a non-configurable read-only global,
            // and that throw here would abort the rest of the cleanup and leak the buffer.
            if (window.__KABOOM_ORIGINAL_WS__) {
                safeAssignGlobal(window, 'WebSocket', window.__KABOOM_ORIGINAL_WS__);
                delete window.__KABOOM_ORIGINAL_WS__;
            }
            delete window.__KABOOM_EARLY_WS__;
        }
    }, 30_000);
})();
export {};
//# sourceMappingURL=early-patch.js.map