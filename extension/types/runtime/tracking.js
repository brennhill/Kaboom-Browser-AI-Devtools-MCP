/**
 * Purpose: Defines tracked-tab continuity phases and runtime message contracts.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
const TRACKING_PHASES = new Set([
    'idle',
    'confirmed',
    'navigation_started',
    'provisional_url',
    'content_injecting',
    'extension_reconnecting',
    'recovery_failed'
]);
function isRecord(value) {
    return typeof value === 'object' && value !== null;
}
function isTrackingState(value) {
    return isRecord(value) && typeof value.isTracked === 'boolean' && typeof value.aiPilotEnabled === 'boolean';
}
export function isTrackingStateChangedMessage(value) {
    return isRecord(value) && value.type === 'tracking_state_changed' && isTrackingState(value.state);
}
export function isGetTrackingStateResponse(value) {
    if (!isRecord(value) || !isRecord(value.state) || !isTrackingState(value.state))
        return false;
    const continuity = value.state.continuity;
    return (isRecord(continuity) &&
        typeof continuity.phase === 'string' &&
        TRACKING_PHASES.has(continuity.phase) &&
        typeof continuity.is_tracked === 'boolean');
}
//# sourceMappingURL=tracking.js.map