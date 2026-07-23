/**
 * Purpose: Key-value context annotations storage that attaches metadata (user flow, feature flags, session info) to captured events.
 * Docs: docs/features/feature/observe/index.md
 */
import type { JsonValue } from '../types/index.js';
/**
 * Get current context annotations as an object
 */
export declare function getContextAnnotations(): Record<string, JsonValue> | null;
/**
 * Set a context annotation
 */
export declare function setContextAnnotation(key: string, value: unknown): boolean;
/**
 * Remove a context annotation
 */
export declare function removeContextAnnotation(key: string): boolean;
/**
 * Clear all context annotations
 */
export declare function clearContextAnnotations(): void;
//# sourceMappingURL=context.d.ts.map