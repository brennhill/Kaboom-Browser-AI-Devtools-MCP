/**
 * Purpose: Map-based command registry and dispatch loop that replaces the monolithic if-chain for routing pending queries to handlers.
 * Why: Extensible design lets new command modules register themselves without modifying central dispatch.
 */
import { initReady } from '../state.js';
import { DebugCategory } from '../debug.js';
import { errorMessage } from '../../lib/error-utils.js';
import { debugLog, sendResult, sendAsyncResult, requiresTargetTab, resolveTargetTab, parseQueryParamsObject, withTargetContext, actionToast, isRestrictedUrl, isBrowserEscapeAction } from './helpers.js';
// =============================================================================
// REGISTRY
// =============================================================================
const handlers = new Map();
export function registerCommand(type, handler) {
    handlers.set(type, handler);
}
// =============================================================================
// DISPATCH
// =============================================================================
function canRunOnRestrictedPage(queryType, paramsObj) {
    return isBrowserEscapeAction(queryType, paramsObj);
}
function pickErrorHint(payload, fallback = 'command_failed') {
    if (payload && typeof payload === 'object') {
        const errValue = payload.error;
        if (typeof errValue === 'string' && errValue.length > 0)
            return errValue;
        const msgValue = payload.message;
        if (typeof msgValue === 'string' && msgValue.length > 0)
            return msgValue;
    }
    return fallback;
}
function createDispatchLifecycle(query, syncClient, wrapResult) {
    let terminalSent = false;
    const sendOnce = (fn, metadata) => {
        if (terminalSent) {
            debugLog(DebugCategory.CONNECTION, 'Ignoring duplicate terminal command response', {
                query_id: query.id,
                query_type: query.type,
                correlation_id: query.correlation_id || null,
                ...metadata
            });
            return;
        }
        terminalSent = true;
        fn();
    };
    const sendResultNormalized = (result) => {
        sendOnce(() => {
            const wrapped = wrapResult(result);
            if (query.correlation_id) {
                sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', wrapped);
            }
            else {
                sendResult(syncClient, query.id, wrapped);
            }
        }, { via: 'sendResult' });
    };
    const sendAsyncResultNormalized = (_client, _queryId, correlationId, status, result, error) => {
        sendOnce(() => {
            const wrapped = wrapResult(result);
            if (query.correlation_id) {
                const effectiveCorrelationId = query.correlation_id || correlationId;
                sendAsyncResult(syncClient, query.id, effectiveCorrelationId, status, wrapped, error);
                return;
            }
            if (status === 'complete') {
                sendResult(syncClient, query.id, wrapped);
                return;
            }
            sendResult(syncClient, query.id, {
                success: false,
                status,
                error: error || pickErrorHint(wrapped, 'command_failed'),
                message: error || pickErrorHint(wrapped, 'command_failed'),
                result: wrapped ?? null
            });
        }, { via: 'sendAsyncResult', status });
    };
    const sendError = (payload, errorHint) => {
        if (query.correlation_id) {
            sendAsyncResultNormalized(syncClient, query.id, query.correlation_id, 'error', payload, errorHint || pickErrorHint(payload));
            return;
        }
        sendResultNormalized(payload);
    };
    return {
        sendResult: sendResultNormalized,
        sendAsyncResult: sendAsyncResultNormalized,
        sendError,
        sent: () => terminalSent
    };
}
export async function dispatch(query, syncClient) {
    // Wait for initialization to complete (max 2s) so pilot cache is populated
    await Promise.race([initReady, new Promise((r) => setTimeout(r, 2000))]);
    debugLog(DebugCategory.CONNECTION, 'handlePendingQuery ENTER', {
        id: query.id,
        type: query.type,
        correlation_id: query.correlation_id || null,
        hasSyncClient: !!syncClient
    });
    // Normalize state_* types to a wildcard key
    let queryType = query.type;
    if (queryType.startsWith('state_')) {
        queryType = 'state_*';
    }
    // Target resolution
    let target;
    const paramsObj = parseQueryParamsObject(query.params);
    const needsTarget = requiresTargetTab(query.type);
    const wrapResult = (result) => {
        if (!target)
            return result;
        return withTargetContext(result, target);
    };
    const lifecycle = createDispatchLifecycle(query, syncClient, wrapResult);
    const handler = handlers.get(queryType);
    if (!handler) {
        debugLog(DebugCategory.CONNECTION, 'Unknown query type', { type: query.type });
        lifecycle.sendError({
            error: 'unknown_query_type',
            message: `Unknown query type: ${query.type}`
        }, 'unknown_query_type');
        return;
    }
    if (needsTarget) {
        try {
            const resolved = await resolveTargetTab(query, paramsObj);
            if (resolved.error) {
                lifecycle.sendError(resolved.error.payload, resolved.error.message);
                return;
            }
            target = resolved.target;
        }
        catch (err) {
            const targetErr = errorMessage(err, 'target_resolution_failed');
            lifecycle.sendError({
                success: false,
                error: 'target_resolution_failed',
                message: targetErr
            }, targetErr);
            return;
        }
    }
    const tabId = target?.tabId ?? 0;
    if (needsTarget && !tabId) {
        const payload = {
            success: false,
            error: 'missing_target',
            message: 'No target tab resolved for query'
        };
        lifecycle.sendError(payload, payload.message);
        return;
    }
    // Restricted page detection: content scripts cannot run on internal browser pages
    if (needsTarget && isRestrictedUrl(target?.url) && !canRunOnRestrictedPage(query.type, paramsObj)) {
        const payload = {
            success: false,
            error: 'csp_blocked_page',
            csp_blocked: true,
            failure_cause: 'csp',
            message: 'Extension connected but this page blocks content scripts (common on Google, Chrome Web Store, internal pages). Navigate to a different page first.',
            retryable: false
        };
        lifecycle.sendError(payload, payload.error);
        return;
    }
    const ctx = {
        query,
        syncClient,
        tabId,
        params: paramsObj,
        target,
        sendResult: lifecycle.sendResult,
        sendAsyncResult: lifecycle.sendAsyncResult,
        actionToast
    };
    try {
        await handler(ctx);
        if (!lifecycle.sent()) {
            lifecycle.sendError({
                error: 'no_result',
                message: `Command handler for '${query.type}' completed without sending a terminal result`
            }, 'no_result');
        }
    }
    catch (err) {
        const errMsg = errorMessage(err, 'Unexpected error handling query');
        debugLog(DebugCategory.CONNECTION, 'Error handling pending query', {
            type: query.type,
            id: query.id,
            error: errMsg
        });
        if (!lifecycle.sent()) {
            lifecycle.sendError({ error: 'query_handler_error', message: errMsg }, errMsg);
        }
    }
}
//# sourceMappingURL=registry.js.map