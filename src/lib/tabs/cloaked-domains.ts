// cloaked-domains.ts — Domain blocklist where Kaboom disables itself.
// Content scripts bail out early on cloaked domains to avoid interference.

import { StorageKey } from '../constants.js'
import { readLocalState } from '../storage/validated.js'

/**
 * Built-in domains where Kaboom should never run.
 * These are also excluded via manifest exclude_matches, but this list
 * serves as a runtime fallback for subdomains or edge cases.
 */
const BUILTIN_CLOAKED: readonly string[] = ['cloudflare.com', 'dash.cloudflare.com']

/**
 * Check if a hostname matches a cloaked domain pattern.
 * Matches exact or subdomain (e.g., "cloudflare.com" matches "dash.cloudflare.com").
 */
function matchesDomain(hostname: string, domain: string): boolean {
  return hostname === domain || hostname.endsWith('.' + domain)
}

/**
 * Check if the current page's domain is cloaked.
 * Returns true if content scripts should bail out.
 */
export async function isDomainCloaked(hostname?: string): Promise<boolean> {
  const host = hostname || (typeof location !== 'undefined' ? location.hostname : '')
  if (!host) return false

  // Check built-in list first (sync, fast)
  for (const domain of BUILTIN_CLOAKED) {
    if (matchesDomain(host, domain)) return true
  }

  // Check user-configured list
  const userDomains = await readUserDomains()
  for (const domain of userDomains) {
    if (matchesDomain(host, domain)) return true
  }

  return false
}

function readUserDomains(): Promise<string[]> {
  return readLocalState<string[]>({
    key: StorageKey.CLOAKED_DOMAINS,
    fallback: [],
    validate: (value): value is string[] =>
      Array.isArray(value) && value.every((domain) => typeof domain === 'string' && domain.length > 0),
    diagnostic: {
      name: 'cloaked_domain_state',
      detail: 'Saved cloaked-domain rules were invalid or unreadable; built-in protections remain active.',
      fix: 'Open extension settings and save the cloaked-domain list again.'
    }
  })
}

/**
 * Get the full list of cloaked domains (built-in + user-configured).
 */
export async function getCloakedDomains(): Promise<string[]> {
  return [...BUILTIN_CLOAKED, ...(await readUserDomains())]
}
