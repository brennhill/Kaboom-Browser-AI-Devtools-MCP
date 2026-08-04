/**
 * Purpose: Owns authenticated page-to-content messages emitted by the injected runtime.
 * Why: Every page-context producer must use the same per-injection nonce boundary.
 */
let cachedNonce;
export function getInjectedPageNonce() {
    if (cachedNonce !== undefined)
        return cachedNonce;
    if (typeof document === 'undefined' || typeof document.querySelector !== 'function') {
        cachedNonce = '';
        return cachedNonce;
    }
    const nonceElement = document.querySelector('script[data-kaboom-nonce]');
    cachedNonce = nonceElement?.getAttribute('data-kaboom-nonce') || '';
    return cachedNonce;
}
export function postAuthenticatedPageMessage(message) {
    window.postMessage({ ...message, _nonce: getInjectedPageNonce() }, window.location.origin);
}
//# sourceMappingURL=channel.js.map