/**
 * Purpose: Captures and restores browser state snapshots (localStorage, cookies, scroll position) and manages element highlighting for the AI Web Pilot.
 * Docs: docs/features/feature/state-time-travel/index.md
 */
/**
 * @fileoverview State Management - Handles browser state capture/restore and
 * element highlighting for the AI Web Pilot.
 */
import type { BrowserStateSnapshot } from '../types/index.js';
/**
 * Highlight result
 */
export interface HighlightResult {
    success: boolean;
    selector?: string;
    bounds?: {
        x: number;
        y: number;
        width: number;
        height: number;
    };
    error?: string;
}
/**
 * Restored state counts
 */
export interface RestoredCounts {
    localStorage: number;
    sessionStorage: number;
    cookies: number;
    skipped: number;
}
/**
 * Restore state result
 */
export interface RestoreStateResult {
    success: boolean;
    restored?: RestoredCounts;
    error?: string;
}
/**
 * Capture browser state (localStorage, sessionStorage, cookies).
 * Returns a snapshot that can be restored later.
 */
export declare function captureState(): BrowserStateSnapshot;
export declare function restoreState(state: BrowserStateSnapshot, includeUrl?: boolean): RestoreStateResult;
/**
 * Highlight a DOM element by injecting a blue glow overlay div.
 */
export declare function highlightElement(selector: string, durationMs?: number): HighlightResult | undefined;
/**
 * Clear any existing highlight
 */
export declare function clearHighlight(): void;
/**
 * Wrapper for sending performance snapshot (exported for compatibility)
 */
export declare function sendPerformanceSnapshotWrapper(): void;
//# sourceMappingURL=state.d.ts.map