/**
 * Purpose: Validates and bounds authenticated page telemetry before extension forwarding.
 * Why: The tracked page is an untrusted producer even after the injected channel is authenticated.
 */
const MAX_PAYLOAD_BYTES = 512 * 1024;
const MAX_PAYLOAD_DEPTH = 12;
const MAX_PAYLOAD_NODES = 5000;
const MAX_ARRAY_ITEMS = 1000;
const MAX_OBJECT_KEYS = 200;
function inspectValue(value, seen, depth, state) {
    if (state.rejection)
        return;
    if (depth > MAX_PAYLOAD_DEPTH) {
        state.rejection = 'payload_too_deep';
        return;
    }
    state.nodes++;
    if (state.nodes > MAX_PAYLOAD_NODES) {
        state.rejection = 'payload_too_large';
        return;
    }
    if (typeof value === 'string') {
        state.bytes += value.length * 2;
    }
    else if (typeof value === 'number' || typeof value === 'bigint') {
        state.bytes += 8;
    }
    else if (typeof value === 'boolean') {
        state.bytes++;
    }
    else if (value && typeof value === 'object') {
        if (seen.has(value)) {
            state.rejection = 'invalid_schema';
            return;
        }
        seen.add(value);
        if (Array.isArray(value)) {
            if (value.length > MAX_ARRAY_ITEMS) {
                state.rejection = 'payload_too_large';
                return;
            }
            for (const item of value)
                inspectValue(item, seen, depth + 1, state);
        }
        else {
            const entries = Object.entries(value);
            if (entries.length > MAX_OBJECT_KEYS) {
                state.rejection = 'payload_too_large';
                return;
            }
            for (const [key, child] of entries) {
                state.bytes += key.length * 2;
                inspectValue(child, seen, depth + 1, state);
            }
        }
        seen.delete(value);
    }
    if (state.bytes > MAX_PAYLOAD_BYTES)
        state.rejection = 'payload_too_large';
}
function isRecord(value) {
    return value !== null && typeof value === 'object' && !Array.isArray(value);
}
function hasString(value, key) {
    return typeof value[key] === 'string';
}
function hasNumber(value, key) {
    return typeof value[key] === 'number' && Number.isFinite(value[key]);
}
function matchesTelemetrySchema(messageType, payload) {
    switch (messageType) {
        case 'kaboom_log':
            return hasString(payload, 'ts') && hasString(payload, 'level');
        case 'kaboom_ws':
            return hasString(payload, 'event') && hasString(payload, 'id');
        case 'kaboom_network_body':
            return hasString(payload, 'method') && hasString(payload, 'url') && hasNumber(payload, 'status');
        case 'kaboom_enhanced_action':
            return hasString(payload, 'type') && hasNumber(payload, 'timestamp');
        case 'kaboom_performance_snapshot':
            return hasString(payload, 'url') && hasString(payload, 'timestamp') && isRecord(payload.timing);
        case 'kaboom_capture_diagnostic':
            return hasString(payload, 'category') && hasString(payload, 'message') && hasString(payload, 'error_type');
        default:
            return false;
    }
}
export function validatePageTelemetry(messageType, payload) {
    if (!isRecord(payload) || !matchesTelemetrySchema(messageType, payload))
        return 'invalid_schema';
    const inspection = { bytes: 0, nodes: 0 };
    inspectValue(payload, new WeakSet(), 0, inspection);
    return inspection.rejection ?? null;
}
//# sourceMappingURL=page-telemetry.js.map