/**
 * Purpose: Enforces memory limits, screenshot rate limiting, and source map LRU caching to prevent resource exhaustion.
 * Docs: docs/features/feature/memory-enforcement/index.md
 */
/**
 * @fileoverview Cache Limits and Memory Management
 *
 * Implements rate limiting and DoS protection for Kaboom:
 *
 * RATE LIMITING:
 * - Screenshot rate limit: 1 per 5 seconds per tab
 * - Screenshot session limit: 30 total per minute per tab
 * - Error group deduplication: 5-second window (identical errors grouped)
 * - Max pending requests: 1000 (circuit breaker if exceeded)
 *
 * MEMORY ENFORCEMENT:
 * - Soft limit: 20MB (reduce capacities, disable some captures)
 * - Hard limit: 50MB (disable network body capture completely)
 * - Checks every 30 seconds via alarm
 * - Estimated using average sizes: log entry 500B, WS event 300B, network body 1KB
 *
 * SOURCE MAP CACHING:
 * - Max 50 source maps in cache (LRU eviction when full)
 * - Maps are parsed SourceMap objects (can be large)
 * - Cache is cleared when source map feature is disabled
 *
 * SECURITY PROPERTIES:
 * - Prevents memory exhaustion attacks
 * - Prevents connection storms via backoff
 * - Prevents duplicate error flooding via deduplication
 * - Graceful degradation under memory pressure
 *
 * Manages source map caching with LRU eviction, screenshot rate limiting,
 * and memory pressure monitoring.
 */
import type { BufferState, MemoryPressureLevel, MemoryPressureState, ParsedSourceMap } from '../types/index.js';
/** Source map cache size limit */
export declare const SOURCE_MAP_CACHE_SIZE = 50;
/** Memory limits */
export declare const MEMORY_SOFT_LIMIT: number;
export declare const MEMORY_HARD_LIMIT: number;
export declare const MEMORY_CHECK_INTERVAL_MS = 30000;
export declare const MEMORY_AVG_LOG_ENTRY_SIZE = 500;
export declare const MEMORY_AVG_WS_EVENT_SIZE = 300;
export declare const MEMORY_AVG_NETWORK_BODY_SIZE = 1000;
export declare const MEMORY_AVG_ACTION_SIZE = 400;
/** Maximum pending buffer size */
export declare const MAX_PENDING_BUFFER = 1000;
/** Rate limit result */
interface RateLimitResult {
    allowed: boolean;
    reason?: 'session_limit' | 'rate_limit';
    nextAllowedIn?: number | null;
}
/**
 * Check if a screenshot is allowed based on rate limiting
 */
export declare function canTakeScreenshot(tabId: number): RateLimitResult;
/**
 * Record a screenshot timestamp
 */
export declare function recordScreenshot(tabId: number): void;
/**
 * Clear screenshot timestamps for a tab
 */
export declare function clearScreenshotTimestamps(tabId: number): void;
/**
 * Estimate total buffer memory usage from buffer contents
 */
export declare function estimateBufferMemory(buffers: BufferState): number;
/**
 * Check memory pressure and take appropriate action
 */
export declare function checkMemoryPressure(buffers: BufferState): {
    level: MemoryPressureLevel;
    action: string;
    estimatedMemory: number;
    alreadyApplied: boolean;
};
/**
 * Get the current memory pressure state
 */
export declare function getMemoryPressureState(): MemoryPressureState;
/**
 * Reset memory pressure state to initial values (for testing)
 */
export declare function resetMemoryPressureState(): void;
/**
 * Check if network body capture is disabled
 */
export declare function isNetworkBodyCaptureDisabled(): boolean;
/**
 * Set source map enabled state
 */
export declare function setSourceMapEnabled(enabled: boolean): void;
/**
 * Check if source maps are enabled
 */
export declare function isSourceMapEnabled(): boolean;
/**
 * Set an entry in the source map cache with LRU eviction
 */
export declare function setSourceMapCacheEntry(url: string, map: ParsedSourceMap | null): void;
/**
 * Get an entry from the source map cache
 */
export declare function getSourceMapCacheEntry(url: string): ParsedSourceMap | null;
/**
 * Get the current size of the source map cache
 */
export declare function getSourceMapCacheSize(): number;
/**
 * Clear the source map cache
 */
export declare function clearSourceMapCache(): void;
export {};
//# sourceMappingURL=cache-limits.d.ts.map