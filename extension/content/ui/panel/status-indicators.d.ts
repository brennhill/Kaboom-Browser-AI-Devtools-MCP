/**
 * Purpose: Renders terminal connection and subscription/API provider indicators.
 * Why: Keeps compact status presentation with the terminal panel shell.
 * Docs: docs/features/feature/terminal/index.md
 */
export declare function updateConnectionIndicator(dot: HTMLSpanElement | null, state: 'connected' | 'disconnected' | 'exited'): void;
export declare function updateExecutionProviderBadge(badge: HTMLSpanElement | null, provider: string, tool: string, onAPIBilling: () => void): void;
//# sourceMappingURL=status-indicators.d.ts.map