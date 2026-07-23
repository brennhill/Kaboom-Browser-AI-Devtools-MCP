/**
 * Purpose: Renders subtitle overlays and recording watermarks on the page for AI-driven action narration.
 * Docs: docs/features/feature/subtitle/index.md
 */
/**
 * Remove the subtitle element, clean up Escape listener.
 */
export declare function clearSubtitle(): void;
/**
 * Show or update a persistent subtitle bar at the bottom of the viewport.
 * Empty text clears the subtitle. Includes a hover close button and
 * Escape key listener for dismissal.
 */
export declare function showSubtitle(text: string): void;
/**
 * Show or hide a recording watermark (Kaboom flame icon) in the bottom-right corner.
 * The icon renders at 64x64px with 50% opacity, captured in the tab video.
 */
export declare function toggleRecordingWatermark(visible: boolean): void;
//# sourceMappingURL=subtitle.d.ts.map