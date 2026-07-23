/**
 * Purpose: Routes all chrome.runtime.onMessage events to type-safe handlers for logs, settings, screenshots, and state management.
 * Why: Centralizes message validation and sender security checks in one place.
 */
/**
 * @fileoverview Message Handlers - Handles all chrome.runtime.onMessage routing
 * with type-safe message discrimination.
 */
import type { LogEntry, ChromeMessageSender, BrowserStateSnapshot, ConnectionStatus, ContextWarning, CircuitBreakerState, MemoryPressureState, WebSocketEvent, EnhancedAction, NetworkBodyPayload, PerformanceSnapshot } from '../types/index.js';
/** Message handler dependencies */
export interface MessageHandlerDependencies {
    getServerUrl: () => string;
    getConnectionStatus: () => ConnectionStatus;
    getDebugMode: () => boolean;
    getScreenshotOnError: () => boolean;
    getSourceMapEnabled: () => boolean;
    getCurrentLogLevel: () => string;
    getContextWarning: () => ContextWarning | null;
    getCircuitBreakerState: () => CircuitBreakerState;
    getMemoryPressureState: () => MemoryPressureState;
    getAiWebPilotEnabled: () => boolean;
    isNetworkBodyCaptureDisabled: () => boolean;
    setServerUrl: (url: string) => void;
    setCurrentLogLevel: (level: string) => void;
    setScreenshotOnError: (enabled: boolean) => void;
    setSourceMapEnabled: (enabled: boolean) => void;
    setDebugMode: (enabled: boolean) => void;
    setAiWebPilotEnabled: (enabled: boolean, callback?: () => void) => void;
    addToLogBatcher: (entry: LogEntry) => void;
    addToWsBatcher: (event: WebSocketEvent) => void;
    addToEnhancedActionBatcher: (action: EnhancedAction) => void;
    addToNetworkBodyBatcher: (body: NetworkBodyPayload) => void;
    addToPerfBatcher: (snapshot: PerformanceSnapshot) => void;
    handleLogMessage: (payload: LogEntry, sender: ChromeMessageSender, tabId?: number) => Promise<void>;
    handleClearLogs: () => Promise<{
        success: boolean;
        error?: string;
    }>;
    captureScreenshot: (tabId: number, relatedErrorId: string | null) => Promise<{
        success: boolean;
        entry?: LogEntry;
        error?: string;
    }>;
    checkConnectionAndUpdate: () => Promise<void>;
    clearSourceMapCache: () => void;
    debugLog: (category: string, message: string, data?: unknown) => void;
    exportDebugLog: () => string;
    clearDebugLog: () => void;
    saveSetting: (key: string, value: unknown) => void;
    forwardToAllContentScripts: (message: {
        type: string;
        [key: string]: unknown;
    }) => void;
}
/**
 * Install the main message listener
 * All messages are validated for sender origin to ensure they come from trusted extension contexts
 */
export declare function installMessageListener(deps: MessageHandlerDependencies): void;
/**
 * Broadcast tracking state to the tracked tab.
 * Used by favicon replacer to show/hide flicker animation.
 * Exported for use in init.ts storage change handlers.
 * @param untrackedTabId - Optional tab ID that was just untracked (to notify it to stop flicker)
 */
export declare function broadcastTrackingState(untrackedTabId?: number | null): Promise<void>;
interface StoredStateSnapshot extends BrowserStateSnapshot {
    name: string;
    size_bytes: number;
}
/**
 * Save a state snapshot to persistent storage
 */
export declare function saveStateSnapshot(name: string, state: BrowserStateSnapshot): Promise<{
    success: boolean;
    snapshot_name: string;
    size_bytes: number;
}>;
/**
 * Load a state snapshot from persistent storage
 */
export declare function loadStateSnapshot(name: string): Promise<StoredStateSnapshot | null>;
/**
 * List all state snapshots with metadata
 */
export declare function listStateSnapshots(): Promise<Array<{
    name: string;
    url: string;
    timestamp: number;
    size_bytes: number;
}>>;
/**
 * Delete a state snapshot from persistent storage
 */
export declare function deleteStateSnapshot(name: string): Promise<{
    success: boolean;
    deleted: string;
}>;
export {};
//# sourceMappingURL=message-handlers.d.ts.map