/**
 * Purpose: Exposes the window.__kaboom developer API for programmatic access to capture capabilities, context annotations, and event buffers.
 * Docs: docs/features/feature/custom-event-api/index.md
 */
import type { LogEntry, ActionEntry, EnhancedAction, SelectorStrategies, WaterfallEntry, PerformanceMark, PerformanceMeasure } from '../types/index.js';
/**
 * KaboomAPI interface exposed on window.__kaboom
 */
export interface KaboomAPI {
    annotate(key: string, value: unknown): void;
    removeAnnotation(key: string): void;
    clearAnnotations(): void;
    getContext(): Record<string, unknown> | null;
    getActions(): ActionEntry[];
    clearActions(): void;
    setActionCapture(enabled: boolean): void;
    setNetworkWaterfall(enabled: boolean): void;
    getNetworkWaterfall(options?: {
        since?: number;
        initiatorTypes?: string[];
    }): WaterfallEntry[];
    setPerformanceMarks(enabled: boolean): void;
    getMarks(options?: {
        since?: number;
    }): PerformanceMark[];
    getMeasures(options?: {
        since?: number;
    }): PerformanceMeasure[];
    enrichError(error: LogEntry): Promise<LogEntry>;
    setAiContext(enabled: boolean): void;
    setStateSnapshot(enabled: boolean): void;
    recordAction(type: string, element: Element, opts?: Record<string, unknown>): void;
    getEnhancedActions(): EnhancedAction[];
    clearEnhancedActions(): void;
    generateScript(opts?: Record<string, unknown>): string;
    getSelectors(element: Element): SelectorStrategies;
    setInputValue(selector: string, value: string | boolean): boolean;
    version: string;
}
declare global {
    interface Window {
        __kaboom?: KaboomAPI;
    }
}
/**
 * Install the window.__kaboom API for developers to interact with Kaboom
 */
export declare function installKaboomAPI(): void;
/**
 * Uninstall the window.__kaboom API
 */
export declare function uninstallKaboomAPI(): void;
//# sourceMappingURL=api.d.ts.map