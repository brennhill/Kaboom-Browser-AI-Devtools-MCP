/**
 * Purpose: Injects inject.bundled.js into the page MAIN world and syncs stored settings to the inject context.
 * Docs: docs/features/feature/csp-safe-execution/index.md
 */
/** Get the page nonce for authenticating postMessages to inject.js */
export declare function getPageNonce(): string;
/** Check if inject script has been loaded into the page context */
export declare function isInjectScriptLoaded(): boolean;
/**
 * Ensure inject script is present, deduplicating concurrent inject attempts.
 * Optionally force a fresh reinjection attempt.
 */
export declare function ensureInjectScriptReady(timeoutMs?: number, force?: boolean): Promise<boolean>;
/**
 * Ensure inject bridge responds to a ping, proving MAIN-world messaging is live.
 */
export declare function ensureInjectBridgeReady(timeoutMs?: number): Promise<boolean>;
/**
 * Initialize script injection (call when DOM is ready)
 */
export declare function initScriptInjection(force?: boolean): void;
//# sourceMappingURL=script-injection.d.ts.map