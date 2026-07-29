/**
 * Purpose: Own service-worker session identity and initialization readiness.
 */
export const EXTENSION_SESSION_ID = `ext_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
let resolveInitialization = null;
export const initReady = new Promise((resolve) => {
    resolveInitialization = resolve;
});
export function markInitComplete() {
    resolveInitialization?.();
    resolveInitialization = null;
}
//# sourceMappingURL=startup-state.js.map