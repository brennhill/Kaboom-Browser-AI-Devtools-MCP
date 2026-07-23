/**
 * Purpose: Safe error-message extraction from unknown caught values.
 */
/**
 * Extract a message string from an unknown caught value.
 * Returns the Error.message if available, otherwise the fallback.
 */
export function errorMessage(err, fallback = 'Unknown error') {
    if (err instanceof Error && err.message)
        return err.message;
    if (typeof err === 'string' && err)
        return err;
    return fallback;
}
//# sourceMappingURL=error-utils.js.map