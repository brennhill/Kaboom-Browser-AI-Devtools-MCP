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
export function safeAssignGlobal(target, key, value) {
    try {
        // eslint-disable-next-line security/detect-object-injection -- key is a keyof T supplied by our own call sites
        target[key] = value;
        // eslint-disable-next-line security/detect-object-injection -- same key, read back to confirm the write landed
        if (target[key] === value)
            return true;
    }
    catch {
        // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
        // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
        // Non-writable — try defineProperty below.
    }
    try {
        Object.defineProperty(target, key, { value, writable: true, configurable: true });
        // eslint-disable-next-line security/detect-object-injection -- same key, read back to confirm the write landed
        return target[key] === value;
    }
    catch {
        // EXPECTED_ABSENCE: a non-configurable page global is normal; logging would mislabel skipped optional capture as failure.
        return false;
    }
}
//# sourceMappingURL=safe-global-patch.js.map