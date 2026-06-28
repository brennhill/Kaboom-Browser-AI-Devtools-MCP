/**
 * Purpose: Anonymous telemetry beacons for error visibility. Disable with the Kaboom telemetry opt-out key.
 */
import { KABOOM_TELEMETRY_ENDPOINT, KABOOM_TELEMETRY_STORAGE_KEY } from './brand.js';
// Opt-out flag cached from chrome.storage.local for synchronous checks.
let telemetryDisabled = false;
// Resolves once the opt-out flag has been hydrated from storage. beacon() awaits
// this so events fired during startup (e.g. extension_start) respect the opt-out.
let telemetryFlagHydrated = Promise.resolve();
// Check opt-out on load
try {
    telemetryFlagHydrated = new Promise((resolve) => {
        try {
            chrome.storage.local.get(KABOOM_TELEMETRY_STORAGE_KEY, (result) => {
                telemetryDisabled = result[KABOOM_TELEMETRY_STORAGE_KEY] === true;
                resolve();
            });
        }
        catch {
            resolve();
        }
    });
    // Listen for runtime changes so opt-out takes effect immediately
    chrome.storage.onChanged.addListener((changes, area) => {
        if (area === 'local' && KABOOM_TELEMETRY_STORAGE_KEY in changes) {
            telemetryDisabled = changes[KABOOM_TELEMETRY_STORAGE_KEY].newValue === true;
        }
    });
}
catch {
    /* not in extension context */
}
/**
 * Fire an anonymous telemetry beacon. Fire-and-forget, never throws.
 * Waits for the opt-out flag to hydrate from storage before sending so
 * opted-out users never emit startup beacons.
 * Uses navigator.sendBeacon when available, falls back to fetch.
 */
export function beacon(event, props = {}) {
    telemetryFlagHydrated
        .then(() => {
        sendBeaconNow(event, props);
    })
        .catch(() => {
        /* never throw */
    });
}
function sendBeaconNow(event, props) {
    if (telemetryDisabled)
        return;
    try {
        const manifest = typeof chrome !== 'undefined' && chrome.runtime ? chrome.runtime.getManifest() : null;
        const version = manifest?.version || 'unknown';
        const payload = JSON.stringify({ event, v: version, props });
        if (typeof navigator !== 'undefined' && navigator.sendBeacon) {
            const blob = new Blob([payload], { type: 'application/json' });
            navigator.sendBeacon(KABOOM_TELEMETRY_ENDPOINT, blob);
            return;
        }
        // Fallback to fetch (fire-and-forget)
        void fetch(KABOOM_TELEMETRY_ENDPOINT, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: payload,
            keepalive: true
        }).catch(() => {
            /* never throw */
        });
    }
    catch {
        /* never throw */
    }
}
//# sourceMappingURL=telemetry-beacon.js.map