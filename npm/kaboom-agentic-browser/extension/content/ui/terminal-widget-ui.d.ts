/**
 * Purpose: Terminal widget DOM creation, resize, minimize, redraw, and iframe messaging.
 * Why: Isolates all DOM manipulation and UI event wiring from session logic and orchestrator.
 * Docs: docs/features/feature/terminal/index.md
 */
export declare function registerUICallbacks(cbs: {
    hideTerminal: () => void;
    exitTerminalSession: () => Promise<void>;
    resetWriteGuardState: () => void;
    scheduleQueuedWriteFlush: (delayMs: number) => void;
}): void;
export declare function showSandboxError(message: string, instruction: string, command: string): void;
export declare function createWidget(token: string): HTMLDivElement;
export declare function handleIframeMessage(event: MessageEvent): void;
export declare function toggleMinimize(widget: HTMLElement, btn: HTMLButtonElement, header: HTMLElement): void;
export declare function notifyIframe(command: string, data?: Record<string, unknown>): void;
//# sourceMappingURL=terminal-widget-ui.d.ts.map