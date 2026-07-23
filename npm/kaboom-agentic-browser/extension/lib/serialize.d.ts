/**
 * Purpose: Pure stateless serialization utilities -- safe value serialization with circular reference detection, DOM element selector generation, and sensitive input detection.
 */
import type { JsonValue } from '../types/index.js';
/**
 * Safely serialize a value, handling circular references and special types
 */
export declare function safeSerialize(value: unknown, depth?: number, seen?: WeakSet<object>): JsonValue;
/**
 * Get element selector for identification
 */
export declare function getElementSelector(element: Element | null): string;
export declare function isSensitiveInput(element: Element | null): boolean;
//# sourceMappingURL=serialize.d.ts.map