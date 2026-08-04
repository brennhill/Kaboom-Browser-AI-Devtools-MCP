/**
 * Purpose: Correlate bounded page-level request initiators and response metadata with resource timings.
 * Docs: docs/features/feature/network-performance-attribution/index.md
 */
import type { WireNetworkWaterfallEntry } from '../../types/wire/wire-network.js';
export interface RequestAttributionStart {
    stack?: string;
    priority?: string;
}
export interface RequestAttributionFinish {
    status?: number;
    server_timing?: string | null;
    request_id?: string | null;
    traceparent?: string | null;
    content_encoding?: string | null;
}
export declare function recordRequestAttribution(url: string, start?: RequestAttributionStart): void;
export declare function completeRequestAttribution(url: string, finish: RequestAttributionFinish): void;
export declare function enrichWaterfallEntries(entries: WireNetworkWaterfallEntry[]): WireNetworkWaterfallEntry[];
export declare function resetRequestAttribution(): void;
//# sourceMappingURL=request-attribution.d.ts.map