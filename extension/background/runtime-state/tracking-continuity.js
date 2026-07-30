/**
 * Purpose: Own the tracked-tab continuity state machine across navigation and reinjection.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
export class TrackingContinuity {
    state = { phase: 'idle', is_tracked: false };
    listeners = new Set();
    snapshot() {
        return { ...this.state };
    }
    subscribe(listener) {
        this.listeners.add(listener);
        return () => this.listeners.delete(listener);
    }
    establish(tabId, url) {
        if (!this.canOwn(tabId))
            return;
        this.setConfirmed(tabId, url);
    }
    confirm(tabId, url) {
        if (this.state.tab_id !== tabId)
            return;
        this.setConfirmed(tabId, url);
    }
    setConfirmed(tabId, url) {
        this.transition({
            tab_id: tabId,
            phase: 'confirmed',
            is_tracked: true,
            ...(url ? { confirmed_url: url } : {})
        });
    }
    navigationStarted(tabId) {
        this.forTrackedTab(tabId, (current) => ({
            ...current,
            phase: 'navigation_started',
            failure: undefined
        }));
    }
    observeProvisionalURL(tabId, url) {
        this.forTrackedTab(tabId, (current) => ({
            ...current,
            phase: 'provisional_url',
            provisional_url: url,
            failure: undefined
        }));
    }
    injectionStarted(tabId) {
        this.forTrackedTab(tabId, (current) => {
            if (current.phase !== 'navigation_started' &&
                current.phase !== 'provisional_url' &&
                current.phase !== 'extension_reconnecting') {
                return current;
            }
            return { ...current, phase: 'content_injecting', failure: undefined };
        });
    }
    extensionReconnectStarted(tabId) {
        this.forTrackedTab(tabId, (current) => ({
            ...current,
            phase: 'extension_reconnecting',
            failure: undefined
        }));
    }
    fail(tabId, failure) {
        this.forTrackedTab(tabId, (current) => ({
            ...current,
            phase: 'recovery_failed',
            failure
        }));
    }
    close(tabId) {
        if (this.state.tab_id !== tabId)
            return;
        this.transition({ phase: 'idle', is_tracked: false });
    }
    canOwn(tabId) {
        return Number.isInteger(tabId) && tabId > 0 && (this.state.tab_id === undefined || this.state.tab_id === tabId);
    }
    forTrackedTab(tabId, update) {
        if (this.state.tab_id !== tabId)
            return;
        const next = update(this.state);
        if (next === this.state)
            return;
        this.transition(next);
    }
    transition(next) {
        this.state = withoutUndefined(next);
        const snapshot = this.snapshot();
        for (const listener of this.listeners)
            listener(snapshot);
    }
}
function withoutUndefined(snapshot) {
    return Object.fromEntries(Object.entries(snapshot).filter(([, value]) => value !== undefined));
}
export const trackingContinuity = new TrackingContinuity();
//# sourceMappingURL=tracking-continuity.js.map