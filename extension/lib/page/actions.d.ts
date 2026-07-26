/**
 * Purpose: Captures and buffers user actions (click, input, scroll, keydown, navigation) with throttling and sensitive data redaction.
 * Docs: docs/features/feature/observe/index.md
 */
interface ActionRecord {
    ts: string;
    type: string;
    target?: string;
    x?: number;
    y?: number;
    text?: string;
    inputType?: string;
    value?: string;
    length?: number;
    scrollX?: number;
    scrollY?: number;
}
/**
 * Record a user action to the buffer
 */
export declare function recordAction(action: Omit<ActionRecord, 'ts'>): void;
/**
 * Get the current action buffer
 */
export declare function getActionBuffer(): ActionRecord[];
/**
 * Clear the action buffer
 */
export declare function clearActionBuffer(): void;
/**
 * Handle click events
 */
export declare function handleClick(event: MouseEvent): void;
/**
 * Handle input events
 */
export declare function handleInput(event: Event): void;
/**
 * Handle scroll events (throttled)
 */
export declare function handleScroll(event: Event): void;
/**
 * Handle keydown events - only records actionable keys
 */
export declare function handleKeydown(event: KeyboardEvent): void;
/**
 * Handle change events on select elements
 */
export declare function handleChange(event: Event): void;
/**
 * Install user action capture
 */
export declare function installActionCapture(): void;
/**
 * Uninstall user action capture
 */
export declare function uninstallActionCapture(): void;
/**
 * Set whether action capture is enabled
 */
export declare function setActionCaptureEnabled(enabled: boolean): void;
/**
 * Install navigation capture to record enhanced actions on navigation events
 */
export declare function installNavigationCapture(): void;
/**
 * Uninstall navigation capture
 */
export declare function uninstallNavigationCapture(): void;
export {};
//# sourceMappingURL=actions.d.ts.map