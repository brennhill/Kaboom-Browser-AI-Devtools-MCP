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
async function getTerminalDevRoot() {
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
            // Sandbox restriction — show actionable instructions to the user.
            if (resp.status === 503 && body.error === 'sandbox_restricted') {
                if (onSandboxError) {
                    onSandboxError(body.message ?? '', body.instruction ?? '', body.command ?? '');
                }
                else {
                    console.warn('[KaBOOM!] Terminal sandbox restriction: ' +
                        (body.message ?? 'no message') +
                        '. ' +
                        (body.instruction ?? 'No instruction provided.'));
                }
                return null;
            }
            // Session already exists — reconnect using the returned token.
            if (resp.status === 409 && body.token) {
                const ss = { sessionId: body.session_id ?? 'default', token: body.token };
                persistSession(ss);
                return ss;
            }
            console.warn('[KaBOOM!] Terminal session rejected (HTTP ' + resp.status + '): ' +
                (body.error ?? 'unknown') + '. Check the daemon logs for details.');
            return null;
        }
        const data = await resp.json();
        const ss = { sessionId: data.session_id, token: data.token };
        persistSession(ss);
        return ss;
    }
    catch (err) {
        console.warn('[KaBOOM!] Terminal session start failed: ' +
            (err instanceof Error ? err.message : String(err)) +
            `. ${getDaemonStartHint()}`);
        return null;
    }
}
//# sourceMappingURL=terminal-widget-session.js.map