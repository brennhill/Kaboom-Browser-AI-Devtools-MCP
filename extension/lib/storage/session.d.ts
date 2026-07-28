/**
 * Purpose: Own ephemeral Chrome session-storage state and lifecycle checks.
 * Why: Keep service-worker recovery and session access policy together.
 */
export declare function getSession(key: string): Promise<unknown>;
export declare function setSession(key: string, value: unknown): Promise<void>;
export declare function removeSession(key: string): Promise<void>;
export declare function removeSessions(keys: string[]): Promise<void>;
export declare function setSessionAccessLevel(accessLevel: 'TRUSTED_CONTEXTS' | 'TRUSTED_AND_UNTRUSTED_CONTEXTS'): Promise<void>;
export declare function getStorageDiagnostics(): {
    sessionStorageAvailable: boolean;
    localStorageAvailable: boolean;
    browserVersion: string;
};
export declare function wasServiceWorkerRestarted(): Promise<boolean>;
export declare function markStateVersion(): Promise<void>;
//# sourceMappingURL=session.d.ts.map