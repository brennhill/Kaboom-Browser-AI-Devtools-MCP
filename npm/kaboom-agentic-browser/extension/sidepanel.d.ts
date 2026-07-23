/**
 * Purpose: Side panel host for the Kaboom terminal.
 * Why: Removes the terminal from page context so CSP on arbitrary sites cannot
 * interfere with the xterm host, while keeping the session and reconnect model intact.
 * Docs: docs/features/feature/terminal/index.md
 */
declare function redrawTerminal(): void;
declare function exitTerminalSession(): Promise<void>;
declare function writeToTerminal(text: string): void;
declare function bootTerminalPanel(forceFresh?: boolean): Promise<void>;
export declare const _terminalPanelForTests: {
    bootTerminalPanel: typeof bootTerminalPanel;
    writeToTerminal: typeof writeToTerminal;
    exitTerminalSession: typeof exitTerminalSession;
    redrawTerminal: typeof redrawTerminal;
};
export {};
//# sourceMappingURL=sidepanel.d.ts.map