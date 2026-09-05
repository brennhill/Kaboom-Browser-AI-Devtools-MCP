/**
 * Purpose: Holds a tab's environment still — clock, timezone, location, viewport, randomness —
 *          and reports exactly what was pinned so the emitted test can state its dependencies.
 * Why: Deterministic replay needs the environment fixed, but a test that silently depends on a
 *      pinned clock is worse than one that pins nothing: it passes only on the machine that
 *      recorded it, and says nothing about why it fails everywhere else.
 * Docs: docs/features/feature/session-to-test/index.md
 */
import type { Lease } from './cdp-session.js';
import type { WireEnvironmentPin } from '../../../types/wire/wire-enhanced-action.js';
/** What a caller asks to hold still. Every field is optional; absent means "leave alone". */
export interface EnvironmentPinSpec {
    readonly clock_epoch_ms?: number;
    readonly timezone_id?: string;
    /**
     * CDP virtual-time policy. Defaults to `advance`, which fixes the clock's ORIGIN and lets
     * it run: `pause` stops time outright, which freezes the page you are trying to record.
     * Pass `pause` only for replay, where nothing is waiting on a timer to make progress.
     */
    readonly virtual_time_policy?: 'advance' | 'pause' | 'pauseIfNetworkFetchesPending';
    readonly latitude?: number;
    readonly longitude?: number;
    readonly accuracy_m?: number;
    readonly viewport_width?: number;
    readonly viewport_height?: number;
    readonly device_scale_factor?: number;
    readonly mobile?: boolean;
    readonly random_seed?: string;
}
/**
 * The snippet run before every document in the pinned tab.
 *
 * It carries no PRNG of its own on purpose. The generator lives in early-patch.ts, which is
 * the only script guaranteed to run in the MAIN world at document_start; duplicating it here
 * would give a page two independent Math.random replacements that disagree.
 */
export declare function seedInstallSnippet(seed: string): string;
/**
 * Apply every knob the spec names and report what actually landed.
 *
 * Each knob is attempted independently: a browser that refuses one override is no reason to
 * abandon the rest, and a refusal is recorded in `unpinned` rather than swallowed — the
 * knobs a session asked for and did not get are exactly the ones a replay diverges on.
 */
export declare function applyEnvironmentPin(lease: Lease, spec: EnvironmentPinSpec): Promise<WireEnvironmentPin>;
/**
 * Release every override this module can set.
 *
 * Best effort per knob for the same reason as apply: one refusal must not leave the others
 * in force, which would hand the user a tab still stuck in a fake timezone.
 */
export declare function clearEnvironmentPin(lease: Lease): Promise<string[]>;
export declare function recordEnvironmentPin(tabId: number, pin: WireEnvironmentPin): void;
export declare function environmentPinFor(tabId: number | undefined): WireEnvironmentPin | undefined;
export declare function forgetEnvironmentPin(tabId: number): void;
//# sourceMappingURL=cdp-env-pin.d.ts.map