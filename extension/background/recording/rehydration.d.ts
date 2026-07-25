/**
 * Purpose: Decides whether in-memory recording state should be rehydrated after a service-worker restart.
 * Why: MV3 service workers restart routinely while the offscreen MediaRecorder keeps recording;
 *      the offscreen document is the source of truth for "is a recording still active".
 * Docs: docs/features/feature/tab-recording/index.md
 */
import type { OffscreenRecordingStateResponse } from '../../types/runtime-messages.js';
/** Recording metadata persisted under StorageKey.RECORDING at recording start. */
export interface PersistedRecordingState {
    active?: boolean;
    name?: string;
    startTime?: number;
    fps?: number;
    audioMode?: string;
    tabId?: number;
    url?: string;
    queryId?: string;
}
/** Fully-populated recording state restored after a service-worker restart. */
export interface RehydratedRecordingState {
    active: true;
    name: string;
    startTime: number;
    fps: number;
    audioMode: string;
    tabId: number;
    url: string;
    queryId: string;
}
export interface RecordingRehydrationDeps {
    /** Ask the offscreen document for its live recording state. Resolves null when unreachable. */
    queryOffscreenRecordingState: () => Promise<OffscreenRecordingStateResponse | null>;
    /** Read persisted recording metadata (StorageKey.RECORDING). Resolves null when absent. */
    getPersistedRecording: () => Promise<PersistedRecordingState | null>;
}
/**
 * Decide whether an active recording survived a service-worker restart.
 * Returns the rehydrated state when the offscreen document reports an active
 * recording (preferring live offscreen values, falling back to persisted
 * metadata), otherwise null — in which case the caller should clear stale
 * persisted recording state.
 */
export declare function resolveRecordingRehydration(deps: RecordingRehydrationDeps): Promise<RehydratedRecordingState | null>;
//# sourceMappingURL=rehydration.d.ts.map