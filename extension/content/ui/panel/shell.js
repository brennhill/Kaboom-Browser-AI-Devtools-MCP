/**
 * Purpose: Builds the side-panel terminal chrome — header, action buttons, and the
 * panel shell that hosts the terminal iframe.
 * Why: This is pure DOM construction. Keeping it beside the panel's lifecycle and
 * message-handling logic made sidepanel.ts exceed the 800-line limit; the folder
 * limit then pushes it into a sub-module rather than another sibling file.
 *
 * The module owns no state: every element it creates is handed back through
 * ShellDeps setters, and every action is an injected callback. The iframe setter is
 * called from load/error handlers, which is why this is a sink rather than a plain
 * return value.
 * Docs: docs/features/feature/terminal/index.md
 */
import { IFRAME_ID, HEADER_ID, TERMINAL_BODY_ID, DISCONNECT_TERMINAL_BUTTON_ID, ANNOTATE_TERMINAL_BUTTON_ID, CLOSE_TERMINAL_BUTTON_ID, REDRAW_TERMINAL_BUTTON_ID, MINIMIZE_TERMINAL_BUTTON_ID, WIDGET_ID, TERMINAL_PROVIDER_BADGE_ID } from '../terminal-widget-types.js';
import { getTerminalServerUrl } from '../../../lib/terminal-server.js';
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
function createTerminalHeader(deps) {
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
        color: '#d8dee9',
        fontSize: '12px',
        fontWeight: '600',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
        whiteSpace: 'nowrap',
        userSelect: 'none'
    });
    const providerBadge = document.createElement('span');
    providerBadge.id = TERMINAL_PROVIDER_BADGE_ID;
    providerBadge.textContent = 'Provider · Detecting';
    providerBadge.title = 'Checking whether this terminal uses a subscription or API billing';
    Object.assign(providerBadge.style, {
        color: '#787c99',
        fontSize: '10px',
        border: '1px solid #414868',
        borderRadius: '999px',
        padding: '2px 6px',
        whiteSpace: 'nowrap'
    });
    const spacer = document.createElement('div');
    spacer.style.flex = '1';
    const disconnectButton = createTerminalHeaderButton({
        id: DISCONNECT_TERMINAL_BUTTON_ID,
        glyph: '\u23FB',
        title: 'End session — stops the shell and closes the panel',
        color: '#f7768e',
        fontSize: '12px',
        onClick: () => deps.onExit()
    });
    const annotateButton = createTerminalHeaderButton({
        id: ANNOTATE_TERMINAL_BUTTON_ID,
        glyph: '\u270E',
        title: 'Annotate the page \u2014 draw and mark up elements for the agent',
        color: '#7aa2f7',
        onClick: () => deps.onAnnotate()
    });
    const redrawButton = createTerminalHeaderButton({
        id: REDRAW_TERMINAL_BUTTON_ID,
        glyph: '\u21BB',
        title: 'Redraw terminal graphics',
        color: '#565f89',
        onClick: () => deps.onRedraw()
    });
    const minimizeButton = createTerminalHeaderButton({
        id: MINIMIZE_TERMINAL_BUTTON_ID,
        glyph: '\u2581',
        title: 'Minimize terminal',
        color: '#565f89',
        onClick: () => deps.onMinimize()
    });
    const closeButton = createTerminalHeaderButton({
        id: CLOSE_TERMINAL_BUTTON_ID,
        glyph: '\u2715',
        title: 'Close panel — the shell keeps running, reopen to come back',
        color: '#c0caf5',
        onClick: () => deps.onClose()
    });
    header.appendChild(statusDot);
    header.appendChild(titleSpan);
    header.appendChild(providerBadge);
    header.appendChild(disconnectButton);
    header.appendChild(spacer);
    header.appendChild(annotateButton);
    header.appendChild(redrawButton);
    header.appendChild(minimizeButton);
    // Rightmost, where every other close control on the platform lives.
    header.appendChild(closeButton);
    deps.setStatusDot(statusDot);
    deps.setProviderBadge(providerBadge);
    deps.setMinimizeButton(minimizeButton);
    return header;
}
export function createPanelShell(token, deps) {
    const root = document.createElement('div');
    root.id = WIDGET_ID;
    root.setAttribute?.('data-kaboom-owned', 'true');
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
    const header = createTerminalHeader(deps);
    const terminalBody = document.createElement('div');
    terminalBody.id = TERMINAL_BODY_ID;
    terminalBody.style.cssText = ['flex:1', 'min-height:0', 'display:block', 'background:#1a1b26'].join(';');
    if (token) {
        const iframe = document.createElement('iframe');
        iframe.id = IFRAME_ID;
        // Synchronous accessor: this builder is not async, but ensureTerminalSession()
        // has already run (it is what produced `token`) and its daemon calls perform
        // the terminal-port discovery, so the cache is warm by the time we get here.
        iframe.src = `${getTerminalServerUrl(deps.serverUrl)}/terminal?token=${encodeURIComponent(token)}`;
        iframe.setAttribute('allow', 'clipboard-write');
        iframe.style.cssText = 'width:100%;height:100%;border:none;background:#1a1b26;display:block;';
        terminalBody.appendChild(iframe);
        deps.setIframe(iframe);
    }
    else {
        deps.setIframe(null);
    }
    terminalShell.appendChild(header);
    // Above the terminal, always visible: the working directory is the single
    // most consequential thing about a shell, and it used to be invisible unless
    // the session had failed to start.
    terminalShell.appendChild(deps.createRootFolderBar());
    terminalShell.appendChild(terminalBody);
    root.appendChild(terminalShell);
    deps.setTerminalShell(terminalShell);
    deps.setTerminalBody(terminalBody);
    deps.setWidget(root);
    return root;
}
//# sourceMappingURL=shell.js.map