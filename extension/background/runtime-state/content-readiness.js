/**
 * Purpose: Gates post-navigation content commands on a correlation-matched content-script acknowledgement.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
import { debugLog, DebugCategory } from '../debug.js';
import { reportStateRecovery, resolveStateRecovery } from './state-recovery.js';
import { trackingContinuity } from './tracking-continuity.js';
const DEFAULT_DELAYS_MS = [25, 75, 150];
const CONTENT_COMMANDS = new Set([
    'a11y',
    'dom',
    'dom_action',
    'draw_mode',
    'execute',
    'explore_page',
    'get_markdown',
    'get_readable',
    'highlight',
    'link_health',
    'page_info',
    'page_inventory',
    'page_structure',
    'page_summary',
    'state_capture',
    'state_save',
    'state_load',
    'upload'
]);
export function requiresContentReadiness(queryType) {
    return CONTENT_COMMANDS.has(queryType);
}
export class ContentReadinessBarrier {
    pending = new Map();
    probe;
    wait;
    delaysMs;
    onReady;
    onTimeout;
    constructor(options) {
        this.probe = options.probe;
        this.wait = options.wait;
        this.delaysMs = options.delays_ms ?? DEFAULT_DELAYS_MS;
        this.onReady = options.onReady;
        this.onTimeout = options.onTimeout;
    }
    begin(tabId, correlationId) {
        this.pending.set(tabId, correlationId);
    }
    hasPending(tabId) {
        return this.pending.has(tabId);
    }
    cancel(tabId, correlationId) {
        if (this.pending.get(tabId) === correlationId)
            this.pending.delete(tabId);
    }
    async waitUntilReady(tabId) {
        const correlationId = this.pending.get(tabId);
        if (!correlationId) {
            return {
                ready: false,
                correlation_id: '',
                attempts: 0,
                error: 'readiness_superseded'
            };
        }
        for (let attempt = 1; attempt <= this.delaysMs.length + 1; attempt += 1) {
            const acknowledgement = await this.probe(tabId, correlationId);
            if (this.pending.get(tabId) !== correlationId) {
                return {
                    ready: false,
                    correlation_id: correlationId,
                    attempts: attempt,
                    error: 'readiness_superseded'
                };
            }
            if (acknowledgement?.ready && acknowledgement.correlation_id === correlationId) {
                this.pending.delete(tabId);
                this.onReady?.(tabId, correlationId, attempt);
                return { ready: true, correlation_id: correlationId, attempts: attempt };
            }
            const delayMs = this.delaysMs[attempt - 1];
            if (delayMs !== undefined)
                await this.wait(delayMs);
        }
        const result = {
            ready: false,
            correlation_id: correlationId,
            attempts: this.delaysMs.length + 1,
            error: 'content_readiness_timeout'
        };
        this.onTimeout?.(tabId, correlationId, result.attempts);
        return result;
    }
}
const failedReadinessTabs = new Set();
export const contentReadiness = new ContentReadinessBarrier({
    probe: async (tabId, correlationId) => {
        try {
            return (await chrome.tabs.sendMessage(tabId, {
                type: 'tracking_readiness_probe',
                correlation_id: correlationId
            }));
        }
        catch {
            // EXPECTED_ABSENCE: the content script is normally unavailable while Chrome
            // reinjects it after navigation; logging each bounded probe miss would
            // misrepresent expected recovery progress as an independent failure.
            return undefined;
        }
    },
    wait: (delayMs) => new Promise((resolve) => setTimeout(resolve, delayMs)),
    onReady: (tabId, correlationId, attempts) => {
        trackingContinuity.confirm(tabId);
        if (failedReadinessTabs.delete(tabId))
            resolveStateRecovery('content_readiness_state');
        debugLog(DebugCategory.CONNECTION, 'Content readiness acknowledged', {
            tab_id: tabId,
            correlation_id: correlationId,
            attempts
        });
    },
    onTimeout: (tabId, correlationId, attempts) => {
        failedReadinessTabs.add(tabId);
        trackingContinuity.fail(tabId, 'content_readiness_timeout');
        reportStateRecovery({
            name: 'content_readiness_state',
            detail: `Content readiness failed after ${attempts} correlated attempts.`,
            fix: 'Reload the tracked tab or reconnect the extension, then retry the command.'
        });
        debugLog(DebugCategory.CONNECTION, 'Content readiness transition failed', {
            tab_id: tabId,
            correlation_id: correlationId,
            attempts,
            error: 'content_readiness_timeout'
        });
    }
});
//# sourceMappingURL=content-readiness.js.map