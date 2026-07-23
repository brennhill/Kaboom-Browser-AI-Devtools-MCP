/**
 * Purpose: Bridges the page hover launcher to the terminal side panel.
 * Why: Keeps the page overlay focused on quick actions while terminal visibility
 * and writes are coordinated through session state and runtime messages.
 * Docs: docs/features/feature/terminal/index.md
 */
type VisibilityListener = (visible: boolean) => void;
export declare function initTerminalPanelBridge(): Promise<void>;
export declare function isTerminalVisible(): boolean;
export declare function onTerminalPanelVisibilityChanged(listener: VisibilityListener): () => void;
export declare function openTerminalPanel(): Promise<boolean>;
export declare function writeToTerminal(text: string): void;
export declare const _terminalPanelBridgeForTests: {
    reset(): void;
};
export {};
//# sourceMappingURL=terminal-panel-bridge.d.ts.map