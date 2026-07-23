/**
 * Purpose: Observes web vitals (FCP, LCP, CLS, INP), long tasks, and resource timing to build comprehensive performance snapshots.
 * Docs: docs/features/feature/performance-audit/index.md
 */
/**
 * @fileoverview Performance snapshot capture.
 * Observes web vitals (FCP, LCP, CLS, INP), long tasks, and resource timing
 * to build comprehensive performance snapshots.
 */
import { MAX_LONG_TASKS, MAX_SLOWEST_REQUESTS, MAX_URL_LENGTH } from './constants.js';
// Performance snapshot state
let perfSnapshotEnabled = true;
let longTaskEntries = [];
let longTaskObserver = null;
let paintObserver = null;
let lcpObserver = null;
let clsObserver = null;
let inpObserver = null;
let fcpValue = null;
let lcpValue = null;
let clsValue = 0;
let inpValue = null;
/**
 * Map resource initiator types to standard categories
 */
export function mapInitiatorType(type) {
    switch (type) {
        case 'script':
            return 'script';
        case 'link':
        case 'css':
            return 'style';
        case 'img':
            return 'image';
        case 'fetch':
        case 'xmlhttprequest':
            return 'fetch';
        case 'font':
            return 'font';
        default:
            return 'other';
    }
}
/**
 * Aggregate resource timing entries into a network summary
 */
export function aggregateResourceTiming() {
    const resources = performance.getEntriesByType('resource') || [];
    const byType = {};
    let transferSize = 0;
    let decodedSize = 0;
    for (const entry of resources) {
        const category = mapInitiatorType(entry.initiatorType);
        // eslint-disable-next-line security/detect-object-injection -- category from mapInitiatorType returns known resource type strings
        if (!byType[category]) {
            // eslint-disable-next-line security/detect-object-injection -- category from mapInitiatorType returns known resource type strings
            byType[category] = { count: 0, size: 0 };
        }
        // eslint-disable-next-line security/detect-object-injection -- category from mapInitiatorType returns known resource type strings
        byType[category].count++;
        // eslint-disable-next-line security/detect-object-injection -- category from mapInitiatorType returns known resource type strings
        byType[category].size += entry.transferSize || 0;
        transferSize += entry.transferSize || 0;
        decodedSize += entry.decodedBodySize || 0;
    }
    // Top N slowest requests
    const sorted = [...resources].sort((a, b) => b.duration - a.duration);
    const slowestRequests = sorted.slice(0, MAX_SLOWEST_REQUESTS).map((r) => ({
        url: r.name.length > MAX_URL_LENGTH ? r.name.slice(0, MAX_URL_LENGTH) : r.name,
        duration: r.duration,
        size: r.transferSize || 0
    }));
    return {
        request_count: resources.length,
        transfer_size: transferSize,
        decoded_size: decodedSize,
        by_type: byType,
        slowest_requests: slowestRequests
    };
}
/**
 * Capture a performance snapshot with navigation timing and network summary
 */
export function capturePerformanceSnapshot() {
    const navEntries = performance.getEntriesByType('navigation') || [];
    if (!navEntries || navEntries.length === 0)
        return null;
    const nav = navEntries[0];
    if (!nav)
        return null;
    const timing = {
        dom_content_loaded: nav.domContentLoadedEventEnd,
        load: nav.loadEventEnd,
        first_contentful_paint: getFCP(),
        largest_contentful_paint: getLCP(),
        interaction_to_next_paint: getINP(),
        time_to_first_byte: nav.responseStart - nav.requestStart,
        dom_interactive: nav.domInteractive
    };
    const network = aggregateResourceTiming();
    const longTasks = getLongTaskMetrics();
    // Capture user timing marks and measures
    const marks = performance.getEntriesByType('mark') || [];
    const measures = performance.getEntriesByType('measure') || [];
    const userTiming = marks.length > 0 || measures.length > 0
        ? {
            marks: marks.slice(-50).map((m) => ({ name: m.name, start_time: m.startTime })),
            measures: measures.slice(-50).map((m) => ({ name: m.name, start_time: m.startTime, duration: m.duration }))
        }
        : undefined;
    return {
        url: window.location.pathname,
        timestamp: new Date().toISOString(),
        timing,
        network,
        long_tasks: longTasks,
        cumulative_layout_shift: getCLS(),
        user_timing: userTiming
    };
}
/**
 * Install performance observers for long tasks, paint, LCP, and CLS
 */
export function installPerfObservers() {
    longTaskEntries = [];
    fcpValue = null;
    lcpValue = null;
    clsValue = 0;
    inpValue = null;
    // Long task observer
    // #lizard forgives
    longTaskObserver = new PerformanceObserver((list) => {
        const entries = list.getEntries();
        for (const entry of entries) {
            if (longTaskEntries.length < MAX_LONG_TASKS) {
                longTaskEntries.push(entry);
            }
        }
    });
    longTaskObserver.observe({ type: 'longtask' });
    // Paint observer (FCP)
    paintObserver = new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
            if (entry.name === 'first-contentful-paint') {
                fcpValue = entry.startTime;
            }
        }
    });
    paintObserver.observe({ type: 'paint', buffered: true });
    // LCP observer
    lcpObserver = new PerformanceObserver((list) => {
        const entries = list.getEntries();
        if (entries.length > 0) {
            const lastEntry = entries[entries.length - 1];
            if (lastEntry) {
                lcpValue = lastEntry.startTime;
            }
        }
    });
    lcpObserver.observe({ type: 'largest-contentful-paint', buffered: true });
    // CLS observer
    // LayoutShift interface extends PerformanceEntry with hadRecentInput and value
    clsObserver = new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
            const clsEntry = entry;
            if (!clsEntry.hadRecentInput) {
                clsValue += clsEntry.value || 0;
            }
        }
    });
    clsObserver.observe({ type: 'layout-shift', buffered: true });
    // INP observer (Interaction to Next Paint)
    // Event timing entries have interactionId and duration properties
    inpObserver = new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
            const inpEntry = entry;
            if (inpEntry.interactionId) {
                if (inpValue === null || inpEntry.duration > inpValue) {
                    inpValue = inpEntry.duration;
                }
            }
        }
    });
    inpObserver.observe({ type: 'event', durationThreshold: 40, buffered: true });
}
/**
 * Disconnect all performance observers
 */
export function uninstallPerfObservers() {
    if (longTaskObserver) {
        longTaskObserver.disconnect();
        longTaskObserver = null;
    }
    if (paintObserver) {
        paintObserver.disconnect();
        paintObserver = null;
    }
    if (lcpObserver) {
        lcpObserver.disconnect();
        lcpObserver = null;
    }
    if (clsObserver) {
        clsObserver.disconnect();
        clsObserver = null;
    }
    if (inpObserver) {
        inpObserver.disconnect();
        inpObserver = null;
    }
    longTaskEntries = [];
}
/**
 * Get accumulated long task metrics
 */
export function getLongTaskMetrics() {
    let totalBlockingTime = 0;
    let longest = 0;
    for (const entry of longTaskEntries) {
        const blocking = entry.duration - 50;
        if (blocking > 0)
            totalBlockingTime += blocking;
        if (entry.duration > longest)
            longest = entry.duration;
    }
    return {
        count: longTaskEntries.length,
        total_blocking_time: totalBlockingTime,
        longest
    };
}
/**
 * Get First Contentful Paint value
 */
export function getFCP() {
    return fcpValue;
}
/**
 * Get Largest Contentful Paint value
 */
export function getLCP() {
    return lcpValue;
}
/**
 * Get Cumulative Layout Shift value
 */
export function getCLS() {
    return clsValue;
}
/**
 * Get Interaction to Next Paint value
 */
export function getINP() {
    return inpValue;
}
/**
 * Send performance snapshot via postMessage to content script
 */
export function sendPerformanceSnapshot() {
    if (!perfSnapshotEnabled)
        return;
    const snapshot = capturePerformanceSnapshot();
    if (!snapshot)
        return;
    window.postMessage({ type: 'kaboom_performance_snapshot', payload: snapshot }, window.location.origin);
}
// Debounce timer for snapshot re-sends triggered by user timing changes
let snapshotResendTimer = null;
/**
 * Schedule a debounced re-send of the performance snapshot.
 * Called when user timing marks/measures are created to keep server data fresh.
 */
export function scheduleSnapshotResend() {
    if (!perfSnapshotEnabled)
        return;
    if (snapshotResendTimer)
        clearTimeout(snapshotResendTimer);
    snapshotResendTimer = setTimeout(() => {
        snapshotResendTimer = null;
        sendPerformanceSnapshot();
    }, 500);
}
/**
 * Check if performance snapshot capture is enabled
 */
export function isPerformanceSnapshotEnabled() {
    return perfSnapshotEnabled;
}
/**
 * Enable or disable performance snapshot capture
 */
export function setPerformanceSnapshotEnabled(enabled) {
    perfSnapshotEnabled = enabled;
}
//# sourceMappingURL=perf-snapshot.js.map