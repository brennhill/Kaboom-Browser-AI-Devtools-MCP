/**
 * Purpose: Chrome runtime messaging, storage, and mic permission logic for recording controls.
 * Why: Separates browser API side-effects from recording UI rendering.
 * Docs: docs/features/feature/playback-engine/index.md
 * Docs: docs/features/feature/tab-recording/index.md
 */
export interface RecordingElements {
    row: HTMLElement;
    label: HTMLElement;
    statusEl: HTMLElement;
    optionsEl: HTMLElement | null;
    saveInfoEl: HTMLElement | null;
    topNoticeEl: HTMLElement | null;
}
export interface RecordingState {
    isRecording: boolean;
    timerInterval: ReturnType<typeof setInterval> | null;
}
export type ShowRecordingFn = (els: RecordingElements, state: RecordingState, name: string, startTime: number) => void;
export type ShowIdleFn = (els: RecordingElements, state: RecordingState) => void;
export type ShowStartErrorFn = (saveInfoEl: HTMLElement | null, errorText: string) => void;
export declare function sendRecordingGestureDecision(type: 'recording_gesture_granted' | 'recording_gesture_denied'): void;
export declare function handleStartClick(els: RecordingElements, state: RecordingState, showRecording: ShowRecordingFn, showIdle: ShowIdleFn, showStartError: ShowStartErrorFn): void;
export declare function handleStopClick(els: RecordingElements, state: RecordingState, showIdle: ShowIdleFn, showSaveResult: (saveInfoEl: HTMLElement | null, resp: {
    status?: string;
    name?: string;
    path?: string;
    error?: string;
} | undefined) => void): void;
/** Screen-recording state as shown by the popup (mirror or live owner). */
export interface PopupRecordingSnapshot {
    readonly active: boolean;
    readonly name: string;
    readonly startTime: number;
}
/**
 * Ask the background recording owner for the live state (rule 18: the storage
 * mirror goes stale if its writer dies without flushing). Returns null when the
 * owner cannot answer, so the caller keeps the mirror instead of guessing.
 */
export declare function queryLiveRecordingInfo(): Promise<PopupRecordingSnapshot | null>;
/** The live owner's answer wins; without one, the mirror stands. */
export declare function reconcileRecordingState(mirror: PopupRecordingSnapshot | null, live: PopupRecordingSnapshot | null): PopupRecordingSnapshot | null;
//# sourceMappingURL=recording-io.d.ts.map