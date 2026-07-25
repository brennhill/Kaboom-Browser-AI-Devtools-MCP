/**
 * Purpose: Side panel host for the Kaboom terminal.
 * Why: Removes the terminal from page context so CSP on arbitrary sites cannot
 * interfere with the xterm host, while keeping the session and reconnect model intact.
 * Docs: docs/features/feature/terminal/index.md
 */
import { StorageKey, TERMINAL_PANEL_PORT } from './lib/constants.js';
import { onStorageChanged } from './lib/storage-utils.js';
import { state, resetAllState, getTerminalServerUrl, WIDGET_ID, IFRAME_ID, HEADER_ID, TERMINAL_BODY_ID, DISCONNECT_TERMINAL_BUTTON_ID, CLOSE_TERMINAL_BUTTON_ID, REDRAW_TERMINAL_BUTTON_ID, MINIMIZE_TERMINAL_BUTTON_ID, TERMINAL_WRITE_SUBMIT_DELAY_MS, TERMINAL_GUARD_POLL_MS } from './content/ui/terminal-widget-types.js';
import { getServerUrl, getTerminalConfig, persistUIState, loadPersistedSession, clearPersistedSession, validateSession, startSession, getTerminalDevRoot, setTerminalDevRoot, stopActiveSession } from './content/ui/terminal-widget-session.js';
import { showActionToast } from './content/ui/toast.js';
import { createRootFolderBar } from './content/ui/terminal-root-folder.js';
import { renderNoSessionState, renderStartFailure } from './content/ui/terminal-panel-states.js';
import { notifyIframe, resetWriteGuardState, shouldDeferQueuedWrite, maybeShowQueuedWriteToast, scheduleQueuedWriteFlush, scheduleQueuedSubmit } from './content/ui/terminal-write-guard.js';
function freshPanelUi() {
    return {
        rootEl: null,
        terminalShellEl: null,
        terminalBodyEl: null,
        statusDotEl: null,
        minimizeButtonEl: null,
        runtimeListenerInstalled: false,
        storageListenerInstalled: false,
        unloadListenerInstalled: false,
        panelReady: false,
        pendingSandboxError: null,
        panelCloseIntent: null,
        presencePort: null,
        rootFolderBar: null,
        bootGeneration: 0
    };
}
const panel = freshPanelUi();
/**
 * Reset all panel-UI state to a clean slate. `panel` is const, so mutate its
 * fields in place (Object.assign) rather than rebind. Lets a test start from a
 * known-empty panel without reloading the module.
 */
export function resetPanelUi() {
    Object.assign(panel, freshPanelUi());
}
/** Backoff before re-announcing presence after a service-worker restart. */
const PRESENCE_RECONNECT_DELAY_MS = 500;
function getHostTabIdFromLocation() {
    try {
        const raw = new URLSearchParams(globalThis.location?.search ?? '').get('tabId');
        if (!raw)
            return undefined;
        const parsed = Number(raw);
        return Number.isFinite(parsed) ? parsed : undefined;
    }
    catch {
        return undefined;
    }
}
async function getHostTabId() {
    const fromLocation = getHostTabIdFromLocation();
    if (fromLocation !== undefined)
        return fromLocation;
    if (!chrome.tabs?.query)
        return undefined;
    try {
        const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
        return tab?.id;
    }
    catch {
        return undefined;
    }
}
/**
 * Close the browser side panel.
 *
 * `chrome.sidePanel.close()` only exists in very recent Chrome. The old code
 * bailed out silently when it was missing, so the close button did nothing at
 * all — combined with unmountPanel() that left a blank panel the user could not
 * close *or* recover. `window.close()` works from the panel document itself on
 * every version that has side panels, so it is the fallback and the last word.
 */
async function closeBrowserSidePanel() {
    if (chrome.sidePanel?.close) {
        const tabId = await getHostTabId();
        if (tabId !== undefined) {
            try {
                await chrome.sidePanel.close({ tabId });
                return;
            }
            catch {
                // Fall through to window.close().
            }
        }
    }
    try {
        window.close();
    }
    catch {
        // Nothing else to try; the panel stays open but remains usable.
    }
}
function setPanelVisible(visible) {
    state.visible = visible;
    if (!panel.rootEl)
        return;
    panel.rootEl.style.opacity = visible ? '1' : '0';
    panel.rootEl.style.pointerEvents = visible ? 'auto' : 'none';
}
// The terminal is a full-height side panel; there is no collapse-in-place state
// (the old MINIMIZED_WIDGET_HEIGHT was leftover from the in-page-widget era and
// was never actually applied \u2014 every caller passed `visible: true`). The body is
// simply shown; "minimize" and "close" both dismiss the whole panel and keep the
// session (see closePanelWithIntent).
function showTerminalBody() {
    if (!panel.terminalBodyEl || !panel.terminalShellEl)
        return;
    panel.terminalBodyEl.style.display = 'block';
    panel.terminalShellEl.style.height = '100%';
    panel.terminalShellEl.style.minHeight = '0';
    panel.terminalShellEl.style.flex = '1 1 auto';
}
/**
 * Show the recoverable no-session state in the terminal body.
 */
function showNoSessionState() {
    if (!panel.terminalBodyEl)
        return;
    renderNoSessionState(panel.terminalBodyEl, () => { void bootTerminalPanel(true); });
}
/**
 * Mount the root-folder bar and keep it showing the current root.
 *
 * The bar is built synchronously so the shell lays out immediately; the stored
 * root arrives a tick later and is filled in.
 */
function createRootFolderBarElement() {
    const bar = createRootFolderBar({
        initialRoot: '',
        onApply: (root) => { void applyRootFolder(root); }
    });
    panel.rootFolderBar = bar;
    void getTerminalDevRoot().then((root) => bar.setRoot(root));
    return bar.element;
}
/**
 * Persist the root folder and restart the shell there. A running PTY cannot be
 * moved — its cwd is fixed at spawn — so the old session is stopped first.
 */
async function applyRootFolder(root) {
    await setTerminalDevRoot(root);
    await stopActiveSession();
    showActionToast('Terminal root folder set', root || '(auto-detect)', 'success', 2500);
    await bootTerminalPanel(true);
}
/**
 * Show a start failure, remembering it so a later remount can show it again.
 */
function showSandboxError(message, instruction, command) {
    if (!panel.terminalBodyEl)
        return;
    panel.pendingSandboxError = { message, instruction, command };
    renderStartFailure(panel.terminalBodyEl, message, instruction, command);
}
function updateStatusDot(dotState) {
    if (!panel.statusDotEl)
        return;
    switch (dotState) {
        case 'connected':
            panel.statusDotEl.style.background = '#9ece6a';
            break;
        case 'disconnected':
            panel.statusDotEl.style.background = '#e0af68';
            break;
        case 'exited':
            panel.statusDotEl.style.background = '#f7768e';
            break;
    }
}
function handleIframeMessage(event) {
    if (!event.data || event.data.source !== 'kaboom-terminal')
        return;
    try {
        const termOrigin = getTerminalServerUrl(state.serverUrl);
        if (event.origin !== termOrigin)
            return;
    }
    catch {
        return;
    }
    switch (event.data.event) {
        case 'connected':
            // Trail for diagnosing "can't type": these WS transitions are where the
            // terminal loses input when the daemon terminal-server (port+1) blinks.
            console.log('[KaBOOM! terminal] ws connected');
            updateStatusDot('connected');
            state.terminalConnected = true;
            if (state.queuedWrites.length > 0 && !state.queuedWriteInFlight) {
                scheduleQueuedWriteFlush(0);
            }
            break;
        case 'disconnected':
            console.log('[KaBOOM! terminal] ws disconnected (input paused; writes will queue)');
            updateStatusDot('disconnected');
            state.terminalConnected = false;
            state.terminalFocused = false;
            break;
        case 'exited':
            console.log('[KaBOOM! terminal] session exited (write-guard reset)');
            updateStatusDot('exited');
            state.terminalConnected = false;
            state.terminalFocused = false;
            resetWriteGuardState();
            break;
        case 'focus':
            state.terminalFocused = Boolean(event.data.data?.focused);
            if (state.terminalFocused) {
                state.lastTypingAt = Date.now();
            }
            else if (state.queuedWrites.length > 0 && !state.queuedWriteInFlight) {
                scheduleQueuedWriteFlush(0);
            }
            break;
        case 'typing': {
            const rawAt = event.data.data?.at;
            const parsedAt = typeof rawAt === 'number' && Number.isFinite(rawAt) ? rawAt : Date.now();
            state.terminalFocused = true;
            state.lastTypingAt = parsedAt;
            break;
        }
    }
}
/**
 * Build one 24×24 icon button for the terminal header. All four header controls
 * (disconnect / redraw / minimize / close) share the same box, hover affordance,
 * and click-swallowing wrapper — only the id, glyph, tooltip, accent colour, and
 * action differ. One factory keeps them from drifting apart (repo rule 19/DRY).
 */
function createTerminalHeaderButton(opts) {
    const button = document.createElement('button');
    button.id = opts.id;
    button.textContent = opts.glyph;
    button.title = opts.title;
    button.type = 'button';
    Object.assign(button.style, {
        width: '24px',
        height: '24px',
        border: 'none',
        background: 'transparent',
        color: opts.color,
        fontSize: opts.fontSize ?? '14px',
        cursor: 'pointer',
        borderRadius: '4px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: '0'
    });
    button.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        opts.onClick();
    });
    return button;
}
function createTerminalHeader() {
    const header = document.createElement('div');
    header.id = HEADER_ID;
    Object.assign(header.style, {
        height: '38px',
        background: '#16161e',
        display: 'flex',
        alignItems: 'center',
        padding: '0 10px 0 12px',
        gap: '8px',
        borderBottom: '1px solid #292e42',
        flexShrink: '0'
    });
    panel.statusDotEl = document.createElement('span');
    panel.statusDotEl.className = 'kaboom-terminal-status-dot';
    Object.assign(panel.statusDotEl.style, {
        width: '8px',
        height: '8px',
        borderRadius: '50%',
        background: '#565f89',
        flexShrink: '0',
        transition: 'background 200ms ease'
    });
    const titleSpan = document.createElement('span');
    titleSpan.textContent = 'KaBOOM! Terminal';
    Object.assign(titleSpan.style, {
        color: '#d8dee9',
        fontSize: '12px',
        fontWeight: '600',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
        whiteSpace: 'nowrap',
        userSelect: 'none'
    });
    const spacer = document.createElement('div');
    spacer.style.flex = '1';
    const disconnectButton = createTerminalHeaderButton({
        id: DISCONNECT_TERMINAL_BUTTON_ID,
        glyph: '\u23FB',
        title: 'End session — stops the shell and closes the panel',
        color: '#f7768e',
        fontSize: '12px',
        onClick: () => void exitTerminalSession()
    });
    const redrawButton = createTerminalHeaderButton({
        id: REDRAW_TERMINAL_BUTTON_ID,
        glyph: '\u21BB',
        title: 'Redraw terminal graphics',
        color: '#565f89',
        onClick: () => void redrawTerminal()
    });
    panel.minimizeButtonEl = createTerminalHeaderButton({
        id: MINIMIZE_TERMINAL_BUTTON_ID,
        glyph: '\u2581',
        title: 'Minimize terminal',
        color: '#565f89',
        onClick: () => void minimizePanel()
    });
    const closeButton = createTerminalHeaderButton({
        id: CLOSE_TERMINAL_BUTTON_ID,
        glyph: '\u2715',
        title: 'Close panel — the shell keeps running, reopen to come back',
        color: '#c0caf5',
        onClick: () => void closePanelKeepingSession()
    });
    header.appendChild(panel.statusDotEl);
    header.appendChild(titleSpan);
    header.appendChild(disconnectButton);
    header.appendChild(spacer);
    header.appendChild(redrawButton);
    header.appendChild(panel.minimizeButtonEl);
    // Rightmost, where every other close control on the platform lives.
    header.appendChild(closeButton);
    return header;
}
function createPanelShell(token) {
    const root = document.createElement('div');
    root.id = WIDGET_ID;
    Object.assign(root.style, {
        position: 'fixed',
        inset: '0',
        zIndex: '2147483644',
        display: 'flex',
        flexDirection: 'column',
        background: '#0f1117',
        color: '#e5e7eb',
        opacity: '1',
        pointerEvents: 'auto',
        transition: 'opacity 180ms ease'
    });
    const terminalShell = document.createElement('div');
    terminalShell.style.cssText = [
        'flex:1 1 auto',
        'height:100%',
        'min-height:0',
        'display:flex',
        'flex-direction:column',
        'background:#11131a'
    ].join(';');
    const header = createTerminalHeader();
    const terminalBody = document.createElement('div');
    terminalBody.id = TERMINAL_BODY_ID;
    terminalBody.style.cssText = [
        'flex:1',
        'min-height:0',
        'display:block',
        'background:#1a1b26'
    ].join(';');
    if (token) {
        const iframe = document.createElement('iframe');
        iframe.id = IFRAME_ID;
        iframe.src = `${getTerminalServerUrl(state.serverUrl)}/terminal?token=${encodeURIComponent(token)}`;
        iframe.setAttribute('allow', 'clipboard-write');
        iframe.style.cssText = 'width:100%;height:100%;border:none;background:#1a1b26;display:block;';
        terminalBody.appendChild(iframe);
        state.iframeEl = iframe;
    }
    else {
        state.iframeEl = null;
    }
    terminalShell.appendChild(header);
    // Above the terminal, always visible: the working directory is the single
    // most consequential thing about a shell, and it used to be invisible unless
    // the session had failed to start.
    terminalShell.appendChild(createRootFolderBarElement());
    terminalShell.appendChild(terminalBody);
    root.appendChild(terminalShell);
    panel.terminalShellEl = terminalShell;
    panel.terminalBodyEl = terminalBody;
    state.widgetEl = root;
    return root;
}
function mountPanel(root) {
    if (panel.rootEl)
        return;
    panel.rootEl = root;
    const target = document.body || document.documentElement;
    if (!target)
        return;
    target.appendChild(panel.rootEl);
    setPanelVisible(true);
    state.visible = true;
    window.addEventListener('message', handleIframeMessage);
}
function unmountPanel() {
    if (panel.rootEl) {
        panel.rootEl.remove();
        panel.rootEl = null;
    }
    panel.terminalShellEl = null;
    panel.terminalBodyEl = null;
    panel.statusDotEl = null;
    panel.minimizeButtonEl = null;
    panel.rootFolderBar = null;
    state.widgetEl = null;
    state.iframeEl = null;
    panel.panelReady = false;
    setPanelVisible(false);
    window.removeEventListener('message', handleIframeMessage);
}
async function redrawTerminal() {
    if (!state.widgetEl || !state.iframeEl)
        return;
    const currentToken = state.sessionState?.token;
    if (!currentToken)
        return;
    // After a daemon restart the token is dead; a plain reload would reconnect
    // forever to a session that no longer exists and the panel would wedge on
    // "disconnected". Confirm the token still maps to a live shell first; if not,
    // rebuild so the panel recovers into a fresh session (or the recoverable
    // no-session state) rather than a permanent disconnect.
    if (!(await validateSession(currentToken))) {
        await bootTerminalPanel(true);
        return;
    }
    const iframe = state.iframeEl;
    // Reloading the iframe tears down the old WebSocket and reconnects from
    // scratch; until the fresh document posts 'connected', the socket is not OPEN.
    // Mark disconnected so a write in that reconnect gap queues instead of being
    // sent-and-dropped (the redraw sub-case of the write-connection race).
    state.terminalConnected = false;
    iframe.src = `${getTerminalServerUrl(state.serverUrl)}/terminal?token=${encodeURIComponent(currentToken)}`;
    showTerminalBody();
    persistUIState('open');
}
/**
 * The single teardown path for dismissing the panel, whatever the reason.
 *
 * Three dismissals (exit / close / minimize) previously copy-pasted the same
 * teardown tail, and the invariant — persist-or-clear intent, then
 * resetWriteGuardState() + unmountPanel() + closeBrowserSidePanel() — lived in
 * each caller, so one drifting would leak the write guard. Now the invariant
 * lives here and no caller can forget a step (repo rule 19).
 *
 * 'clear' fully ends the session (the shell is already being stopped by the
 * caller); 'closed'/'minimized' keep the session and persist the UI state so
 * reopening reconnects.
 */
async function closePanelWithIntent(intent) {
    panel.panelCloseIntent = intent;
    if (intent === 'clear') {
        clearPersistedSession();
        resetAllState();
    }
    else {
        persistUIState(intent);
    }
    resetWriteGuardState();
    // Hide immediately, then ask the browser to close the panel BEFORE tearing down
    // the DOM. On mainstream Chrome window.close() destroys this document (its
    // presence port drops so the background can reopen); doing the authoritative
    // browser close first means we never leave an unmounted, blank document holding
    // a stale-open presence port on the latent path where the close is refused.
    setPanelVisible(false);
    await closeBrowserSidePanel();
    unmountPanel();
}
async function exitTerminalSession() {
    // Set the intent before the network call in case a disconnect races the stop.
    panel.panelCloseIntent = 'clear';
    if (state.sessionState) {
        try {
            const termUrl = getTerminalServerUrl(state.serverUrl);
            await fetch(`${termUrl}/terminal/stop`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ id: state.sessionState.sessionId }),
                signal: AbortSignal.timeout(3000)
            });
        }
        catch {
            // daemon unreachable or timeout — tear down locally
        }
    }
    await closePanelWithIntent('clear');
}
/**
 * Close the drawer and leave the shell running.
 *
 * This is what "close" has to mean for a terminal: exitTerminalSession() kills
 * the PTY, so a user who just wanted the panel out of the way lost their shell
 * and had no way back. Reopening the panel reconnects to this session.
 */
async function closePanelKeepingSession() {
    await closePanelWithIntent('closed');
}
async function minimizePanel() {
    await closePanelWithIntent('minimized');
}
function writeToTerminal(text) {
    if (!state.visible || !state.iframeEl)
        return;
    // Queue (do NOT send now) when the user is mid-keystroke OR the socket is not
    // yet OPEN. Sending while disconnected is silently dropped by the iframe host
    // (ws.readyState !== OPEN), and the deferred submit would then fire a bare Enter
    // with the text gone — the AI's write / annotation vanishes with no error. The
    // flush poller replays queued writes once `terminalConnected` (the same gate the
    // drainers use). Fail loud, not silent (rule 25).
    const typing = shouldDeferQueuedWrite();
    if (typing || !state.terminalConnected) {
        if (state.queuedWrites.length >= 200) {
            state.queuedWrites.shift();
        }
        state.queuedWrites.push(text);
        if (typing)
            maybeShowQueuedWriteToast();
        scheduleQueuedWriteFlush(TERMINAL_GUARD_POLL_MS);
        return;
    }
    if (state.queuedWriteInFlight) {
        if (state.queuedWrites.length >= 200) {
            state.queuedWrites.shift();
        }
        state.queuedWrites.push(text);
        return;
    }
    state.queuedWriteInFlight = true;
    notifyIframe('redraw');
    notifyIframe('write', { text });
    scheduleQueuedSubmit(TERMINAL_WRITE_SUBMIT_DELAY_MS);
}
function installRuntimeListener() {
    if (panel.runtimeListenerInstalled)
        return;
    panel.runtimeListenerInstalled = true;
    chrome.runtime.onMessage.addListener((message, sender) => {
        if (sender.id !== chrome.runtime.id)
            return false;
        // The background cannot close a side panel document on every Chrome version,
        // but this document can, so it asks us to.
        if (message.type === 'close_terminal_panel') {
            void closePanelKeepingSession();
            return false;
        }
        if (message.type !== 'terminal_panel_write')
            return false;
        if (typeof message.text === 'string')
            writeToTerminal(message.text);
        return false;
    });
}
/**
 * Announce that a panel document exists, for as long as it exists.
 *
 * This is how the background answers "is there a panel?" before deciding whether
 * a toggle should open or close. It has to be a port: closing the panel with
 * Chrome's own X destroys this document without running any of our teardown, so
 * a stored flag would stay "open" forever and the toggle would never open again.
 * Chrome disconnects the port either way.
 *
 * The reconnect covers service-worker restarts, which drop every port while this
 * document happily stays open.
 */
function connectPresencePort() {
    if (panel.presencePort || typeof chrome === 'undefined' || !chrome.runtime?.connect)
        return;
    try {
        const port = chrome.runtime.connect({ name: TERMINAL_PANEL_PORT });
        panel.presencePort = port;
        port.onMessage.addListener((message) => {
            if (message?.type === 'close_terminal_panel')
                void closePanelKeepingSession();
            if (message?.type === 'restore_terminal_panel')
                void restoreTerminalPanel();
        });
        port.onDisconnect.addListener(() => {
            panel.presencePort = null;
            // Only worth retrying while this document is still around and the runtime
            // is still valid; both stop being true on teardown, and connect() throws.
            if (!panel.rootEl)
                return;
            setTimeout(connectPresencePort, PRESENCE_RECONNECT_DELAY_MS);
        });
    }
    catch {
        panel.presencePort = null; // Extension context invalidated — nothing to announce to.
    }
}
function installStorageListener() {
    if (panel.storageListenerInstalled)
        return;
    panel.storageListenerInstalled = true;
    onStorageChanged((changes, areaName) => {
        if (areaName !== 'session')
            return;
        const change = changes[StorageKey.TERMINAL_UI_STATE];
        if (!change)
            return;
        const uiState = change.newValue;
        if (uiState === 'closed') {
            state.visible = false;
            if (panel.rootEl)
                panel.rootEl.style.opacity = '0';
            return;
        }
        state.visible = true;
        if (panel.rootEl)
            panel.rootEl.style.opacity = '1';
    });
}
function installUnloadListener() {
    if (panel.unloadListenerInstalled)
        return;
    panel.unloadListenerInstalled = true;
    window.addEventListener('pagehide', () => {
        if (panel.panelCloseIntent !== null)
            return;
        persistUIState('closed');
    });
}
/**
 * Bring the terminal back after an open request that landed on a panel document
 * that was already here.
 *
 * Chrome answers `sidePanel.open()` on an existing panel by focusing it, which
 * runs no code in this document. A panel left minimized — or unmounted because
 * `window.close()` was refused — would just sit there blank. Opening must always
 * mean "there is a working terminal in front of me".
 */
async function restoreTerminalPanel() {
    panel.panelCloseIntent = null;
    if (panel.rootEl && state.iframeEl) {
        // A live terminal is already mounted — it was only minimized or hidden.
        setPanelVisible(true);
        showTerminalBody();
        persistUIState('open');
        // Nothing here was rebuilt, so a root changed elsewhere (the options page,
        // another panel) would otherwise still read as the old one.
        void getTerminalDevRoot().then((root) => panel.rootFolderBar?.setRoot(root));
        return;
    }
    // No terminal in here: either unmounted, or mounted with no session (the
    // daemon was down when it booted). Rebuild, revalidating the token so the
    // xterm reconnects to a shell that is actually alive rather than rendering a
    // dead one.
    if (panel.rootEl)
        unmountPanel();
    panel.panelReady = false;
    await bootTerminalPanel();
}
async function ensureTerminalSession() {
    const persisted = await loadPersistedSession();
    if (persisted.session) {
        const alive = await validateSession(persisted.session.token);
        if (alive) {
            state.sessionState = persisted.session;
            return;
        }
        clearPersistedSession();
    }
    const config = await getTerminalConfig();
    const ss = await startSession(config, showSandboxError);
    if (!ss)
        return;
    state.sessionState = ss;
}
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
function bootTerminalPanel(forceFresh = false) {
    if (panel.panelReady && !forceFresh)
        return Promise.resolve();
    const myGen = ++panel.bootGeneration;
    return bootTerminalPanelInner(forceFresh, myGen);
}
async function bootTerminalPanelInner(forceFresh, gen) {
    // Drop the existing panel DOM before (re)building. mountPanel() early-returns
    // while `panel.rootEl` is set, so without this the freshly-built shell — bound to the
    // NEW session in the just-selected folder — is never attached, and the user
    // keeps staring at the old panel wired to the session we just stopped (the
    // "terminal won't start after picking a folder" bug, and the retry button).
    // Done before `panel.panelReady = true` because unmountPanel() clears that flag;
    // restoreTerminalPanel already unmounts, applyRootFolder did not.
    if (forceFresh)
        unmountPanel();
    panel.panelReady = true;
    panel.panelCloseIntent = null;
    panel.pendingSandboxError = null;
    try {
        state.serverUrl = await getServerUrl();
        if (gen !== panel.bootGeneration)
            return; // superseded by a newer boot
        installRuntimeListener();
        installStorageListener();
        installUnloadListener();
        connectPresencePort();
        if (forceFresh) {
            resetAllState();
            state.serverUrl = await getServerUrl();
            if (gen !== panel.bootGeneration)
                return;
        }
        await ensureTerminalSession();
        // A newer boot superseded this one while we awaited the (possibly slow, possibly
        // hung) session call — abandon BEFORE mutating the DOM so we can't orphan its iframe.
        if (gen !== panel.bootGeneration)
            return;
        const token = state.sessionState?.token;
        const root = createPanelShell(token ?? '');
        mountPanel(root);
        showTerminalBody();
        persistUIState('open');
        if (!token) {
            const error = panel.pendingSandboxError;
            if (error) {
                showSandboxError(error.message, error.instruction, error.command);
            }
            else {
                showNoSessionState();
            }
        }
    }
    catch (err) {
        // A boot that threw (network error, extension context invalidated) must not
        // wedge the button: clear panelReady so the next "Start terminal" boots fresh,
        // and — if this is still the current boot and a body exists — show the
        // retriable no-session state instead of a blank/half-built panel.
        if (gen === panel.bootGeneration) {
            panel.panelReady = false;
            console.log('[KaBOOM! terminal] boot failed:', String(err));
            if (panel.terminalBodyEl)
                showNoSessionState();
        }
    }
}
/**
 * Entry point: boot the panel once the document is ready. Auto-invoked at module
 * scope in the real side-panel document; the `process === undefined` guard keeps
 * it from firing under Node test imports. Named + exported so it is an explicit,
 * callable entry rather than an anonymous top-level side effect.
 */
export function main() {
    if (typeof document === 'undefined' || typeof globalThis.process !== 'undefined') {
        return;
    }
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => {
            void bootTerminalPanel();
        });
    }
    else {
        void bootTerminalPanel();
    }
}
main();
export const _terminalPanelForTests = {
    bootTerminalPanel,
    applyRootFolder,
    writeToTerminal,
    exitTerminalSession,
    redrawTerminal,
    resetPanelUi
};
//# sourceMappingURL=sidepanel.js.map