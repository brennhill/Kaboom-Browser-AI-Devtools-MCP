/**
 * Purpose: Shared daemon HTTP request helpers (headers + JSON body init) used by extension modules.
 * Why: Keep daemon request contracts consistent across background, popup, and options surfaces.
 *
 * Canonical daemon-fetch pattern — follow this for every new daemon call site:
 *
 *   1. Build the URL as a template literal with the path suffix pinned
 *      directly after the serverUrl (e.g. `${serverUrl}/health`). The URL
 *      must literally contain the path string so that
 *      scripts/check-openapi-url-drift.js can statically verify the path
 *      exists in cmd/browser-agent/openapi.json.
 *
 *   2. Use the helpers in this module (postDaemonJSON, buildDaemonJSONRequestInit,
 *      or buildDaemonHeaders) rather than hand-rolling fetch init objects. This
 *      keeps Origin + X-Kaboom-Client headers identical across callers, which is
 *      what extensionOnly middleware matches on.
 *
 *   3. Type the response via src/generated/openapi-types.ts:
 *        import type { components, operations } from '../generated/openapi-types.js'
 *        type Health = components['schemas']['HealthResponse']
 *      This is the ONLY guard against field-name drift (e.g. `available_version`
 *      vs `availableVersion`) — tsc will reject typos at compile time. Untyped
 *      response consumers are drift-vulnerable; prefer the generated types.
 *
 *   4. If the endpoint isn't in openapi.json yet, add it before landing the
 *      caller. The URL-drift check in CI blocks new unspec'd paths.
 */
const DEFAULT_CLIENT_NAME = 'kaboom-extension';
/**
 * Build standard daemon request headers with a shared extension client identifier.
 */
export function buildDaemonHeaders(options = {}) {
    const { clientName = DEFAULT_CLIENT_NAME, extensionVersion, contentType = 'application/json', additionalHeaders = {} } = options;
    const normalizedVersion = typeof extensionVersion === 'string' && extensionVersion.trim().length > 0
        ? extensionVersion.trim()
        : '';
    const headers = {
        'X-Kaboom-Client': normalizedVersion ? `${clientName}/${normalizedVersion}` : clientName
    };
    if (contentType !== null) {
        headers['Content-Type'] = contentType;
    }
    if (normalizedVersion) {
        headers['X-Kaboom-Extension-Version'] = normalizedVersion;
    }
    return {
        ...headers,
        ...additionalHeaders
    };
}
/**
 * Build a JSON request init object for daemon endpoints.
 */
export function buildDaemonJSONRequestInit(payload, options = {}) {
    const { method = 'POST', signal, ...headerOptions } = options;
    return {
        method,
        headers: buildDaemonHeaders(headerOptions),
        body: JSON.stringify(payload),
        ...(signal ? { signal } : {})
    };
}
/**
 * POST JSON to a daemon endpoint with optional timeout handling.
 */
export async function postDaemonJSON(url, payload, options = {}) {
    const { timeoutMs, signal, ...requestOptions } = options;
    const effectiveSignal = signal ||
        (typeof timeoutMs === 'number' && timeoutMs > 0 && typeof AbortSignal.timeout === 'function'
            ? AbortSignal.timeout(timeoutMs)
            : undefined);
    return fetch(url, buildDaemonJSONRequestInit(payload, { ...requestOptions, signal: effectiveSignal }));
}
//# sourceMappingURL=daemon-http.js.map