/**
 * Purpose: Executes JavaScript in page context with world-aware routing (content script relay, chrome.scripting, or CSP-safe structured executor).
 * Docs: docs/features/feature/csp-safe-execution/index.md
 */
/** Result of probing a tab's Content Security Policy restrictions */
export interface CSPProbeResult {
    csp_restricted: boolean;
    csp_level: 'none' | 'script_exec' | 'page_blocked';
}
/**
 * Probe whether a tab's CSP blocks dynamic script execution (new Function).
 * Returns one of three levels:
 * - "none": No CSP restrictions — execute_js is safe
 * - "script_exec": new Function() blocked — use dom/get_readable instead
 * - "page_blocked": Privileged URL (chrome://, devtools://) — no extension access
 */
export declare function probeCSPStatus(tabId: number): Promise<CSPProbeResult>;
/** Result shape from JS execution */
export interface ExecutionResult {
    success: boolean;
    error?: string;
    message?: string;
    result?: unknown;
    stack?: string;
    execution_mode?: string;
}
/**
 * Execute JavaScript via chrome.scripting.executeScript.
 * Used as fallback when MAIN world execution fails due to page CSP,
 * or when inject script is not loaded.
 * The func is injected natively by Chrome's extension system.
 */
export declare function executeViaScriptingAPI(tabId: number, script: string, timeoutMs: number, world?: 'MAIN' | 'ISOLATED'): Promise<ExecutionResult>;
/**
 * Execute JS with world-aware routing.
 * - isolated: structured executor in ISOLATED world (skips new Function — always fails in MV3)
 * - main: send to content script (MAIN world via inject)
 * - auto: try content script → scripting API MAIN → structured executor MAIN
 */
export declare function executeWithWorldRouting(tabId: number, queryParams: string | Record<string, unknown>, world: string): Promise<ExecutionResult>;
//# sourceMappingURL=query-execution.d.ts.map