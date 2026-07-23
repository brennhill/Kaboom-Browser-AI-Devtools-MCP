/**
 * Purpose: Terminal widget DOM creation, resize, minimize, redraw, and iframe messaging.
 * Why: Isolates all DOM manipulation and UI event wiring from session logic and orchestrator.
 * Docs: docs/features/feature/terminal/index.md
 */
import { state, getTerminalServerUrl, WIDGET_ID, IFRAME_ID, HEADER_ID, DISCONNECT_TERMINAL_BUTTON_ID, REDRAW_TERMINAL_BUTTON_ID, MINIMIZE_TERMINAL_BUTTON_ID, CLOSE_TERMINAL_BUTTON_ID, DEFAULT_WIDGET_WIDTH, DEFAULT_WIDGET_HEIGHT, MIN_WIDGET_WIDTH, MIN_WIDGET_HEIGHT, MAX_WIDGET_WIDTH, MAX_WIDGET_HEIGHT, MINIMIZED_WIDGET_HEIGHT } from './terminal-widget-types.js';
import { persistUIState } from './terminal-widget-session.js';
// ---------------------------------------------------------------------------
// Forward-declared callbacks — set by orchestrator to avoid circular imports.
// ---------------------------------------------------------------------------
let _hideTerminalCb = null;
let _exitTerminalSessionCb = null;
let _resetWriteGuardStateCb = null;
let _scheduleQueuedWriteFlushCb = null;
export function registerUICallbacks(cbs) {
    _hideTerminalCb = cbs.hideTerminal;
    _exitTerminalSessionCb = cbs.exitTerminalSession;
    _resetWriteGuardStateCb = cbs.resetWriteGuardState;
    _scheduleQueuedWriteFlushCb = cbs.scheduleQueuedWriteFlush;
}
// ---------------------------------------------------------------------------
// Sandbox error overlay
// ---------------------------------------------------------------------------
export function showSandboxError(message, instruction, command) {
    // Remove any existing widget/error overlay
    const existing = document.getElementById(WIDGET_ID);
    if (existing)
        existing.remove();
    const overlay = document.createElement('div');
    overlay.id = WIDGET_ID;
    Object.assign(overlay.style, {
        position: 'fixed',
        bottom: '16px',
        right: '16px',
        width: '420px',
        maxWidth: 'calc(100vw - 32px)',
        zIndex: '2147483644',
        background: '#1a1b26',
        border: '1px solid #f7768e',
        borderRadius: '12px',
        padding: '20px',
        boxShadow: '0 8px 32px rgba(0, 0, 0, 0.4)',
        fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
        color: '#a9b1d6'
    });
    const title = document.createElement('div');
    title.textContent = 'Terminal Unavailable';
    Object.assign(title.style, {
        fontSize: '14px',
        fontWeight: '600',
        color: '#f7768e',
        marginBottom: '8px'
    });
    const msg = document.createElement('div');
    msg.textContent = message;
    Object.assign(msg.style, {
        fontSize: '12px',
        color: '#787c99',
        marginBottom: '12px',
        lineHeight: '1.4'
    });
    const inst = document.createElement('div');
    inst.textContent = instruction;
    Object.assign(inst.style, {
        fontSize: '12px',
        color: '#a9b1d6',
        marginBottom: '8px'
    });
    const cmdBox = document.createElement('div');
    Object.assign(cmdBox.style, {
        background: '#16161e',
        border: '1px solid #292e42',
        borderRadius: '6px',
        padding: '10px 12px',
        fontFamily: '"SF Mono", "Fira Code", Menlo, Monaco, monospace',
        fontSize: '12px',
        color: '#9ece6a',
        cursor: 'pointer',
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
        marginBottom: '12px'
    });
    const cmdText = document.createElement('span');
    cmdText.textContent = command;
    cmdText.style.flex = '1';
    const copyIcon = document.createElement('span');
    copyIcon.textContent = 'Copy';
    Object.assign(copyIcon.style, {
        fontSize: '11px',
        color: '#565f89',
        flexShrink: '0'
    });
    cmdBox.appendChild(cmdText);
    cmdBox.appendChild(copyIcon);
    cmdBox.addEventListener('click', () => {
        void navigator.clipboard.writeText(command).then(() => {
            copyIcon.textContent = 'Copied!';
            copyIcon.style.color = '#9ece6a';
            setTimeout(() => {
                copyIcon.textContent = 'Copy';
                copyIcon.style.color = '#565f89';
            }, 2000);
        }).catch(() => {
            copyIcon.textContent = 'Select & copy manually';
            copyIcon.style.color = '#f7768e';
        });
    });
    const closeBtn = document.createElement('button');
    closeBtn.textContent = 'Dismiss';
    closeBtn.type = 'button';
    Object.assign(closeBtn.style, {
        background: '#292e42',
        border: 'none',
        borderRadius: '6px',
        padding: '6px 16px',
        color: '#a9b1d6',
        fontSize: '12px',
        cursor: 'pointer',
        width: '100%'
    });
    closeBtn.addEventListener('click', () => {
        overlay.remove();
        state.widgetEl = null;
        state.visible = false;
    });
    overlay.appendChild(title);
    overlay.appendChild(msg);
    overlay.appendChild(inst);
    overlay.appendChild(cmdBox);
    overlay.appendChild(closeBtn);
    state.widgetEl = overlay;
    state.visible = true;
    const target = document.body || document.documentElement;
    if (target)
        target.appendChild(overlay);
}
// ---------------------------------------------------------------------------
// Widget creation
// ---------------------------------------------------------------------------
export function createWidget(token) {
    state.terminalConnected = false;
    const widget = document.createElement('div');
    widget.id = WIDGET_ID;
    Object.assign(widget.style, {
        position: 'fixed',
        bottom: '0',
        right: '0',
        width: DEFAULT_WIDGET_WIDTH,
        height: DEFAULT_WIDGET_HEIGHT,
        minWidth: MIN_WIDGET_WIDTH,
        minHeight: MIN_WIDGET_HEIGHT,
        maxWidth: MAX_WIDGET_WIDTH,
        maxHeight: MAX_WIDGET_HEIGHT,
        zIndex: '2147483644',
        display: 'flex',
        flexDirection: 'column',
        borderRadius: '12px 0 0 0',
        overflow: 'hidden',
        boxShadow: '0 -4px 24px rgba(0, 0, 0, 0.3)',
        fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
        transition: 'opacity 200ms ease, transform 200ms ease',
        transformOrigin: 'bottom right'
    });
    // Resize handle (top-left corner)
    const resizeHandle = document.createElement('div');
    Object.assign(resizeHandle.style, {
        position: 'absolute',
        top: '0',
        left: '0',
        width: '12px',
        height: '12px',
        cursor: 'nw-resize',
        zIndex: '10'
    });
    setupResize(resizeHandle, widget);
    state.resizeHandleEl = resizeHandle;
    widget.appendChild(resizeHandle);
    // Header bar
    const header = document.createElement('div');
    header.id = HEADER_ID;
    Object.assign(header.style, {
        height: '32px',
        background: '#16161e',
        display: 'flex',
        alignItems: 'center',
        padding: '0 8px 0 12px',
        gap: '8px',
        borderBottom: '1px solid #292e42',
        cursor: 'default',
        flexShrink: '0'
    });
    // Connection status dot
    const statusDot = document.createElement('span');
    statusDot.className = 'kaboom-terminal-status-dot';
    Object.assign(statusDot.style, {
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
        color: '#787c99',
        fontSize: '12px',
        fontWeight: '600',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
        whiteSpace: 'nowrap',
        userSelect: 'none'
    });
    // Minimize button
    const minimizeTerminalButton = document.createElement('button');
    minimizeTerminalButton.id = MINIMIZE_TERMINAL_BUTTON_ID;
    minimizeTerminalButton.textContent = '\u2581'; // block
    minimizeTerminalButton.title = 'Minimize terminal';
    minimizeTerminalButton.type = 'button';
    Object.assign(minimizeTerminalButton.style, {
        width: '24px',
        height: '24px',
        border: 'none',
        background: 'transparent',
        color: '#565f89',
        fontSize: '14px',
        cursor: 'pointer',
        borderRadius: '4px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: '0'
    });
    minimizeTerminalButton.addEventListener('mouseenter', () => {
        minimizeTerminalButton.style.background = '#292e42';
        minimizeTerminalButton.style.color = '#a9b1d6';
    });
    minimizeTerminalButton.addEventListener('mouseleave', () => {
        minimizeTerminalButton.style.background = 'transparent';
        minimizeTerminalButton.style.color = '#565f89';
    });
    minimizeTerminalButton.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        toggleMinimize(widget, minimizeTerminalButton, header);
    });
    // Exit session button — kills the PTY. Placed left, next to title, glows red.
    const disconnectTerminalButton = document.createElement('button');
    disconnectTerminalButton.id = DISCONNECT_TERMINAL_BUTTON_ID;
    disconnectTerminalButton.textContent = '\u23FB'; // power symbol
    disconnectTerminalButton.title = 'disconnect terminal & and end session';
    disconnectTerminalButton.type = 'button';
    Object.assign(disconnectTerminalButton.style, {
        width: '24px',
        height: '24px',
        border: 'none',
        background: 'transparent',
        color: '#f7768e',
        fontSize: '12px',
        cursor: 'pointer',
        borderRadius: '4px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: '0',
        opacity: '0.7',
        transition: 'opacity 150ms ease, background 150ms ease, box-shadow 150ms ease'
    });
    disconnectTerminalButton.addEventListener('mouseenter', () => {
        disconnectTerminalButton.style.background = '#3b1219';
        disconnectTerminalButton.style.opacity = '1';
        disconnectTerminalButton.style.boxShadow = '0 0 8px rgba(247, 118, 142, 0.4)';
    });
    disconnectTerminalButton.addEventListener('mouseleave', () => {
        disconnectTerminalButton.style.background = 'transparent';
        disconnectTerminalButton.style.opacity = '0.7';
        disconnectTerminalButton.style.boxShadow = 'none';
    });
    disconnectTerminalButton.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        if (_exitTerminalSessionCb)
            void _exitTerminalSessionCb();
    });
    // Spacer pushes minimize/close to the right
    const spacer = document.createElement('div');
    spacer.style.flex = '1';
    const redrawTerminalButton = document.createElement('button');
    redrawTerminalButton.id = REDRAW_TERMINAL_BUTTON_ID;
    redrawTerminalButton.textContent = '\u21BB'; // reload arrow
    redrawTerminalButton.title = 'Redraw terminal graphics';
    redrawTerminalButton.type = 'button';
    Object.assign(redrawTerminalButton.style, {
        width: '24px',
        height: '24px',
        border: 'none',
        background: 'transparent',
        color: '#565f89',
        fontSize: '14px',
        cursor: 'pointer',
        borderRadius: '4px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: '0'
    });
    redrawTerminalButton.addEventListener('mouseenter', () => {
        redrawTerminalButton.style.background = '#292e42';
        redrawTerminalButton.style.color = '#a9b1d6';
    });
    redrawTerminalButton.addEventListener('mouseleave', () => {
        redrawTerminalButton.style.background = 'transparent';
        redrawTerminalButton.style.color = '#565f89';
    });
    redrawTerminalButton.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        redrawTerminal(widget, header, minimizeTerminalButton);
    });
    const closeTerminalButton = document.createElement('button');
    closeTerminalButton.id = CLOSE_TERMINAL_BUTTON_ID;
    closeTerminalButton.textContent = '\u2715';
    closeTerminalButton.title = 'Close terminal';
    closeTerminalButton.type = 'button';
    Object.assign(closeTerminalButton.style, {
        width: '24px',
        height: '24px',
        border: 'none',
        background: 'transparent',
        color: '#565f89',
        fontSize: '14px',
        cursor: 'pointer',
        borderRadius: '4px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: '0'
    });
    closeTerminalButton.addEventListener('mouseenter', () => {
        closeTerminalButton.style.background = '#292e42';
        closeTerminalButton.style.color = '#a9b1d6';
    });
    closeTerminalButton.addEventListener('mouseleave', () => {
        closeTerminalButton.style.background = 'transparent';
        closeTerminalButton.style.color = '#565f89';
    });
    closeTerminalButton.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        if (_hideTerminalCb)
            _hideTerminalCb();
    });
    // Title bar click restores when minimized
    header.addEventListener('click', () => {
        if (!state.minimized)
            return;
        toggleMinimize(widget, minimizeTerminalButton, header);
    });
    header.appendChild(statusDot);
    header.appendChild(titleSpan);
    header.appendChild(disconnectTerminalButton);
    header.appendChild(spacer);
    header.appendChild(redrawTerminalButton);
    header.appendChild(minimizeTerminalButton);
    header.appendChild(closeTerminalButton);
    // Iframe
    const iframe = document.createElement('iframe');
    iframe.id = IFRAME_ID;
    iframe.src = `${getTerminalServerUrl(state.serverUrl)}/terminal?token=${encodeURIComponent(token)}`;
    Object.assign(iframe.style, {
        flex: '1',
        width: '100%',
        border: 'none',
        background: '#1a1b26'
    });
    iframe.setAttribute('allow', 'clipboard-write');
    widget.appendChild(header);
    widget.appendChild(iframe);
    state.iframeEl = iframe;
    // Listen for messages from the terminal iframe
    window.addEventListener('message', handleIframeMessage);
    return widget;
}
// ---------------------------------------------------------------------------
// Status dot
// ---------------------------------------------------------------------------
function updateStatusDot(dotState) {
    const dot = state.widgetEl?.querySelector('.kaboom-terminal-status-dot');
    if (!dot)
        return;
    switch (dotState) {
        case 'connected':
            dot.style.background = '#9ece6a'; // green
            break;
        case 'disconnected':
            dot.style.background = '#e0af68'; // orange
            break;
        case 'exited':
            dot.style.background = '#f7768e'; // red
            break;
    }
}
// ---------------------------------------------------------------------------
// Iframe message handler
// ---------------------------------------------------------------------------
export function handleIframeMessage(event) {
    if (!event.data || event.data.source !== 'kaboom-terminal')
        return;
    // Only accept messages from the terminal server's origin (localhost:port+1)
    try {
        const termOrigin = getTerminalServerUrl(state.serverUrl);
        if (event.origin !== termOrigin)
            return;
    }
    catch {
        return; // Malformed serverUrl — reject all messages
    }
    // Handle terminal connection lifecycle events
    switch (event.data.event) {
        case 'connected':
            updateStatusDot('connected');
            state.terminalConnected = true;
            if (state.queuedWrites.length > 0 && !state.queuedWriteInFlight) {
                if (_scheduleQueuedWriteFlushCb)
                    _scheduleQueuedWriteFlushCb(0);
            }
            break;
        case 'disconnected':
            updateStatusDot('disconnected');
            state.terminalConnected = false;
            state.terminalFocused = false;
            break;
        case 'exited':
            updateStatusDot('exited');
            state.terminalConnected = false;
            state.terminalFocused = false;
            if (_resetWriteGuardStateCb)
                _resetWriteGuardStateCb();
            break;
        case 'focus':
            state.terminalFocused = Boolean(event.data.data?.focused);
            if (state.terminalFocused) {
                state.lastTypingAt = Date.now();
            }
            else if (state.queuedWrites.length > 0 && !state.queuedWriteInFlight) {
                if (_scheduleQueuedWriteFlushCb)
                    _scheduleQueuedWriteFlushCb(0);
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
// ---------------------------------------------------------------------------
// Resize
// ---------------------------------------------------------------------------
function setupResize(handle, widget) {
    let startX = 0;
    let startY = 0;
    let startWidth = 0;
    let startHeight = 0;
    function onMouseDown(e) {
        e.preventDefault();
        startX = e.clientX;
        startY = e.clientY;
        startWidth = widget.offsetWidth;
        startHeight = widget.offsetHeight;
        document.addEventListener('mousemove', onMouseMove);
        document.addEventListener('mouseup', onMouseUp);
        // Prevent iframe from stealing mouse events during resize
        if (state.iframeEl)
            state.iframeEl.style.pointerEvents = 'none';
    }
    function onMouseMove(e) {
        const newWidth = startWidth - (e.clientX - startX);
        const newHeight = startHeight - (e.clientY - startY);
        widget.style.width = Math.max(400, Math.min(window.innerWidth, newWidth)) + 'px';
        widget.style.height = Math.max(250, Math.min(window.innerHeight * 0.8, newHeight)) + 'px';
    }
    function onMouseUp() {
        document.removeEventListener('mousemove', onMouseMove);
        document.removeEventListener('mouseup', onMouseUp);
        if (state.iframeEl)
            state.iframeEl.style.pointerEvents = 'auto';
        // Notify iframe to refit terminal
        notifyIframe('resize');
    }
    handle.addEventListener('mousedown', onMouseDown);
}
// ---------------------------------------------------------------------------
// Redraw / Minimize / Notify
// ---------------------------------------------------------------------------
function redrawTerminal(widget, header, minimizeButton) {
    if (state.minimized) {
        toggleMinimize(widget, minimizeButton, header);
    }
    state.savedHeight = DEFAULT_WIDGET_HEIGHT;
    widget.style.bottom = '0';
    widget.style.right = '0';
    widget.style.width = DEFAULT_WIDGET_WIDTH;
    widget.style.height = DEFAULT_WIDGET_HEIGHT;
    widget.style.minWidth = MIN_WIDGET_WIDTH;
    widget.style.minHeight = MIN_WIDGET_HEIGHT;
    widget.style.maxWidth = MAX_WIDGET_WIDTH;
    widget.style.maxHeight = MAX_WIDGET_HEIGHT;
    widget.style.opacity = '1';
    widget.style.transform = 'translateY(0) scale(1)';
    widget.style.pointerEvents = 'auto';
    if (state.iframeEl) {
        state.iframeEl.style.display = 'block';
        updateStatusDot('disconnected');
        state.iframeEl.src = state.iframeEl.src;
    }
    if (state.resizeHandleEl)
        state.resizeHandleEl.style.display = 'block';
    minimizeButton.textContent = '\u2581'; // block
    minimizeButton.title = 'Minimize terminal';
    header.style.cursor = 'default';
    header.style.borderBottom = '1px solid #292e42';
    state.visible = true;
    requestAnimationFrame(() => {
        notifyIframe('resize');
        notifyIframe('focus');
    });
    persistUIState('open');
}
export function toggleMinimize(widget, btn, header) {
    if (state.minimized) {
        // Restore
        state.minimized = false;
        widget.style.height = state.savedHeight || DEFAULT_WIDGET_HEIGHT;
        widget.style.minHeight = MIN_WIDGET_HEIGHT;
        if (state.iframeEl)
            state.iframeEl.style.display = 'block';
        if (state.resizeHandleEl)
            state.resizeHandleEl.style.display = 'block';
        btn.textContent = '\u2581'; // block
        btn.title = 'Minimize terminal';
        header.style.cursor = 'default';
        header.style.borderBottom = '1px solid #292e42';
        notifyIframe('resize');
        persistUIState('open');
    }
    else {
        // Minimize
        state.minimized = true;
        state.savedHeight = widget.style.height || DEFAULT_WIDGET_HEIGHT;
        widget.style.height = MINIMIZED_WIDGET_HEIGHT;
        widget.style.minHeight = MINIMIZED_WIDGET_HEIGHT;
        if (state.iframeEl)
            state.iframeEl.style.display = 'none';
        if (state.resizeHandleEl)
            state.resizeHandleEl.style.display = 'none';
        btn.textContent = '\u25A1'; // square
        btn.title = 'Restore terminal';
        header.style.cursor = 'pointer';
        header.style.borderBottom = 'none';
        persistUIState('minimized');
    }
}
export function notifyIframe(command, data) {
    if (!state.iframeEl?.contentWindow)
        return;
    let origin = '*';
    try {
        origin = getTerminalServerUrl(state.serverUrl);
    }
    catch { /* fall back to wildcard */ }
    state.iframeEl.contentWindow.postMessage({
        target: 'kaboom-terminal',
        command,
        ...data
    }, origin);
}
//# sourceMappingURL=terminal-widget-ui.js.map