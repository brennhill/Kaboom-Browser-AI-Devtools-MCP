/**
 * Purpose: Observes web vitals (FCP, LCP, CLS, INP), long tasks, and resource timing to build comprehensive performance snapshots.
 * Docs: docs/features/feature/performance-audit/index.md
 */
interface ResourceByType {
    count: number;
    size: number;
}
interface SlowRequest {
    url: string;
    duration: number;
    size: number;
}
interface ResourceTimingSummary {
    request_count: number;
    transfer_size: number;
    decoded_size: number;
    by_type: Record<string, ResourceByType>;
    slowest_requests: SlowRequest[];
}
interface LongTaskMetrics {
    count: number;
    total_blocking_time: number;
    longest: number;
}
interface NetworkTiming {
    dom_content_loaded: number;
    load: number;
    first_contentful_paint: number | null;
    largest_contentful_paint: number | null;
    interaction_to_next_paint: number | null;
    time_to_first_byte: number;
    dom_interactive: number;
}
interface UserTimingEntry {
    name: string;
    start_time: number;
    duration?: number;
}
interface PerformanceSnapshotData {
    url: string;
    timestamp: string;
    timing: NetworkTiming;
    network: ResourceTimingSummary;
    long_tasks: LongTaskMetrics;
    cumulative_layout_shift: number;
    user_timing?: {
        marks: UserTimingEntry[];
        measures: UserTimingEntry[];
    };
}
/**
 * Map resource initiator types to standard categories
 */
export declare function mapInitiatorType(type: string): string;
/**
 * Aggregate resource timing entries into a network summary
 */
export declare function aggregateResourceTiming(): ResourceTimingSummary;
/**
 * Capture a performance snapshot with navigation timing and network summary
 */
export declare function capturePerformanceSnapshot(): PerformanceSnapshotData | null;
/**
 * Install performance observers for long tasks, paint, LCP, and CLS
 */
export declare function installPerfObservers(): void;
/**
 * Disconnect all performance observers
 */
export declare function uninstallPerfObservers(): void;
/**
 * Get accumulated long task metrics
 */
export declare function getLongTaskMetrics(): LongTaskMetrics;
/**
 * Get First Contentful Paint value
 */
export declare function getFCP(): number | null;
/**
 * Get Largest Contentful Paint value
 */
export declare function getLCP(): number | null;
/**
 * Get Cumulative Layout Shift value
 */
export declare function getCLS(): number;
/**
 * Get Interaction to Next Paint value
 */
export declare function getINP(): number | null;
/**
 * Send performance snapshot via postMessage to content script
 */
export declare function sendPerformanceSnapshot(): void;
/**
 * Schedule a debounced re-send of the performance snapshot.
 * Called when user timing marks/measures are created to keep server data fresh.
 */
export declare function scheduleSnapshotResend(): void;
/**
 * Check if performance snapshot capture is enabled
 */
export declare function isPerformanceSnapshotEnabled(): boolean;
/**
 * Enable or disable performance snapshot capture
 */
export declare function setPerformanceSnapshotEnabled(enabled: boolean): void;
export {};
//# sourceMappingURL=perf-snapshot.d.ts.map