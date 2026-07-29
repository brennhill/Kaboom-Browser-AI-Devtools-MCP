/**
 * Purpose: Defines debug log category constants used across background modules.
 * Why: Standalone module to break circular dependencies between index.ts and its consumers.
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
export declare function debugLog(category: string, message: string, data?: unknown): void;
//# sourceMappingURL=debug.d.ts.map