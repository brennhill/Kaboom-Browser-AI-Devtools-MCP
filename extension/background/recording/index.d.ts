/**
 * Purpose: Manages recording lifecycle (start/stop) and recording state, delegating capture plumbing and listener registration to sub-modules.
 * Docs: docs/features/feature/flow-recording/index.md
 */
import { type RecordingStartContext } from './utils.js';
/**
 * Kick off recording rehydration. Call once from background init.
 *
 * MV3 service workers restart routinely while the offscreen MediaRecorder keeps
 * recording, so on startup we ask the offscreen document whether a recording is
 * still active: rehydrate state + badge timer if so, otherwise clear stale
 * storage (e.g. a browser crash during a previous recording).
 *
 * This is an explicit call, NOT a module-load side effect: importing this module
 * merely to reach isRecording()/startRecording() must not fire chrome messaging
 * and a storage read (which forced every test to stub chrome before import, and
 * re-ran rehydration for every importer). Only initializeExtension() calls it.
 */
export declare function initRecording(): Promise<void>;
/** Returns whether a recording is currently active. */
export declare function isRecording(): boolean;
/** Returns current recording info for popup sync. */
export declare function getRecordingInfo(): {
    active: boolean;
    name: string;
    startTime: number;
};
/**
 * Start recording a target tab (or the active tab when no target is provided).
 * @param name — Pre-generated filename from the Go server (e.g., "checkout-bug--2026-02-07-1423")
 * @param fps — Framerate (5–60, default 15)
 * @param audio — Audio mode: 'tab', 'mic', 'both', or '' (no audio)
 * @param context — Request context: query resolution, origin, target tab, generation guard
 */
export declare function startRecording(name: string, fps?: number, audio?: string, context?: RecordingStartContext): Promise<{
    status: string;
    name: string;
    startTime?: number;
    error?: string;
}>;
/**
 * Stop recording and save the video.
 * @param truncated — true if auto-stopped due to memory guard or tab close
 */
export declare function stopRecording(truncated?: boolean, connectionGeneration?: number): Promise<{
    status: string;
    name: string;
    duration_seconds?: number;
    size_bytes?: number;
    truncated?: boolean;
    path?: string;
    error?: string;
}>;
//# sourceMappingURL=index.d.ts.map