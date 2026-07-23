/**
 * Purpose: Typing-aware write queue for the terminal — defers agent writes while
 * the user is typing, and holds submits until the socket is back.
 * Why: Text injected mid-keystroke corrupts what the user was writing, and an
 * Enter sent while disconnected is simply lost.
 * Docs: docs/features/feature/terminal/index.md
 */
/** Post a command to the terminal iframe. No-op when it is not mounted. */
export declare function notifyIframe(command: string, data?: Record<string, unknown>): void;
export declare function resetWriteGuardState(): void;
export declare function shouldDeferQueuedWrite(nowMs?: number): boolean;
export declare function maybeShowQueuedWriteToast(nowMs?: number): void;
export declare function scheduleQueuedWriteFlush(delayMs?: number): void;
export declare function scheduleQueuedSubmit(delayMs: number): void;
export declare function flushQueuedWrites(): void;
//# sourceMappingURL=terminal-write-guard.d.ts.map