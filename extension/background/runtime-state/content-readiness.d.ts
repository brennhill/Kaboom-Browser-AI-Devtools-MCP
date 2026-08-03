/**
 * Purpose: Gates post-navigation content commands on a correlation-matched content-script acknowledgement.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
export interface ContentReadinessAcknowledgement {
    readonly ready: true;
    readonly correlation_id: string;
    readonly connection_generation: number;
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
    readonly probe: (tabId: number, correlationId: string, connectionGeneration: number) => Promise<ContentReadinessAcknowledgement | undefined>;
    readonly wait: (delayMs: number) => Promise<void>;
    readonly get_generation?: () => number;
    readonly delays_ms?: readonly number[];
    readonly onReady?: (tabId: number, correlationId: string, attempts: number) => void;
    readonly onTimeout?: (tabId: number, correlationId: string, attempts: number) => void;
    readonly onSuperseded?: (tabId: number, correlationId: string, expectedGeneration: number, currentGeneration: number) => void;
}
export declare function requiresContentReadiness(queryType: string): boolean;
export declare class ContentReadinessBarrier {
    private readonly pending;
    private readonly probe;
    private readonly wait;
    private readonly delaysMs;
    private readonly onReady?;
    private readonly onTimeout?;
    private readonly onSuperseded?;
    private readonly getGeneration;
    constructor(options: ContentReadinessOptions);
    begin(tabId: number, correlationId: string): void;
    hasPending(tabId: number): boolean;
    cancel(tabId: number, correlationId: string): void;
    waitUntilReady(tabId: number): Promise<ContentReadinessResult>;
}
export declare const contentReadiness: ContentReadinessBarrier;
export {};
//# sourceMappingURL=content-readiness.d.ts.map