/**
 * Purpose: Timeout scaling helpers that read KABOOM_TEST_TIMEOUT_SCALE to accelerate timeouts during automated tests.
 */
function readTestScale() {
    const globalScale = typeof globalThis !== 'undefined' &&
        typeof globalThis.KABOOM_TEST_TIMEOUT_SCALE === 'number'
        ? globalThis.KABOOM_TEST_TIMEOUT_SCALE
        : null;
    if (globalScale !== null)
        return globalScale;
    if (typeof process !== 'undefined' && process.env) {
        const raw = process.env.KABOOM_TEST_TIMEOUT_SCALE || process.env.KABOOM_TEST_TIME_SCALE;
        if (raw) {
            const parsed = Number(raw);
            if (Number.isFinite(parsed))
                return parsed;
        }
    }
    return 1;
}
export function scaleTimeout(ms) {
    const scale = readTestScale();
    if (!Number.isFinite(scale) || scale <= 0 || scale === 1) {
        return ms;
    }
    return Math.max(5, Math.round(ms * scale));
}
//# sourceMappingURL=timeouts.js.map