/**
 * Purpose: Defines shared utility and helper TypeScript types used across extension modules.
 * Why: Keeps common type patterns centralized to reduce duplication and improve type consistency.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */
// =============================================================================
// TYPE GUARDS
// =============================================================================
/**
 * Check if a value is a non-null object
 */
export function isObject(value) {
    return typeof value === 'object' && value !== null && !Array.isArray(value);
}
/**
 * Check if a value is a non-empty string
 */
export function isNonEmptyString(value) {
    return typeof value === 'string' && value.length > 0;
}
/**
 * Check if a value has a specific type property
 */
export function hasType(value, type) {
    return isObject(value) && 'type' in value && value.type === type;
}
/**
 * Check if value is a valid JSON value
 */
export function isJsonValue(value) {
    if (value === null)
        return true;
    const type = typeof value;
    if (type === 'string' || type === 'number' || type === 'boolean')
        return true;
    if (Array.isArray(value))
        return value.every(isJsonValue);
    if (isObject(value))
        return Object.values(value).every(isJsonValue);
    return false;
}
/**
 * Check if a message has a specific type (type guard factory)
 */
export function createTypeGuard(type) {
    return (value) => hasType(value, type);
}
//# sourceMappingURL=utils.js.map