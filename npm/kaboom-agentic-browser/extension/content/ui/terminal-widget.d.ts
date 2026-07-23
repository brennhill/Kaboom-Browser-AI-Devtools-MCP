/**
 * Purpose: In-browser terminal widget orchestrator — public API, write guard, mount/unmount.
 * Why: Provides a Lovable-like experience — chat with any CLI (claude, codex, aider) from
 * a browser overlay while seeing code edits reflected via hot reload on the tracked page.
 * Docs: docs/features/feature/terminal/index.md
 */
import { type TerminalConfig } from './terminal-widget-types.js';
export declare function isTerminalVisible(): boolean;
export declare function toggleTerminal(): Promise<void>;
/** Restore terminal on page load if it was previously open/minimized. */
export declare function restoreTerminalIfNeeded(): Promise<void>;
export declare function writeToTerminal(text: string): void;
export type { TerminalConfig };
//# sourceMappingURL=terminal-widget.d.ts.map