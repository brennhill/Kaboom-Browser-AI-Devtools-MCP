/**
 * Purpose: Enforces memory limits, screenshot rate limiting, and source map LRU caching to prevent resource exhaustion.
 * Docs: docs/features/feature/memory-enforcement/index.md
 */
// =============================================================================
// CONSTANTS
// =============================================================================
/** Screenshot rate limit in milliseconds */
const SCREENSHOT_RATE_LIMIT_MS = 5000;
/** Maximum screenshots per session */
const SCREENSHOT_MAX_PER_SESSION = 30;
/** Source map cache size limit */
export const SOURCE_MAP_CACHE_SIZE = 50;
/** Memory limits */
export const MEMORY_SOFT_LIMIT = 20 * 1024 * 1024;
export const MEMORY_HARD_LIMIT = 50 * 1024 * 1024;
export const MEMORY_CHECK_INTERVAL_MS = 30000;
export const MEMORY_AVG_LOG_ENTRY_SIZE = 500;
export const MEMORY_AVG_WS_EVENT_SIZE = 300;
export const MEMORY_AVG_NETWORK_BODY_SIZE = 1000;
export const MEMORY_AVG_ACTION_SIZE = 400;
/** Maximum pending buffer size */
export const MAX_PENDING_BUFFER = 1000;
// =============================================================================
// STATE
// =============================================================================
/** Screenshot rate limiting state */
const screenshotTimestamps = new Map();
/**
 * Source map cache — intentionally NOT persisted across service worker restarts.
 * Source maps are a cache (not state) and will rebuild on next error occurrence.
 * Persisting parsed source maps would be expensive and unnecessary.
 */
const sourceMapCache = new Map();
/** Memory pressure state */
let memoryPressureLevel = 'normal';
let lastMemoryCheck = 0;
let networkBodyCaptureDisabled = false;
let reducedCapacities = false;
/** Source map enabled flag */
let sourceMapEnabled = false;
// =============================================================================
// SCREENSHOT RATE LIMITING
// =============================================================================
/**
 * Check if a screenshot is allowed based on rate limiting
 */
export function canTakeScreenshot(tabId) {
    const now = Date.now();
    if (!screenshotTimestamps.has(tabId)) {
        screenshotTimestamps.set(tabId, []);
    }
    const timestamps = screenshotTimestamps.get(tabId);
    const recentTimestamps = timestamps.filter((t) => now - t < 60000);
    if (recentTimestamps.length >= SCREENSHOT_MAX_PER_SESSION) {
        return { allowed: false, reason: 'session_limit', nextAllowedIn: null };
    }
    const lastTimestamp = recentTimestamps[recentTimestamps.length - 1];
    if (lastTimestamp && now - lastTimestamp < SCREENSHOT_RATE_LIMIT_MS) {
        return {
            allowed: false,
            reason: 'rate_limit',
            nextAllowedIn: SCREENSHOT_RATE_LIMIT_MS - (now - lastTimestamp)
        };
    }
    return { allowed: true };
}
/**
 * Record a screenshot timestamp
 */
export function recordScreenshot(tabId) {
    if (!screenshotTimestamps.has(tabId)) {
        screenshotTimestamps.set(tabId, []);
    }
    screenshotTimestamps.get(tabId).push(Date.now());
}
/**
 * Clear screenshot timestamps for a tab
 */
export function clearScreenshotTimestamps(tabId) {
    screenshotTimestamps.delete(tabId);
}
// =============================================================================
// MEMORY ENFORCEMENT
// =============================================================================
/**
 * Estimate total buffer memory usage from buffer contents
 */
export function estimateBufferMemory(buffers) {
    let total = 0;
    total += buffers.logEntries.length * MEMORY_AVG_LOG_ENTRY_SIZE;
    for (const event of buffers.wsEvents) {
        total += MEMORY_AVG_WS_EVENT_SIZE;
        if (event.data && typeof event.data === 'string') {
            total += event.data.length;
        }
    }
    for (const body of buffers.networkBodies) {
        total += MEMORY_AVG_NETWORK_BODY_SIZE;
        if (body.request_body && typeof body.request_body === 'string') {
            total += body.request_body.length;
        }
        if (body.response_body && typeof body.response_body === 'string') {
            total += body.response_body.length;
        }
    }
    total += buffers.enhancedActions.length * MEMORY_AVG_ACTION_SIZE;
    return total;
}
/**
 * Check memory pressure and take appropriate action
 */
export function checkMemoryPressure(buffers) {
    const estimatedMemory = estimateBufferMemory(buffers);
    lastMemoryCheck = Date.now();
    if (estimatedMemory >= MEMORY_HARD_LIMIT) {
        const alreadyApplied = memoryPressureLevel === 'hard';
        memoryPressureLevel = 'hard';
        networkBodyCaptureDisabled = true;
        reducedCapacities = true;
        return {
            level: 'hard',
            action: 'disable_network_capture',
            estimatedMemory,
            alreadyApplied
        };
    }
    if (estimatedMemory >= MEMORY_SOFT_LIMIT) {
        const alreadyApplied = memoryPressureLevel === 'soft' || memoryPressureLevel === 'hard';
        memoryPressureLevel = 'soft';
        reducedCapacities = true;
        if (networkBodyCaptureDisabled && estimatedMemory < MEMORY_HARD_LIMIT) {
            networkBodyCaptureDisabled = false;
        }
        return {
            level: 'soft',
            action: 'reduce_capacities',
            estimatedMemory,
            alreadyApplied
        };
    }
    memoryPressureLevel = 'normal';
    reducedCapacities = false;
    networkBodyCaptureDisabled = false;
    return {
        level: 'normal',
        action: 'none',
        estimatedMemory,
        alreadyApplied: false
    };
}
/**
 * Get the current memory pressure state
 */
export function getMemoryPressureState() {
    return {
        memoryPressureLevel,
        lastMemoryCheck,
        networkBodyCaptureDisabled,
        reducedCapacities
    };
}
/**
 * Reset memory pressure state to initial values (for testing)
 */
export function resetMemoryPressureState() {
    memoryPressureLevel = 'normal';
    lastMemoryCheck = 0;
    networkBodyCaptureDisabled = false;
    reducedCapacities = false;
}
/**
 * Check if network body capture is disabled
 */
export function isNetworkBodyCaptureDisabled() {
    return networkBodyCaptureDisabled;
}
// =============================================================================
// SOURCE MAP CACHE MANAGEMENT
// =============================================================================
/**
 * Set source map enabled state
 */
export function setSourceMapEnabled(enabled) {
    sourceMapEnabled = enabled;
}
/**
 * Check if source maps are enabled
 */
export function isSourceMapEnabled() {
    return sourceMapEnabled;
}
/**
 * Set an entry in the source map cache with LRU eviction
 */
export function setSourceMapCacheEntry(url, map) {
    if (!sourceMapCache.has(url) && sourceMapCache.size >= SOURCE_MAP_CACHE_SIZE) {
        const firstKey = sourceMapCache.keys().next().value;
        if (firstKey) {
            sourceMapCache.delete(firstKey);
        }
    }
    sourceMapCache.delete(url);
    sourceMapCache.set(url, map);
}
/**
 * Get an entry from the source map cache
 */
export function getSourceMapCacheEntry(url) {
    return sourceMapCache.get(url) || null;
}
/**
 * Get the current size of the source map cache
 */
export function getSourceMapCacheSize() {
    return sourceMapCache.size;
}
/**
 * Clear the source map cache
 */
export function clearSourceMapCache() {
    sourceMapCache.clear();
}
//# sourceMappingURL=cache-limits.js.map