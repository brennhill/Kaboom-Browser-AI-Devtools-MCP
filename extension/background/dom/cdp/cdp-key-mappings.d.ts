/**
 * Purpose: Key code mappings and character-to-key resolution for CDP Input.dispatchKeyEvent.
 * Why: Separates keyboard layout data from CDP protocol dispatch logic for maintainability.
 * Docs: docs/features/feature/interact-explore/index.md
 */
export declare const KEY_CODES: Record<string, {
    code: string;
    keyCode: number;
}>;
export declare function charToKeyInfo(char: string): {
    key: string;
    code: string;
    keyCode: number;
    shiftKey: boolean;
};
/**
 * CDP `Input.dispatch*Event` modifier bitmask. Chrome defines these bits, not us:
 * Alt=1, Ctrl=2, Meta/Command=4, Shift=8. A wrong bit silently produces a plain
 * click — the page never sees the ctrl/shift the agent asked for.
 */
export declare const MODIFIER_BITS: Record<string, number>;
/**
 * Fold modifier names into the CDP bitmask. Unknown names are ignored rather than
 * rejected: an agent asking for a modifier Chrome has no bit for still gets its click.
 */
export declare function modifierBitmask(modifiers?: readonly string[]): number;
/** Chrome's shift bit. Shift is the one modifier that still produces text: shift+a IS "A". */
export declare const SHIFT_BIT = 8;
/**
 * Whether a held mask makes the keystroke a shortcut rather than text.
 *
 * ctrl/alt/cmd held means the key is a command — ctrl+a selects all and inserts nothing.
 */
export declare function isModifierShortcut(mask: number): boolean;
/** One `Input.dispatchKeyEvent` payload. A plain object so it passes as CDP command params. */
export type CDPKeyEvent = {
    type: 'keyDown' | 'keyUp';
    key: string;
    code: string;
    windowsVirtualKeyCode: number;
    nativeVirtualKeyCode: number;
    modifiers: number;
    text?: string;
    unmodifiedText?: string;
};
/**
 * The CDP key events one string produces, with whatever modifier the caller is holding.
 *
 * Two things go wrong if this is skipped. Dropping the mask leaves the page seeing an
 * unmodified keystroke, so the shortcut the agent asked for never fires while the call reports
 * success. Keeping `text` alongside a ctrl/alt/cmd bit is worse: Chrome inserts whatever `text`
 * says regardless of the modifiers, so ctrl+a types an "a" into the field instead of selecting
 * it. A real modified keystroke carries no text, so neither does this one.
 */
export declare function keyEventsForText(text: string, held?: readonly string[]): CDPKeyEvent[];
//# sourceMappingURL=cdp-key-mappings.d.ts.map