/**
 * Purpose: Renders subtitle overlays and recording watermarks on the page for AI-driven action narration.
 * Docs: docs/features/feature/subtitle/index.md
 */
// subtitle.ts — Subtitle overlay, recording watermark, and shared DOM helpers for content UI.
/** Active Escape key listener reference for subtitle dismiss */
let subtitleEscapeHandler = null;
/** Auto-clear timer: subtitles auto-dismiss after 60 seconds to prevent stale overlays */
const SUBTITLE_AUTO_TIMEOUT_MS = 60_000;
let subtitleAutoTimer = null;
function clearAutoTimer() {
    if (subtitleAutoTimer) {
        clearTimeout(subtitleAutoTimer);
        subtitleAutoTimer = null;
    }
}
/** Fade out a DOM element and remove it after transition completes */
// #lizard forgives
function fadeOutAndRemove(elementId, delayMs) {
    const el = document.getElementById(elementId);
    if (!el)
        return;
    el.style.opacity = '0';
    setTimeout(() => el.remove(), delayMs);
}
/** Detach the active Escape key listener if one exists */
function detachEscapeListener() {
    if (!subtitleEscapeHandler)
        return;
    document.removeEventListener('keydown', subtitleEscapeHandler);
    subtitleEscapeHandler = null;
}
/**
 * Remove the subtitle element, clean up Escape listener.
 */
export function clearSubtitle() {
    clearAutoTimer();
    fadeOutAndRemove('kaboom-subtitle', 200);
    detachEscapeListener();
}
/**
 * Show or update a persistent subtitle bar at the bottom of the viewport.
 * Empty text clears the subtitle. Includes a hover close button and
 * Escape key listener for dismissal.
 */
export function showSubtitle(text) {
    const ELEMENT_ID = 'kaboom-subtitle';
    const CLOSE_BTN_ID = 'kaboom-subtitle-close';
    if (!text) {
        clearSubtitle();
        return;
    }
    let bar = document.getElementById(ELEMENT_ID);
    if (!bar) {
        bar = document.createElement('div');
        bar.id = ELEMENT_ID;
        Object.assign(bar.style, {
            position: 'fixed',
            bottom: '24px',
            left: '50%',
            transform: 'translateX(-50%)',
            width: 'auto',
            maxWidth: '80%',
            padding: '12px 20px',
            background: 'rgba(0, 0, 0, 0.85)',
            color: '#fff',
            fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
            fontSize: '16px',
            lineHeight: '1.4',
            textAlign: 'center',
            borderRadius: '4px',
            zIndex: '2147483646',
            pointerEvents: 'auto',
            opacity: '0',
            transition: 'opacity 0.2s ease-in',
            maxHeight: '4.2em', // ~3 lines
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            boxSizing: 'border-box'
        });
        // Close button — visible only on hover over the subtitle bar
        const closeBtn = document.createElement('button');
        closeBtn.id = CLOSE_BTN_ID;
        closeBtn.textContent = '\u00d7'; // multiplication sign (x)
        Object.assign(closeBtn.style, {
            position: 'absolute',
            top: '-6px',
            right: '-6px',
            width: '16px',
            height: '16px',
            padding: '0',
            margin: '0',
            border: 'none',
            borderRadius: '50%',
            background: 'rgba(255, 255, 255, 0.25)',
            color: '#fff',
            fontSize: '12px',
            lineHeight: '16px',
            textAlign: 'center',
            cursor: 'pointer',
            pointerEvents: 'auto',
            opacity: '0',
            transition: 'opacity 0.15s ease-in',
            fontFamily: 'sans-serif'
        });
        closeBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            clearSubtitle();
        });
        bar.appendChild(closeBtn);
        // Show close button on hover
        bar.addEventListener('mouseenter', () => {
            const btn = document.getElementById(CLOSE_BTN_ID);
            if (btn)
                btn.style.opacity = '1';
        });
        bar.addEventListener('mouseleave', () => {
            const btn = document.getElementById(CLOSE_BTN_ID);
            if (btn)
                btn.style.opacity = '0';
        });
        const target = document.body || document.documentElement;
        if (!target)
            return;
        target.appendChild(bar);
    }
    // Update text content while preserving the close button
    const closeBtn = document.getElementById(CLOSE_BTN_ID);
    // Set text on bar, then re-append close button so it stays on top
    bar.textContent = text;
    if (closeBtn) {
        bar.appendChild(closeBtn);
    }
    // Register Escape key listener (replace any existing one)
    if (subtitleEscapeHandler) {
        document.removeEventListener('keydown', subtitleEscapeHandler);
    }
    subtitleEscapeHandler = (e) => {
        if (e.key === 'Escape') {
            clearSubtitle();
        }
    };
    document.addEventListener('keydown', subtitleEscapeHandler);
    // Force reflow so the browser registers opacity:0, then set to 1
    // for the CSS transition. No timer needed — avoids rAF (paused in
    // background tabs) and setTimeout (throttled to 1s in background tabs).
    void bar.offsetHeight;
    bar.style.opacity = '1';
    // Auto-clear after 60s to prevent stale overlays from persisting indefinitely
    clearAutoTimer();
    subtitleAutoTimer = setTimeout(() => {
        clearSubtitle();
    }, SUBTITLE_AUTO_TIMEOUT_MS);
}
/**
 * Show or hide a recording watermark (Kaboom flame icon) in the bottom-right corner.
 * The icon renders at 64x64px with 50% opacity, captured in the tab video.
 */
export function toggleRecordingWatermark(visible) {
    const ELEMENT_ID = 'kaboom-recording-watermark';
    if (!visible) {
        const existing = document.getElementById(ELEMENT_ID);
        if (existing) {
            existing.style.opacity = '0';
            setTimeout(() => existing.remove(), 300);
        }
        return;
    }
    // Don't create a duplicate
    if (document.getElementById(ELEMENT_ID))
        return;
    const container = document.createElement('div');
    container.id = ELEMENT_ID;
    Object.assign(container.style, {
        position: 'fixed',
        bottom: '16px',
        right: '16px',
        width: '64px',
        height: '64px',
        opacity: '0',
        transition: 'opacity 0.3s ease-in',
        zIndex: '2147483645',
        pointerEvents: 'none'
    });
    const img = document.createElement('img');
    img.src = chrome.runtime.getURL('icons/icon.svg');
    Object.assign(img.style, { width: '100%', height: '100%', opacity: '0.5' });
    container.appendChild(img);
    const target = document.body || document.documentElement;
    if (!target)
        return;
    target.appendChild(container);
    // Trigger reflow then fade in
    void container.offsetHeight;
    container.style.opacity = '1';
}
//# sourceMappingURL=subtitle.js.map