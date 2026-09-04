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
//# sourceMappingURL=cdp-key-mappings.d.ts.map