/**
 * Purpose: Wraps performance.mark/measure to capture calls and uses PerformanceObserver for error-time performance snapshots.
 * Docs: docs/features/feature/performance-audit/index.md
 */
import type { PerformanceMark, PerformanceMeasure } from '../types/index.js';
/**
 * Get performance marks
 */
export declare function getPerformanceMarks(options?: {
    since?: number;
}): Array<Omit<PerformanceMark, 'entryType'> & {
    detail?: unknown | null;
}>;
/**
 * Get performance measures
 */
export declare function getPerformanceMeasures(options?: {
    since?: number;
}): Array<Omit<PerformanceMeasure, 'entryType'>>;
/**
 * Get captured marks from wrapper
 */
export declare function getCapturedMarks(): Array<PerformanceMark & {
    detail?: unknown;
    capturedAt: string;
}>;
/**
 * Get captured measures from wrapper
 */
export declare function getCapturedMeasures(): Array<PerformanceMeasure & {
    capturedAt: string;
}>;
/**
 * Install performance capture wrapper
 */
export declare function installPerformanceCapture(): void;
/**
 * Uninstall performance capture wrapper
 */
export declare function uninstallPerformanceCapture(): void;
/**
 * Check if performance capture is active
 */
export declare function isPerformanceCaptureActive(): boolean;
interface PerformanceSnapshot {
    type: 'performance';
    ts: string;
    _enrichments: readonly string[];
    _errorTs?: string;
    marks: Array<Omit<PerformanceMark, 'entryType'> & {
        detail?: unknown | null;
    }>;
    measures: Array<Omit<PerformanceMeasure, 'entryType'>>;
    navigation: {
        type?: string;
        startTime: number;
        domContentLoadedEventEnd: number;
        loadEventEnd: number;
    } | null;
}
/**
 * Get performance snapshot for an error
 */
export declare function getPerformanceSnapshotForError(errorEntry: {
    ts?: string;
}): Promise<PerformanceSnapshot | null>;
/**
 * Set whether performance marks are enabled
 */
export declare function setPerformanceMarksEnabled(enabled: boolean): void;
/**
 * Check if performance marks are enabled
 */
export declare function isPerformanceMarksEnabled(): boolean;
export {};
//# sourceMappingURL=performance.d.ts.map