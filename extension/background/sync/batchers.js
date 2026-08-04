/**
 * Purpose: Implements debounced batching with circuit breaker integration for rate-limited server requests.
 * Why: Prevents connection storms and provides backoff when the MCP server is unavailable.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import { createCircuitBreaker } from './circuit-breaker.js';
import { MAX_PENDING_BUFFER } from '../caches/cache-limits.js';
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
    const capacity = options.maxPendingEntries ?? MAX_PENDING_BUFFER;
    const runtime = options.runtime ?? {
        now: Date.now,
        setTimeout: (callback, delay) => {
            const id = setTimeout(callback, delay);
            const nodeTimer = id;
            nodeTimer.unref?.();
            return id;
        },
        clearTimeout: (id) => clearTimeout(id)
    };
    const backoffSchedule = RATE_LIMIT_CONFIG.backoffSchedule;
    const localConnectionStatus = { connected: true };
    const isSharedCB = !!options.sharedCircuitBreaker;
    const cb = options.sharedCircuitBreaker ||
        createCircuitBreaker(sendFn, {
            maxFailures,
            resetTimeout,
            initialBackoff: 0,
            maxBackoff: 0,
            now: runtime.now
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
    let recoveryTimeoutId = null;
    let flushPromise = null;
    let accepted = 0;
    let delivered = 0;
    let dropped = 0;
    let pressureActive = false;
    function reportDrop(reason, count) {
        if (count <= 0)
            return;
        dropped += count;
        pressureActive = true;
        options.onPressure?.({ reason, dropped: count, pending: pending.length, capacity, total_dropped: dropped });
    }
    function resolvePressureAfterDelivery() {
        if (!pressureActive || pending.length >= capacity)
            return;
        pressureActive = false;
        options.onPressureRecovered?.();
    }
    function scheduleRecovery() {
        if (recoveryTimeoutId !== null || pending.length === 0)
            return;
        recoveryTimeoutId = runtime.setTimeout(() => {
            recoveryTimeoutId = null;
            void flushWithCircuitBreaker();
        }, resetTimeout);
    }
    function requeueEntries(entries) {
        const combined = entries.concat(pending);
        pending = combined.slice(0, capacity);
        reportDrop('requeue_overflow', combined.length - pending.length);
        if (cb.getState() === 'open')
            scheduleRecovery();
    }
    async function retryWithBackoff(entries) {
        let retriesLeft = retryBudget - 1;
        while (retriesLeft > 0) {
            retriesLeft--;
            const stats = cb.getStats();
            const backoff = getScheduledBackoff(stats.consecutiveFailures);
            if (backoff > 0) {
                await new Promise((r) => {
                    runtime.setTimeout(r, backoff);
                });
            }
            try {
                await attemptSend(entries);
                delivered += entries.length;
                localConnectionStatus.connected = true;
                resolvePressureAfterDelivery();
                return true;
            }
            catch {
                localConnectionStatus.connected = false;
                if (cb.getState() === 'open') {
                    requeueEntries(entries);
                    return false;
                }
            }
        }
        requeueEntries(entries);
        return false;
    }
    async function runFlushChain() {
        while (pending.length > 0) {
            const shouldContinue = await flushPendingOnce();
            if (!shouldContinue)
                return;
        }
    }
    async function flushPendingOnce() {
        if (pending.length === 0)
            return false;
        const entries = pending;
        pending = [];
        if (timeoutId) {
            runtime.clearTimeout(timeoutId);
            timeoutId = null;
        }
        if (cb.getState() === 'open') {
            requeueEntries(entries);
            return false;
        }
        try {
            await attemptSend(entries);
            delivered += entries.length;
            localConnectionStatus.connected = true;
            resolvePressureAfterDelivery();
            return true;
        }
        catch {
            localConnectionStatus.connected = false;
            if (cb.getState() === 'open') {
                requeueEntries(entries);
                return false;
            }
            return await retryWithBackoff(entries);
        }
    }
    function flushWithCircuitBreaker() {
        if (flushPromise)
            return flushPromise;
        flushPromise = runFlushChain().finally(() => {
            flushPromise = null;
        });
        return flushPromise;
    }
    const scheduleFlush = () => {
        if (timeoutId)
            return;
        timeoutId = runtime.setTimeout(() => {
            timeoutId = null;
            flushWithCircuitBreaker();
        }, debounceMs);
    };
    const batcher = {
        add(entry) {
            accepted++;
            if (pending.length >= capacity) {
                reportDrop('capacity', 1);
                return;
            }
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
            reportDrop('clear', pending.length);
            pending = [];
            if (timeoutId) {
                runtime.clearTimeout(timeoutId);
                timeoutId = null;
            }
            if (recoveryTimeoutId !== null) {
                runtime.clearTimeout(recoveryTimeoutId);
                recoveryTimeoutId = null;
            }
        },
        getPending() {
            return [...pending];
        }
    };
    return {
        batcher,
        circuitBreaker: wrappedCircuitBreaker,
        getConnectionStatus: () => ({ ...localConnectionStatus }),
        getPressureStats: () => ({
            accepted,
            delivered,
            dropped,
            pending: pending.length,
            capacity,
            saturated: dropped > 0
        })
    };
}
//# sourceMappingURL=batchers.js.map