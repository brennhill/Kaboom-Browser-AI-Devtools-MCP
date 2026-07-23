/**
 * Purpose: Inline chat widget for browser-to-AI message push.
 * Why: Lets users type messages to the AI without leaving the browser.
 * Docs: docs/features/feature/browser-push/index.md
 */
// chat-widget.ts — Lower-corner command palette with pin toggle for chat push.
const WIDGET_ID = 'kaboom-chat-widget';
const INPUT_ID = 'kaboom-chat-input';
const PIN_ID = 'kaboom-chat-pin';
const STATUS_ID = 'kaboom-chat-status';
/** Whether the widget should persist after sending (pin toggle) */
let isPinned = false;
/** Current client name for display */
let currentClientName = 'AI';
/** Escape key handler reference */
let chatEscapeHandler = null;
/** Guard against rapid toggle race conditions */
let isRemoving = false;
/**
 * Toggle the chat widget visibility.
 * If visible, hides it. If hidden, shows it.
 */
export function toggleChatWidget(clientName) {
    if (clientName)
        currentClientName = clientName;
    const existing = document.getElementById(WIDGET_ID);
    if (existing && !isRemoving) {
        removeChatWidget();
    }
    else if (!existing && !isRemoving) {
        showChatWidget();
    }
}
/** Show the chat widget in the lower-right corner. */
function showChatWidget() {
    if (document.getElementById(WIDGET_ID))
        return;
    const widget = document.createElement('div');
    widget.id = WIDGET_ID;
    widget.setAttribute('role', 'dialog');
    widget.setAttribute('aria-label', `Push message to ${currentClientName}`);
    Object.assign(widget.style, {
        position: 'fixed',
        bottom: '20px',
        right: '20px',
        width: '340px',
        background: '#1a1a2e',
        borderRadius: '12px',
        boxShadow: '0 8px 32px rgba(0, 0, 0, 0.4), 0 0 0 1px rgba(255, 255, 255, 0.08)',
        zIndex: '2147483643',
        fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
        overflow: 'hidden',
        opacity: '0',
        transform: 'translateY(10px)',
        transition: 'opacity 0.15s ease-out, transform 0.15s ease-out'
    });
    // Stop all keydown events from propagating to the page
    widget.addEventListener('keydown', (e) => {
        e.stopPropagation();
    });
    // Header bar
    const header = document.createElement('div');
    Object.assign(header.style, {
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '10px 14px',
        background: 'rgba(255, 255, 255, 0.04)',
        borderBottom: '1px solid rgba(255, 255, 255, 0.06)'
    });
    // Left side: label
    const headerLeft = document.createElement('div');
    Object.assign(headerLeft.style, { display: 'flex', alignItems: 'center', gap: '8px' });
    const label = document.createElement('span');
    label.textContent = `Push to ${currentClientName}`;
    Object.assign(label.style, {
        color: '#e0e0e0',
        fontSize: '12px',
        fontWeight: '600',
        letterSpacing: '0.3px'
    });
    headerLeft.appendChild(label);
    header.appendChild(headerLeft);
    // Right side: pin toggle + close
    const headerRight = document.createElement('div');
    Object.assign(headerRight.style, { display: 'flex', alignItems: 'center', gap: '6px' });
    const pinBtn = document.createElement('button');
    pinBtn.id = PIN_ID;
    pinBtn.title = isPinned ? 'Unpin (close after sending)' : 'Pin (keep open after sending)';
    pinBtn.textContent = 'Pin';
    pinBtn.setAttribute('aria-pressed', String(isPinned));
    Object.assign(pinBtn.style, {
        background: isPinned ? 'rgba(59, 130, 246, 0.3)' : 'transparent',
        border: '1px solid ' + (isPinned ? 'rgba(59, 130, 246, 0.5)' : 'rgba(255, 255, 255, 0.1)'),
        borderRadius: '4px',
        color: isPinned ? '#60a5fa' : '#999',
        fontSize: '11px',
        cursor: 'pointer',
        padding: '2px 8px',
        transition: 'all 0.15s ease'
    });
    pinBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        isPinned = !isPinned;
        pinBtn.setAttribute('aria-pressed', String(isPinned));
        pinBtn.title = isPinned ? 'Unpin (close after sending)' : 'Pin (keep open after sending)';
        Object.assign(pinBtn.style, {
            background: isPinned ? 'rgba(59, 130, 246, 0.3)' : 'transparent',
            border: '1px solid ' + (isPinned ? 'rgba(59, 130, 246, 0.5)' : 'rgba(255, 255, 255, 0.1)'),
            color: isPinned ? '#60a5fa' : '#999'
        });
    });
    headerRight.appendChild(pinBtn);
    const closeBtn = document.createElement('button');
    closeBtn.textContent = '\u00d7';
    closeBtn.title = 'Close';
    closeBtn.setAttribute('aria-label', 'Close chat widget');
    Object.assign(closeBtn.style, {
        background: 'transparent',
        border: 'none',
        color: '#999',
        fontSize: '16px',
        cursor: 'pointer',
        padding: '0 2px',
        lineHeight: '1'
    });
    closeBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        removeChatWidget();
    });
    headerRight.appendChild(closeBtn);
    header.appendChild(headerRight);
    widget.appendChild(header);
    // Input area
    const inputWrap = document.createElement('div');
    Object.assign(inputWrap.style, { padding: '12px 14px' });
    const input = document.createElement('textarea');
    input.id = INPUT_ID;
    input.placeholder = 'Type a message to push...';
    input.rows = 2;
    input.maxLength = 10000;
    input.setAttribute('aria-label', 'Message to push');
    Object.assign(input.style, {
        width: '100%',
        background: 'rgba(255, 255, 255, 0.06)',
        border: '1px solid rgba(255, 255, 255, 0.1)',
        borderRadius: '8px',
        color: '#e0e0e0',
        fontSize: '13px',
        lineHeight: '1.5',
        padding: '10px 12px',
        resize: 'none',
        outline: 'none',
        fontFamily: 'inherit',
        boxSizing: 'border-box',
        minHeight: '44px',
        maxHeight: '120px',
        transition: 'border-color 0.15s ease'
    });
    input.addEventListener('focus', () => {
        input.style.borderColor = 'rgba(59, 130, 246, 0.5)';
    });
    input.addEventListener('blur', () => {
        input.style.borderColor = 'rgba(255, 255, 255, 0.1)';
    });
    // Enter to send, Shift+Enter for newline, Escape to close
    input.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            sendChatMessage();
        }
        else if (e.key === 'Escape') {
            e.preventDefault();
            e.stopImmediatePropagation();
            removeChatWidget();
        }
    });
    inputWrap.appendChild(input);
    widget.appendChild(inputWrap);
    // Footer: status + hint
    const footer = document.createElement('div');
    Object.assign(footer.style, {
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '0 14px 10px',
        fontSize: '11px',
        color: '#999'
    });
    const status = document.createElement('span');
    status.id = STATUS_ID;
    status.setAttribute('aria-live', 'polite');
    status.textContent = '';
    footer.appendChild(status);
    const hint = document.createElement('span');
    hint.textContent = 'Enter send | Shift+Enter newline | Esc close';
    Object.assign(hint.style, { color: '#aaa' });
    footer.appendChild(hint);
    widget.appendChild(footer);
    const target = document.body || document.documentElement;
    if (!target)
        return;
    target.appendChild(widget);
    // Register Escape handler (document-level fallback)
    if (chatEscapeHandler) {
        document.removeEventListener('keydown', chatEscapeHandler);
    }
    chatEscapeHandler = (e) => {
        if (e.key === 'Escape') {
            e.stopImmediatePropagation();
            removeChatWidget();
        }
    };
    document.addEventListener('keydown', chatEscapeHandler, { capture: true });
    // Focus trapping: Tab cycles between input, pin, and close buttons
    const focusable = [input, pinBtn, closeBtn];
    widget.addEventListener('keydown', (e) => {
        if (e.key !== 'Tab')
            return;
        const focused = document.activeElement;
        if (!focused)
            return;
        const idx = focusable.indexOf(focused);
        if (idx < 0)
            return;
        e.preventDefault();
        const next = e.shiftKey ? (idx - 1 + focusable.length) % focusable.length : (idx + 1) % focusable.length;
        const el = focusable[next];
        if (el)
            el.focus();
    });
    // Animate in
    requestAnimationFrame(() => {
        widget.style.opacity = '1';
        widget.style.transform = 'translateY(0)';
        input.focus();
    });
}
/** Remove the chat widget immediately (no race-prone delay). */
function removeChatWidget() {
    if (isRemoving)
        return;
    const widget = document.getElementById(WIDGET_ID);
    if (!widget)
        return;
    isRemoving = true;
    widget.style.opacity = '0';
    widget.style.transform = 'translateY(10px)';
    setTimeout(() => {
        widget.remove();
        isRemoving = false;
    }, 150);
    if (chatEscapeHandler) {
        document.removeEventListener('keydown', chatEscapeHandler, { capture: true });
        chatEscapeHandler = null;
    }
}
/** Send the chat message via the background script. */
function sendChatMessage() {
    const input = document.getElementById(INPUT_ID);
    if (!input)
        return;
    const message = input.value.trim();
    if (!message) {
        // Flash border for empty input feedback
        input.style.borderColor = 'rgba(239, 68, 68, 0.5)';
        setTimeout(() => {
            input.style.borderColor = 'rgba(59, 130, 246, 0.5)';
        }, 600);
        return;
    }
    const statusEl = document.getElementById(STATUS_ID);
    // Disable input to debounce rapid Enter presses
    input.disabled = true;
    // Show sending state
    if (statusEl) {
        statusEl.textContent = 'Sending...';
        statusEl.style.color = '#60a5fa';
    }
    chrome.runtime.sendMessage({
        type: 'kaboom_push_chat',
        message,
        page_url: window.location.href
    }, (response) => {
        if (chrome.runtime.lastError || !response?.success) {
            if (statusEl) {
                statusEl.textContent = response?.error || 'Send failed';
                statusEl.style.color = '#f87171';
            }
            input.disabled = false;
            return;
        }
        // Success
        if (statusEl) {
            const deliveryText = response.status === 'delivered' ? 'Sent' : 'Queued';
            statusEl.textContent = deliveryText;
            statusEl.style.color = '#22c55e';
        }
        input.value = '';
        input.disabled = false;
        if (!isPinned) {
            setTimeout(() => removeChatWidget(), 1200);
        }
        else {
            input.focus();
            // Clear status after a moment
            setTimeout(() => {
                if (statusEl) {
                    statusEl.textContent = '';
                }
            }, 2000);
        }
    });
}
//# sourceMappingURL=chat-widget.js.map