/**
 * Purpose: Background script for push delivery — screenshot push, chat push, capability tracking.
 * Why: Enables browser-to-AI message injection via keyboard shortcuts.
 * Docs: docs/features/feature/browser-push/index.md
 */
/** Per-session push capability state from the daemon. */
export interface PushCapabilities {
    push_enabled: boolean;
    supports_sampling: boolean;
    supports_notifications: boolean;
    client_name: string;
    inbox_count: number;
}
/**
 * Install the push_screenshot keyboard shortcut listener.
 * When Alt+Shift+S is pressed, captures the active tab's screenshot
 * and pushes to the daemon.
 */
export declare function installPushCommandListener(logFn?: (message: string) => void): void;
/**
 * Install the push_chat keyboard shortcut listener.
 * When Alt+Shift+C is pressed, sends a message to the content script
 * to show/toggle the chat widget.
 */
export declare function installChatCommandListener(logFn?: (message: string) => void): void;
/**
 * Push a chat message to the daemon's push pipeline.
 */
export declare function pushChatMessage(message: string, pageUrl: string, tabId: number): Promise<{
    status: string;
    event_id?: string;
} | null>;
//# sourceMappingURL=push-handler.d.ts.map