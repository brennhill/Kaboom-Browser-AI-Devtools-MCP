/**
 * Purpose: Holds a tab's environment still — clock, timezone, location, viewport, randomness —
 *          and reports exactly what was pinned so the emitted test can state its dependencies.
 * Why: Deterministic replay needs the environment fixed, but a test that silently depends on a
 *      pinned clock is worse than one that pins nothing: it passes only on the machine that
 *      recorded it, and says nothing about why it fails everywhere else.
 * Docs: docs/features/feature/session-to-test/index.md
 */
import { errorMessage } from '../../../lib/error-utils.js';
import { debugLog, DebugCategory } from '../../debug.js';
/** Set by early-patch when a seed is installed; read back to confirm the seed actually landed. */
const SEED_ACTIVE_GLOBAL = '__KABOOM_RANDOM_SEED_ACTIVE__';
const SEED_GLOBAL = '__KABOOM_RANDOM_SEED__';
const SEED_INSTALLER_GLOBAL = '__KABOOM_SEED_RANDOM__';
/**
 * The snippet run before every document in the pinned tab.
 *
 * It carries no PRNG of its own on purpose. The generator lives in early-patch.ts, which is
 * the only script guaranteed to run in the MAIN world at document_start; duplicating it here
 * would give a page two independent Math.random replacements that disagree.
 */
export function seedInstallSnippet(seed) {
    const literal = JSON.stringify(seed);
    return `(function(){window.${SEED_GLOBAL}=${literal};if(typeof window.${SEED_INSTALLER_GLOBAL}==='function'){window.${SEED_INSTALLER_GLOBAL}(${literal})}})()`;
}
function clockSteps(spec) {
    const policy = spec.virtual_time_policy ?? 'advance';
    return [
        {
            name: 'clock',
            wanted: typeof spec.clock_epoch_ms === 'number',
            run: (lease) => lease
                .send('Emulation.setVirtualTimePolicy', {
                policy,
                // CDP takes TimeSinceEpoch in SECONDS; passing milliseconds puts the page
                // roughly 55,000 years into the future and every date on it reads as invalid.
                initialVirtualTime: (spec.clock_epoch_ms ?? 0) / 1000
            })
                .then(() => undefined)
        },
        {
            name: 'timezone',
            wanted: Boolean(spec.timezone_id),
            run: (lease) => lease.send('Emulation.setTimezoneOverride', { timezoneId: spec.timezone_id }).then(() => undefined)
        }
    ];
}
function placeSteps(spec) {
    return [
        {
            name: 'geolocation',
            wanted: typeof spec.latitude === 'number' && typeof spec.longitude === 'number',
            run: (lease) => lease
                .send('Emulation.setGeolocationOverride', {
                latitude: spec.latitude,
                longitude: spec.longitude,
                accuracy: spec.accuracy_m ?? 1
            })
                .then(() => undefined)
        },
        {
            name: 'viewport',
            wanted: typeof spec.viewport_width === 'number' && typeof spec.viewport_height === 'number',
            run: (lease) => lease
                .send('Emulation.setDeviceMetricsOverride', {
                width: spec.viewport_width,
                height: spec.viewport_height,
                deviceScaleFactor: spec.device_scale_factor ?? 1,
                mobile: spec.mobile ?? false
            })
                .then(() => undefined)
        }
    ];
}
/**
 * Install the seed for both the current document and every document that follows.
 *
 * Reloads and same-tab navigations each get a fresh MAIN world, so a one-shot evaluate would
 * seed only the page that happened to be open when pinning ran.
 */
async function runSeedStep(lease, seed) {
    await lease.ensureDomain('Page');
    const snippet = seedInstallSnippet(seed);
    await lease.send('Page.addScriptToEvaluateOnNewDocument', { source: snippet });
    await lease.send('Runtime.evaluate', { expression: snippet, returnByValue: true });
    const active = (await lease.send('Runtime.evaluate', {
        expression: `window.${SEED_ACTIVE_GLOBAL} === true`,
        returnByValue: true
    }));
    // Setting a seed nothing reads is not pinning. early-patch is absent on cloaked domains
    // and on any page whose CSP blocked it, and reporting that as pinned would put a claim in
    // the emitted test that the recording never honoured.
    if (active?.result?.value !== true) {
        throw new Error('early patch did not install the seeded generator on this page');
    }
}
function seedStep(spec) {
    return {
        name: 'random seed',
        wanted: Boolean(spec.random_seed),
        run: (lease) => runSeedStep(lease, spec.random_seed ?? '')
    };
}
function pinSteps(spec) {
    return [...clockSteps(spec), ...placeSteps(spec), seedStep(spec)];
}
/**
 * Report only the knobs that landed.
 *
 * Reporting a requested value the browser refused would put a claim in the emitted test that
 * the recording never honoured — the refusal belongs in `unpinned`, not in the pin.
 */
function clockReport(spec, landed) {
    const clock = landed.has('clock');
    const timezone = landed.has('timezone');
    if (!clock && !timezone)
        return undefined;
    return {
        ...(clock ? { epoch_ms: spec.clock_epoch_ms, virtual_time_policy: spec.virtual_time_policy ?? 'advance' } : {}),
        ...(timezone ? { timezone_id: spec.timezone_id } : {})
    };
}
function geoReport(spec, landed) {
    if (!landed.has('geolocation'))
        return undefined;
    return { latitude: spec.latitude ?? 0, longitude: spec.longitude ?? 0, accuracy_m: spec.accuracy_m ?? 1 };
}
function viewportReport(spec, landed) {
    if (!landed.has('viewport'))
        return undefined;
    return {
        width: spec.viewport_width ?? 0,
        height: spec.viewport_height ?? 0,
        device_scale_factor: spec.device_scale_factor ?? 1,
        ...(spec.mobile ? { mobile: true } : {})
    };
}
function buildPinReport(spec, landed, unpinned) {
    const clock = clockReport(spec, landed);
    const geolocation = geoReport(spec, landed);
    const viewport = viewportReport(spec, landed);
    return {
        ...(clock ? { clock } : {}),
        ...(geolocation ? { geolocation } : {}),
        ...(viewport ? { viewport } : {}),
        ...(landed.has('random seed') ? { random_seed: spec.random_seed } : {}),
        ...(unpinned.length > 0 ? { unpinned } : {})
    };
}
/**
 * Apply every knob the spec names and report what actually landed.
 *
 * Each knob is attempted independently: a browser that refuses one override is no reason to
 * abandon the rest, and a refusal is recorded in `unpinned` rather than swallowed — the
 * knobs a session asked for and did not get are exactly the ones a replay diverges on.
 */
export async function applyEnvironmentPin(lease, spec) {
    const landed = new Set();
    const unpinned = [];
    for (const step of pinSteps(spec)) {
        if (!step.wanted)
            continue;
        try {
            await step.run(lease);
            landed.add(step.name);
        }
        catch (err) {
            const reason = errorMessage(err, 'the browser refused the override');
            unpinned.push(`${step.name} (${reason})`);
            debugLog(DebugCategory.LIFECYCLE, 'Environment pin knob refused', { knob: step.name, reason });
        }
    }
    return buildPinReport(spec, landed, unpinned);
}
/**
 * Release every override this module can set.
 *
 * Best effort per knob for the same reason as apply: one refusal must not leave the others
 * in force, which would hand the user a tab still stuck in a fake timezone.
 */
export async function clearEnvironmentPin(lease) {
    const failures = [];
    const calls = [
        ['clock', 'Emulation.setVirtualTimePolicy', { policy: 'advance' }],
        ['timezone', 'Emulation.setTimezoneOverride', { timezoneId: '' }],
        ['geolocation', 'Emulation.clearGeolocationOverride', {}],
        ['viewport', 'Emulation.clearDeviceMetricsOverride', {}]
    ];
    for (const [name, method, params] of calls) {
        try {
            await lease.send(method, params);
        }
        catch (err) {
            const reason = errorMessage(err, 'the browser refused to clear the override');
            failures.push(`${name} (${reason})`);
            debugLog(DebugCategory.LIFECYCLE, 'Environment pin release failed', { knob: name, reason });
        }
    }
    return failures;
}
// =============================================================================
// PER-SESSION REGISTRY
//
// Pinning is opt-in: this map is empty until a caller pins a tab, and an action recorded in
// an unpinned tab carries no environment block at all. That absence is what the emitted
// artifact reports as "Environment not pinned".
// =============================================================================
const pinnedTabs = new Map();
export function recordEnvironmentPin(tabId, pin) {
    pinnedTabs.set(tabId, pin);
}
export function environmentPinFor(tabId) {
    if (typeof tabId !== 'number')
        return undefined;
    return pinnedTabs.get(tabId);
}
export function forgetEnvironmentPin(tabId) {
    pinnedTabs.delete(tabId);
}
//# sourceMappingURL=cdp-env-pin.js.map