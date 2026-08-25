/**
 * Purpose: Renders action toast overlays showing real-time status (trying/success/error) for AI-driven browser actions.
 * Docs: docs/features/feature/interact-explore/index.md
 */
// toast.ts — Action toast overlay rendering for AI-driven browser actions.
/** Color themes for each toast state */
const TOAST_THEMES = {
    trying: { bg: 'linear-gradient(135deg, #3b82f6 0%, #2563eb 100%)', shadow: 'rgba(59, 130, 246, 0.4)' },
    success: { bg: 'linear-gradient(135deg, #22c55e 0%, #16a34a 100%)', shadow: 'rgba(34, 197, 94, 0.4)' },
    warning: { bg: 'linear-gradient(135deg, #f59e0b 0%, #d97706 100%)', shadow: 'rgba(245, 158, 11, 0.4)' },
    error: { bg: 'linear-gradient(135deg, #ef4444 0%, #dc2626 100%)', shadow: 'rgba(239, 68, 68, 0.4)' },
    audio: { bg: 'linear-gradient(135deg, #f97316 0%, #ea580c 100%)', shadow: 'rgba(249, 115, 22, 0.5)' }
};
/** Pre-built CSS for toast animations — extracted to reduce function complexity */
// nosemgrep: missing-template-string-indicator
const TOAST_ANIMATION_CSS = [
    '@keyframes kaboomArrowBounceUp {',
    '  0%, 100% { transform: translateY(0); opacity: 1; }',
    '  50% { transform: translateY(-6px); opacity: 0.7; }',
    '}',
    '@keyframes kaboomToastPulse {',
    '  0%, 100% { box-shadow: 0 4px 20px var(--toast-shadow); }',
    '  50% { box-shadow: 0 8px 32px var(--toast-shadow-intense); }',
    '}',
    '.kaboom-toast-arrow {',
    '  display: inline-block; margin-left: 8px;',
    '  animation: kaboomArrowBounceUp 1.5s ease-in-out infinite;',
    '}',
    '.kaboom-toast-pulse { animation: kaboomToastPulse 2s ease-in-out infinite; }'
].join('\n');
/** Add animation keyframes to document */
function injectToastAnimationStyles() {
    if (document.getElementById('kaboom-toast-animations'))
        return;
    const style = document.createElement('style');
    style.id = 'kaboom-toast-animations';
    style.textContent = TOAST_ANIMATION_CSS;
    document.head.appendChild(style);
}
function isAudioPromptToast(state, detail) {
    if (state === 'audio')
        return true;
    if (!detail)
        return false;
    const lower = detail.toLowerCase();
    return lower.includes('audio') && lower.includes('click');
}
function appendAudioIcon(toast) {
    const icon = document.createElement('img');
    icon.src = chrome.runtime.getURL('icons/icon-48.png');
    Object.assign(icon.style, {
        width: '20px',
        height: '20px',
        marginRight: '8px',
        flexShrink: '0'
    });
    toast.appendChild(icon);
}
function appendAudioArrow(toast) {
    const arrow = document.createElement('span');
    arrow.className = 'kaboom-toast-arrow';
    arrow.textContent = '\u2191';
    Object.assign(arrow.style, {
        fontSize: '16px',
        fontWeight: '700',
        marginLeft: '12px',
        display: 'inline-block'
    });
    toast.appendChild(arrow);
}
function appendToastText(toast, text, detail) {
    // Build content without truncation: status and recovery guidance must remain actionable.
    const label = document.createElement('span');
    label.textContent = text;
    Object.assign(label.style, { fontWeight: '700' });
    toast.appendChild(label);
    if (detail) {
        const sep = document.createElement('span');
        sep.textContent = '  ';
        Object.assign(sep.style, { opacity: '0.6', margin: '0 4px' });
        toast.appendChild(sep);
        const det = document.createElement('span');
        det.textContent = detail;
        Object.assign(det.style, { fontWeight: '400', opacity: '0.9' });
        toast.appendChild(det);
    }
}
function toastStyleFor(theme, isAudioPrompt) {
    return {
        position: 'fixed',
        top: '16px',
        right: isAudioPrompt ? '80px' : 'auto',
        left: isAudioPrompt ? 'auto' : '50%',
        transform: isAudioPrompt ? 'none' : 'translateX(-50%)',
        padding: isAudioPrompt ? '12px 24px' : '8px 20px',
        background: theme.bg,
        color: '#fff',
        fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
        fontSize: isAudioPrompt ? '14px' : '13px',
        fontWeight: isAudioPrompt ? '600' : '400',
        borderRadius: '8px',
        boxShadow: `0 4px 20px ${theme.shadow}`,
        zIndex: '2147483647',
        pointerEvents: 'none',
        opacity: '0',
        transition: 'opacity 0.2s ease-in',
        width: 'max-content',
        maxWidth: isAudioPrompt ? 'min(320px, calc(100vw - 32px))' : 'min(500px, calc(100vw - 32px))',
        whiteSpace: 'normal',
        overflow: 'visible',
        overflowWrap: 'anywhere',
        display: 'flex',
        alignItems: 'center',
        gap: '0',
        '--toast-shadow': theme.shadow,
        '--toast-shadow-intense': theme.shadow.replace('0.4)', '0.7)')
    };
}
/**
 * Show a brief visual toast overlay for AI actions.
 * Supports color-coded states and fully readable structured content.
 * For audio-related toasts, adds animated arrow pointing to extension icon.
 */
export function showActionToast(text, detail, state = 'trying', durationMs = 3000) {
    // Remove existing toast
    const existing = document.getElementById('kaboom-action-toast');
    if (existing)
        existing.remove();
    // Inject animation styles once
    injectToastAnimationStyles();
    const theme = TOAST_THEMES[state] ?? TOAST_THEMES.trying;
    const isAudioPrompt = isAudioPromptToast(state, detail);
    const toast = document.createElement('div');
    toast.id = 'kaboom-action-toast';
    toast.setAttribute?.('data-kaboom-owned', 'true');
    if (isAudioPrompt) {
        toast.className = 'kaboom-toast-pulse';
    }
    // Add kaboom icon for audio/extension-click prompts
    if (isAudioPrompt) {
        appendAudioIcon(toast);
    }
    appendToastText(toast, text, detail);
    // Add animated arrow for audio prompts (pointing to extension toolbar)
    if (isAudioPrompt) {
        appendAudioArrow(toast);
    }
    Object.assign(toast.style, toastStyleFor(theme, isAudioPrompt));
    const target = document.body || document.documentElement;
    if (!target)
        return;
    target.appendChild(toast);
    // Fade in
    requestAnimationFrame(() => {
        toast.style.opacity = '1';
    });
    // Fade out and remove
    setTimeout(() => {
        toast.style.opacity = '0';
        setTimeout(() => toast.remove(), 300);
    }, durationMs);
}
//# sourceMappingURL=toast.js.map