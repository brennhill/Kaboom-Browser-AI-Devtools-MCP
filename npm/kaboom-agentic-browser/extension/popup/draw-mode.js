/**
 * Purpose: Handles popup-side draw mode launch flow and user-facing failure messaging.
 * Why: Provides a deterministic handoff from popup controls to content-script annotation capture.
 * Docs: docs/features/feature/annotated-screenshots/index.md
 */
/**
 * @fileoverview Draw Mode Button Module for Popup
 * Manages the draw mode activation button and error handling.
 */
function showDrawModeError(label, message) {
    label.textContent = message;
    label.style.color = '#f85149';
    setTimeout(() => {
        label.textContent = 'Draw';
        label.style.color = '';
    }, 3000);
}
export function setupDrawModeButton() {
    const row = document.getElementById('draw-mode-row');
    const label = document.getElementById('draw-mode-label');
    if (!row || !label)
        return;
    const statusEl = document.getElementById('draw-mode-status');
    if (statusEl) {
        const hasNavigator = typeof navigator !== 'undefined';
        const isMac = hasNavigator &&
            (navigator.platform?.toUpperCase().includes('MAC') ||
                navigator.userAgentData?.platform === 'macOS');
        statusEl.textContent = isMac ? '⌥⇧D' : 'Alt+Shift+D';
    }
    row.addEventListener('click', () => {
        chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
            const tab = tabs[0];
            if (!tab?.id) {
                showDrawModeError(label, 'No active tab');
                return;
            }
            if (tab.url?.startsWith('chrome://') ||
                tab.url?.startsWith('about:') ||
                tab.url?.startsWith('chrome-extension://')) {
                showDrawModeError(label, 'Cannot draw on internal pages');
                return;
            }
            label.textContent = 'Starting...';
            chrome.tabs.sendMessage(tab.id, { type: 'kaboom_draw_mode_start', started_by: 'user' }, (resp) => {
                if (chrome.runtime.lastError) {
                    showDrawModeError(label, 'Content script not loaded — try refreshing the page');
                    return;
                }
                if (resp?.error) {
                    showDrawModeError(label, resp.message || 'Draw mode failed');
                    return;
                }
                window.close();
            });
        });
    });
}
//# sourceMappingURL=draw-mode.js.map