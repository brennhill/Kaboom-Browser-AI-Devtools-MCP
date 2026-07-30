/**
 * Purpose: Captures and uploads visible-tab screenshots for background error enrichment.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import type { LogEntry } from '../../types/capture/telemetry.js';
interface ScreenshotRateCheck {
    allowed: boolean;
    reason?: string;
    nextAllowedIn?: number | null;
}
interface ScreenshotResult {
    success: boolean;
    entry?: LogEntry;
    error?: string;
    nextAllowedIn?: number | null;
}
export declare function captureScreenshot(tabId: number, serverUrl: string, relatedErrorId: string | null, canTakeScreenshot: (tabId: number) => ScreenshotRateCheck, recordScreenshot: (tabId: number) => void, debugLog?: (category: string, message: string, data?: unknown) => void): Promise<ScreenshotResult>;
export {};
//# sourceMappingURL=screenshot.d.ts.map