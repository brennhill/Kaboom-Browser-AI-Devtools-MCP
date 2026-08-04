/**
 * Purpose: Owns authenticated page-to-content messages emitted by the injected runtime.
 * Why: Every page-context producer must use the same per-injection nonce boundary.
 */
export declare function getInjectedPageNonce(): string;
export declare function postAuthenticatedPageMessage(message: object): void;
//# sourceMappingURL=channel.d.ts.map