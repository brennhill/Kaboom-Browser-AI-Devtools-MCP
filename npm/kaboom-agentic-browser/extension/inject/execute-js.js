/**
 * Purpose: JavaScript execution sandbox for evaluating arbitrary scripts in page context with safe serialization and timeout support.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import { createDeferredPromise } from '../lib/timeout-utils.js';
/**
 * Safe serialization for complex objects returned from executeJavaScript.
 */
// #lizard forgives
function serializeObject(obj, depth, seen) {
    if (seen.has(obj))
        return '[Circular]';
    seen.add(obj);
    if (Array.isArray(obj))
        return obj.slice(0, 100).map((v) => safeSerializeForExecute(v, depth + 1, seen));
    if (obj instanceof Error)
        return { error: obj.message, stack: obj.stack };
    if (obj instanceof Date)
        return obj.toISOString();
    if (obj instanceof RegExp)
        return obj.toString();
    if (typeof Node !== 'undefined' && obj instanceof Node) {
        const node = obj;
        return `[${node.nodeName}${node.id ? '#' + node.id : ''}]`;
    }
    // Browser host objects (DOMRect, DOMPoint, DOMMatrix) have prototype getters
    // that Object.keys() misses. Their toJSON() returns a plain object.
    if (typeof obj.toJSON === 'function') {
        try {
            return safeSerializeForExecute(obj.toJSON(), depth + 1, seen);
        }
        catch {
            // Fall through to Object.keys() enumeration
        }
    }
    const result = {};
    const keys = Object.keys(obj).slice(0, 50);
    // Host objects like DOMRect/CSSStyleDeclaration expose values via prototype getters,
    // so Object.keys() can be empty even when useful primitive fields exist.
    if (keys.length === 0) {
        try {
            const proto = Object.getPrototypeOf(obj);
            if (proto && proto !== Object.prototype) {
                const hostResult = {};
                const propNames = Object.getOwnPropertyNames(proto).slice(0, 120);
                for (const key of propNames) {
                    if (key === 'constructor')
                        continue;
                    try {
                        const hostValue = obj[key];
                        const hostType = typeof hostValue;
                        if (hostValue === undefined || hostType === 'function')
                            continue;
                        if (hostType === 'string' || hostType === 'number' || hostType === 'boolean' || hostValue === null) {
                            hostResult[key] = hostValue;
                        }
                    }
                    catch {
                        // Ignore getter access errors and continue.
                    }
                    if (Object.keys(hostResult).length >= 50)
                        break;
                }
                if (Object.keys(hostResult).length > 0)
                    return hostResult;
            }
        }
        catch {
            // Fall through to default enumeration.
        }
    }
    for (const key of keys) {
        try {
            result[key] = safeSerializeForExecute(obj[key], depth + 1, seen);
        }
        catch {
            result[key] = '[unserializable]';
        }
    }
    if (Object.keys(obj).length > 50) {
        result['...'] = `[${Object.keys(obj).length - 50} more keys]`;
    }
    return result;
}
export function safeSerializeForExecute(value, depth = 0, seen = new WeakSet()) {
    if (depth > 10)
        return '[max depth exceeded]';
    if (value === null || value === undefined)
        return value;
    const type = typeof value;
    if (type === 'string' || type === 'number' || type === 'boolean')
        return value;
    if (type === 'function')
        return `[Function: ${value.name || 'anonymous'}]`;
    if (type === 'symbol')
        return value.toString();
    if (type === 'object')
        return serializeObject(value, depth, seen);
    return String(value);
}
/**
 * Execute arbitrary JavaScript in the page context with timeout handling.
 */
export function executeJavaScript(script, timeoutMs = 5000) {
    const deferred = createDeferredPromise();
    // #lizard forgives
    const executeWithTimeoutProtection = async () => {
        const timeoutHandle = setTimeout(() => {
            deferred.resolve({
                success: false,
                error: 'execution_timeout',
                message: `Script exceeded ${timeoutMs}ms timeout. RECOMMENDED ACTIONS:

1. Check for infinite loops or blocking operations in your script
2. Break the task into smaller pieces (< 2s execution time works best)
3. Verify the script logic - test with simpler operations first

Tip: Run small test scripts to isolate the issue, then build up complexity.`
            });
        }, timeoutMs);
        try {
            const cleanScript = script.trim();
            // Try expression form first (captures return values from IIFEs, expressions).
            // If it throws SyntaxError (statements like try/catch, if/else), fall back to statement form.
            let fn;
            try {
                // eslint-disable-next-line no-new-func
                fn = new Function(`"use strict"; return (${cleanScript});`); // nosemgrep: javascript.lang.security.eval.rule-eval-with-expression -- Function() constructor for controlled sandbox execution
            }
            catch {
                // eslint-disable-next-line no-new-func
                fn = new Function(`"use strict"; ${cleanScript}`); // nosemgrep: javascript.lang.security.eval.rule-eval-with-expression -- Function() constructor for controlled sandbox execution
            }
            const result = fn();
            // Handle promises
            if (result && typeof result.then === 'function') {
                ;
                result
                    .then((value) => {
                    clearTimeout(timeoutHandle);
                    deferred.resolve({ success: true, result: safeSerializeForExecute(value) });
                })
                    .catch((err) => {
                    clearTimeout(timeoutHandle);
                    deferred.resolve({
                        success: false,
                        error: 'promise_rejected',
                        message: err.message,
                        stack: err.stack
                    });
                });
            }
            else {
                clearTimeout(timeoutHandle);
                deferred.resolve({ success: true, result: safeSerializeForExecute(result) });
            }
        }
        catch (err) {
            clearTimeout(timeoutHandle);
            const error = err;
            if (error.message &&
                (error.message.includes('Content Security Policy') ||
                    error.message.includes('unsafe-eval') ||
                    error.message.includes('Trusted Type'))) {
                deferred.resolve({
                    success: false,
                    error: 'csp_blocked',
                    message: 'This page has a Content Security Policy that blocks script execution in the MAIN world. ' +
                        'Use world: "isolated" to bypass CSP (DOM access only, no page JS globals). ' +
                        'With world: "auto" (default), this fallback happens automatically.'
                });
            }
            else {
                deferred.resolve({
                    success: false,
                    error: 'execution_error',
                    message: error.message,
                    stack: error.stack
                });
            }
        }
    };
    executeWithTimeoutProtection().catch((err) => {
        console.error('[KaBOOM!] Unexpected error in executeJavaScript:', err);
        deferred.resolve({
            success: false,
            error: 'execution_error',
            message: 'Unexpected error during script execution'
        });
    });
    return deferred.promise;
}
//# sourceMappingURL=execute-js.js.map