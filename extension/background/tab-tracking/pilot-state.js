/**
 * Purpose: Own AI Web Pilot cache initialization and pending initialization callback.
 */
let enabled = true;
let initialized = false;
let pendingInit = null;
export function isAiWebPilotEnabled() {
    return enabled;
}
export function setAiWebPilotEnabledCache(value) {
    enabled = value;
}
export function isAiWebPilotCacheInitialized() {
    return initialized;
}
export function setAiWebPilotCacheInitialized(value) {
    initialized = value;
}
export function getPilotInitCallback() {
    return pendingInit;
}
export function setPilotInitCallback(callback) {
    pendingInit = callback;
}
export function resetPilotCacheForTesting(value = false) {
    enabled = value;
}
//# sourceMappingURL=pilot-state.js.map