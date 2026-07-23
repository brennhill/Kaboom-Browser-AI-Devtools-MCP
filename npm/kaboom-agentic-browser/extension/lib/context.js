/**
 * Purpose: Key-value context annotations storage that attaches metadata (user flow, feature flags, session info) to captured events.
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview Context annotations storage and management.
 * Provides key-value annotations that attach to captured events for
 * richer debugging context (e.g., user flow, feature flag, session info).
 */
import { MAX_CONTEXT_SIZE, MAX_CONTEXT_VALUE_SIZE } from './constants.js';
import { KABOOM_LOG_PREFIX } from './brand.js';
import { safeSerialize } from './serialize.js';
// Context annotations storage
const contextAnnotations = new Map();
/**
 * Get current context annotations as an object
 */
export function getContextAnnotations() {
    if (contextAnnotations.size === 0)
        return null;
    const result = {};
    for (const [key, value] of contextAnnotations) {
        result[key] = value;
    }
    return result;
}
/**
 * Set a context annotation
 */
export function setContextAnnotation(key, value) {
    if (typeof key !== 'string' || key.length === 0) {
        console.warn(KABOOM_LOG_PREFIX, 'annotate() requires a non-empty string key');
        return false;
    }
    if (key.length > 100) {
        console.warn(KABOOM_LOG_PREFIX, 'annotate() key must be 100 characters or less');
        return false;
    }
    // Enforce max context keys
    if (!contextAnnotations.has(key) && contextAnnotations.size >= MAX_CONTEXT_SIZE) {
        console.warn(`${KABOOM_LOG_PREFIX} Maximum context annotations (${MAX_CONTEXT_SIZE}) reached`);
        return false;
    }
    // Serialize and check size
    const serialized = safeSerialize(value);
    const serializedStr = JSON.stringify(serialized);
    if (serializedStr.length > MAX_CONTEXT_VALUE_SIZE) {
        console.warn(`${KABOOM_LOG_PREFIX} Context value for "${key}" exceeds max size, truncating`);
        contextAnnotations.set(key, '[Value too large]');
        return false;
    }
    contextAnnotations.set(key, serialized);
    return true;
}
/**
 * Remove a context annotation
 */
export function removeContextAnnotation(key) {
    return contextAnnotations.delete(key);
}
/**
 * Clear all context annotations
 */
export function clearContextAnnotations() {
    contextAnnotations.clear();
}
//# sourceMappingURL=context.js.map