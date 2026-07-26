/**
 * Purpose: Implements the circuit breaker pattern with exponential backoff to protect server communication from cascading failures.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
/**
 * Circuit breaker with exponential backoff for server communication.
 * Prevents the extension from hammering a down/slow server.
 */
export function createCircuitBreaker(sendFn, options = {}) {
    const maxFailures = options.maxFailures ?? 5;
    const resetTimeout = options.resetTimeout ?? 30000;
    const initialBackoff = options.initialBackoff ?? 1000;
    const maxBackoff = options.maxBackoff ?? 30000;
    let state = 'closed';
    let consecutiveFailures = 0;
    let totalFailures = 0;
    let totalSuccesses = 0;
    let currentBackoff = 0;
    let lastFailureTime = 0;
    let probeInFlight = false;
    let lastResetReason = null;
    const stateChangeCallbacks = [];
    const transitionHistory = [];
    const maxHistorySize = 20;
    // Add initial callback if provided
    if (options.onStateChange) {
        stateChangeCallbacks.push(options.onStateChange);
    }
    function recordTransition(from, to, reason) {
        if (from === to)
            return;
        transitionHistory.push({ from, to, reason, timestamp: Date.now() });
        if (transitionHistory.length > maxHistorySize) {
            transitionHistory.shift();
        }
        // Notify callbacks
        for (const callback of stateChangeCallbacks) {
            try {
                callback(from, to, reason);
            }
            catch (err) {
                console.error('[CircuitBreaker] State change callback error:', err);
            }
        }
    }
    function getState() {
        const oldState = state;
        if (state === 'open' && Date.now() - lastFailureTime >= resetTimeout) {
            state = 'half-open';
            recordTransition(oldState, state, 'reset_timeout_elapsed');
        }
        return state;
    }
    // #lizard forgives
    function getStats() {
        return {
            state: getState(),
            consecutiveFailures,
            totalFailures,
            totalSuccesses,
            currentBackoff
        };
    }
    function getExtendedStats() {
        return {
            ...getStats(),
            lastFailureTime,
            lastResetReason,
            transitionHistory: [...transitionHistory]
        };
    }
    function reset(reason = 'manual_reset') {
        const oldState = state;
        state = 'closed';
        consecutiveFailures = 0;
        currentBackoff = 0;
        probeInFlight = false;
        lastResetReason = reason;
        recordTransition(oldState, 'closed', reason);
        console.log(`[CircuitBreaker] Reset: ${reason}`);
    }
    function onSuccess() {
        const oldState = state;
        consecutiveFailures = 0;
        currentBackoff = 0;
        totalSuccesses++;
        state = 'closed';
        probeInFlight = false;
        if (oldState !== 'closed') {
            recordTransition(oldState, 'closed', 'request_success');
        }
    }
    function onFailure() {
        const oldState = state;
        consecutiveFailures++;
        totalFailures++;
        lastFailureTime = Date.now();
        probeInFlight = false;
        if (consecutiveFailures >= maxFailures && state !== 'open') {
            state = 'open';
            recordTransition(oldState, 'open', `consecutive_failures_${consecutiveFailures}`);
        }
        if (consecutiveFailures > 1) {
            currentBackoff = Math.min(initialBackoff * Math.pow(2, consecutiveFailures - 2), maxBackoff);
        }
        else {
            currentBackoff = 0;
        }
    }
    async function execute(args) {
        const currentState = getState();
        if (currentState === 'open') {
            throw new Error(`Server connection blocked: circuit breaker is open after ${consecutiveFailures} failures. Retrying in ${currentBackoff}ms.`);
        }
        if (currentState === 'half-open') {
            if (probeInFlight) {
                throw new Error('Server connection blocked: circuit breaker testing connection recovery');
            }
            probeInFlight = true;
        }
        if (currentBackoff > 0) {
            await new Promise((r) => {
                setTimeout(r, currentBackoff);
            });
        }
        try {
            const result = (await sendFn(args));
            onSuccess();
            return result;
        }
        catch (err) {
            onFailure();
            throw err;
        }
    }
    function recordFailure() {
        const oldState = state;
        consecutiveFailures++;
        totalFailures++;
        lastFailureTime = Date.now();
        if (consecutiveFailures >= maxFailures && state !== 'open') {
            state = 'open';
            recordTransition(oldState, 'open', `consecutive_failures_${consecutiveFailures}`);
        }
        currentBackoff =
            consecutiveFailures >= 2 ? Math.min(initialBackoff * Math.pow(2, consecutiveFailures - 2), maxBackoff) : 0;
    }
    function onStateChange(callback) {
        stateChangeCallbacks.push(callback);
        return () => {
            const index = stateChangeCallbacks.indexOf(callback);
            if (index > -1)
                stateChangeCallbacks.splice(index, 1);
        };
    }
    return { execute, getState, getStats, getExtendedStats, reset, recordFailure, onStateChange };
}
//# sourceMappingURL=circuit-breaker.js.map