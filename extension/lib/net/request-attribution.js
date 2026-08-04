/**
 * Purpose: Correlate bounded page-level request initiators and response metadata with resource timings.
 * Docs: docs/features/feature/network-performance-attribution/index.md
 */
const MAX_ATTRIBUTIONS = 200;
const MAX_STACK_FRAMES = 12;
const attributions = [];
export function recordRequestAttribution(url, start = {}) {
    if (!url)
        return;
    attributions.push({ url: normalizeURL(url), ...start, complete: false });
    if (attributions.length > MAX_ATTRIBUTIONS)
        attributions.splice(0, attributions.length - MAX_ATTRIBUTIONS);
}
export function completeRequestAttribution(url, finish) {
    const normalized = normalizeURL(url);
    for (let index = attributions.length - 1; index >= 0; index--) {
        const candidate = attributions[index];
        if (candidate && candidate.url === normalized && !candidate.complete) {
            Object.assign(candidate, finish, { complete: true });
            return;
        }
    }
}
export function enrichWaterfallEntries(entries) {
    const enriched = entries.map((entry) => enrichEntry(entry, consumeAttribution(entry.url)));
    assignDuplicateGroups(enriched);
    return enriched;
}
export function resetRequestAttribution() {
    attributions.length = 0;
}
function consumeAttribution(url) {
    const normalized = normalizeURL(url);
    const index = attributions.findIndex((candidate) => candidate.url === normalized);
    if (index < 0)
        return undefined;
    return attributions.splice(index, 1)[0];
}
function enrichEntry(entry, attribution) {
    if (!attribution)
        return entry;
    const stack = cleanStack(attribution.stack);
    const semantic = semanticInitiator(stack);
    return {
        ...entry,
        ...(attribution.priority ? { priority: attribution.priority } : {}),
        ...(attribution.status !== undefined ? { status: attribution.status } : {}),
        ...(attribution.request_id ? { request_id: attribution.request_id.slice(0, 256) } : {}),
        ...(attribution.traceparent ? { traceparent: attribution.traceparent.slice(0, 256) } : {}),
        ...(attribution.content_encoding ? { content_encoding: attribution.content_encoding.slice(0, 64) } : {}),
        ...(attribution.server_timing ? { server_timing: parseServerTiming(attribution.server_timing) } : {}),
        ...(stack.length > 0
            ? {
                initiator_stack: stack,
                source_map_status: stack.some(isOriginalSourceFrame) ? 'mapped_or_source' : 'browser_stack'
            }
            : {}),
        ...semantic
    };
}
function cleanStack(raw) {
    if (!raw)
        return [];
    return raw
        .split('\n')
        .map((line) => line.trim())
        .filter((line) => line.startsWith('at ') &&
        !line.includes('kaboom') &&
        !line.includes('request-attribution') &&
        !line.includes('/lib/net/network.') &&
        !line.includes('/inject/observers.'))
        .slice(0, MAX_STACK_FRAMES);
}
function isOriginalSourceFrame(frame) {
    return /(?:\/src\/|\.(?:tsx?|jsx?)(?::\d+){1,2}\)?$)/.test(frame);
}
function semanticInitiator(stack) {
    let reactComponent;
    let routeLoader;
    let storeAction;
    for (const frame of stack) {
        const name = /^at\s+([^\s(]+)/.exec(frame)?.[1];
        if (!name)
            continue;
        if (!reactComponent && /^[A-Z][A-Za-z0-9_$]*$/.test(name))
            reactComponent = name;
        if (!routeLoader && /(?:loader|route)/i.test(name))
            routeLoader = name;
        if (!storeAction && /(?:store|dispatch|action|mutation)/i.test(name))
            storeAction = name;
    }
    return {
        ...(reactComponent ? { react_component: reactComponent } : {}),
        ...(routeLoader ? { route_loader: routeLoader } : {}),
        ...(storeAction ? { store_action: storeAction } : {})
    };
}
function parseServerTiming(raw) {
    return raw
        .split(',')
        .slice(0, 20)
        .map((metric) => {
        const [namePart = '', ...params] = metric.trim().split(';');
        const result = { name: namePart.slice(0, 128) };
        for (const param of params) {
            const [key, rawValue = ''] = param.trim().split('=', 2);
            const value = rawValue.replace(/^"|"$/g, '');
            if (key === 'dur' && Number.isFinite(Number(value)))
                result.duration_ms = Number(value);
            if (key === 'desc' && value)
                result.description = value.slice(0, 256);
        }
        return result;
    })
        .filter((metric) => metric.name.length > 0);
}
function assignDuplicateGroups(entries) {
    const byURL = new Map();
    for (const entry of entries) {
        const group = byURL.get(entry.url) ?? [];
        group.push(entry);
        byURL.set(entry.url, group);
    }
    for (const [url, group] of byURL) {
        if (group.length < 2 || !hasOverlap(group))
            continue;
        const id = `dup-${stableHash(`${url}:${Math.min(...group.map((entry) => entry.start_time))}`)}`;
        for (const entry of group) {
            Object.assign(entry, { duplicate_group_id: id, duplicate_count: group.length });
        }
    }
}
function hasOverlap(entries) {
    const sorted = [...entries].sort((a, b) => a.start_time - b.start_time);
    let latestEnd = -Infinity;
    for (const entry of sorted) {
        if (entry.start_time < latestEnd)
            return true;
        latestEnd = Math.max(latestEnd, entry.response_end ?? entry.start_time + entry.duration);
    }
    return false;
}
function stableHash(value) {
    let hash = 2166136261;
    for (let index = 0; index < value.length; index++) {
        hash ^= value.charCodeAt(index);
        hash = Math.imul(hash, 16777619);
    }
    return (hash >>> 0).toString(36);
}
function normalizeURL(value) {
    try {
        return new URL(value, typeof location !== 'undefined' ? location.href : undefined).href;
    }
    catch {
        // EXPECTED_ABSENCE: relative or malformed page-owned URLs are normal app
        // input and remain correlatable by literal value; logging would misclassify them.
        return value;
    }
}
//# sourceMappingURL=request-attribution.js.map