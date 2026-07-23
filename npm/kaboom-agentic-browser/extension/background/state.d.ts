/**
 * Purpose: Owns all mutable module-level state (connection status, settings, flags) for the background service worker.
 * Why: Separates state ownership from business logic so mutations are explicit and testable.
 */
/** Session ID for detecting extension reloads */
export declare const EXTENSION_SESSION_ID: string;
/** Connection status (mutable internal state) */
export interface MutableConnectionStatus {
    connected: boolean;
    entries: number;
    maxEntries: number;
    errorCount: number;
    logFile: string;
    logFileSize?: number;
    serverVersion?: string;
    extensionVersion?: string;
    versionMismatch?: boolean;
    securityMode?: 'normal' | 'insecure_proxy';
    productionParity?: boolean;
    insecureRewritesApplied?: string[];
}
export interface ExtensionLogQueueEntry {
    timestamp: string;
    level: string;
    message: string;
    source: string;
    category: string;
    data?: unknown;
}
export declare function getServerUrl(): string;
export declare function isDebugMode(): boolean;
export declare function getConnectionStatus(): Readonly<MutableConnectionStatus>;
export declare function getCurrentLogLevel(): string;
export declare function isScreenshotOnError(): boolean;
export declare function isAiControlled(): boolean;
export declare function isConnectionCheckRunning(): boolean;
export declare function isAiWebPilotCacheInitialized(): boolean;
export declare function getPilotInitCallback(): (() => void) | null;
export declare function getExtensionLogQueue(): ExtensionLogQueueEntry[];
export declare function clearExtensionLogQueue(): void;
export declare function pushExtensionLog(entry: ExtensionLogQueueEntry): void;
export declare function capExtensionLogs(maxEntries: number): void;
export declare const initReady: Promise<void>;
export declare function markInitComplete(): void;
export declare function setServerUrl(url: string): void;
/** Low-level flag setter. Use index.setDebugMode for the version that also logs. */
export declare function _setDebugModeRaw(enabled: boolean): void;
export declare function setCurrentLogLevel(level: string): void;
export declare function setScreenshotOnError(enabled: boolean): void;
export declare function setConnectionStatus(patch: Partial<MutableConnectionStatus>): void;
export declare function setConnectionCheckRunning(running: boolean): void;
export declare function setAiWebPilotEnabledCache(enabled: boolean): void;
export declare function setAiWebPilotCacheInitialized(initialized: boolean): void;
export declare function setPilotInitCallback(callback: (() => void) | null): void;
export declare function applyCaptureOverrides(overrides: Record<string, string>): void;
/**
 * Reset pilot cache for testing
 */
export declare function _resetPilotCacheForTesting(value?: boolean): void;
/**
 * Check if AI Web Pilot is enabled
 */
export declare function isAiWebPilotEnabled(): boolean;
//# sourceMappingURL=state.d.ts.map