/**
 * Purpose: Command handlers for the observe MCP tool (screenshot capture, network waterfall, page info, tab listing).
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * Self-contained function injected via chrome.scripting.executeScript.
 * Temporarily expands scrollable containers so CDP captures full content.
 * Stores original styles in data attributes for restoration.
 */
export declare function screenshotExpandContainers(): {
    expanded: number;
    content_height_hint: number;
};
/** Self-contained: restore containers after full-page capture. */
export declare function screenshotRestoreContainers(): void;
/** Derive bounded screenshot dimensions with fallback defaults and optional expanded-content hint. */
export declare function computeFullPageCaptureDimensions(contentWidth: number, contentHeight: number, hintedHeight: number): {
    width: number;
    height: number;
};
//# sourceMappingURL=observe.d.ts.map