/**
 * Purpose: Chrome context menu installation and click handlers for Kaboom actions.
 * Split from event-listeners.ts to keep files under 800 LOC.
 */
import type { ScreenRecordingHandlers, RecordingShortcutHandlers } from './keyboard-shortcuts.js';
/**
 * Create context menu items for Kaboom actions.
 * Chrome auto-groups multiple items under a parent with the extension icon.
 */
export declare function installContextMenus(recordingHandlers: ScreenRecordingHandlers, actionRecordingHandlers: RecordingShortcutHandlers, logFn?: (message: string) => void): void;
//# sourceMappingURL=context-menus.d.ts.map