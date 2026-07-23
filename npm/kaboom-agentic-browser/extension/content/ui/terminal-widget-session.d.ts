/**
 * Purpose: Terminal session lifecycle — config persistence, session start/validate/persist.
 * Why: Isolates all daemon HTTP calls and chrome.storage I/O from UI and orchestrator logic.
 * Docs: docs/features/feature/terminal/index.md
 */
import { type TerminalConfig, type TerminalSessionState, type TerminalUIState } from './terminal-widget-types.js';
export type TerminalSandboxErrorHandler = (message: string, instruction: string, command: string) => void;
export declare function getServerUrl(): Promise<string>;
export declare function getTerminalConfig(): Promise<TerminalConfig>;
export declare function saveTerminalConfig(config: TerminalConfig): void;
export declare function clearPersistedSession(): void;
export declare function persistUIState(uiState: TerminalUIState): void;
export declare function loadPersistedSession(): Promise<{
    session: TerminalSessionState | null;
    uiState: TerminalUIState;
}>;
/** Validate that a persisted token is still alive on the daemon. */
export declare function validateSession(token: string): Promise<boolean>;
export declare function startSession(config: TerminalConfig, onSandboxError?: TerminalSandboxErrorHandler): Promise<TerminalSessionState | null>;
//# sourceMappingURL=terminal-widget-session.d.ts.map