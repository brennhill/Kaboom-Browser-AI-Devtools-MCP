/**
 * Purpose: Compares extension and server versions from /health responses and signals when an update is available.
 */
/**
 * Get the extension version from manifest
 */
export declare function getExtensionVersion(): string;
/**
 * Check if a new version is available (from last /health response)
 */
export declare function isNewVersionAvailable(): boolean;
/**
 * Get the available version from last /health response
 */
export declare function getAvailableVersion(): string | null;
/**
 * Update version state from /health response
 * Called when extension receives /health endpoint data
 */
export declare function updateVersionFromHealth(healthResponse: {
    version?: string;
    availableVersion?: string;
}, debugLogFn?: (category: string, message: string, data?: unknown) => void): void;
/**
 * Update extension badge to show version update indicator
 * If newVersionAvailable, shows a "⬆" indicator on the icon
 */
export declare function updateVersionBadge(): void;
/**
 * Get update information for display in popup
 */
export declare function getUpdateInfo(): {
    available: boolean;
    currentVersion: string;
    availableVersion: string | null;
    downloadUrl: string;
};
/**
 * Reset version check state (useful for testing)
 */
export declare function resetVersionCheck(): void;
//# sourceMappingURL=version-check.d.ts.map