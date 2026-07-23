/**
 * Purpose: Acquires tab capture streams, manages offscreen documents, and handles user gesture flow for video recording.
 * Docs: docs/features/feature/flow-recording/index.md
 */
/** Ensure the offscreen document exists for recording. */
export declare function ensureOffscreenDocument(): Promise<void>;
/**
 * Get a media stream ID, recovering from "active stream" errors by closing the
 * stale offscreen document (which releases leaked streams) and retrying once.
 */
export declare function getStreamIdWithRecovery(tabId: number): Promise<string>;
/**
 * Request user gesture for recording permission (used for MCP-initiated recordings).
 * Shows a toast prompting the user to open the Kaboom popup and approve.
 */
export declare function requestRecordingGesture(tab: chrome.tabs.Tab, name: string, fps: number, audio: string, mediaType: string): Promise<{
    status: string;
    name: string;
    error?: string;
}>;
//# sourceMappingURL=recording-capture.d.ts.map