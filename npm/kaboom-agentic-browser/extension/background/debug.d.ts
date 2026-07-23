/**
 * Purpose: Defines debug log category constants used across background modules.
 * Why: Standalone module to break circular dependencies between index.ts and its consumers.
 */
/**
 * @fileoverview Debug Logging Utilities
 * Standalone module to avoid circular dependencies.
 */
/** Log categories for debug output */
export declare const DebugCategory: {
    CONNECTION: "connection";
    CAPTURE: "capture";
    ERROR: "error";
    LIFECYCLE: "lifecycle";
    SETTINGS: "settings";
    SOURCEMAP: "sourcemap";
    QUERY: "query";
};
export type DebugCategoryType = (typeof DebugCategory)[keyof typeof DebugCategory];
//# sourceMappingURL=debug.d.ts.map