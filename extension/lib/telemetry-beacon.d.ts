/**
 * Purpose: Anonymous telemetry beacons for error visibility. Disable with the Kaboom telemetry opt-out key.
 */
/**
 * Fire an anonymous telemetry beacon. Fire-and-forget, never throws.
 * Waits for the opt-out flag to hydrate from storage before sending so
 * opted-out users never emit startup beacons.
 * Uses navigator.sendBeacon when available, falls back to fetch.
 */
export declare function beacon(event: string, props?: Record<string, string>): void;
//# sourceMappingURL=telemetry-beacon.d.ts.map