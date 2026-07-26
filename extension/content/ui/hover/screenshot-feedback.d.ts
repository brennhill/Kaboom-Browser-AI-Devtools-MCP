/**
 * Purpose: Presentational feedback for a screenshot capture — the shutter sound and
 * the full-viewport flash.
 * Why: Both are self-contained, own their only module state (a primed AudioContext),
 * and are unrelated to the launcher's hover/panel logic. Extracted so
 * tracked-hover-launcher.ts stays within the 800-line limit.
 * Docs: docs/features/feature/terminal/index.md
 */
/**
 * Create the AudioContext while a user gesture is still on the stack.
 *
 * Chrome blocks audio created outside a gesture, and the shutter plays later (after
 * the capture round-trip), by which time the gesture is gone — so the context must
 * be primed at click time or the sound is silently dropped.
 */
export declare function primeShutterAudio(): void;
export declare function playShutterSound(): void;
export declare function showScreenshotFlash(success: boolean): void;
//# sourceMappingURL=screenshot-feedback.d.ts.map