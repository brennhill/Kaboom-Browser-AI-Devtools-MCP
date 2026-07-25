/**
 * Purpose: Side panel host for the Kaboom terminal.
 * Why: Removes the terminal from page context so CSP on arbitrary sites cannot
 * interfere with the xterm host, while keeping the session and reconnect model intact.
 * Docs: docs/features/feature/terminal/index.md
 */
/**
 * Persist the root folder and restart the shell there. A running PTY cannot be
 * moved — its cwd is fixed at spawn — so the old session is stopped first.
 */
declare function applyRootFolder(root: string): Promise<void>;
declare function redrawTerminal(): Promise<void>;
declare function exitTerminalSession(): Promise<void>;
declare function writeToTerminal(text: string): void;
/**
 * Boot (or rebuild) the terminal panel — serialized entry point.
 *
 * A second trigger arriving mid-boot (double-clicked Start, a rapid folder
 * re-pick, a redraw while an earlier boot is still awaiting the network) MUST
 * NOT run concurrently with the first: createPanelShell() unconditionally
 * rebinds `state.iframeEl`, while mountPanel() early-returns once `rootEl` is
 * set. Interleaved, that leaves the *visible* terminal on iframe #1 but
 * `state.iframeEl` pointing at a detached iframe #2 — so every later
 * write/annotation/redraw vanishes into the off-screen frame with no error
 * (the very "writes disappear" class the terminal work set out to kill).
 *
 * Chaining on `bootChain` forces boots to run one at a time; the latest wins.
 * The `panelReady && !forceFresh` no-op check is evaluated AFTER the previous
 * boot settles, so a plain boot that lands behind a forceFresh rebuild correctly
 * sees the finished panel and does nothing instead of racing it.
 */
declare function bootTerminalPanel(forceFresh?: boolean): Promise<void>;
export declare const _terminalPanelForTests: {
    bootTerminalPanel: typeof bootTerminalPanel;
    applyRootFolder: typeof applyRootFolder;
    writeToTerminal: typeof writeToTerminal;
    exitTerminalSession: typeof exitTerminalSession;
    redrawTerminal: typeof redrawTerminal;
};
export {};
//# sourceMappingURL=sidepanel.d.ts.map