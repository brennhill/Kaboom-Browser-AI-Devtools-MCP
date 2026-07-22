/**
 * Purpose: Safe error-message extraction from unknown caught values.
 */

/**
 * Extract a message string from an unknown caught value.
 * Returns the Error.message if available, otherwise the fallback.
 */
export function errorMessage(err: unknown, fallback = 'Unknown error'): string {
  if (err instanceof Error && err.message) return err.message
  if (typeof err === 'string' && err) return err
  return fallback
}

/**
 * True when a chrome.runtime/chrome.tabs sendMessage rejection is the benign
 * "nobody is listening" case — the receiving context (a closed popup, or a page
 * with no content script) simply does not exist. This is expected, not an error:
 * a background broadcast to the popup rejects with this whenever the popup is
 * closed. Callers should swallow it rather than log it as an error.
 */
export function isNoReceiverError(err: unknown): boolean {
  const message = errorMessage(err, '')
  return message.includes('Receiving end does not exist') || message.includes('Could not establish connection')
}
