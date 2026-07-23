/**
 * Purpose: Captures unhandled exceptions and promise rejections via window.onerror/unhandledrejection, enriching them with AI context before posting.
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview Exception and unhandled rejection capture.
 * Monkey-patches window.onerror and listens for unhandledrejection events,
 * enriching errors with AI context before posting via bridge.
 */
import { postLog } from './bridge.js';
import { enrichErrorWithAiContext } from './ai-context.js';
// Exception capture state
let originalOnerror = null;
let unhandledrejectionHandler = null;
/**
 * Install exception capture
 */
function enrichAndPost(entry) {
    void (async () => {
        try {
            const enriched = await enrichErrorWithAiContext(entry);
            postLog(enriched);
        }
        catch {
            postLog(entry);
        }
    })().catch((err) => {
        console.error('[KaBOOM!] Exception enrichment error:', err);
        try {
            postLog(entry);
        }
        catch (postErr) {
            console.error('[KaBOOM!] Failed to log entry:', postErr);
        }
    });
}
function extractRejectionInfo(reason) {
    if (reason instanceof Error)
        return { message: reason.message, stack: reason.stack || '' };
    if (typeof reason === 'string')
        return { message: reason, stack: '' };
    return { message: String(reason), stack: '' };
}
export function installExceptionCapture() {
    originalOnerror = window.onerror;
    window.onerror = function (message, filename, lineno, colno, error) {
        const messageStr = typeof message === 'string' ? message : message.type || 'Error';
        const entry = {
            level: 'error',
            type: 'exception',
            message: messageStr,
            source: filename ? `${filename}:${lineno || 0}` : '',
            filename: filename || '',
            lineno: lineno || 0,
            colno: colno || 0,
            stack: error?.stack || ''
        };
        enrichAndPost(entry);
        if (originalOnerror)
            return originalOnerror(message, filename, lineno, colno, error);
        return false;
    };
    unhandledrejectionHandler = function (event) {
        const { message, stack } = extractRejectionInfo(event.reason);
        enrichAndPost({
            level: 'error',
            type: 'exception',
            message: `Unhandled Promise Rejection: ${message}`,
            stack
        });
    };
    window.addEventListener('unhandledrejection', unhandledrejectionHandler);
}
/**
 * Uninstall exception capture
 */
export function uninstallExceptionCapture() {
    if (originalOnerror !== null) {
        window.onerror = originalOnerror;
        originalOnerror = null;
    }
    if (unhandledrejectionHandler) {
        window.removeEventListener('unhandledrejection', unhandledrejectionHandler);
        unhandledrejectionHandler = null;
    }
}
//# sourceMappingURL=exceptions.js.map