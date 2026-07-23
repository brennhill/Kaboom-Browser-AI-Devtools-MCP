/**
 * Purpose: Renders a tracked-tab hover launcher for fast annotate/record/screenshot actions.
 * Why: Reduces popup churn by exposing common capture actions directly on tracked pages.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
import { RuntimeMessageName, StorageKey } from '../../lib/constants.js';
import { KABOOM_DOCS_URL, KABOOM_REPOSITORY_URL } from '../../lib/brand.js';
import { requestAudit } from '../../lib/request-audit.js';
import { getLocal, setLocal, removeLocal, onStorageChanged } from '../../lib/storage-utils.js';
import { initTerminalPanelBridge, isTerminalVisible, onTerminalPanelVisibilityChanged, openTerminalPanel, writeToTerminal } from './terminal-panel-bridge.js';
const ROOT_ID = 'kaboom-tracked-hover-launcher';
const PANEL_ID = 'kaboom-tracked-hover-panel';
const TOGGLE_ID = 'kaboom-tracked-hover-toggle';
const SETTINGS_MENU_ID = 'kaboom-tracked-hover-settings-menu';
let rootEl = null;
let panelEl = null;
let settingsMenuEl = null;
let stopButtonEl = null;
let toggleEl = null;
let panelPinned = false;
let settingsMenuOpen = false;
let recordingActive = false;
let trackedEnabled = false;
let hiddenUntilPopupOpen = false;
let hideTimer = null;
let recordingStorageListener = null;
let recordingStorageUnsubscribe = null;
let runtimeListenerInstalled = false;
let annotationListenerInstalled = false;
let terminalVisibilityUnsubscribe = null;
function clearHideTimer() {
    if (!hideTimer)
        return;
    clearTimeout(hideTimer);
    hideTimer = null;
}
function setPanelOpen(open) {
    if (!panelEl)
        return;
    panelEl.style.opacity = open ? '1' : '0';
    panelEl.style.transform = open ? 'translateX(0) scale(1)' : 'translateX(12px) scale(0.96)';
    panelEl.style.pointerEvents = open ? 'auto' : 'none';
}
function setSettingsMenuOpen(open) {
    settingsMenuOpen = open;
    if (!settingsMenuEl)
        return;
    settingsMenuEl.style.opacity = open ? '1' : '0';
    settingsMenuEl.style.transform = open ? 'translateY(0) scale(1)' : 'translateY(-8px) scale(0.96)';
    settingsMenuEl.style.pointerEvents = open ? 'auto' : 'none';
}
function stopRecordingAction() {
    try {
        chrome.runtime.sendMessage({ type: 'screen_recording_stop' }, (response) => {
            void chrome.runtime.lastError;
            if (response?.status === 'saved') {
                updateStopButtonVisibility(false);
            }
        });
    }
    catch {
        // Extension context invalidated
    }
}
function updateStopButtonVisibility(active) {
    recordingActive = active;
    if (!stopButtonEl)
        return;
    stopButtonEl.style.display = active ? 'flex' : 'none';
}
async function syncRecordingStateFromStorage() {
    try {
        const value = await getLocal(StorageKey.RECORDING);
        const rec = value;
        const active = rec != null && typeof rec === 'object' && Boolean(rec.active);
        updateStopButtonVisibility(active);
    }
    catch {
        // Extension context invalidated
    }
}
function installRecordingStorageSync() {
    if (recordingStorageListener)
        return;
    recordingStorageListener = (changes, areaName) => {
        if (areaName !== 'local')
            return;
        const change = changes[StorageKey.RECORDING];
        if (!change)
            return;
        const rec = change.newValue;
        const active = rec != null && typeof rec === 'object' && Boolean(rec.active);
        updateStopButtonVisibility(active);
    };
    recordingStorageUnsubscribe = onStorageChanged(recordingStorageListener);
}
function uninstallRecordingStorageSync() {
    if (!recordingStorageListener)
        return;
    if (recordingStorageUnsubscribe) {
        recordingStorageUnsubscribe();
        recordingStorageUnsubscribe = null;
    }
    recordingStorageListener = null;
}
async function syncHiddenStateFromStorage() {
    try {
        const value = await getLocal(StorageKey.TRACKED_HOVER_LAUNCHER_HIDDEN);
        hiddenUntilPopupOpen = Boolean(value);
    }
    catch {
        // Extension context invalidated — proceed with defaults
    }
}
function persistHiddenState(hidden) {
    try {
        if (hidden) {
            void setLocal(StorageKey.TRACKED_HOVER_LAUNCHER_HIDDEN, true);
            return;
        }
        void removeLocal(StorageKey.TRACKED_HOVER_LAUNCHER_HIDDEN);
    }
    catch {
        // Extension context invalidated — hidden state won't persist but functionality is unaffected
    }
}
function hideLauncherUntilPopupReopen() {
    hiddenUntilPopupOpen = true;
    persistHiddenState(true);
    setSettingsMenuOpen(false);
    unmountLauncher();
}
function handleReshowRequest() {
    hiddenUntilPopupOpen = false;
    persistHiddenState(false);
    applyVisibilityFromState();
}
function installRuntimeListener() {
    if (runtimeListenerInstalled)
        return;
    runtimeListenerInstalled = true;
    chrome.runtime.onMessage.addListener((message, sender) => {
        if (sender.id !== chrome.runtime.id)
            return false;
        if (message.type !== RuntimeMessageName.SHOW_TRACKED_HOVER_LAUNCHER)
            return false;
        handleReshowRequest();
        return false;
    });
}
function installTerminalVisibilitySync() {
    if (terminalVisibilityUnsubscribe)
        return;
    terminalVisibilityUnsubscribe = onTerminalPanelVisibilityChanged(() => {
        applyVisibilityFromState();
    });
}
function applyVisibilityFromState() {
    if (trackedEnabled && !hiddenUntilPopupOpen && !isTerminalVisible()) {
        mountLauncher();
        return;
    }
    unmountLauncher();
}
function formatAnnotationsForTerminal(annotations, pageUrl) {
    if (annotations.length === 0)
        return '';
    const lines = [
        'The user just annotated the page with the following feedback. Please review and implement these changes:',
        '',
        `Page: ${pageUrl}`,
        ''
    ];
    for (let i = 0; i < annotations.length; i++) {
        const a = annotations[i];
        const text = a.text || '(no label)';
        const sel = a.selector || 'unknown';
        const r = a.rect;
        const loc = r ? ` (${Math.round(r.x)},${Math.round(r.y)} ${Math.round(r.width)}x${Math.round(r.height)})` : '';
        lines.push(`${i + 1}. "${text}" — ${sel}${loc}`);
    }
    lines.push('');
    lines.push('The annotations are available via analyze(what="annotations").');
    lines.push('');
    return lines.join('\n');
}
function handleAnnotationsReady(event) {
    const detail = event.detail;
    if (!detail?.annotations?.length)
        return;
    if (!isTerminalVisible())
        return;
    const text = formatAnnotationsForTerminal(detail.annotations, detail.page_url || location.href);
    if (text)
        writeToTerminal(text);
}
function installAnnotationListener() {
    if (annotationListenerInstalled)
        return;
    annotationListenerInstalled = true;
    window.addEventListener('kaboom-annotations-ready', handleAnnotationsReady);
}
function uninstallAnnotationListener() {
    if (!annotationListenerInstalled)
        return;
    annotationListenerInstalled = false;
    window.removeEventListener('kaboom-annotations-ready', handleAnnotationsReady);
}
async function startDrawMode() {
    try {
        if (!chrome?.runtime?.getURL) {
            console.warn('[KaBOOM!] Draw mode unavailable: extension context invalidated. Refresh the page to restore.');
            return;
        }
        const drawModeModule = await import(/* webpackIgnore: true */ chrome.runtime.getURL('content/draw-mode.js'));
        if (typeof drawModeModule.activateDrawMode === 'function') {
            drawModeModule.activateDrawMode('user');
        }
    }
    catch (err) {
        console.warn('[KaBOOM!] Draw mode failed to load: ' + (err instanceof Error ? err.message : String(err)) +
            '. The extension may need to be reloaded at chrome://extensions.');
    }
}
// Primed AudioContext — created during user gesture so it won't be blocked.
// Reused across captures; closed lazily by the browser when the page unloads.
let shutterAudioCtx = null;
function playShutterSound() {
    try {
        if (!shutterAudioCtx || shutterAudioCtx.state === 'closed') {
            shutterAudioCtx = new AudioContext();
        }
        const ctx = shutterAudioCtx;
        // Resume in case the context was suspended (autoplay policy)
        if (ctx.state === 'suspended')
            void ctx.resume();
        const duration = 0.08;
        const buffer = ctx.createBuffer(1, Math.ceil(ctx.sampleRate * duration), ctx.sampleRate);
        const data = buffer.getChannelData(0);
        for (let i = 0; i < data.length; i++) {
            const t = i / data.length;
            const envelope = t < 0.1 ? t * 10 : Math.exp(-12 * (t - 0.1));
            data[i] = (Math.random() * 2 - 1) * envelope * 0.3;
        }
        const source = ctx.createBufferSource();
        source.buffer = buffer;
        source.connect(ctx.destination);
        source.start();
    }
    catch {
        // Audio unavailable — silent fallback
    }
}
function showScreenshotFlash(success) {
    const flash = document.createElement('div');
    Object.assign(flash.style, {
        position: 'fixed',
        inset: '0',
        zIndex: '2147483647',
        background: success ? 'rgba(250,204,21,0.3)' : 'rgba(239,68,68,0.25)',
        pointerEvents: 'none',
        opacity: '1'
    });
    document.documentElement.appendChild(flash);
    // Hold the flash visible for 120ms before fading out
    setTimeout(() => {
        flash.style.transition = 'opacity 300ms ease-out';
        flash.style.opacity = '0';
    }, 120);
    setTimeout(() => flash.remove(), 450);
}
function runScreenshotCapture() {
    // Prime the AudioContext during the user gesture (click) so Chrome allows playback.
    if (!shutterAudioCtx || shutterAudioCtx.state === 'closed') {
        try {
            shutterAudioCtx = new AudioContext();
        }
        catch { /* no audio */ }
    }
    try {
        chrome.runtime.sendMessage({ type: 'capture_screenshot' }, (response) => {
            const err = chrome.runtime.lastError;
            const success = !err && response !== undefined && response.success !== false;
            showScreenshotFlash(success);
            if (success)
                playShutterSound();
        });
    }
    catch {
        showScreenshotFlash(false);
    }
}
function createActionButton(label, title, onClick) {
    const button = document.createElement('button');
    button.textContent = label;
    button.title = title;
    button.type = 'button';
    Object.assign(button.style, {
        height: '34px',
        minWidth: '48px',
        borderRadius: '9px',
        border: '1px solid #d1d5db',
        background: '#f3f4f6',
        color: '#1f2937',
        fontSize: '22px',
        lineHeight: '1',
        fontWeight: '600',
        cursor: 'pointer',
        padding: '0 10px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        transition: 'transform 140ms cubic-bezier(0.2, 0.8, 0.2, 1), box-shadow 160ms ease, background-color 160ms ease, border-color 160ms ease, color 160ms ease'
    });
    button.addEventListener('mouseenter', () => {
        button.style.transform = 'translateY(-1px)';
        button.style.boxShadow = '0 4px 12px rgba(15, 23, 42, 0.12)';
        button.style.color = '#ea580c';
    });
    button.addEventListener('mouseleave', () => {
        button.style.transform = 'translateY(0)';
        button.style.boxShadow = 'none';
        button.style.color = '#1f2937';
    });
    button.addEventListener('click', (event) => {
        event.preventDefault();
        event.stopPropagation();
        onClick();
    });
    return button;
}
const ICON_DOCS = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/></svg>';
const ICON_GITHUB = '<svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z"/></svg>';
const ICON_HIDE = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>';
function createSettingsMenuItem(iconSvg, label) {
    const item = document.createElement('div');
    Object.assign(item.style, {
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
        color: '#111827',
        fontSize: '12px',
        fontWeight: '600',
        padding: '8px 10px',
        borderRadius: '8px',
        background: '#f9fafb',
        cursor: 'pointer',
        transition: 'transform 120ms ease, background-color 140ms ease'
    });
    const iconSpan = document.createElement('span');
    iconSpan.innerHTML = iconSvg;
    Object.assign(iconSpan.style, { display: 'flex', alignItems: 'center', flexShrink: '0' });
    const textSpan = document.createElement('span');
    textSpan.textContent = label;
    item.appendChild(iconSpan);
    item.appendChild(textSpan);
    item.addEventListener('mouseenter', () => {
        item.style.transform = 'translateX(1px)';
        item.style.background = '#f3f4f6';
    });
    item.addEventListener('mouseleave', () => {
        item.style.transform = 'translateX(0)';
        item.style.background = '#f9fafb';
    });
    return item;
}
function createSettingsMenuLink(iconSvg, label, href) {
    const link = document.createElement('a');
    link.href = href;
    link.target = '_blank';
    link.rel = 'noopener noreferrer';
    Object.assign(link.style, {
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
        color: '#111827',
        textDecoration: 'none',
        fontSize: '12px',
        fontWeight: '600',
        padding: '8px 10px',
        borderRadius: '8px',
        background: '#f9fafb',
        transition: 'transform 120ms ease, background-color 140ms ease'
    });
    const iconSpan = document.createElement('span');
    iconSpan.innerHTML = iconSvg;
    Object.assign(iconSpan.style, { display: 'flex', alignItems: 'center', flexShrink: '0' });
    const textSpan = document.createElement('span');
    textSpan.textContent = label;
    link.appendChild(iconSpan);
    link.appendChild(textSpan);
    link.addEventListener('mouseenter', () => {
        link.style.transform = 'translateX(1px)';
        link.style.background = '#f3f4f6';
    });
    link.addEventListener('mouseleave', () => {
        link.style.transform = 'translateX(0)';
        link.style.background = '#f9fafb';
    });
    link.addEventListener('click', () => {
        panelPinned = false;
        setPanelOpen(false);
        setSettingsMenuOpen(false);
    });
    return link;
}
function createLauncherUi() {
    const root = document.createElement('div');
    root.id = ROOT_ID;
    Object.assign(root.style, {
        position: 'fixed',
        top: '33vh',
        right: '18px',
        zIndex: '2147483643',
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
        fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
        opacity: '0.65',
        transition: 'opacity 200ms ease',
        pointerEvents: 'none' // Only the toggle circle is interactive when collapsed
    });
    const panel = document.createElement('div');
    panel.id = PANEL_ID;
    Object.assign(panel.style, {
        display: 'flex',
        alignItems: 'center',
        gap: '2px',
        padding: '3px',
        borderRadius: '11px',
        background: '#ffffff',
        border: '1px solid rgba(15, 23, 42, 0.12)',
        boxShadow: '0 8px 24px rgba(15, 23, 42, 0.2)',
        opacity: '0',
        transform: 'translateX(12px) scale(0.96)',
        transformOrigin: 'right center',
        transition: 'opacity 220ms cubic-bezier(0.16, 1, 0.3, 1), transform 220ms cubic-bezier(0.16, 1, 0.3, 1)',
        pointerEvents: 'none',
        backdropFilter: 'saturate(160%) blur(6px)',
        willChange: 'opacity, transform'
    });
    const drawButton = createActionButton('\u270E', 'Annotate the page — draw, highlight, and mark up elements', () => {
        panelPinned = false;
        setPanelOpen(false);
        void startDrawMode();
    });
    drawButton.style.fontSize = '25px';
    const screenshotButton = createActionButton('\u2316', 'Screenshot — capture the current page and send to AI', () => {
        panelPinned = false;
        setPanelOpen(false);
        runScreenshotCapture();
    });
    screenshotButton.style.fontSize = '26px';
    screenshotButton.style.paddingBottom = '5px';
    const terminalButton = createActionButton('_\u276F', 'Terminal — open the side panel terminal', () => {
        panelPinned = false;
        setPanelOpen(false);
        void openTerminalPanel();
    });
    terminalButton.style.fontSize = '21px';
    const settingsButton = createActionButton('\u2699', 'Settings — docs, GitHub, hide launcher', () => {
        panelPinned = true;
        setSettingsMenuOpen(!settingsMenuOpen);
    });
    settingsButton.style.fontSize = '31px';
    settingsButton.style.paddingBottom = '3px';
    const stopButton = createActionButton('\u23F9', 'Stop recording', () => {
        stopRecordingAction();
    });
    stopButton.style.fontSize = '24px';
    stopButton.style.background = '#c0392b';
    stopButton.style.color = '#fff';
    stopButton.style.borderColor = '#a93226';
    stopButton.style.display = 'none';
    stopButton.addEventListener('mouseenter', () => {
        stopButton.style.color = '#fff';
    });
    stopButton.addEventListener('mouseleave', () => {
        stopButton.style.color = '#fff';
    });
    stopButtonEl = stopButton;
    let auditLaunchDebounce = 0;
    const auditButton = createActionButton('\u2691', 'Audit — run the KaBOOM! audit workflow', () => {
        const now = Date.now();
        if (now - auditLaunchDebounce < 500)
            return;
        auditLaunchDebounce = now;
        panelPinned = false;
        setPanelOpen(false);
        void requestAudit(location.href);
    });
    auditButton.style.fontSize = '20px';
    panel.appendChild(drawButton);
    panel.appendChild(stopButton);
    panel.appendChild(screenshotButton);
    panel.appendChild(auditButton);
    panel.appendChild(terminalButton);
    const dotSep = document.createElement('span');
    dotSep.textContent = '\u22EE';
    Object.assign(dotSep.style, {
        color: '#9ca3af',
        fontSize: '16px',
        lineHeight: '1',
        padding: '0 1px',
        userSelect: 'none',
        pointerEvents: 'none'
    });
    panel.appendChild(dotSep);
    panel.appendChild(settingsButton);
    const settingsMenu = document.createElement('div');
    settingsMenu.id = SETTINGS_MENU_ID;
    Object.assign(settingsMenu.style, {
        position: 'absolute',
        top: '40px',
        right: '0',
        minWidth: '220px',
        display: 'flex',
        flexDirection: 'column',
        gap: '6px',
        padding: '10px',
        borderRadius: '12px',
        background: '#ffffff',
        border: '1px solid rgba(15, 23, 42, 0.12)',
        boxShadow: '0 10px 30px rgba(15, 23, 42, 0.18)',
        opacity: '0',
        transform: 'translateY(-8px) scale(0.96)',
        transformOrigin: 'top right',
        transition: 'opacity 180ms cubic-bezier(0.2, 0.8, 0.2, 1), transform 180ms cubic-bezier(0.2, 0.8, 0.2, 1)',
        pointerEvents: 'none',
        willChange: 'opacity, transform'
    });
    const docsLink = createSettingsMenuLink(ICON_DOCS, 'Docs', KABOOM_DOCS_URL);
    const repoLink = createSettingsMenuLink(ICON_GITHUB, 'GitHub Repository', KABOOM_REPOSITORY_URL);
    const hideButton = createSettingsMenuItem(ICON_HIDE, 'Hide KaBOOM! Devtool');
    hideButton.addEventListener('click', () => {
        hideLauncherUntilPopupReopen();
    });
    settingsMenu.appendChild(docsLink);
    settingsMenu.appendChild(repoLink);
    settingsMenu.appendChild(hideButton);
    const toggle = document.createElement('button');
    toggle.id = TOGGLE_ID;
    toggle.type = 'button';
    toggle.title = 'KaBOOM! quick actions';
    const toggleIcon = document.createElement('img');
    toggleIcon.src = chrome.runtime.getURL('icons/icon.svg');
    toggleIcon.alt = 'KaBOOM!';
    Object.assign(toggleIcon.style, {
        width: '36px',
        height: '36px',
        borderRadius: '50%',
        pointerEvents: 'none'
    });
    toggle.appendChild(toggleIcon);
    Object.assign(toggle.style, {
        width: '36px',
        height: '36px',
        borderRadius: '50%',
        border: 'none',
        background: 'transparent',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        cursor: 'pointer',
        padding: '0',
        boxShadow: '0 8px 24px rgba(15, 23, 42, 0.25)',
        transition: 'transform 180ms cubic-bezier(0.2, 0.8, 0.2, 1), box-shadow 180ms ease',
        overflow: 'hidden',
        pointerEvents: 'auto' // Always interactive, even when root is pointer-events:none
    });
    toggle.addEventListener('mouseenter', () => {
        toggle.style.transform = 'translateY(-1px)';
        toggle.style.boxShadow = '0 10px 26px rgba(15, 23, 42, 0.28)';
        toggleIcon.src = chrome.runtime.getURL('icons/logo-animated.svg');
    });
    toggle.addEventListener('mouseleave', () => {
        toggle.style.transform = 'translateY(0)';
        toggle.style.boxShadow = '0 8px 24px rgba(15, 23, 42, 0.25)';
        toggleIcon.src = chrome.runtime.getURL('icons/icon.svg');
    });
    toggle.addEventListener('click', (event) => {
        event.preventDefault();
        event.stopPropagation();
        panelPinned = !panelPinned;
        clearHideTimer();
        setPanelOpen(panelPinned);
        if (!panelPinned)
            setSettingsMenuOpen(false);
    });
    toggleEl = toggle;
    // Hover trigger on the toggle circle only — not the full root container.
    // The root spans the panel width even when collapsed, which would intercept
    // page interactions in the invisible panel region.
    toggle.addEventListener('mouseenter', () => {
        root.style.opacity = '1';
        root.style.pointerEvents = 'auto';
        clearHideTimer();
        setPanelOpen(true);
    });
    // Leave handler stays on root so the panel remains open while mousing across buttons.
    root.addEventListener('mouseleave', () => {
        if (!panelPinned && !settingsMenuOpen) {
            root.style.opacity = '0.65';
            root.style.pointerEvents = 'none';
            // Re-enable pointer events on toggle so it remains hoverable
            toggle.style.pointerEvents = 'auto';
        }
        if (panelPinned || settingsMenuOpen)
            return;
        clearHideTimer();
        hideTimer = setTimeout(() => {
            setPanelOpen(false);
            setSettingsMenuOpen(false);
        }, 120);
    });
    root.appendChild(panel);
    root.appendChild(toggle);
    root.appendChild(settingsMenu);
    panelEl = panel;
    settingsMenuEl = settingsMenu;
    syncRecordingStateFromStorage();
    return root;
}
function mountLauncher() {
    if (hiddenUntilPopupOpen)
        return;
    if (isTerminalVisible())
        return;
    if (rootEl || document.getElementById(ROOT_ID))
        return;
    rootEl = createLauncherUi();
    const target = document.body || document.documentElement;
    if (!target || !rootEl)
        return;
    target.appendChild(rootEl);
    installRecordingStorageSync();
    installAnnotationListener();
}
function unmountLauncher() {
    clearHideTimer();
    panelPinned = false;
    setSettingsMenuOpen(false);
    panelEl = null;
    settingsMenuEl = null;
    stopButtonEl = null;
    toggleEl = null;
    recordingActive = false;
    if (rootEl) {
        rootEl.remove();
        rootEl = null;
    }
    uninstallRecordingStorageSync();
    uninstallAnnotationListener();
}
function syncTerminalPanelVisibility() {
    if (isTerminalVisible()) {
        unmountLauncher();
        return;
    }
    applyVisibilityFromState();
}
export async function setTrackedHoverLauncherEnabled(enabled) {
    trackedEnabled = enabled;
    installRuntimeListener();
    await initTerminalPanelBridge();
    installTerminalVisibilitySync();
    await syncHiddenStateFromStorage();
    applyVisibilityFromState();
}
//# sourceMappingURL=tracked-hover-launcher.js.map