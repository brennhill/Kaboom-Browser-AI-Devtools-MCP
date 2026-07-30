/**
 * Purpose: Owns background-to-content-script liveness, broadcast, overlay, and toast messaging.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */
export declare function pingContentScript(tabId: number, timeoutMs?: number): Promise<boolean>;
export declare function forwardToAllContentScripts(message: {
    type: string;
    [key: string]: unknown;
}, debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<void>;
export declare function setKaboomOverlayVisibility(tabId: number, visible: boolean): Promise<void>;
export declare function sendTabToast(tabId: number, text: string, detail?: string, state?: 'trying' | 'success' | 'warning' | 'error' | 'audio', duration_ms?: number): void;
//# sourceMappingURL=content-script-bridge.d.ts.map