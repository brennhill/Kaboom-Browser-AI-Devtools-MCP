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
//# sourceMappingURL=recording-io.d.ts.map