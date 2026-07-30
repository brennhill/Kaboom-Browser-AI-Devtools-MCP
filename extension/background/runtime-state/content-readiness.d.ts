/**
 * Purpose: Gates post-navigation content commands on a correlation-matched content-script acknowledgement.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
export interface ContentReadinessAcknowledgement {
    readonly ready: true;
    readonly correlation_id: string;
}
export type ContentReadinessResult = {
    readonly ready: true;
    readonly correlation_id: string;
    readonly attempts: number;
} | {
    readonly ready: false;
    readonly correlation_id: string;
    readonly attempts: number;
    readonly error: 'content_readiness_timeout' | 'readiness_superseded';
};
interface ContentReadinessOptions {
    readonly probe: (tabId: number, correlationId: string) => Promise<ContentReadinessAcknowledgement | undefined>;
    readonly wait: (delayMs: number) => Promise<void>;
    readonly delays_ms?: readonly number[];
    readonly onReady?: (tabId: number, correlationId: string, attempts: number) => void;
    readonly onTimeout?: (tabId: number, correlationId: string, attempts: number) => void;
}
export declare function requiresContentReadiness(queryType: string): boolean;
export declare class ContentReadinessBarrier {
    private readonly pending;
    private readonly probe;
    private readonly wait;
    private readonly delaysMs;
    private readonly onReady?;
    private readonly onTimeout?;
    constructor(options: ContentReadinessOptions);
    begin(tabId: number, correlationId: string): void;
    hasPending(tabId: number): boolean;
    cancel(tabId: number, correlationId: string): void;
    waitUntilReady(tabId: number): Promise<ContentReadinessResult>;
}
export declare const contentReadiness: ContentReadinessBarrier;
export {};
//# sourceMappingURL=content-readiness.d.ts.map