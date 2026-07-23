/**
 * Purpose: Registers and manages runtime observers for DOM mutations, network requests, performance entries, and WebSocket events in the page context.
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * Wrap fetch to capture network errors
 */
export declare function wrapFetch(originalFetchFn: typeof fetch): typeof fetch;
/**
 * Install fetch capture.
 * Uses wrapFetchWithBodies to capture request/response bodies for all requests,
 * then wraps that with wrapFetch to also capture error details for 4xx/5xx responses.
 * If the early-patch script ran first, uses the saved original fetch (not the early wrapper).
 */
export declare function installFetchCapture(): void;
/**
 * Install XHR body capture (wraps XMLHttpRequest.prototype.open/send)
 */
export declare function installXHRCapture(): void;
/**
 * Uninstall XHR body capture
 */
export declare function uninstallXHRCapture(): void;
/**
 * Uninstall fetch capture
 */
export declare function uninstallFetchCapture(): void;
/**
 * Install all capture hooks
 */
export declare function install(): void;
/**
 * Uninstall all capture hooks
 */
export declare function uninstall(): void;
/**
 * Check if heavy intercepts should be deferred until page load
 */
export declare function shouldDeferIntercepts(): boolean;
/**
 * Memory pressure check state
 */
interface MemoryPressureState {
    memoryUsageMB: number;
    networkBodiesEnabled: boolean;
    wsBufferCapacity: number;
    networkBufferCapacity: number;
}
/**
 * Check memory pressure and adjust buffer capacities
 */
export declare function checkMemoryPressure(state: MemoryPressureState): MemoryPressureState;
/**
 * Phase 1 (Immediate): Lightweight, non-intercepting setup.
 */
export declare function installPhase1(): void;
/**
 * Phase 2 (Deferred): Heavy interceptors.
 */
export declare function installPhase2(): void;
/**
 * Get the current deferral state for diagnostics and testing.
 */
export interface DeferralState {
    deferralEnabled: boolean;
    phase2Installed: boolean;
    injectionTimestamp: number;
    phase2Timestamp: number;
}
export declare function getDeferralState(): DeferralState;
/**
 * Set whether interception deferral is enabled.
 */
export declare function setDeferralEnabled(enabled: boolean): void;
export {};
//# sourceMappingURL=observers.d.ts.map