/**
 * Purpose: Executes JavaScript in page context with world-aware routing (content script relay, chrome.scripting, or CSP-safe structured executor).
 * Docs: docs/features/feature/csp-safe-execution/index.md
 */
// query-execution.ts — JavaScript execution with world-aware routing and CSP fallback.
// Handles execute_js queries via content script relay, chrome.scripting API, or structured executor.
import { DebugCategory, debugLog } from '../debug.js';
import { scaleTimeout } from '../../lib/timeouts.js';
import { parseExpression } from './csp-safe/parser.js';
import { cspSafeExecutor } from './csp-safe/executor.js';
import { errorMessage } from '../../lib/error-utils.js';
/**
 * Probe whether a tab's CSP blocks dynamic script execution (new Function).
 * Returns one of three levels:
 * - "none": No CSP restrictions — execute_js is safe
 * - "script_exec": new Function() blocked — use dom/get_readable instead
 * - "page_blocked": Privileged URL (chrome://, devtools://) — no extension access
 */
export async function probeCSPStatus(tabId) {
    try {
        const results = await chrome.scripting.executeScript({
            target: { tabId },
            world: 'MAIN',
            func: () => {
                try {
                    new Function('return 1')();
                    return 'ok';
                }
                catch {
                    // EXPECTED_ABSENCE: CSP rejection is the expected probe result; logging would falsely report the successful measurement.
                    return 'csp_blocked';
                }
            }
        });
        const val = results?.[0]?.result;
        if (val === 'ok')
            return { csp_restricted: false, csp_level: 'none' };
        if (val === 'csp_blocked')
            return { csp_restricted: true, csp_level: 'script_exec' };
        return { csp_restricted: true, csp_level: 'page_blocked' };
    }
    catch {
        return { csp_restricted: true, csp_level: 'page_blocked' };
    }
}
/**
 * Execute JavaScript via chrome.scripting.executeScript.
 * Used as fallback when MAIN world execution fails due to page CSP,
 * or when inject script is not loaded.
 * The func is injected natively by Chrome's extension system.
 */
export async function executeViaScriptingAPI(tabId, script, timeoutMs, world = 'MAIN') {
    const timeoutPromise = new Promise((_, reject) => {
        setTimeout(() => reject(new Error(`Script exceeded ${timeoutMs}ms timeout`)), timeoutMs + 2000);
    });
    const executionPromise = chrome.scripting.executeScript({
        target: { tabId },
        world: world,
        func: (code) => {
            try {
                const cleaned = code.trim();
                // Try expression form first (captures return values from IIFEs, expressions).
                // If SyntaxError (statements like try/catch, if/else), fall back to statement form.
                let fn;
                try {
                    // eslint-disable-next-line no-new-func
                    fn = new Function(`"use strict"; return (${cleaned});`); // nosemgrep: javascript.lang.security.eval.rule-eval-with-expression -- chrome.scripting.executeScript API, not eval()
                }
                catch {
                    // eslint-disable-next-line no-new-func
                    fn = new Function(`"use strict"; ${cleaned}`); // nosemgrep: javascript.lang.security.eval.rule-eval-with-expression -- chrome.scripting.executeScript API, not eval()
                }
                const result = fn();
                if (result !== null && result !== undefined && typeof result.then === 'function') {
                    return result
                        .then((v) => {
                        return { success: true, result: serialize(v) };
                    })
                        .catch((err) => {
                        const e = err;
                        return { success: false, error: 'promise_rejected', message: e.message };
                    });
                }
                return { success: true, result: serialize(result) };
            }
            catch (err) {
                const e = err;
                const msg = e.message || '';
                if (msg.includes('Content Security Policy') || msg.includes('Trusted Type') || msg.includes('unsafe-eval')) {
                    return {
                        success: false,
                        error: 'csp_blocked_all_worlds',
                        message: 'Page CSP blocks dynamic script execution. ' +
                            'Use query_dom for DOM operations or navigate away from this CSP-restricted page.'
                    };
                }
                return { success: false, error: 'execution_error', message: msg, stack: e.stack };
            }
            function serialize(value, depth = 0, seen = new WeakSet()) {
                // Track cycles and arrays: returns undefined when the value is neither.
                function serializeCycleOrArray(v, d, s) {
                    if (s.has(v))
                        return '[Circular]';
                    s.add(v);
                    if (Array.isArray(v))
                        return v.slice(0, 100).map((item) => serialize(item, d + 1, s));
                    return undefined;
                }
                // Recognized non-plain objects. Wrapped so toJSON() returning undefined is preserved.
                function serializeSpecialValue(v, d, s) {
                    if (v instanceof Error)
                        return { value: { error: v.message } };
                    if (v instanceof Date)
                        return { value: v.toISOString() };
                    if (v instanceof RegExp)
                        return { value: String(v) };
                    // DOM node duck-type check (works across worlds)
                    if ('nodeType' in v && 'nodeName' in v) {
                        const node = v;
                        return { value: `[${node.nodeName}${node.id ? '#' + node.id : ''}]` };
                    }
                    // Browser host objects (DOMRect, DOMPoint, DOMMatrix) have prototype getters
                    // that Object.keys() misses. Their toJSON() returns a plain object.
                    if (typeof v.toJSON === 'function') {
                        try {
                            return { value: serialize(v.toJSON(), d + 1, s) };
                        }
                        catch {
                            // EXPECTED_ABSENCE: optional enrichment can normally fail while the primary
                            // operation keeps a valid fallback; logging it would misleadingly report fallback as failure.
                            // Fall through to Object.keys() enumeration
                        }
                    }
                    return null;
                }
                function readHostPrimitive(v, key) {
                    try {
                        const propValue = v[key];
                        const valueType = typeof propValue;
                        if (propValue === undefined || valueType === 'function')
                            return null;
                        if (valueType === 'string' || valueType === 'number' || valueType === 'boolean' || propValue === null) {
                            return { value: propValue };
                        }
                        return null;
                    }
                    catch {
                        // EXPECTED_ABSENCE: optional enrichment can normally fail while the primary
                        // operation keeps a valid fallback; logging it would misleadingly report fallback as failure.
                        // Ignore getter access errors.
                        return null;
                    }
                }
                // #389: Some host objects expose data only via prototype getters
                // (DOMRect/CSSStyleDeclaration-like values). If no enumerable keys exist,
                // introspect prototype property names and capture primitive getter values.
                function serializeHostObject(v) {
                    try {
                        const proto = Object.getPrototypeOf(v);
                        if (proto && proto !== Object.prototype) {
                            const hostResult = {};
                            const propNames = Object.getOwnPropertyNames(proto).slice(0, 120);
                            for (const key of propNames) {
                                if (key === 'constructor')
                                    continue;
                                const primitive = readHostPrimitive(v, key);
                                if (primitive)
                                    hostResult[key] = primitive.value;
                                if (Object.keys(hostResult).length >= 50)
                                    break;
                            }
                            if (Object.keys(hostResult).length > 0)
                                return hostResult;
                        }
                    }
                    catch {
                        // EXPECTED_ABSENCE: optional enrichment can normally fail while the primary
                        // operation keeps a valid fallback; logging it would misleadingly report fallback as failure.
                        // Fall through to default object key enumeration.
                    }
                    return undefined;
                }
                function serializePlainObject(v, keys, d, s) {
                    const result = {};
                    for (const key of keys) {
                        try {
                            result[key] = serialize(v[key], d + 1, s);
                        }
                        catch {
                            result[key] = '[unserializable]';
                        }
                    }
                    return result;
                }
                if (depth > 10)
                    return '[max depth]';
                if (value === null || value === undefined)
                    return value;
                const t = typeof value;
                if (t === 'string' || t === 'number' || t === 'boolean')
                    return value;
                if (t === 'function')
                    return '[Function]';
                if (t === 'symbol')
                    return String(value);
                if (t !== 'object')
                    return String(value);
                const obj = value;
                const arrayOrCycle = serializeCycleOrArray(obj, depth, seen);
                if (arrayOrCycle !== undefined)
                    return arrayOrCycle;
                const special = serializeSpecialValue(obj, depth, seen);
                if (special)
                    return special.value;
                const keys = Object.keys(obj).slice(0, 50);
                if (keys.length === 0) {
                    const host = serializeHostObject(obj);
                    if (host)
                        return host;
                }
                return serializePlainObject(obj, keys, depth, seen);
            }
        },
        args: [script]
    });
    try {
        const results = await Promise.race([executionPromise, timeoutPromise]);
        const firstResult = results?.[0]?.result;
        if (firstResult && typeof firstResult === 'object') {
            return firstResult;
        }
        return { success: false, error: 'no_result', message: 'chrome.scripting.executeScript produced no result' };
    }
    catch (err) {
        const msg = errorMessage(err) || '';
        if (msg.includes('timeout')) {
            return { success: false, error: 'execution_timeout', message: msg };
        }
        return { success: false, error: 'scripting_api_error', message: msg };
    }
}
// =============================================================================
// CSP-SAFE STRUCTURED EXECUTION (tier 3)
// =============================================================================
/**
 * Execute JavaScript by parsing it into a structured command and running
 * a pre-compiled executor function via chrome.scripting.executeScript.
 * This bypasses CSP because no string-to-code conversion happens.
 */
async function executeViaStructuredCommand(tabId, script, timeoutMs, world = 'MAIN') {
    const modeTag = world === 'ISOLATED' ? 'isolated_structured' : 'csp_safe_structured';
    const parseResult = parseExpression(script);
    if (!parseResult.ok) {
        return {
            success: false,
            error: 'csp_blocked_unparseable',
            message: `This expression cannot be converted to a structured command: ${parseResult.reason}. ` +
                'Only simple property access, method calls, and assignments are supported. ' +
                'Use interact DOM primitives (click, type, get_text, get_attribute, list_interactive) instead.',
            execution_mode: modeTag
        };
    }
    const timeoutPromise = new Promise((_, reject) => {
        setTimeout(() => reject(new Error(`Structured execution exceeded ${timeoutMs}ms timeout`)), timeoutMs + 2000);
    });
    const executionPromise = chrome.scripting.executeScript({
        target: { tabId },
        world: world,
        func: cspSafeExecutor,
        args: [parseResult.command]
    });
    try {
        const results = await Promise.race([executionPromise, timeoutPromise]);
        const firstResult = results?.[0]?.result;
        if (firstResult && typeof firstResult === 'object') {
            const execResult = firstResult;
            return { ...execResult, execution_mode: modeTag };
        }
        return {
            success: false,
            error: 'no_result',
            message: 'Structured executor produced no result',
            execution_mode: modeTag
        };
    }
    catch (err) {
        const msg = errorMessage(err) || '';
        if (msg.includes('timeout')) {
            return { success: false, error: 'execution_timeout', message: msg, execution_mode: modeTag };
        }
        return { success: false, error: 'structured_execution_error', message: msg, execution_mode: modeTag };
    }
}
/** Auto-fallback routing for a failed content-script result in "auto" world mode. */
async function routeAutoFallback(tabId, result, script, timeoutMs) {
    // CSP errors → structured executor in MAIN world
    if (result.error === 'csp_blocked') {
        debugLog(DebugCategory.CONNECTION, 'CSP fallback: structured executor in MAIN world', { tabId });
        return executeViaStructuredCommand(tabId, script, timeoutMs, 'MAIN');
    }
    // Inject not loaded/responding → try MAIN world via scripting API
    if (result.error === 'inject_not_loaded' || result.error === 'inject_not_responding') {
        debugLog(DebugCategory.CONNECTION, 'Auto-fallback to chrome.scripting API (MAIN)', {
            error: result.error,
            tabId
        });
        return executeViaScriptingAPI(tabId, script, timeoutMs, 'MAIN');
    }
    return result;
}
/** Recover when chrome.tabs.sendMessage throws (content script not reachable). */
async function recoverFromTabSendError(tabId, err, world, script, timeoutMs) {
    let message = errorMessage(err, 'Tab communication failed');
    // Auto-fallback: content script not reachable — try scripting API MAIN, then structured
    if (world === 'auto' && message.includes('Receiving end does not exist')) {
        debugLog(DebugCategory.CONNECTION, 'Auto-fallback (content script unreachable)', { tabId });
        const mainResult = await executeViaScriptingAPI(tabId, script, timeoutMs, 'MAIN');
        if (mainResult.success)
            return mainResult;
        if (mainResult.error === 'csp_blocked_all_worlds') {
            return executeViaStructuredCommand(tabId, script, timeoutMs, 'MAIN');
        }
        return mainResult;
    }
    if (message.includes('Receiving end does not exist')) {
        message =
            'Content script not loaded. REQUIRED ACTION: Refresh the page first using this command:\n\ninteract({what: "refresh"})\n\nThen retry your command.';
    }
    return { success: false, error: 'content_script_not_loaded', message };
}
/**
 * Execute JS with world-aware routing.
 * - isolated: structured executor in ISOLATED world (skips new Function — always fails in MV3)
 * - main: send to content script (MAIN world via inject)
 * - auto: try content script → scripting API MAIN → structured executor MAIN
 */
export async function executeWithWorldRouting(tabId, queryParams, world) {
    let parsedParams;
    try {
        parsedParams = typeof queryParams === 'string' ? JSON.parse(queryParams) : queryParams;
    }
    catch {
        parsedParams = {};
    }
    const script = parsedParams.script || '';
    const timeoutMs = parsedParams.timeout_ms || scaleTimeout(5000);
    // ISOLATED: go directly to structured executor — new Function() always fails in MV3's ISOLATED world
    if (world === 'isolated') {
        return executeViaStructuredCommand(tabId, script, timeoutMs, 'ISOLATED');
    }
    // MAIN or AUTO: try content script (MAIN world) first
    try {
        const result = (await chrome.tabs.sendMessage(tabId, {
            type: 'kaboom_execute_query',
            params: queryParams
        }));
        // Auto-fallback: split by error type
        if (world === 'auto' && result && !result.success) {
            return routeAutoFallback(tabId, result, script, timeoutMs);
        }
        return result;
    }
    catch (err) {
        return recoverFromTabSendError(tabId, err, world, script, timeoutMs);
    }
}
//# sourceMappingURL=query-execution.js.map