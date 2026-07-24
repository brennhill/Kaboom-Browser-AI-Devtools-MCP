/**
 * Purpose: Assign browser globals that a page may have made read-only.
 * Why: Plain assignment throws in strict mode when the property is non-writable,
 * and an uncaught throw aborts the rest of our patching.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
/**
 * Replace `target[key]` with `value`, tolerating hardened pages.
 *
 * Some sites define `fetch` / `WebSocket` as non-writable (anti-tampering
 * shims, frozen globals). `window.fetch = ...` then throws
 * "Cannot assign to read only property 'fetch' of object '#<Window>'", and
 * because the assignment sat unguarded the throw escaped and everything after
 * it — the rest of the patch, and the capture it installs — never ran.
 *
 * Order: plain assignment, then defineProperty for non-writable-but-configurable
 * properties, then give up. Returns whether the value actually took, so callers
 * can degrade instead of assuming capture is live.
 */
export declare function safeAssignGlobal<T extends object, K extends keyof T>(target: T, key: K, value: T[K]): boolean;
//# sourceMappingURL=safe-global-patch.d.ts.map