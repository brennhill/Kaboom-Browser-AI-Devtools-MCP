/**
 * Purpose: Typing-aware write queue for the terminal — defers agent writes while
 * the user is typing, and holds submits until the socket is back.
 * Why: Text injected mid-keystroke corrupts what the user was writing, and an
 * Enter sent while disconnected is simply lost.
 * Docs: docs/features/feature/terminal/index.md
 */
/**
 * Post a command to the terminal iframe. No-op when it is not mounted.
 *
 * `target: 'kaboom-terminal'` is MANDATORY: terminal.html's message listener
 * drops any message whose `target` is not exactly that. The eb248ff6 refactor
 * dropped this field, so every agent/annotation write, focus, and redraw was
 * silently discarded — user keystrokes hid the gap because they go straight to
 * the socket (iframe -> WS), never through here. Post to the terminal server's
 * own origin so the message can't leak to a swapped-in frame; fall back to '*'
 * only when the URL is unparseable.
 */
export declare function notifyIframe(command: string, data?: Record<string, unknown>): void;
/**
 * Enqueue a write, bounding the backlog by BOTH entry count and total bytes.
 *
 * The count cap alone bounded nothing that matters — 200 one-megabyte writes was a
 * legal state (finding S14). Eviction is oldest-first and runs down to empty, so a
 * single write larger than the byte cap is evicted like any other rather than
 * lodging in the queue forever.
 *
 * Dropping a write is a state-mutating loss, so it must not be silent (rule 25):
 * warn to the console (which the daemon captures via observe(what:"errors")) so an
 * overflow is diagnosable rather than a write vanishing without a trace.
 *
 * Lives here, with the rest of the queue's lifecycle (reset, drain, drop-on-give-up),
 * so the bound cannot be bypassed by a second enqueue site (rule 19).
 */
export declare function enqueueBoundedWrite(text: string): void;
export declare function resetWriteGuardState(): void;
export declare function shouldDeferQueuedWrite(nowMs?: number): boolean;
export declare function maybeShowQueuedWriteToast(nowMs?: number): void;
export declare function scheduleQueuedWriteFlush(delayMs?: number): void;
export declare function scheduleQueuedSubmit(delayMs: number): void;
export declare function flushQueuedWrites(): void;
//# sourceMappingURL=terminal-write-guard.d.ts.map