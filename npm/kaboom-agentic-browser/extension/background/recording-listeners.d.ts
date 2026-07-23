/**
 * Purpose: Installs Chrome runtime message listeners for recording start/stop, auto-stop from offscreen memory guard, and mic permission flow.
 * Docs: docs/features/feature/flow-recording/index.md
 */
import type { ScreenRecordingHandlers } from './keyboard-shortcuts.js';
/** Dependencies injected by recording.ts to avoid circular imports. */
export interface RecordingListenerDeps extends Omit<ScreenRecordingHandlers, 'isRecording'> {
    isActive: () => boolean;
    getTabId: () => number;
    setInactive: () => void;
    clearRecordingState: () => Promise<void>;
    getServerUrl: () => string;
}
/**
 * Install all chrome.runtime.onMessage listeners for recording.
 * Must be called once at module load time, guarded by chrome runtime availability.
 */
export declare function installRecordingListeners(deps: RecordingListenerDeps): void;
//# sourceMappingURL=recording-listeners.d.ts.map