/**
 * Purpose: The terminal panel's non-terminal states — no session, and start failure.
 * Why: Both are recoverable states with actions in them, not error text, and
 * keeping them out of sidepanel.ts keeps that file within the size limit.
 * Docs: docs/features/feature/terminal/index.md
 */
/**
 * Render a recoverable "no shell" state into `container`.
 *
 * The panel used to print a dead sentence and stop, so an ended or failed
 * session left a panel with nothing to click — the user could neither retry nor
 * change anything without digging through the options page. The root-folder bar
 * above the terminal covers the other half; this covers starting one.
 */
export declare function renderNoSessionState(container: HTMLElement, onStart: () => void): void;
/**
 * Render a live "starting…" state into `container`.
 *
 * The daemon retries a transient fork/exec EPERM before giving up, so a spawn can
 * legitimately take a few hundred milliseconds. Without this the panel body was
 * visually identical to a dead one for that whole window and the user could not
 * tell "working on it" from "broken".
 */
export declare function renderStartPending(container: HTMLElement, label?: string): void;
/**
 * Render a start failure: what happened, what to do, and the command to run.
 */
export declare function renderStartFailure(container: HTMLElement, message: string, instruction: string, command: string): void;
//# sourceMappingURL=terminal-panel-states.d.ts.map