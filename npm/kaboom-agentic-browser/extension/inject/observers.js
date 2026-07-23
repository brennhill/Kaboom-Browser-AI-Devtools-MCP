/**
 * Purpose: Registers and manages runtime observers for DOM mutations, network requests, performance entries, and WebSocket events in the page context.
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview Observers - Observer registration and management for DOM, network,
 * performance, and WebSocket events.
 */
import { installPerformanceCapture, uninstallPerformanceCapture } from '../lib/performance.js';
import { installPerfObservers } from '../lib/perf-snapshot.js';
import { installWebSocketCapture, uninstallWebSocketCapture } from '../lib/websocket.js';
import { wrapFetchWithBodies, wrapXHRWithBodies, unwrapXHR, adoptEarlyBodies, sanitizeHeaders } from '../lib/network.js';
import { installConsoleCapture, uninstallConsoleCapture } from '../lib/console.js';
import { installExceptionCapture, uninstallExceptionCapture } from '../lib/exceptions.js';
import { installActionCapture, uninstallActionCapture, installNavigationCapture, uninstallNavigationCapture } from '../lib/actions.js';
import { installTransientCapture, uninstallTransientCapture } from '../lib/transient-capture.js';
import { postLog } from '../lib/bridge.js';
import { MAX_RESPONSE_LENGTH, MEMORY_SOFT_LIMIT_MB, MEMORY_HARD_LIMIT_MB } from '../lib/constants.js';
import { errorMessage } from '../lib/error-utils.js';
// Store original fetch for restoration
let originalFetch = null;
// Interception deferral state (Phase 1/Phase 2 split)
let deferralEnabled = true;
let phase2Installed = false;
let injectionTimestamp = 0;
let phase2Timestamp = 0;
/**
 * Wrap fetch to capture network errors
 */
// #lizard forgives
export function wrapFetch(originalFetchFn) {
    // #lizard forgives
    return async function (input, init) {
        const startTime = Date.now();
        const url = typeof input === 'string' ? input : input.url;
        const method = init?.method || (typeof input === 'object' && 'method' in input ? input.method : 'GET') || 'GET';
        try {
            const response = await originalFetchFn(input, init);
            const duration = Date.now() - startTime;
            // Capture errors (4xx, 5xx)
            if (!response.ok) {
                let responseBody = '';
                try {
                    const cloned = response.clone();
                    responseBody = await cloned.text();
                    if (responseBody.length > MAX_RESPONSE_LENGTH) {
                        responseBody = responseBody.slice(0, MAX_RESPONSE_LENGTH) + '... [truncated]';
                    }
                }
                catch {
                    responseBody = '[Could not read response]';
                }
                // Filter sensitive headers (check both init.headers and Request object headers)
                const rawHeaders = init?.headers || (typeof input === 'object' && 'headers' in input ? input.headers : null);
                const safeHeaders = sanitizeHeaders(rawHeaders);
                const logPayload = {
                    level: 'error',
                    type: 'network',
                    method: method.toUpperCase(),
                    url,
                    status: response.status,
                    statusText: response.statusText,
                    duration,
                    response: responseBody,
                    ...(Object.keys(safeHeaders).length > 0 ? { headers: safeHeaders } : {})
                };
                postLog(logPayload);
            }
            return response;
        }
        catch (error) {
            const duration = Date.now() - startTime;
            // Filter sensitive headers for the error path
            const rawHeaders = init?.headers || (typeof input === 'object' && 'headers' in input ? input.headers : null);
            const safeHeaders = sanitizeHeaders(rawHeaders);
            const logPayload = {
                level: 'error',
                type: 'network',
                method: method.toUpperCase(),
                url,
                error: errorMessage(error),
                duration,
                ...(Object.keys(safeHeaders).length > 0 ? { headers: safeHeaders } : {})
            };
            postLog(logPayload);
            throw error;
        }
    };
}
/**
 * Install fetch capture.
 * Uses wrapFetchWithBodies to capture request/response bodies for all requests,
 * then wraps that with wrapFetch to also capture error details for 4xx/5xx responses.
 * If the early-patch script ran first, uses the saved original fetch (not the early wrapper).
 */
export function installFetchCapture() {
    // Check for early-patch: use the saved original, not the early-patch wrapper
    const earlyOriginal = window.__KABOOM_ORIGINAL_FETCH__;
    originalFetch = earlyOriginal || window.fetch;
    // Layer 1: wrapFetchWithBodies captures request/response bodies for ALL requests
    // Layer 2: wrapFetch captures detailed error logging for 4xx/5xx responses
    // Use unknown intermediate cast to handle TypeScript's strict fetch overload types
    // This is necessary because the DOM lib defines fetch with multiple overloads
    // that TypeScript cannot reconcile with our simpler function signature
    const wrappedWithBodies = wrapFetchWithBodies(originalFetch);
    window.fetch = wrapFetch(wrappedWithBodies);
}
/**
 * Install XHR body capture (wraps XMLHttpRequest.prototype.open/send)
 */
export function installXHRCapture() {
    wrapXHRWithBodies();
}
/**
 * Uninstall XHR body capture
 */
export function uninstallXHRCapture() {
    unwrapXHR();
}
/**
 * Uninstall fetch capture
 */
export function uninstallFetchCapture() {
    if (originalFetch) {
        window.fetch = originalFetch;
        originalFetch = null;
    }
}
/**
 * Install all capture hooks
 */
export function install() {
    installConsoleCapture();
    installFetchCapture();
    installXHRCapture();
    installExceptionCapture();
    installActionCapture();
    installNavigationCapture();
    installWebSocketCapture();
    installPerformanceCapture();
    installTransientCapture();
}
/**
 * Uninstall all capture hooks
 */
export function uninstall() {
    uninstallConsoleCapture();
    uninstallFetchCapture();
    uninstallXHRCapture();
    uninstallExceptionCapture();
    uninstallActionCapture();
    uninstallNavigationCapture();
    uninstallWebSocketCapture();
    uninstallPerformanceCapture();
    uninstallTransientCapture();
}
/**
 * Check if heavy intercepts should be deferred until page load
 */
export function shouldDeferIntercepts() {
    if (typeof document === 'undefined')
        return false;
    return document.readyState === 'loading';
}
/**
 * Check memory pressure and adjust buffer capacities
 */
export function checkMemoryPressure(state) {
    const result = { ...state };
    if (state.memoryUsageMB >= MEMORY_HARD_LIMIT_MB) {
        // Hard limit: disable network bodies
        result.networkBodiesEnabled = false;
        result.wsBufferCapacity = Math.floor(state.wsBufferCapacity * 0.25);
        result.networkBufferCapacity = Math.floor(state.networkBufferCapacity * 0.25);
    }
    else if (state.memoryUsageMB >= MEMORY_SOFT_LIMIT_MB) {
        // Soft limit: reduce buffers
        result.wsBufferCapacity = Math.floor(state.wsBufferCapacity * 0.5);
        result.networkBufferCapacity = Math.floor(state.networkBufferCapacity * 0.5);
    }
    return result;
}
/**
 * Phase 1 (Immediate): Lightweight, non-intercepting setup.
 */
export function installPhase1() {
    console.log('[KaBOOM!] Phase 1 installing (lightweight API + perf observers)');
    injectionTimestamp = performance.now();
    phase2Installed = false;
    phase2Timestamp = 0;
    // Start PerformanceObservers (passive observers, no prototype modification)
    installPerformanceCapture();
    // Now handle Phase 2 scheduling
    if (!deferralEnabled) {
        // Deferral disabled: install Phase 2 immediately
        installPhase2();
    }
    else {
        const installDeferred = () => {
            if (!phase2Installed)
                setTimeout(installPhase2, 100);
        };
        if (document.readyState === 'complete') {
            // Page already loaded, defer by 100ms
            installDeferred();
        }
        else {
            // Wait for load event, then defer by 100ms
            window.addEventListener('load', installDeferred, { once: true });
            // 10-second timeout fallback
            setTimeout(() => {
                if (!phase2Installed)
                    installPhase2();
            }, 10000);
        }
    }
}
/**
 * Phase 2 (Deferred): Heavy interceptors.
 */
export function installPhase2() {
    // Double-injection guard
    if (phase2Installed)
        return;
    // Environment guard
    if (typeof window === 'undefined' || typeof document === 'undefined')
        return;
    console.log('[KaBOOM!] Phase 2 installing (heavy interceptors: console, fetch, WS, errors, actions)');
    phase2Timestamp = performance.now();
    phase2Installed = true;
    // Install all heavy interceptors
    install();
    // Adopt fetch/XHR bodies buffered by the early-patch script
    adoptEarlyBodies();
    // FCP/LCP/CLS/INP/long-task observers (buffered: true replays pre-Phase-2 entries)
    installPerfObservers();
}
export function getDeferralState() {
    return {
        deferralEnabled,
        phase2Installed,
        injectionTimestamp,
        phase2Timestamp
    };
}
/**
 * Set whether interception deferral is enabled.
 */
export function setDeferralEnabled(enabled) {
    deferralEnabled = enabled;
}
//# sourceMappingURL=observers.js.map