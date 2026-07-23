/**
 * Purpose: Implements debounced batching with circuit breaker integration for rate-limited server requests.
 * Why: Prevents connection storms and provides backoff when the MCP server is unavailable.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import { createCircuitBreaker } from './circuit-breaker.js';
import { MAX_PENDING_BUFFER } from './state-manager.js';
const DEFAULT_DEBOUNCE_MS = 100;
const DEFAULT_MAX_BATCH_SIZE = 50;
/** Rate limit configuration */
export const RATE_LIMIT_CONFIG = {
    maxFailures: 5,
    resetTimeout: 30000,
    backoffSchedule: [100, 500, 2000],
    retryBudget: 3
};
/**
 * Creates a batcher wired with circuit breaker logic for rate limiting.
 */
export function createBatcherWithCircuitBreaker(sendFn, options = {}) {
    const debounceMs = options.debounceMs ?? DEFAULT_DEBOUNCE_MS;
    const maxBatchSize = options.maxBatchSize ?? DEFAULT_MAX_BATCH_SIZE;
    const retryBudget = options.retryBudget ?? RATE_LIMIT_CONFIG.retryBudget;
    const maxFailures = options.maxFailures ?? RATE_LIMIT_CONFIG.maxFailures;
    const resetTimeout = options.resetTimeout ?? RATE_LIMIT_CONFIG.resetTimeout;
    const backoffSchedule = RATE_LIMIT_CONFIG.backoffSchedule;
    const localConnectionStatus = { connected: true };
    const isSharedCB = !!options.sharedCircuitBreaker;
    const cb = options.sharedCircuitBreaker ||
        createCircuitBreaker(sendFn, {
            maxFailures,
            resetTimeout,
            initialBackoff: 0,
            maxBackoff: 0
        });
    function getScheduledBackoff(failures) {
        if (failures <= 0)
            return 0;
        const idx = Math.min(failures - 1, backoffSchedule.length - 1);
        return backoffSchedule[idx];
    }
    const wrappedCircuitBreaker = {
        getState: () => cb.getState(),
        getStats: () => {
            const stats = cb.getStats();
            return {
                ...stats,
                currentBackoff: getScheduledBackoff(stats.consecutiveFailures)
            };
        },
        reset: () => cb.reset()
    };
    async function attemptSend(entries) {
        if (!isSharedCB) {
            return await cb.execute(entries);
        }
        const state = cb.getState();
        if (state === 'open') {
            const stats = cb.getStats();
            throw new Error(`Cannot send batch: circuit breaker is open after ${stats.consecutiveFailures} consecutive failures. Will retry automatically.`);
        }
        try {
            const result = await sendFn(entries);
            cb.reset();
            return result;
        }
        catch (err) {
            cb.recordFailure();
            throw err;
        }
    }
    let pending = [];
    let timeoutId = null;
    function requeueEntries(entries) {
        pending = entries.concat(pending).slice(0, MAX_PENDING_BUFFER);
    }
    async function retryWithBackoff(entries) {
        let retriesLeft = retryBudget - 1;
        while (retriesLeft > 0) {
            retriesLeft--;
            const stats = cb.getStats();
            const backoff = getScheduledBackoff(stats.consecutiveFailures);
            if (backoff > 0) {
                await new Promise((r) => {
                    setTimeout(r, backoff);
                });
            }
            try {
                await attemptSend(entries);
                localConnectionStatus.connected = true;
                return;
            }
            catch {
                localConnectionStatus.connected = false;
                if (cb.getState() === 'open') {
                    requeueEntries(entries);
                    return;
                }
            }
        }
    }
    async function flushWithCircuitBreaker() {
        if (pending.length === 0)
            return;
        const entries = pending;
        pending = [];
        if (timeoutId) {
            clearTimeout(timeoutId);
            timeoutId = null;
        }
        if (cb.getState() === 'open') {
            requeueEntries(entries);
            return;
        }
        try {
            await attemptSend(entries);
            localConnectionStatus.connected = true;
        }
        catch {
            localConnectionStatus.connected = false;
            if (cb.getState() === 'open') {
                requeueEntries(entries);
                return;
            }
            await retryWithBackoff(entries);
        }
    }
    const scheduleFlush = () => {
        if (timeoutId)
            return;
        timeoutId = setTimeout(() => {
            timeoutId = null;
            flushWithCircuitBreaker();
        }, debounceMs);
    };
    const batcher = {
        add(entry) {
            if (pending.length >= MAX_PENDING_BUFFER)
                return;
            pending.push(entry);
            if (pending.length >= maxBatchSize) {
                flushWithCircuitBreaker();
            }
            else {
                scheduleFlush();
            }
        },
        async flush() {
            await flushWithCircuitBreaker();
        },
        clear() {
            pending = [];
            if (timeoutId) {
                clearTimeout(timeoutId);
                timeoutId = null;
            }
        },
        getPending() {
            return [...pending];
        }
    };
    return {
        batcher,
        circuitBreaker: wrappedCircuitBreaker,
        getConnectionStatus: () => ({ ...localConnectionStatus })
    };
}
/**
 * Create a simple log batcher without circuit breaker
 */
export function createLogBatcher(flushFn, options = {}) {
    const debounceMs = options.debounceMs ?? DEFAULT_DEBOUNCE_MS;
    const maxBatchSize = options.maxBatchSize ?? DEFAULT_MAX_BATCH_SIZE;
    const memoryPressureGetter = options.memoryPressureGetter ?? null;
    let pending = [];
    let timeoutId = null;
    const getEffectiveMaxBatchSize = () => {
        if (memoryPressureGetter) {
            const state = memoryPressureGetter();
            if (state.reducedCapacities) {
                return Math.floor(maxBatchSize / 2);
            }
        }
        return maxBatchSize;
    };
    const flush = () => {
        if (pending.length === 0)
            return;
        const entries = pending;
        pending = [];
        if (timeoutId) {
            clearTimeout(timeoutId);
            timeoutId = null;
        }
        flushFn(entries);
    };
    const scheduleFlush = () => {
        if (timeoutId)
            return;
        timeoutId = setTimeout(() => {
            timeoutId = null;
            flush();
        }, debounceMs);
    };
    return {
        add(entry) {
            if (pending.length >= MAX_PENDING_BUFFER)
                return;
            pending.push(entry);
            const effectiveMax = getEffectiveMaxBatchSize();
            if (pending.length >= effectiveMax) {
                flush();
            }
            else {
                scheduleFlush();
            }
        },
        flush() {
            flush();
        },
        clear() {
            pending = [];
            if (timeoutId) {
                clearTimeout(timeoutId);
                timeoutId = null;
            }
        }
    };
}
//# sourceMappingURL=batchers.js.map