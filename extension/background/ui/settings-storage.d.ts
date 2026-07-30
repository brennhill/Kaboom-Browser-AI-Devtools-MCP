/**
 * Purpose: Owns persisted extension setting reads and writes used during background startup.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */
export interface SavedSettings {
    serverUrl?: string;
    logLevel?: string;
    screenshotOnError?: boolean;
    sourceMapEnabled?: boolean;
    debugMode?: boolean;
}
export declare function loadSavedSettings(): Promise<SavedSettings>;
export declare function loadAiWebPilotState(logFn?: (message: string) => void): Promise<boolean>;
export declare function loadDebugModeState(): Promise<boolean>;
export declare function saveSetting(key: string, value: unknown): void;
export declare function getAllConfigSettings(): Promise<Record<string, boolean | string | undefined>>;
//# sourceMappingURL=settings-storage.d.ts.map