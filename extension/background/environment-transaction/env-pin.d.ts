/**
 * Purpose: Registers the env_pin / env_unpin commands that hold a tab's environment still for
 *          the length of a recording session, and report what was actually pinned.
 * Why: Pinning has to be opt-in and per session. A tab that was never pinned carries no
 *      environment block at all, which is what makes "Environment not pinned" in the emitted
 *      artifact a statement of fact rather than an absence of reporting.
 * Docs: docs/features/feature/session-to-test/index.md
 */
import { type EnvironmentPinSpec } from '../dom/cdp/cdp-env-pin.js';
/**
 * Read the pin spec out of loosely-typed command params.
 *
 * A value of the wrong type is dropped rather than coerced: `viewport_width: "wide"` coerced
 * to NaN would be sent to CDP, refused, and reported as a knob the browser would not pin —
 * blaming the browser for a caller's typo.
 */
export declare function parseEnvironmentPinSpec(params: Readonly<Record<string, unknown>>): EnvironmentPinSpec;
export declare function registerEnvironmentPinCommands(): void;
//# sourceMappingURL=env-pin.d.ts.map