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
export declare function getTerminalDevRoot(): Promise<string>;
export declare function clearPersistedSession(): void;
export declare function persistUIState(uiState: TerminalUIState): void;
export declare function loadPersistedSession(): Promise<{
    session: TerminalSessionState | null;
    uiState: TerminalUIState;
}>;
/** Validate that a persisted token is still alive on the daemon. */
export declare function validateSession(token: string): Promise<boolean>;
/** One selectable directory from the daemon's listing. */
export interface TerminalDirEntry {
    name: string;
    path: string;
}
/** A directory and its immediate sub-directories. */
export interface TerminalDirListing {
    path: string;
    parent: string;
    entries: TerminalDirEntry[];
    truncated: boolean;
}
/**
 * Why a directory listing could not be produced.
 * - `unreachable`: the daemon did not answer (down, refused, timed out).
 * - `outdated`: the daemon answered 404 with no error body — it predates
 *   `/terminal/dirs`. A version problem, not a connectivity or path one.
 * - `not_found`: the daemon answered 404 *with* a `not_found` error body — it has
 *   the endpoint, but the requested folder does not exist (e.g. a saved root that
 *   was since deleted). A current daemon, so telling the user to update it is wrong.
 * - `denied`: the daemon answered 403 — the folder exists but cannot be read
 *   (permissions). Also a reachable, current daemon.
 */
export type TerminalDirsFailure = 'unreachable' | 'outdated' | 'not_found' | 'denied';
/** The listing, or the reason it could not be fetched. */
export type TerminalDirsResult = {
    ok: true;
    listing: TerminalDirListing;
} | {
    ok: false;
    reason: TerminalDirsFailure;
};
/**
 * List the sub-directories of `path`, or of the user's home when empty.
 *
 * The browser cannot resolve an absolute path by itself — `webkitdirectory` and
 * showDirectoryPicker() both withhold it — so picking a working directory has to
 * go through the daemon, which is already running shells in these directories.
 *
 * Distinguishes a daemon that is down from one that is merely too old to have the
 * endpoint: a 404 is a reachable daemon, and telling the user it is unreachable
 * sends them debugging a connection that is fine.
 */
export declare function listTerminalDirs(path: string): Promise<TerminalDirsResult>;
/** Persist the terminal root folder (the cwd new sessions spawn in). */
export declare function setTerminalDevRoot(root: string): Promise<void>;
/**
 * Stop the active PTY and forget it locally.
 *
 * Used when a setting that is fixed at spawn time changes (the working
 * directory), and by the explicit end-session control.
 */
export declare function stopActiveSession(): Promise<void>;
export declare function startSession(config: TerminalConfig, onSandboxError?: TerminalSandboxErrorHandler): Promise<TerminalSessionState | null>;
//# sourceMappingURL=terminal-widget-session.d.ts.map