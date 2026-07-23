/**
 * Purpose: Shared daemon HTTP request helpers (headers + JSON body init) used by extension modules.
 * Why: Keep daemon request contracts consistent across background, popup, and options surfaces.
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