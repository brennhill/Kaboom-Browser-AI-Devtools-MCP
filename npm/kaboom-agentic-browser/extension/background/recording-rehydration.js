/**
 * Purpose: Decides whether in-memory recording state should be rehydrated after a service-worker restart.
 * Why: MV3 service workers restart routinely while the offscreen MediaRecorder keeps recording;
 *      the offscreen document is the source of truth for "is a recording still active".
 * Docs: docs/features/feature/tab-recording/index.md
 */
/**
 * Decide whether an active recording survived a service-worker restart.
 * Returns the rehydrated state when the offscreen document reports an active
 * recording (preferring live offscreen values, falling back to persisted
 * metadata), otherwise null — in which case the caller should clear stale
 * persisted recording state.
 */
export async function resolveRecordingRehydration(deps) {
    const offscreen = await deps.queryOffscreenRecordingState();
    if (!offscreen?.active)
        return null;
    let persisted = null;
    try {
        persisted = await deps.getPersistedRecording();
    }
    catch {
        persisted = null;
    }
    return {
        active: true,
        name: offscreen.name || persisted?.name || '',
        startTime: offscreen.startTime || persisted?.startTime || Date.now(),
        fps: offscreen.fps || persisted?.fps || 15,
        audioMode: offscreen.audioMode || persisted?.audioMode || '',
        tabId: offscreen.tabId || persisted?.tabId || 0,
        url: offscreen.url || persisted?.url || '',
        queryId: persisted?.queryId ?? ''
    };
}
//# sourceMappingURL=recording-rehydration.js.map