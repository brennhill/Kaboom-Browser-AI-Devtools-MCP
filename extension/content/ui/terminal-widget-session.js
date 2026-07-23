/**
 * Purpose: Terminal session lifecycle — config persistence, session start/validate/persist.
 * Why: Isolates all daemon HTTP calls and chrome.storage I/O from UI and orchestrator logic.
 * Docs: docs/features/feature/terminal/index.md
 */
import { DEFAULT_SERVER_URL, StorageKey } from '../../lib/constants.js';
import { getDaemonStartHint } from '../../lib/brand.js';
import { getLocal, setSession, getSession, removeSessions, setLocal } from '../../lib/storage-utils.js';
import { state, getTerminalServerUrl } from './terminal-widget-types.js';
// =============================================================================
// CONFIG HELPERS — read/write chrome.storage.local
// =============================================================================
export async function getServerUrl() {
    try {
        const value = await getLocal(StorageKey.SERVER_URL);
        const url = value || DEFAULT_SERVER_URL;
        state.serverUrl = url;
        return url;
    }
    catch {
        return DEFAULT_SERVER_URL; // Extension context invalidated
    }
}
export async function getTerminalConfig() {
    try {
        const value = await getLocal(StorageKey.TERMINAL_CONFIG);
        const config = value || {};
        return config;
    }
    catch {
        return {}; // Extension context invalidated
    }
}
export function saveTerminalConfig(config) {
    try {
        void setLocal(StorageKey.TERMINAL_CONFIG, config);
    }
    catch {
        // Extension context invalidated — config won't persist but session still works
    }
}
async function getTerminalAICommand() {
    try {
        const value = await getLocal(StorageKey.TERMINAL_AI_COMMAND);
        const cmd = value || 'claude';
        return cmd;
    }
    catch {
        return 'claude';
    }
}
export async function getTerminalDevRoot() {
    try {
        const value = await getLocal(StorageKey.TERMINAL_DEV_ROOT);
        return value || '';
    }
    catch {
        return '';
    }
}
// =============================================================================
// SESSION PERSISTENCE — survives page refresh via chrome.storage.session
// =============================================================================
function persistSession(ss) {
    try {
        void setSession(StorageKey.TERMINAL_SESSION, ss);
    }
    catch { /* extension context invalidated */ }
}
export function clearPersistedSession() {
    try {
        void removeSessions([StorageKey.TERMINAL_SESSION, StorageKey.TERMINAL_UI_STATE]);
    }
    catch { /* extension context invalidated */ }
}
export function persistUIState(uiState) {
    try {
        void setSession(StorageKey.TERMINAL_UI_STATE, uiState);
    }
    catch { /* extension context invalidated */ }
}
export async function loadPersistedSession() {
    try {
        const sessionValue = await getSession(StorageKey.TERMINAL_SESSION);
        const uiValue = await getSession(StorageKey.TERMINAL_UI_STATE);
        const session = sessionValue;
        const uiState = uiValue || 'closed';
        return { session: session || null, uiState };
    }
    catch {
        return { session: null, uiState: 'closed' };
    }
}
// =============================================================================
// SESSION LIFECYCLE — start, validate
// =============================================================================
/** Validate that a persisted token is still alive on the daemon. */
export async function validateSession(token) {
    try {
        const base = await getServerUrl();
        const termUrl = getTerminalServerUrl(base);
        const resp = await fetch(`${termUrl}/terminal/validate?token=${encodeURIComponent(token)}`, { signal: AbortSignal.timeout(2000) });
        if (!resp.ok)
            return false;
        const data = await resp.json();
        return data.valid === true;
    }
    catch {
        return false;
    }
}
/**
 * List the sub-directories of `path`, or of the user's home when empty.
 *
 * The browser cannot resolve an absolute path by itself — `webkitdirectory` and
 * showDirectoryPicker() both withhold it — so picking a working directory has to
 * go through the daemon, which is already running shells in these directories.
 */
export async function listTerminalDirs(path) {
    try {
        const base = await getServerUrl();
        const termUrl = getTerminalServerUrl(base);
        const resp = await fetch(`${termUrl}/terminal/dirs?path=${encodeURIComponent(path)}`, { signal: AbortSignal.timeout(3000) });
        if (!resp.ok)
            return null;
        const data = await resp.json();
        return {
            path: data.path ?? path,
            parent: data.parent ?? '',
            entries: Array.isArray(data.entries) ? data.entries : [],
            truncated: data.truncated === true
        };
    }
    catch {
        return null; // Daemon unreachable; the caller falls back to typing a path.
    }
}
/** Persist the terminal root folder (the cwd new sessions spawn in). */
export async function setTerminalDevRoot(root) {
    try {
        await setLocal(StorageKey.TERMINAL_DEV_ROOT, root);
    }
    catch {
        // Extension context invalidated — nothing to persist into.
    }
}
/**
 * Stop the active PTY and forget it locally.
 *
 * Used when a setting that is fixed at spawn time changes (the working
 * directory), and by the explicit end-session control.
 */
export async function stopActiveSession() {
    const persisted = await loadPersistedSession();
    const sessionId = persisted.session?.sessionId;
    clearPersistedSession();
    if (!sessionId)
        return;
    try {
        const base = await getServerUrl();
        const termUrl = getTerminalServerUrl(base);
        await fetch(`${termUrl}/terminal/stop`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id: sessionId }),
            signal: AbortSignal.timeout(3000)
        });
    }
    catch {
        // Daemon unreachable — the local state is cleared either way.
    }
}
export async function startSession(config, onSandboxError) {
    const base = await getServerUrl();
    const termUrl = getTerminalServerUrl(base);
    const aiCommand = await getTerminalAICommand();
    const devRoot = await getTerminalDevRoot();
    try {
        // Build init_command: unset CLAUDECODE to avoid nesting detection, then launch the AI tool.
        const initCommand = aiCommand ? `unset CLAUDECODE 2>/dev/null; ${aiCommand}` : '';
        const resp = await fetch(`${termUrl}/terminal/start`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                cmd: config.cmd || '',
                args: config.args || [],
                dir: config.dir || devRoot || '',
                init_command: initCommand
            })
        });
        if (!resp.ok) {
            const body = await resp.json();
            // Session already exists — reconnect using the returned token.
            if (resp.status === 409 && body.token) {
                const ss = { sessionId: body.session_id ?? 'default', token: body.token };
                persistSession(ss);
                return ss;
            }
            // Sandbox restriction — the daemon's message is its diagnosis, `detail` is
            // the underlying error. Show both: the diagnosis can be wrong, the error can't.
            if (resp.status === 503 && body.error === 'sandbox_restricted') {
                const message = body.detail
                    ? `${body.message ?? 'Terminal start was refused.'} (${body.detail})`
                    : (body.message ?? 'Terminal start was refused.');
                reportStartFailure(message, body.instruction ?? '', body.command ?? '', onSandboxError);
                return null;
            }
            // Any other rejection. This used to only console.warn, so the side panel
            // rendered nothing at all and the terminal looked simply broken.
            reportStartFailure(`Terminal start was refused (HTTP ${resp.status}): ${body.error ?? 'unknown error'}.`, '', '', onSandboxError);
            return null;
        }
        const data = await resp.json();
        const ss = { sessionId: data.session_id, token: data.token };
        persistSession(ss);
        return ss;
    }
    catch (err) {
        reportStartFailure('Terminal session start failed: ' + (err instanceof Error ? err.message : String(err)) + '.', getDaemonStartHint(), '', onSandboxError);
        return null;
    }
}
/**
 * Route a start failure to the panel when a handler is available, and always log
 * it. A failure that only reaches the console leaves the panel blank, which reads
 * as "the terminal is broken" rather than "here is what went wrong".
 */
function reportStartFailure(message, instruction, command, onError) {
    console.warn(`[KaBOOM!] ${message} ${instruction} ${command}`.trimEnd());
    onError?.(message, instruction, command);
}
//# sourceMappingURL=terminal-widget-session.js.map