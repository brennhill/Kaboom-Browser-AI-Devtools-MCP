/**
 * Purpose: Validates and bounds authenticated page telemetry before extension forwarding.
 * Why: The tracked page is an untrusted producer even after the injected channel is authenticated.
 */
const MAX_PAYLOAD_BYTES = 512 * 1024;
const MAX_PAYLOAD_DEPTH = 12;
const MAX_PAYLOAD_NODES = 5000;
const MAX_ARRAY_ITEMS = 1000;
const MAX_OBJECT_KEYS = 200;
function accountScalarBytes(value, state) {
    if (typeof value === 'string') {
        state.bytes += value.length * 2;
        return true;
    }
    if (typeof value === 'number' || typeof value === 'bigint') {
        state.bytes += 8;
        return true;
    }
    if (typeof value === 'boolean') {
        state.bytes++;
        return true;
    }
    return false;
}
function inspectArrayItems(value, seen, depth, state) {
    if (value.length > MAX_ARRAY_ITEMS) {
        state.rejection = 'payload_too_large';
        return;
    }
    for (const item of value)
        inspectValue(item, seen, depth + 1, state);
}
function inspectObjectEntries(value, seen, depth, state) {
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
function inspectContainer(value, seen, depth, state) {
    if (seen.has(value)) {
        state.rejection = 'invalid_schema';
        return;
    }
    seen.add(value);
    if (Array.isArray(value)) {
        inspectArrayItems(value, seen, depth, state);
    }
    else {
        inspectObjectEntries(value, seen, depth, state);
    }
    seen.delete(value);
}
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
    if (value && typeof value === 'object') {
        inspectContainer(value, seen, depth, state);
    }
    else {
        accountScalarBytes(value, state);
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
const TELEMETRY_SCHEMA_VALIDATORS = {
    kaboom_log: (p) => hasString(p, 'ts') && hasString(p, 'level'),
    kaboom_ws: (p) => hasString(p, 'event') && hasString(p, 'id'),
    kaboom_network_body: (p) => hasString(p, 'method') && hasString(p, 'url') && hasNumber(p, 'status'),
    kaboom_enhanced_action: (p) => hasString(p, 'type') && hasNumber(p, 'timestamp'),
    kaboom_performance_snapshot: (p) => hasString(p, 'url') && hasString(p, 'timestamp') && isRecord(p.timing),
    kaboom_capture_diagnostic: (p) => hasString(p, 'category') && hasString(p, 'message') && hasString(p, 'error_type')
};
function matchesTelemetrySchema(messageType, payload) {
    if (!Object.prototype.hasOwnProperty.call(TELEMETRY_SCHEMA_VALIDATORS, messageType))
        return false;
    const validator = TELEMETRY_SCHEMA_VALIDATORS[messageType];
    return validator ? validator(payload) : false;
}
export function validatePageTelemetry(messageType, payload) {
    if (!isRecord(payload) || !matchesTelemetrySchema(messageType, payload))
        return 'invalid_schema';
    const inspection = { bytes: 0, nodes: 0 };
    inspectValue(payload, new WeakSet(), 0, inspection);
    return inspection.rejection ?? null;
}
//# sourceMappingURL=page-telemetry.js.map