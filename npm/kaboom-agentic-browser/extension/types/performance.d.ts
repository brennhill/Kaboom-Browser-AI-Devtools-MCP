/**
 * Purpose: Defines performance timing/resource type contracts used by capture snapshots and performance tooling.
 * Why: Keeps browser performance payloads compatible with server-side regression/diff analysis.
 * Docs: docs/features/feature/performance-audit/index.md
 */
/**
 * @fileoverview Performance Types
 * Performance marks, measures, long tasks, and web vitals
 */
/**
 * Performance mark entry (browser-side only, not a wire type)
 */
export interface PerformanceMark {
    readonly name: string;
    readonly startTime: number;
    readonly entryType: 'mark';
}
/**
 * Performance measure entry (browser-side only, not a wire type)
 */
export interface PerformanceMeasure {
    readonly name: string;
    readonly startTime: number;
    readonly duration: number;
    readonly entryType: 'measure';
}
/**
 * Performance snapshot — re-exported from wire type (canonical HTTP payload shape).
 * The stale interface previously used camelCase fields (vitals, longTasks, totalSize, etc.)
 * that didn't match the actual runtime data or Go server expectations.
 */
export type { WirePerformanceSnapshot as PerformanceSnapshot } from './wire-performance-snapshot.js';
//# sourceMappingURL=performance.d.ts.map