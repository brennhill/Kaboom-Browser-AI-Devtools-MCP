/**
 * Purpose: Applies runtime setting changes (network capture, performance marks, WebSocket mode, action replay) and handles state save/load commands in the inject context.
 * Docs: docs/features/feature/state-time-travel/index.md
 */
import type { BrowserStateSnapshot, StateAction } from '../types/index.js';
/**
 * Valid setting names from content script — imported from canonical constants.
 */
export declare const VALID_SETTINGS: ReadonlySet<string>;
/**
 * Setting message from content script
 */
export interface SettingMessageData {
    type: 'kaboom_setting';
    setting: string;
    enabled?: boolean;
    mode?: string;
    url?: string;
}
/**
 * State command message from content script
 */
export interface StateCommandMessageData {
    type: 'kaboom_state_command';
    messageId: string;
    action: StateAction;
    state?: BrowserStateSnapshot;
    include_url?: boolean;
}
export declare function isValidSettingPayload(data: SettingMessageData): boolean;
export declare function handleSetting(data: SettingMessageData): void;
export declare function handleStateCommand(data: StateCommandMessageData, captureStateFn: () => BrowserStateSnapshot, restoreStateFn: (state: BrowserStateSnapshot, includeUrl: boolean) => unknown): void;
//# sourceMappingURL=settings.d.ts.map