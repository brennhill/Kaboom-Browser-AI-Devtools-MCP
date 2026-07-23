/**
 * Check if the current page's domain is cloaked.
 * Returns true if content scripts should bail out.
 */
export declare function isDomainCloaked(hostname?: string): Promise<boolean>;
/**
 * Get the full list of cloaked domains (built-in + user-configured).
 */
export declare function getCloakedDomains(): Promise<string[]>;
//# sourceMappingURL=cloaked-domains.d.ts.map