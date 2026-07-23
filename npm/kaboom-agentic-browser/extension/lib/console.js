/**
 * Purpose: Monkey-patches console.log/warn/error/info/debug to capture messages and forward them via the bridge while preserving original behavior.
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview Console method capture.
 * Monkey-patches console.log/warn/error/info/debug to capture messages
 * and forward them via postLog, while preserving original behavior.
 */
import { safeSerialize } from './serialize.js';
import { postLog } from './bridge.js';
// Store original methods
let originalConsole = {};
/**
 * Install console capture hooks
 */
export function installConsoleCapture() {
    const methods = ['log', 'warn', 'error', 'info', 'debug'];
    methods.forEach((method) => {
        // eslint-disable-next-line security/detect-object-injection -- method from known-safe local array of console methods
        originalConsole[method] = console[method];
        // eslint-disable-next-line security/detect-object-injection -- method from known-safe local array of console methods
        console[method] = function (...args) {
            // Post to extension
            postLog({
                level: method,
                type: 'console',
                args: args.map((arg) => safeSerialize(arg))
            });
            // Call original
            // eslint-disable-next-line security/detect-object-injection -- method from known-safe local array of console methods
            originalConsole[method].apply(console, args);
        };
    });
}
/**
 * Uninstall console capture hooks
 */
export function uninstallConsoleCapture() {
    Object.keys(originalConsole).forEach((method) => {
        // eslint-disable-next-line security/detect-object-injection -- method from Object.keys of our own originalConsole storage
        console[method] = originalConsole[method];
    });
    originalConsole = {};
}
//# sourceMappingURL=console.js.map