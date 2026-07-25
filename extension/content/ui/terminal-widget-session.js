/**
 * Purpose: Terminal session lifecycle — config persistence, session start/validate/persist.
 * Why: Isolates all daemon HTTP calls and chrome.storage I/O from UI and orchestrator logic.
 * Docs: docs/features/feature/terminal/index.md
 */
import { DEFAULT_SERVER_URL, StorageKey } from '../../lib/constants.js';
import { getDaemonStartHint } from '../../lib/brand.js';
import { getLocal, setSession, getSession, removeSessions, setLocal, persist } from '../../lib/storage-utils.js';
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
        persist(setLocal(StorageKey.TERMINAL_CONFIG, config), 'terminal-config');
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
        persist(setSession(StorageKey.TERMINAL_SESSION, ss), 'terminal-session');
    }
    catch { /* extension context invalidated */ }
}
export function clearPersistedSession() {
    try {
        persist(removeSessions([StorageKey.TERMINAL_SESSION, StorageKey.TERMINAL_UI_STATE]), 'terminal-session-clear');
    }
    catch { /* extension context invalidated */ }
}
export function persistUIState(uiState) {
    try {
        persist(setSession(StorageKey.TERMINAL_UI_STATE, uiState), 'terminal-ui-state');
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
 *
 * Distinguishes a daemon that is down from one that is merely too old to have the
 * endpoint: a 404 is a reachable daemon, and telling the user it is unreachable
 * sends them debugging a connection that is fine.
 */
export async function listTerminalDirs(path) {
    let resp;
    try {
        const base = await getServerUrl();
        const termUrl = getTerminalServerUrl(base);
        resp = await fetch(`${termUrl}/terminal/dirs?path=${encodeURIComponent(path)}`, { signal: AbortSignal.timeout(3000) });
    }
    catch {
        return { ok: false, reason: 'unreachable' }; // No answer at all.
    }
    if (resp.status === 404) {
        // 404 is ambiguous: a daemon that predates /terminal/dirs 404s the whole
        // route with Chrome's plain-text ServeMux default (no error body), while a
        // current daemon 404s a *missing directory* with {"error":"not_found"}.
        // Telling a user whose folder was deleted to update a current daemon sends
        // them fixing the wrong thing, so distinguish by the presence of our body.
        const daemonError = await readDaemonError(resp);
        return { ok: false, reason: daemonError === 'not_found' ? 'not_found' : 'outdated' };
    }
    // 403 is a reachable, current daemon that cannot read the folder (permissions),
    // which is a different problem — and message — from a daemon that is down.
    if (resp.status === 403)
        return { ok: false, reason: 'denied' };
    if (!resp.ok)
        return { ok: false, reason: 'unreachable' };
    try {
        const data = await resp.json();
        return {
            ok: true,
            listing: {
                path: data.path ?? path,
                parent: data.parent ?? '',
                entries: Array.isArray(data.entries) ? data.entries : [],
                truncated: data.truncated === true
            }
        };
    }
    catch {
        return { ok: false, reason: 'unreachable' }; // Reached it, but the body was unusable.
    }
}
/**
 * Read the daemon's structured `error` code from a response body, or '' when the
 * body is not our JSON shape — which is exactly how an old daemon's plain-text
 * 404 (no `/terminal/dirs` route) is told apart from a current daemon's
 * `{"error":"not_found"}` for a directory that does not exist.
 */
async function readDaemonError(resp) {
    try {
        const body = await resp.json();
        return typeof body.error === 'string' ? body.error : '';
    }
    catch {
        return ''; // Plain-text / empty body — not one of our structured errors.
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
 * Poll `/terminal/validate` until the token no longer maps to a live session, or
 * a bounded number of attempts elapse. Used to CONFIRM a stop actually landed.
 */
async function waitForSessionTornDown(token, attempts = 5, delayMs = 200) {
    for (let i = 0; i < attempts; i++) {
        if (!(await validateSession(token)))
            return true; // gone
        await new Promise((resolve) => setTimeout(resolve, delayMs));
    }
    return false; // still alive after retries — the daemon is wedged
}
/**
 * Stop the active PTY and forget it locally.
 *
 * Used when a setting that is fixed at spawn time changes (the working
 * directory), and by the explicit end-session control.
 *
 * The stop must be CONFIRMED, not fire-and-forget: the session id is a fixed
 * "default", so if the stop times out but the old session survives, the following
 * /terminal/start returns 409 and the client silently reconnects to the OLD
 * working directory while the UI shows the newly-picked one. A 200 (or a 404 =
 * already gone) confirms teardown; otherwise poll validate before returning so a
 * fresh start cannot 409-reattach to a stale cwd.
 */
export async function stopActiveSession() {
    const persisted = await loadPersistedSession();
    const sessionId = persisted.session?.sessionId;
    const token = persisted.session?.token;
    clearPersistedSession();
    if (!sessionId)
        return;
    try {
        const base = await getServerUrl();
        const termUrl = getTerminalServerUrl(base);
        const resp = await fetch(`${termUrl}/terminal/stop`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id: sessionId }),
            signal: AbortSignal.timeout(3000)
        });
        // Stop is synchronous server-side: 200 = torn down, 404 = already gone.
        if (resp.ok || resp.status === 404)
            return;
    }
    catch {
        // Timed out / unreachable — the stop is unconfirmed. Verify below.
    }
    if (token)
        await waitForSessionTornDown(token);
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
                reportStartFailure(message, body.instruction ?? '', body.command ?? '', 'sandbox', onSandboxError);
                return null;
            }
            // Any other rejection from a reachable daemon. This used to only
            // console.warn, so the side panel rendered nothing at all and the terminal
            // looked simply broken. Classified `unavailable` (reachable but not ready)
            // so the UI shows the recoverable no-session state, not a dead-end error.
            reportStartFailure(`Terminal start was refused (HTTP ${resp.status}): ${body.error ?? 'unknown error'}.`, '', '', 'unavailable', onSandboxError);
            return null;
        }
        const data = await resp.json();
        const ss = { sessionId: data.session_id, token: data.token };
        persistSession(ss);
        return ss;
    }
    catch (err) {
        // Transport failure — the daemon did not answer at all. This is `unreachable`:
        // a real failure the user must see even when no panel body is mounted yet.
        reportStartFailure('Terminal session start failed: ' + (err instanceof Error ? err.message : String(err)) + '.', getDaemonStartHint(), '', 'unreachable', onSandboxError);
        return null;
    }
}
/**
 * Route a start failure to the panel when a handler is available, and always log
 * it. A failure that only reaches the console leaves the panel blank, which reads
 * as "the terminal is broken" rather than "here is what went wrong".
 */
function reportStartFailure(message, instruction, command, kind, onError) {
    console.warn(`[KaBOOM!] (${kind}) ${message} ${instruction} ${command}`.trimEnd());
    onError?.(message, instruction, command, kind);
}
//# sourceMappingURL=terminal-widget-session.js.map