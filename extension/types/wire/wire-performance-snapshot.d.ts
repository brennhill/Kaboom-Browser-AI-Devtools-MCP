/**
 * @fileoverview Wire types for performance snapshots — matches internal/performance/wire_performance.go
 *
 * Canonical TypeScript definitions for the PerformanceSnapshot HTTP payload.
 * Changes here MUST be mirrored in the Go counterpart. Run `make check-wire-drift`.
 */
/**
 * WirePerformanceTiming holds navigation timing metrics.
 */
export interface WirePerformanceTiming {
    readonly dom_content_loaded: number;
    readonly load: number;
    readonly first_contentful_paint: number | null;
    readonly largest_contentful_paint: number | null;
    readonly interaction_to_next_paint?: number | null;
    readonly time_to_first_byte: number;
    readonly dom_interactive: number;
}
/**
 * WireTypeSummary holds per-type resource metrics.
 */
export interface WireTypeSummary {
    readonly count: number;
    readonly size: number;
}
/**
 * WireSlowRequest represents one of the slowest network requests.
 */
export interface WireSlowRequest {
    readonly url: string;
    readonly duration: number;
    readonly size: number;
}
/**
 * WireNetworkSummary holds aggregated network resource metrics.
 */
export interface WireNetworkSummary {
    readonly request_count: number;
    readonly transfer_size: number;
    readonly decoded_size: number;
    readonly by_type: Readonly<Record<string, WireTypeSummary>>;
    readonly slowest_requests: readonly WireSlowRequest[];
}
/**
 * WireLongTaskMetrics holds accumulated long task data.
 */
export interface WireLongTaskMetrics {
    readonly count: number;
    readonly total_blocking_time: number;
    readonly longest: number;
}
/**
 * WireUserTimingEntry represents a single performance mark or measure.
 */
export interface WireUserTimingEntry {
    readonly name: string;
    readonly start_time: number;
    readonly duration?: number;
}
/**
 * WireUserTimingData holds captured performance.mark() and performance.measure() entries.
 */
export interface WireUserTimingData {
    readonly marks: readonly WireUserTimingEntry[];
    readonly measures: readonly WireUserTimingEntry[];
}
/**
 * WirePerformanceSnapshot is the JSON shape sent over HTTP for performance data.
 */
export interface WirePerformanceSnapshot {
    readonly url: string;
    readonly timestamp: string;
    readonly timing: WirePerformanceTiming;
    readonly network: WireNetworkSummary;
    readonly long_tasks: WireLongTaskMetrics;
    readonly cumulative_layout_shift?: number | null;
    readonly user_timing?: WireUserTimingData;
}
//# sourceMappingURL=wire-performance-snapshot.d.ts.map