/**
 * Purpose: Build bounded, content-free attribution for browser Web Vitals entries.
 * Docs: docs/features/feature/web-vitals/index.md
 */
import { reportPageCaptureFailure } from '../diagnostics/page-capture.js';
const MAX_CLASSES = 4;
const MAX_SHIFTS = 10;
const MAX_SHIFT_NODES = 5;
const MAX_LONG_TASKS = 20;
let lcpEntry = null;
let inpEntry = null;
const clsShifts = [];
const longTasks = [];
export function resetVitalsAttribution() {
    lcpEntry = null;
    inpEntry = null;
    clsShifts.length = 0;
    longTasks.length = 0;
}
export function recordLCPAttribution(entry) {
    lcpEntry = entry;
}
export function recordINPAttribution(entry) {
    inpEntry = entry;
}
export function recordCLSAttribution(entry) {
    const shift = entry;
    if (clsShifts.length >= MAX_SHIFTS)
        return;
    const nodes = (shift.sources ?? [])
        .slice(0, MAX_SHIFT_NODES)
        .map((source) => describeElement(source.node))
        .filter((node) => node !== undefined);
    clsShifts.push({ value: finite(shift.value), start_time: finite(shift.startTime), nodes });
}
export function recordLongTaskAttribution(entry) {
    if (longTasks.length >= MAX_LONG_TASKS)
        return;
    const task = entry;
    longTasks.push({
        name: bounded(task.name || 'longtask', 64),
        start_time: finite(task.startTime),
        duration: finite(task.duration),
        source_stack_status: 'unavailable'
    });
}
export function getVitalsAttribution(responseStart = 0) {
    return {
        ...(lcpEntry ? { lcp: buildLCPAttribution(lcpEntry, responseStart) } : {}),
        ...(inpEntry ? { inp: buildINPAttribution(inpEntry) } : {}),
        cls: {
            shifts: clsShifts.map((shift) => ({ ...shift, nodes: shift.nodes.map((node) => ({ ...node })) })),
            attribution_status: clsShifts.some((shift) => shift.nodes.length > 0) ? 'available' : 'nodes_unavailable'
        },
        long_tasks: longTasks.map((task) => ({ ...task }))
    };
}
function buildLCPAttribution(entry, responseStart) {
    const loadTime = finite(entry.loadTime);
    const renderTime = finite(entry.renderTime || entry.startTime);
    const resource = matchingResourceTiming(entry.url);
    const element = describeElement(entry.element);
    return {
        ...(element ? { element } : {}),
        time_to_first_byte_ms: finite(responseStart),
        ...(resource
            ? {
                resource_load_delay_ms: Math.max(0, finite(resource.requestStart) - finite(responseStart)),
                resource_load_duration_ms: Math.max(0, finite(resource.responseEnd) - finite(resource.requestStart))
            }
            : {}),
        element_render_delay_ms: Math.max(0, renderTime - (resource ? finite(resource.responseEnd) : loadTime)),
        attribution_status: element ? 'available' : 'element_unavailable',
        resource_timing_status: resource ? 'available' : 'unavailable'
    };
}
function matchingResourceTiming(url) {
    if (!url || typeof performance === 'undefined')
        return undefined;
    try {
        return performance.getEntriesByType('resource').find((entry) => entry.name === url);
    }
    catch (error) {
        reportPageCaptureFailure('web_vitals', error);
        return undefined;
    }
}
function buildINPAttribution(entry) {
    const processingStart = finite(entry.processingStart);
    const processingEnd = finite(entry.processingEnd);
    const startTime = finite(entry.startTime);
    const duration = finite(entry.duration);
    const target = describeElement(entry.target);
    return {
        event_type: bounded(entry.name || 'event', 64),
        ...(target ? { target } : {}),
        input_delay_ms: Math.max(0, processingStart - startTime),
        processing_ms: Math.max(0, processingEnd - processingStart),
        presentation_delay_ms: Math.max(0, startTime + duration - processingEnd),
        interaction_id: finite(entry.interactionId)
    };
}
function describeElement(element) {
    if (!element)
        return undefined;
    const tag = element.tagName?.toLowerCase();
    if (!tag)
        return undefined;
    const id = bounded(element.id ?? '', 128);
    const classes = Array.from(element.classList ?? [])
        .filter((name) => typeof name === 'string' && name.length > 0)
        .slice(0, MAX_CLASSES)
        .map((name) => bounded(name, 64));
    const role = bounded(element.getAttribute?.('role') ?? '', 64);
    return {
        tag: bounded(tag, 32),
        ...(id ? { id } : {}),
        ...(classes.length > 0 ? { classes } : {}),
        ...(role ? { role } : {})
    };
}
function finite(value) {
    return Number.isFinite(value) ? Number(value) : 0;
}
function bounded(value, max) {
    return value.slice(0, max);
}
//# sourceMappingURL=vitals-attribution.js.map