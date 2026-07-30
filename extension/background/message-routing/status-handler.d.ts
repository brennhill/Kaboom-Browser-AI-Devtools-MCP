/**
 * Purpose: Own status, log-clearing, and debug diagnostic messages.
 */
import type { CircuitBreakerState, ConnectionStatus, ContextWarning, MemoryPressureState } from '../../types/runtime/state.js';
import type { MessageHandlerOwner } from './types.js';
export interface StatusHandlerDependencies {
    getConnectionStatus: () => ConnectionStatus;
    getServerUrl: () => string;
    getScreenshotOnError: () => boolean;
    getSourceMapEnabled: () => boolean;
    getDebugMode: () => boolean;
    getContextWarning: () => ContextWarning | null;
    getCircuitBreakerState: () => CircuitBreakerState;
    getMemoryPressureState: () => MemoryPressureState;
    clearLogs: () => Promise<{
        success: boolean;
        error?: string;
    }>;
    exportDebugLog: () => string;
    clearDebugLog: () => void;
    debugLog: (category: string, message: string, data?: unknown) => void;
}
export declare function createStatusMessageHandler(deps: StatusHandlerDependencies): MessageHandlerOwner;
//# sourceMappingURL=status-handler.d.ts.map