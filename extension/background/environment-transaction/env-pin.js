/**
 * Purpose: Registers the env_pin / env_unpin commands that hold a tab's environment still for
 *          the length of a recording session, and report what was actually pinned.
 * Why: Pinning has to be opt-in and per session. A tab that was never pinned carries no
 *      environment block at all, which is what makes "Environment not pinned" in the emitted
 *      artifact a statement of fact rather than an absence of reporting.
 * Docs: docs/features/feature/session-to-test/index.md
 */
import { registerCommand } from '../commands/registry.js';
import { cdpSessions } from '../dom/cdp/cdp-session.js';
import { applyEnvironmentPin, clearEnvironmentPin, environmentPinFor, forgetEnvironmentPin, recordEnvironmentPin } from '../dom/cdp/cdp-env-pin.js';
import { errorMessage } from '../../lib/error-utils.js';
const NUMERIC_KEYS = [
    'clock_epoch_ms',
    'latitude',
    'longitude',
    'accuracy_m',
    'viewport_width',
    'viewport_height',
    'device_scale_factor'
];
const VIRTUAL_TIME_POLICIES = new Set(['advance', 'pause', 'pauseIfNetworkFetchesPending']);
/**
 * Read the pin spec out of loosely-typed command params.
 *
 * A value of the wrong type is dropped rather than coerced: `viewport_width: "wide"` coerced
 * to NaN would be sent to CDP, refused, and reported as a knob the browser would not pin —
 * blaming the browser for a caller's typo.
 */
export function parseEnvironmentPinSpec(params) {
    const source = (params.environment && typeof params.environment === 'object' ? params.environment : params);
    const spec = {};
    for (const key of NUMERIC_KEYS) {
        const value = source[key];
        if (typeof value === 'number' && Number.isFinite(value))
            spec[key] = value;
    }
    if (typeof source.timezone_id === 'string' && source.timezone_id)
        spec.timezone_id = source.timezone_id;
    if (typeof source.random_seed === 'string' && source.random_seed)
        spec.random_seed = source.random_seed;
    if (source.mobile === true)
        spec.mobile = true;
    if (typeof source.virtual_time_policy === 'string' && VIRTUAL_TIME_POLICIES.has(source.virtual_time_policy)) {
        spec.virtual_time_policy = source.virtual_time_policy;
    }
    return spec;
}
/** True when the caller asked for nothing at all — an empty pin is a caller error, not a no-op. */
function isEmptySpec(spec) {
    return Object.keys(spec).length === 0;
}
export function registerEnvironmentPinCommands() {
    registerCommand('env_pin', async (ctx) => {
        const spec = parseEnvironmentPinSpec(ctx.params);
        if (isEmptySpec(spec)) {
            ctx.sendResult({
                error: 'empty_environment_pin',
                message: 'pin_environment needs at least one knob, for example environment={"timezone_id":"UTC","random_seed":"run-1"}.'
            });
            return;
        }
        const sessions = cdpSessions();
        if (!sessions) {
            ctx.sendResult({
                error: 'cdp_unavailable',
                message: 'Environment pinning needs the Chrome debugger, which is unavailable here.'
            });
            return;
        }
        let lease;
        try {
            lease = await sessions.acquire(ctx.tabId);
        }
        catch (err) {
            ctx.sendResult({ error: 'cdp_attach_failed', message: errorMessage(err, 'Could not attach the debugger') });
            return;
        }
        try {
            const pin = await applyEnvironmentPin(lease, spec);
            recordEnvironmentPin(ctx.tabId, pin);
            ctx.sendResult({
                success: true,
                action: 'pin_environment',
                tab_id: ctx.tabId,
                environment: pin,
                // Reported separately from the pin so a caller can tell "nothing was asked for" from
                // "everything was asked for and the browser refused it".
                unpinned_count: pin.unpinned?.length ?? 0
            });
        }
        finally {
            lease.release();
        }
    });
    registerCommand('env_unpin', async (ctx) => {
        const sessions = cdpSessions();
        if (!sessions) {
            ctx.sendResult({ error: 'cdp_unavailable', message: 'Releasing an environment pin needs the Chrome debugger.' });
            return;
        }
        const had = environmentPinFor(ctx.tabId) !== undefined;
        let lease;
        try {
            lease = await sessions.acquire(ctx.tabId);
        }
        catch (err) {
            ctx.sendResult({ error: 'cdp_attach_failed', message: errorMessage(err, 'Could not attach the debugger') });
            return;
        }
        try {
            const failures = await clearEnvironmentPin(lease);
            // Forget the pin only once the overrides are gone. Forgetting first would stop stamping
            // actions that are still being recorded under a pinned clock.
            forgetEnvironmentPin(ctx.tabId);
            ctx.sendResult({
                success: failures.length === 0,
                action: 'unpin_environment',
                tab_id: ctx.tabId,
                was_pinned: had,
                ...(failures.length > 0 ? { error: 'environment_release_failed', not_released: failures } : {})
            });
        }
        finally {
            lease.release();
        }
    });
}
//# sourceMappingURL=env-pin.js.map