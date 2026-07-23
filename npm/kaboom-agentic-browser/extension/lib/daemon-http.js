/**
 * Purpose: Shared daemon HTTP request helpers (headers + JSON body init) used by extension modules.
 * Why: Keep daemon request contracts consistent across background, popup, and options surfaces.
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