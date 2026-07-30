import { KABOOM_LOG_PREFIX } from '../brand.js';
export function reportStateRecovery(diagnostic) {
    console.warn(`${KABOOM_LOG_PREFIX} persisted state fallback active: ${diagnostic.name}`);
    sendTransition('active', diagnostic);
}
export function resolveStateRecovery(name) {
    sendTransition('recovered', { name, detail: '', fix: '' });
}
function sendTransition(lifecycle, diagnostic) {
    if (typeof chrome === 'undefined' || !chrome.runtime?.sendMessage)
        return;
    const message = {
        type: 'report_state_recovery',
        lifecycle,
        diagnostic
    };
    try {
        const pending = chrome.runtime.sendMessage(message);
        if (pending && typeof pending.catch === 'function') {
            void pending.catch(() => {
                console.warn(`${KABOOM_LOG_PREFIX} state recovery transition was not delivered: ${lifecycle}/${diagnostic.name}`);
            });
        }
    }
    catch {
        // Console warning remains available when the background worker is unavailable.
    }
}
//# sourceMappingURL=recovery.js.map