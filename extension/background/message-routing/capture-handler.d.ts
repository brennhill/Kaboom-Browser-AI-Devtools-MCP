import type { LogEntry } from '../../types/capture/telemetry.js';
import type { MessageHandlerOwner } from './types.js';
export interface CaptureHandlerDependencies {
    getServerUrl: () => string;
    captureScreenshot: (tabId: number, relatedErrorId: string | null) => Promise<{
        success: boolean;
        entry?: LogEntry;
        error?: string;
    }>;
    addLog: (entry: LogEntry) => void;
    debugLog: (category: string, message: string, data?: unknown) => void;
}
export declare function createCaptureMessageHandler(deps: CaptureHandlerDependencies): MessageHandlerOwner;
//# sourceMappingURL=capture-handler.d.ts.map