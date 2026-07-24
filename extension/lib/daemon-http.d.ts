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
export interface DaemonHeaderOptions {
    clientName?: string;
    extensionVersion?: string;
    contentType?: string | null;
    additionalHeaders?: Record<string, string>;
}
export interface DaemonJSONRequestOptions extends DaemonHeaderOptions {
    method?: string;
    signal?: AbortSignal;
}
export interface PostDaemonJSONOptions extends DaemonJSONRequestOptions {
    timeoutMs?: number;
}
/**
 * Build standard daemon request headers with a shared extension client identifier.
 */
export declare function buildDaemonHeaders(options?: DaemonHeaderOptions): Record<string, string>;
/**
 * Build a JSON request init object for daemon endpoints.
 */
export declare function buildDaemonJSONRequestInit(payload: unknown, options?: DaemonJSONRequestOptions): RequestInit;
/**
 * POST JSON to a daemon endpoint with optional timeout handling.
 */
export declare function postDaemonJSON(url: string, payload: unknown, options?: PostDaemonJSONOptions): Promise<Response>;
//# sourceMappingURL=daemon-http.d.ts.map