/**
 * @fileoverview Wire types for network telemetry — matches internal/types/wire_network.go
 *
 * Canonical TypeScript definitions for NetworkBody and NetworkWaterfall HTTP payloads.
 * Changes here MUST be mirrored in the Go counterpart. Run `make check-wire-drift`.
 */
/**
 * WireNetworkBody is the JSON shape for captured network request/response bodies.
 */
export interface WireNetworkBody {
    readonly method: string;
    readonly url: string;
    readonly status: number;
    readonly request_body?: string;
    readonly response_body?: string;
    readonly content_type?: string;
    readonly duration?: number;
    readonly request_truncated?: boolean;
    readonly response_truncated?: boolean;
    readonly tab_id?: number;
}
/**
 * WireNetworkWaterfallEntry is the JSON shape for a single PerformanceResourceTiming entry.
 */
export interface WireNetworkWaterfallEntry {
    readonly name: string;
    readonly url: string;
    readonly initiator_type: string;
    readonly duration: number;
    readonly start_time: number;
    readonly fetch_start?: number;
    readonly response_end?: number;
    readonly transfer_size: number;
    readonly decoded_body_size: number;
    readonly encoded_body_size: number;
    readonly page_url?: string;
    readonly queueing_ms?: number;
    readonly dns_ms?: number;
    readonly tls_ms?: number;
    readonly connect_ms?: number;
    readonly ttfb_ms?: number;
    readonly download_ms?: number;
    readonly priority?: string;
    readonly protocol?: string;
    readonly cache_source?: string;
    readonly compression_ratio?: number;
    readonly content_encoding?: string;
    readonly status?: number;
    readonly server_timing?: readonly WireServerTiming[];
    readonly request_id?: string;
    readonly traceparent?: string;
    readonly initiator_stack?: readonly string[];
    readonly react_component?: string;
    readonly route_loader?: string;
    readonly store_action?: string;
    readonly source_map_status?: string;
    readonly duplicate_group_id?: string;
    readonly duplicate_count?: number;
}
export interface WireServerTiming {
    readonly name: string;
    readonly duration_ms?: number;
    readonly description?: string;
}
/**
 * WireNetworkWaterfallPayload is the top-level shape POSTed to /network-waterfall.
 */
export interface WireNetworkWaterfallPayload {
    readonly entries: readonly WireNetworkWaterfallEntry[];
    readonly page_url: string;
}
//# sourceMappingURL=wire-network.d.ts.map