// cloaked-domains.ts — Domain blocklist where Kaboom disables itself.
// Content scripts bail out early on cloaked domains to avoid interference.
import { StorageKey } from './constants.js';
import { getLocal } from './storage-utils.js';
/**
 * Built-in domains where Kaboom should never run.
 * These are also excluded via manifest exclude_matches, but this list
 * serves as a runtime fallback for subdomains or edge cases.
 */
const BUILTIN_CLOAKED = [
    'cloudflare.com',
    'dash.cloudflare.com'
];
/**
 * Check if a hostname matches a cloaked domain pattern.
 * Matches exact or subdomain (e.g., "cloudflare.com" matches "dash.cloudflare.com").
 */
function matchesDomain(hostname, domain) {
    return hostname === domain || hostname.endsWith('.' + domain);
}
/**
 * Check if the current page's domain is cloaked.
 * Returns true if content scripts should bail out.
 */
export async function isDomainCloaked(hostname) {
    const host = hostname || (typeof location !== 'undefined' ? location.hostname : '');
    if (!host)
        return false;
    // Check built-in list first (sync, fast)
    for (const domain of BUILTIN_CLOAKED) {
        if (matchesDomain(host, domain))
            return true;
    }
    // Check user-configured list
    try {
        const userDomains = (await getLocal(StorageKey.CLOAKED_DOMAINS));
        if (userDomains && Array.isArray(userDomains)) {
            for (const domain of userDomains) {
                if (matchesDomain(host, domain))
                    return true;
            }
        }
    }
    catch {
        // Storage unavailable — allow by default
    }
    return false;
}
/**
 * Get the full list of cloaked domains (built-in + user-configured).
 */
export async function getCloakedDomains() {
    const userDomains = (await getLocal(StorageKey.CLOAKED_DOMAINS));
    return [...BUILTIN_CLOAKED, ...(userDomains || [])];
}
//# sourceMappingURL=cloaked-domains.js.map