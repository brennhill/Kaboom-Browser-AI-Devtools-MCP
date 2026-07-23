/**
 * Purpose: Implements the extension options page state, persistence, and background synchronization handlers.
 * Why: Keeps operator-facing runtime settings explicit and immediately applied without extension restarts.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */
/**
 * @fileoverview options.ts — Extension settings page for user-configurable options.
 * Manages server URL, domain filters (allowlist/blocklist), screenshot-on-error toggle,
 * source map resolution toggle, and interception deferral toggle.
 * Persists settings via chrome.storage.local and notifies the background worker
 * of changes so they take effect without requiring extension reload.
 * Design: Toggle controls use CSS class 'active' for state. Domain filters are
 * stored as newline-separated strings, parsed to arrays on save.
 */
import { SettingName, StorageKey, DEFAULT_SERVER_URL } from './lib/constants.js';
import { buildDaemonHeaders, buildDaemonJSONRequestInit } from './lib/daemon-http.js';
import { getLocal, getLocals, setLocals } from './lib/storage-utils.js';
/**
 * Apply persisted theme as early as possible without inline HTML scripts.
 * Keeps options page CSP-compliant (MV3 disallows inline scripts by default).
 */
function bootstrapTheme() {
    if (typeof document === 'undefined' || typeof chrome === 'undefined' || !chrome.storage?.local)
        return;
    void getLocal(StorageKey.THEME).then((value) => {
        if (value === 'light') {
            document.body?.classList.add('light-theme');
        }
    });
}
bootstrapTheme();
/**
 * Sync the terminal dev root to the daemon's active_codebase config.
 * Best-effort — failure doesn't block the save flow.
 */
function syncDevRootToDaemon(serverUrl, devRoot) {
    fetch(`${serverUrl}/config/active-codebase`, buildDaemonJSONRequestInit({ path: devRoot }, { method: 'PUT', signal: AbortSignal.timeout(3000) })).catch(() => {
        // Best-effort sync — daemon may be offline
    });
}
/**
 * Load the active_codebase from the daemon and update the dev root input if empty.
 * Called during options load to pull daemon-side changes (e.g., set via MCP).
 */
function loadActiveCodebaseFromDaemon(serverUrl) {
    fetch(`${serverUrl}/config/active-codebase`, {
        signal: AbortSignal.timeout(3000),
        headers: buildDaemonHeaders({ contentType: null })
    }).then(resp => {
        if (!resp.ok)
            return;
        return resp.json();
    }).then(data => {
        if (!data?.active_codebase)
            return;
        const devRootInput = document.getElementById('terminal-dev-root');
        // Only fill if the input is currently empty (don't overwrite user edits)
        if (devRootInput && !devRootInput.value.trim()) {
            devRootInput.value = data.active_codebase;
        }
    }).catch(() => {
        // Daemon unreachable — ignore
    });
}
/**
 * Load saved options
 */
export async function loadOptions() {
    const result = await getLocals([
        StorageKey.SERVER_URL,
        StorageKey.SCREENSHOT_ON_ERROR,
        StorageKey.SOURCE_MAP_ENABLED,
        StorageKey.DEFERRAL_ENABLED,
        StorageKey.DEBUG_MODE,
        StorageKey.THEME,
        StorageKey.TERMINAL_AI_COMMAND,
        StorageKey.TERMINAL_DEV_ROOT
    ]);
    // Set server URL
    const serverUrlInput = document.getElementById('server-url-input');
    if (serverUrlInput) {
        serverUrlInput.value = result.serverUrl || DEFAULT_SERVER_URL;
    }
    // Set theme toggle state (default: dark, toggle active = light)
    const themeToggle = document.getElementById('theme-toggle');
    if (result.theme === 'light') {
        themeToggle?.classList.add('active');
        document.body.classList.add('light-theme');
    }
    // Set screenshot toggle state
    const screenshotToggle = document.getElementById('screenshot-toggle');
    if (result.screenshotOnError) {
        screenshotToggle?.classList.add('active');
    }
    // Set source map toggle state
    const sourcemapToggle = document.getElementById('sourcemap-toggle');
    if (result.sourceMapEnabled) {
        sourcemapToggle?.classList.add('active');
    }
    // Set deferral toggle state (default: enabled/active)
    const deferralToggle = document.getElementById('deferral-toggle');
    if (result.deferralEnabled !== false) {
        deferralToggle?.classList.add('active');
    }
    // Set debug mode toggle state
    const debugToggle = document.getElementById('debug-mode-toggle');
    if (result.debugMode) {
        debugToggle?.classList.add('active');
    }
    // Set terminal AI command
    const aiCmdInput = document.getElementById('terminal-ai-command');
    if (aiCmdInput) {
        aiCmdInput.value = result.kaboom_terminal_ai_command || 'claude';
    }
    // Set terminal dev root
    const devRootInput = document.getElementById('terminal-dev-root');
    if (devRootInput) {
        devRootInput.value = result.kaboom_terminal_dev_root || '';
    }
}
/**
 * Save options to storage and notify background
 * ARCHITECTURE: Options page writes to storage directly (for immediate persistence),
 * then sends messages to background so it can update its internal state.
 * Background is the authoritative source of truth for actual behavior.
 * Example: debugMode=true in storage enables logging immediately, AND background
 * updates its debugMode variable so new logs use the new setting.
 */
// #lizard forgives
export function saveOptions() {
    const serverUrlInput = document.getElementById('server-url-input');
    const serverUrl = serverUrlInput?.value.trim() || DEFAULT_SERVER_URL;
    const screenshotToggle = document.getElementById('screenshot-toggle');
    const screenshotOnError = screenshotToggle?.classList.contains('active') || false;
    const sourcemapToggle = document.getElementById('sourcemap-toggle');
    const sourceMapEnabled = sourcemapToggle?.classList.contains('active') || false;
    const deferralToggle = document.getElementById('deferral-toggle');
    const deferralEnabled = deferralToggle?.classList.contains('active') || false;
    const debugToggle = document.getElementById('debug-mode-toggle');
    const debugMode = debugToggle?.classList.contains('active') || false;
    const themeToggle = document.getElementById('theme-toggle');
    const theme = themeToggle?.classList.contains('active') ? 'light' : 'dark';
    const aiCmdInput = document.getElementById('terminal-ai-command');
    const terminalAICommand = aiCmdInput?.value.trim() || '';
    const devRootInput = document.getElementById('terminal-dev-root');
    const terminalDevRoot = devRootInput?.value.trim() || '';
    setLocals({
        serverUrl,
        screenshotOnError,
        sourceMapEnabled,
        deferralEnabled,
        debugMode,
        theme,
        [StorageKey.TERMINAL_AI_COMMAND]: terminalAICommand,
        [StorageKey.TERMINAL_DEV_ROOT]: terminalDevRoot
    }).then(() => {
        // Show saved message
        const message = document.getElementById('saved-message');
        message?.classList.add('show');
        // Notify background of changes so it can update its in-memory state
        chrome.runtime.sendMessage({ type: SettingName.SERVER_URL, url: serverUrl });
        chrome.runtime.sendMessage({ type: 'set_screenshot_on_error', enabled: screenshotOnError });
        chrome.runtime.sendMessage({ type: 'set_source_map_enabled', enabled: sourceMapEnabled });
        chrome.runtime.sendMessage({ type: SettingName.DEFERRAL, enabled: deferralEnabled });
        chrome.runtime.sendMessage({ type: 'set_debug_mode', enabled: debugMode });
        // Sync terminal dev root to daemon so MCP and terminal use the same CWD
        if (terminalDevRoot) {
            syncDevRootToDaemon(serverUrl, terminalDevRoot);
        }
        // Hide message after 2 seconds
        setTimeout(() => {
            message?.classList.remove('show');
        }, 2000);
    });
}
/**
 * Toggle screenshot setting
 */
function toggleScreenshot() {
    const toggle = document.getElementById('screenshot-toggle');
    toggle?.classList.toggle('active');
}
/**
 * Toggle source map setting
 */
function toggleSourceMap() {
    const toggle = document.getElementById('sourcemap-toggle');
    toggle?.classList.toggle('active');
}
/**
 * Toggle deferral setting
 */
export function toggleDeferral() {
    const toggle = document.getElementById('deferral-toggle');
    toggle?.classList.toggle('active');
}
/**
 * Toggle debug mode setting
 */
export function toggleDebugMode() {
    const toggle = document.getElementById('debug-mode-toggle');
    toggle?.classList.toggle('active');
}
/**
 * Toggle theme between dark (default) and light
 */
export function toggleTheme() {
    const toggle = document.getElementById('theme-toggle');
    toggle?.classList.toggle('active');
    document.body.classList.toggle('light-theme');
}
/**
 * Test connection to server
 */
export async function testConnection() {
    const btn = document.getElementById('test-connection-btn');
    const resultEl = document.getElementById('test-result');
    const serverUrlInput = document.getElementById('server-url-input');
    const serverUrl = serverUrlInput?.value.trim() || DEFAULT_SERVER_URL;
    if (btn) {
        btn.disabled = true;
        btn.textContent = '...';
    }
    if (resultEl) {
        resultEl.style.display = 'block';
        resultEl.style.background = 'rgba(88, 166, 255, 0.1)';
        resultEl.style.color = '#58a6ff';
        resultEl.textContent = 'Connecting...';
    }
    try {
        const resp = await fetch(`${serverUrl}/health`, {
            signal: AbortSignal.timeout(3000),
            headers: buildDaemonHeaders({ contentType: null })
        });
        if (!resp.ok) {
            throw new Error(`Failed to check server health at ${serverUrl}: HTTP ${resp.status} ${resp.statusText}`);
        }
        const data = (await resp.json());
        if (resultEl) {
            resultEl.style.background = 'rgba(63, 185, 80, 0.1)';
            resultEl.style.color = '#3fb950';
            resultEl.textContent = `Connected — v${data.version}, ${data.logs?.entries ?? 0} entries`;
        }
    }
    catch (err) {
        if (resultEl) {
            resultEl.style.background = 'rgba(248, 81, 73, 0.1)';
            resultEl.style.color = '#f85149';
            const errorMsg = err instanceof Error ? err.message : 'Unknown error';
            if (errorMsg.includes('timeout')) {
                resultEl.textContent = `Failed — server not responding at ${serverUrl}. Is it running? Run: npx kaboom-mcp`;
            }
            else if (errorMsg.includes('HTTP 404')) {
                resultEl.textContent = `Failed — server running but health endpoint not found. Is this KaBOOM! MCP v5.8.0+?`;
            }
            else if (errorMsg.includes('HTTP')) {
                resultEl.textContent = `Failed — server error (${errorMsg}). Check server logs.`;
            }
            else {
                resultEl.textContent = `Failed — ${errorMsg}. Is the server running? Run: npx kaboom-mcp`;
            }
        }
    }
    finally {
        if (btn) {
            btn.disabled = false;
            btn.textContent = 'Test';
        }
    }
}
/**
 * Export debug log to a downloadable file
 */
export async function handleExportDebugLog() {
    const exportBtn = document.getElementById('export-debug-btn');
    if (exportBtn) {
        exportBtn.disabled = true;
        exportBtn.textContent = 'Exporting...';
    }
    return new Promise((resolve) => {
        chrome.runtime.sendMessage({ type: 'get_debug_log' }, (response) => {
            if (exportBtn) {
                exportBtn.disabled = false;
                exportBtn.textContent = 'Export Debug Log';
            }
            if (response?.log) {
                // Create downloadable blob
                const blob = new Blob([response.log], { type: 'application/json' });
                const url = URL.createObjectURL(blob);
                const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
                const filename = `kaboom-debug-${timestamp}.json`;
                // Trigger download
                const a = document.createElement('a');
                a.href = url;
                a.download = filename;
                document.body.appendChild(a);
                a.click();
                document.body.removeChild(a);
                URL.revokeObjectURL(url);
                resolve({ success: true, filename });
            }
            else {
                resolve({ success: false, error: 'Failed to get debug log' });
            }
        });
    });
}
/**
 * Clear the debug log buffer
 */
export async function handleClearDebugLog() {
    return new Promise((resolve) => {
        chrome.runtime.sendMessage({ type: 'clear_debug_log' }, (response) => {
            resolve(response || { success: false });
        });
    });
}
// Initialize
document.addEventListener('DOMContentLoaded', () => {
    void loadOptions();
    // After chrome.storage options load, also pull active_codebase from daemon
    // to sync any MCP-side changes back to the extension options UI.
    void getLocal(StorageKey.SERVER_URL).then((value) => {
        const url = value || DEFAULT_SERVER_URL;
        loadActiveCodebaseFromDaemon(url);
    });
    const saveBtn = document.getElementById('save-btn');
    saveBtn?.addEventListener('click', saveOptions);
    const screenshotToggle = document.getElementById('screenshot-toggle');
    screenshotToggle?.addEventListener('click', toggleScreenshot);
    const sourcemapToggle = document.getElementById('sourcemap-toggle');
    sourcemapToggle?.addEventListener('click', toggleSourceMap);
    const deferralToggle = document.getElementById('deferral-toggle');
    deferralToggle?.addEventListener('click', toggleDeferral);
    const debugToggle = document.getElementById('debug-mode-toggle');
    debugToggle?.addEventListener('click', toggleDebugMode);
    const themeToggle = document.getElementById('theme-toggle');
    themeToggle?.addEventListener('click', toggleTheme);
    const testBtn = document.getElementById('test-connection-btn');
    testBtn?.addEventListener('click', testConnection);
    // Debug log buttons
    const exportDebugBtn = document.getElementById('export-debug-btn');
    if (exportDebugBtn) {
        exportDebugBtn.addEventListener('click', handleExportDebugLog);
    }
    const clearDebugBtn = document.getElementById('clear-debug-btn');
    if (clearDebugBtn) {
        clearDebugBtn.addEventListener('click', handleClearDebugLog);
    }
});
//# sourceMappingURL=options.js.map