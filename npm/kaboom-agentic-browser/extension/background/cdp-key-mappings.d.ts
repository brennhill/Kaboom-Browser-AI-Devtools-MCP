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
//# sourceMappingURL=cdp-key-mappings.d.ts.map