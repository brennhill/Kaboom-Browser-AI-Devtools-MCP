/**
 * Purpose: Side panel host for the Kaboom terminal.
 * Why: Removes the terminal from page context so CSP on arbitrary sites cannot
 * interfere with the xterm host, while keeping the session and reconnect model intact.
 * Docs: docs/features/feature/terminal/index.md
 */
/**
 * Reset all panel-UI state to a clean slate. `panel` is const, so mutate its
 * fields in place (Object.assign) rather than rebind. Lets a test start from a
 * known-empty panel without reloading the module.
 */
export declare function resetPanelUi(): void;
/**
 * Persist the root folder and restart the shell there. A running PTY cannot be
 * moved — its cwd is fixed at spawn — so the old session is stopped first.
 */
declare function applyRootFolder(root: string): Promise<void>;
declare function redrawTerminal(): Promise<void>;
declare function exitTerminalSession(): Promise<void>;
declare function writeToTerminal(text: string): void;
/**
 * Boot (or rebuild) the terminal panel — GENERATION-based, not serialized.
 *
 * Each boot claims a generation (`panel.bootGeneration`); a newer boot supersedes
 * older ones. Any boot whose generation is stale by the time it reaches DOM
 * mutation aborts before touching the panel. This gives BOTH guarantees at once:
 *
 *  - No iframe-orphan race: two concurrent forceFresh boots can't both mount —
 *    the older aborts at the guard before createPanelShell()/mountPanel(), so the
 *    visible iframe and `state.iframeEl` never diverge (the "writes disappear" bug).
 *  - "Start terminal" ALWAYS works: we never await a previous boot, so a boot that
 *    STALLED on the network (a daemon that isn't answering) can neither block nor
 *    corrupt a fresh Start — the panic button always re-attempts and resets state.
 *    (Serializing on a bootChain regressed exactly this: a hung boot froze Start.)
 */
declare function bootTerminalPanel(forceFresh?: boolean): Promise<void>;
/**
 * Entry point: boot the panel once the document is ready. Auto-invoked at module
 * scope in the real side-panel document; the `process === undefined` guard keeps
 * it from firing under Node test imports. Named + exported so it is an explicit,
 * callable entry rather than an anonymous top-level side effect.
 */
export declare function main(): void;
export declare const _terminalPanelForTests: {
    bootTerminalPanel: typeof bootTerminalPanel;
    applyRootFolder: typeof applyRootFolder;
    writeToTerminal: typeof writeToTerminal;
    exitTerminalSession: typeof exitTerminalSession;
    redrawTerminal: typeof redrawTerminal;
    resetPanelUi: typeof resetPanelUi;
};
export {};
//# sourceMappingURL=sidepanel.d.ts.map